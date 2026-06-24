# 浏览器实例生命周期优化方案

**目标：** 减少启动卡顿，提高批量启动稳定性

---

## 📊 问题分析

### **当前问题**

通过分析 `app_instance.go` 的 `browserInstanceStartInternal` 函数，发现以下问题：

```go
func (a *App) browserInstanceStartInternal(...) (*BrowserProfile, error) {
    log := logger.New("Browser")
    a.browserMgr.Mutex.Lock()          // ❌ 第33行：立即加锁
    defer a.browserMgr.Mutex.Unlock()  // ❌ 函数结束才释放
    
    // ... 42-100行：各种检查和准备工作（持锁）
    // ... 101-226行：构建启动参数（持锁）
    // ... 227-241行：启动浏览器进程（持锁）
    // ... 242-250行：等待调试端口稳定（持锁）
}
```

### **问题详情**

| 问题 | 影响 | 代码位置 |
|------|------|---------|
| **长时间持锁** | 批量启动时串行化，UI卡顿 | 第33-34行 |
| **启动进程时持锁** | 进程启动耗时1-3秒 | 第227-241行 |
| **等待端口时持锁** | 端口稳定检测耗时2-5秒 | 第242-250行 |
| **无并发控制** | 同时启动大量实例资源竞争 | - |
| **状态不明确** | 只有running/stopped | - |
| **无启动前检查** | 启动失败才发现问题 | - |

---

## 🎯 优化方案

### **1. 锁范围优化**

#### **原则**
- ✅ 只在读写共享数据时持锁
- ✅ I/O操作不持锁
- ✅ 进程启动不持锁
- ✅ 端口等待不持锁

#### **重构策略**
```go
func (a *App) browserInstanceStartInternal(...) (*BrowserProfile, error) {
    // 阶段1：读取配置（需要锁）
    a.browserMgr.Mutex.Lock()
    profile, exists := a.browserMgr.Profiles[profileId]
    profileCopy := *profile // 复制一份，避免外部修改
    a.browserMgr.Mutex.Unlock()
    
    // 阶段2：启动前检查（不需要锁）
    if err := validateStartupPreconditions(&profileCopy); err != nil {
        return nil, err
    }
    
    // 阶段3：更新状态为starting（需要锁）
    a.browserMgr.Mutex.Lock()
    profile.Status = "starting"
    a.browserMgr.Mutex.Unlock()
    
    // 阶段4：启动浏览器进程（不需要锁）
    cmd, pid, err := startBrowserProcess(&profileCopy)
    if err != nil {
        a.setProfileStatus(profileId, "stopped", err.Error())
        return nil, err
    }
    
    // 阶段5：等待调试端口（不需要锁）
    debugPort, err := waitForDebugPort(assignedPort, timeout)
    if err != nil {
        a.setProfileStatus(profileId, "crashed", err.Error())
        return nil, err
    }
    
    // 阶段6：更新状态为running（需要锁）
    a.browserMgr.Mutex.Lock()
    profile.Status = "running"
    profile.Pid = pid
    profile.DebugPort = debugPort
    a.browserMgr.Mutex.Unlock()
    
    return profile, nil
}
```

---

### **2. 启动队列**

#### **设计**
```go
// StartupQueue 启动队列
type StartupQueue struct {
    maxConcurrent int                    // 最大并发数（2-5）
    semaphore     chan struct{}         // 信号量
    waiting       []string               // 等待队列
    mu            sync.Mutex
}

func NewStartupQueue(maxConcurrent int) *StartupQueue {
    return &StartupQueue{
        maxConcurrent: maxConcurrent,
        semaphore:     make(chan struct{}, maxConcurrent),
        waiting:       []string{},
    }
}

// Acquire 获取启动许可
func (q *StartupQueue) Acquire(profileId string) error {
    q.mu.Lock()
    q.waiting = append(q.waiting, profileId)
    queuePosition := len(q.waiting)
    q.mu.Unlock()
    
    // 等待信号量
    select {
    case q.semaphore <- struct{}{}:
        q.mu.Lock()
        q.waiting = removeFromSlice(q.waiting, profileId)
        q.mu.Unlock()
        return nil
    case <-time.After(30 * time.Second):
        return fmt.Errorf("等待启动超时（队列位置：%d）", queuePosition)
    }
}

// Release 释放启动许可
func (q *StartupQueue) Release() {
    <-q.semaphore
}
```

#### **集成**
```go
type Manager struct {
    // ... 现有字段
    startupQueue *StartupQueue
}

func NewManager(...) *Manager {
    return &Manager{
        // ...
        startupQueue: NewStartupQueue(3), // 默认并发3个
    }
}

func (a *App) browserInstanceStartInternal(...) (*BrowserProfile, error) {
    // 获取启动许可
    if err := a.browserMgr.startupQueue.Acquire(profileId); err != nil {
        return nil, err
    }
    defer a.browserMgr.startupQueue.Release()
    
    // 继续启动流程...
}
```

---

### **3. 启动前检查**

#### **检查清单**
```go
type StartupPreconditions struct {
    ChromeBinaryPath string
    UserDataDir      string
    ProxyConfig      string
    DiskSpace        uint64
}

func validateStartupPreconditions(profile *Profile, mgr *Manager) error {
    checks := []struct {
        name string
        fn   func() error
    }{
        {"内核路径", func() error {
            path, err := mgr.ResolveChromeBinary(profile)
            if err != nil {
                return err
            }
            if _, err := os.Stat(path); err != nil {
                return fmt.Errorf("内核文件不存在: %s", path)
            }
            // 检查可执行权限
            if err := checkExecutable(path); err != nil {
                return fmt.Errorf("内核文件无执行权限: %s", path)
            }
            return nil
        }},
        
        {"用户数据目录", func() error {
            dir := mgr.ResolveUserDataDir(profile)
            // 检查父目录权限
            parent := filepath.Dir(dir)
            if err := checkWriteable(parent); err != nil {
                return fmt.Errorf("无法写入用户数据目录: %s", err)
            }
            return nil
        }},
        
        {"代理配置", func() error {
            if profile.ProxyConfig == "" && profile.ProxyId == "" {
                return nil // 无代理，跳过
            }
            // 解析代理配置
            resolvedProxy := resolveProxyConfig(profile, mgr.Proxies)
            if !isValidProxyURL(resolvedProxy) {
                return fmt.Errorf("代理格式无效: %s", resolvedProxy)
            }
            // 检查代理可达性（可选，快速超时）
            if err := checkProxyReachable(resolvedProxy, 2*time.Second); err != nil {
                // 仅警告，不阻止启动
                logger.Warn("代理可能不可用", "proxy", resolvedProxy, "error", err)
            }
            return nil
        }},
        
        {"磁盘空间", func() error {
            dir := mgr.ResolveUserDataDir(profile)
            available, err := getDiskSpace(dir)
            if err != nil {
                return fmt.Errorf("无法检查磁盘空间: %w", err)
            }
            minRequired := uint64(100 * 1024 * 1024) // 100MB
            if available < minRequired {
                return fmt.Errorf("磁盘空间不足: 需要 %d MB，可用 %d MB",
                    minRequired/1024/1024, available/1024/1024)
            }
            return nil
        }},
        
        {"端口可用性", func() error {
            // 预检查调试端口
            testPort, err := nextAvailablePort()
            if err != nil {
                return fmt.Errorf("无可用调试端口: %w", err)
            }
            _ = testPort // 端口暂不占用，启动时再分配
            return nil
        }},
    }
    
    for _, check := range checks {
        if err := check.fn(); err != nil {
            return fmt.Errorf("启动前检查失败[%s]: %w", check.name, err)
        }
    }
    
    return nil
}
```

---

### **4. 状态管理增强**

#### **新状态定义**
```go
type ProfileStatus string

const (
    StatusStopped      ProfileStatus = "stopped"       // 已停止
    StatusStarting     ProfileStatus = "starting"      // 启动中
    StatusRunning      ProfileStatus = "running"       // 运行中
    StatusDebugPending ProfileStatus = "debug_pending" // 进程启动，等待调试端口
    StatusStopping     ProfileStatus = "stopping"      // 停止中
    StatusCrashed      ProfileStatus = "crashed"       // 崩溃
)

type Profile struct {
    // ... 现有字段
    Status         ProfileStatus `json:"status"`
    StatusMessage  string        `json:"statusMessage"`  // 状态描述
    LastStateChange time.Time    `json:"lastStateChange"` // 状态变更时间
}
```

#### **状态机**
```
stopped → starting → debug_pending → running
   ↑         ↓            ↓             ↓
   └─────────┴────────────┴─────────────┘
                      ↓
                   crashed
                      ↓
                   stopped
```

#### **状态转换**
```go
func (m *Manager) setProfileStatus(profileId string, status ProfileStatus, message string) {
    m.Mutex.Lock()
    defer m.Mutex.Unlock()
    
    profile, exists := m.Profiles[profileId]
    if !exists {
        return
    }
    
    oldStatus := profile.Status
    profile.Status = status
    profile.StatusMessage = message
    profile.LastStateChange = time.Now()
    
    logger.Info("实例状态变更",
        "profile_id", profileId,
        "old_status", oldStatus,
        "new_status", status,
        "message", message,
    )
    
    // 触发事件
    if m.eventEmitter != nil {
        m.eventEmitter.Emit("profile:status:changed", map[string]interface{}{
            "profileId":    profileId,
            "profileName":  profile.ProfileName,
            "oldStatus":    oldStatus,
            "newStatus":    status,
            "message":      message,
        })
    }
}
```

---

### **5. 前端事件实时更新**

#### **后端事件发送**
```go
// 在启动流程的关键点发送事件
func (a *App) browserInstanceStartInternal(...) (*BrowserProfile, error) {
    // 开始启动
    a.emitProfileEvent("profile:status:changed", profileId, "starting", "正在启动...")
    
    // 进程已启动
    cmd.Start()
    a.emitProfileEvent("profile:status:changed", profileId, "debug_pending", "等待调试端口...")
    
    // 调试端口就绪
    debugPort, _ := waitForDebugPort(...)
    a.emitProfileEvent("profile:status:changed", profileId, "running", "运行中")
}
```

#### **前端监听**
```typescript
// frontend/src/modules/browser/hooks/useProfileEvents.ts
export function useProfileEvents() {
    useEffect(() => {
        const runtime = (window as any).runtime
        
        // 监听状态变更事件
        const unsubscribe = runtime.EventsOn('profile:status:changed', (event: any) => {
            console.log('Profile status changed:', event)
            
            // 更新本地状态（不需要全量刷新）
            updateProfileStatus(event.profileId, event.newStatus, event.message)
        })
        
        return () => unsubscribe()
    }, [])
}
```

---

## 📊 优化效果预期

| 指标 | 优化前 | 优化后 | 改进 |
|------|--------|--------|------|
| **单实例启动时间** | 3-5秒 | 3-5秒 | 无变化 |
| **批量启动10个实例** | 30-50秒（串行） | 15-20秒（并发） | **50%提升** |
| **UI响应性** | 卡顿 | 流畅 | **显著改善** |
| **启动失败率** | 10-15% | 5%以下 | **降低50%** |
| **错误可追踪性** | 模糊 | 清晰 | **明确定位** |

---

## 🚀 实施计划

### **阶段1：锁优化（1.5小时）**
1. 分析锁使用范围
2. 重构启动函数，缩小锁范围
3. 测试并发启动

### **阶段2：启动队列（1小时）**
1. 实现StartupQueue
2. 集成到Manager
3. 配置并发数（默认3）

### **阶段3：启动前检查（1小时）**
1. 实现各项检查函数
2. 集成到启动流程
3. 测试各种失败场景

### **阶段4：状态管理（1.5小时）**
1. 定义状态枚举
2. 实现状态机
3. 更新数据库schema

### **阶段5：前端事件（1小时）**
1. 添加事件监听
2. 实时更新UI
3. 测试用户体验

**总计：6小时**

---

## ✅ 验收标准

1. ✅ **批量启动时UI不假死**
   - 同时启动10个实例
   - UI保持响应
   - 可以继续操作

2. ✅ **状态变化清晰**
   - 显示starting/debug_pending/running
   - 每个状态都有描述
   - 状态实时更新

3. ✅ **失败原因可追踪**
   - 明确的错误信息
   - 失败在哪个阶段
   - 如何解决

4. ✅ **并发控制有效**
   - 最多3个实例同时启动
   - 其他实例排队等待
   - 队列位置可见

5. ✅ **启动前检查生效**
   - 内核路径不存在→提前报错
   - 磁盘空间不足→提前报错
   - 权限问题→提前报错

---

准备开始实施！🚀
