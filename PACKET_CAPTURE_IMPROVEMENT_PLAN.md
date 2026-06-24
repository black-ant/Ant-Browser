# 抓包工具完善计划

## 📊 当前实现分析

### ✅ 已实现的功能

#### 1. 核心抓包能力
- **网络请求捕获**：基于 CDP Network 域实时抓包
- **控制台日志**：Runtime 域按需启用（降低指纹风险）
- **HAR 导出**：符合 HAR 1.2 规范的标准格式
- **JavaScript 执行**：页面上下文代码执行
- **截图功能**：PNG 格式页面截图
- **存储读取**：localStorage/sessionStorage 数据查看
- **性能统计**：请求成功率、大小、耗时等指标

#### 2. UI 功能
- **实时筛选**：按 URL、HTTP 方法、状态码过滤
- **多维排序**：时间、大小、耗时、状态码排序
- **请求详情**：Headers、Body、Timing 详细查看
- **WebSocket 连接**：通过后端 CDP 会话避免 Origin 限制

#### 3. 架构优势
- **后端代理**：Go dialer 绕过浏览器 WebSocket Origin 限制
- **串行写入**：解决 gorilla/websocket 并发写崩溃问题
- **自动重连**：连接断开自动重试（最多 5 次）
- **惰性启用**：Runtime 域仅在查看控制台时启用，降低检测风险

---

## 🎯 完善建议（按优先级）

### 🔴 高优先级（核心功能增强）

#### 1. **请求/响应体内容捕获增强**

**现状问题**：
```go
// backend/internal/cdp/events.go
// 当前只在 Network.responseReceived 时获取响应体，很多场景下响应体为空
```

**改进方案**：
- 添加 `Network.getResponseBody` 调用获取完整响应
- 支持大文件分块获取（避免内存溢出）
- 添加响应体大小限制配置（默认 10MB）

```go
// 伪代码示例
func (s *CDPSession) fetchResponseBody(requestID string) {
    result, err := s.SendCommand("Network.getResponseBody", map[string]interface{}{
        "requestId": requestID,
    })
    if err == nil {
        if body, ok := result["body"].(string); ok {
            // 检查大小限制
            if len(body) <= maxResponseBodySize {
                req.ResponseBody = body
            }
        }
    }
}
```

**收益**：
- ✅ HAR 导出包含完整响应内容
- ✅ 支持重放和调试 API 请求
- ✅ 便于排查接口返回异常

---

#### 2. **WebSocket 抓包支持**

**现状问题**：
当前仅捕获 HTTP/HTTPS 请求，不支持 WebSocket 消息抓包。

**改进方案**：
添加 WebSocket 消息监听：

```typescript
// 新增事件类型
type WSMessage = {
  connectionId: string
  direction: 'send' | 'receive'
  timestamp: number
  opcode: number // 1=text, 2=binary
  data: string
  masked: boolean
}

// CDP事件监听
Network.webSocketCreated
Network.webSocketFrameSent
Network.webSocketFrameReceived
Network.webSocketClosed
```

**UI 展示**：
- 新增「WebSocket」标签页
- 显示连接列表 + 消息时序图
- 支持按连接 ID 筛选消息
- 文本/二进制消息格式化显示

**收益**：
- ✅ 支持实时通信应用调试
- ✅ 捕获 WebSocket 握手过程
- ✅ 便于排查推送消息问题

---

#### 3. **请求重放功能**

**现状问题**：
捕获的请求无法直接重新发送测试。

**改进方案**：

**后端 API**：
```go
// 新增 CDPReplayRequest 接口
func (s *CDPSession) ReplayRequest(req *NetworkRequest) (*NetworkRequest, error) {
    // 使用 Fetch 域拦截并修改请求
    // 或通过 fetch() API 在页面上下文重放
}
```

**前端 UI**：
- 请求详情增加「重放」按钮
- 支持编辑 Headers/Body 后重放
- 显示重放结果对比（原始 vs 重放）

**高级功能**：
- 批量重放（压力测试）
- 修改参数重放（参数化测试）
- 保存为 cURL 命令

**收益**：
- ✅ 快速验证 API 修改效果
- ✅ 无需切换到 Postman/Insomnia
- ✅ 支持简单的接口压测

---

#### 4. **请求拦截与修改**

**现状问题**：
只能被动观察，无法主动修改请求/响应。

**改进方案**：
使用 CDP `Fetch` 域实现请求拦截：

```go
// 启用 Fetch 域
Fetch.enable({
    patterns: [
        { urlPattern: "*", requestStage: "Request" },
        { urlPattern: "*", requestStage: "Response" }
    ]
})

// 监听事件
Fetch.requestPaused -> 修改/继续/失败
```

**UI 功能**：
- 「拦截规则」配置面板
  - URL 模式匹配（支持通配符）
  - 修改 Headers/Body/状态码
  - 延迟响应时间（模拟慢网络）
  - 阻止某些请求（模拟失败）

**使用场景**：
- 模拟后端返回异常数据
- 测试前端错误处理逻辑
- 替换 API 返回内容（mock）
- 模拟网络延迟/丢包

**收益**：
- ✅ 无需修改后端代码即可测试异常
- ✅ 前端开发独立于后端进度
- ✅ 更强大的调试能力

---

### 🟠 中优先级（易用性提升）

#### 5. **请求筛选增强**

**当前筛选**：URL 关键词、方法、状态码

**新增筛选维度**：
- **资源类型**：Document / Script / Stylesheet / Image / Media / Font / XHR / Fetch / WebSocket
- **域名**：按域名分组显示
- **大小范围**：`> 1MB` / `< 100KB`
- **耗时范围**：`> 2s` / `< 100ms`
- **缓存状态**：from cache / from network
- **发起者**：显示请求调用栈

**UI 改进**：
```tsx
// 添加高级筛选面板
<FilterPanel>
  <Select label="资源类型" options={['all', 'xhr', 'script', 'image', ...]} />
  <Input label="域名" placeholder="api.example.com" />
  <RangeInput label="大小" min={0} max={10MB} />
  <RangeInput label="耗时" min={0} max={10s} />
</FilterPanel>
```

**收益**：
- ✅ 快速定位特定类型请求
- ✅ 识别性能瓶颈（慢请求/大资源）
- ✅ 更精细的数据分析

---

#### 6. **HAR 导出增强**

**当前问题**：
- 响应体不完整
- 缺少详细的 Timing 信息
- 不支持导入现有 HAR 文件

**改进方案**：

**完整 Timing 数据**：
```go
// 从 Network.responseReceived 的 timing 字段提取
Timings: HARTimings{
    Blocked: timing.dnsStart,
    DNS:     timing.dnsEnd - timing.dnsStart,
    Connect: timing.connectEnd - timing.connectStart,
    SSL:     timing.sslEnd - timing.sslStart,
    Send:    timing.sendEnd - timing.sendStart,
    Wait:    timing.receiveHeadersEnd - timing.sendEnd,
    Receive: endTime - timing.receiveHeadersEnd,
}
```

**HAR 导入功能**：
- 支持加载 .har 文件查看历史抓包
- 对比两次抓包差异（回归测试）
- 从 Chrome DevTools 导入的 HAR

**HAR 编辑**：
- 移除敏感信息（Cookie/Token）
- 过滤某些请求后再导出
- 添加注释/标记

**收益**：
- ✅ 与其他工具（Charles/Fiddler）互操作
- ✅ 归档历史抓包数据
- ✅ 团队协作分享抓包结果

---

#### 7. **性能分析增强**

**当前统计**：总数、成功、失败、总大小、平均耗时

**新增分析**：

**瀑布图（Waterfall）**：
```tsx
// 类似 Chrome DevTools Network 的时间线
<WaterfallChart>
  {requests.map(req => (
    <Bar
      start={req.startTime}
      duration={req.duration}
      color={getColorByType(req.type)}
    />
  ))}
</WaterfallChart>
```

**详细统计**：
- 按资源类型分组统计（XHR/JS/CSS/图片各占多少）
- 首字节时间（TTFB）分布
- DNS/SSL/Connect 时间分析
- 并发连接数曲线
- 最慢的 10 个请求

**火焰图**：
- 请求调用链可视化
- 哪个请求触发了后续请求

**收益**：
- ✅ 直观定位性能问题
- ✅ 优化页面加载速度
- ✅ 专业的性能分析报告

---

### 🟢 低优先级（锦上添花）

#### 8. **控制台增强**

**日志分组**：
- 相同日志折叠显示（`×5` 表示重复 5 次）
- 按来源文件分组

**交互式对象查看**：
```tsx
// 当前只显示 message 字符串
// 改进：支持展开对象结构
console.log({ user: { name: 'Alice', age: 30 } })
// UI: 可展开的树形结构
```

**日志导出**：
- 导出为 .txt 文件
- 支持正则表达式搜索
- 高亮关键词

---

#### 9. **Cookie 管理**

**查看 Cookie**：
- 当前页面所有 Cookie 列表
- 按域名/路径/过期时间筛选

**编辑 Cookie**：
- 添加/修改/删除 Cookie
- 批量导入/导出

**会话管理**：
- 保存当前会话 Cookie
- 恢复到某个会话状态

---

#### 10. **持久化与历史记录**

**会话保存**：
- 自动保存最近 10 次抓包会话
- 加载历史会话查看

**对比功能**：
- 对比两次抓包差异
- 高亮新增/删除/修改的请求

**自动抓包**：
- 实例启动时自动开始抓包
- 配置抓包规则（只抓某些域名）

---

## 🔧 技术改进建议

### 1. **响应体捕获优化**

**当前问题**：
```go
// events.go - handleNetworkResponseReceived
// 响应体在此时可能还未完整接收
```

**改进**：
```go
func (s *CDPSession) handleNetworkLoadingFinished(params map[string]interface{}) {
    requestID := params["requestId"].(string)
    
    // 在 loadingFinished 时获取完整响应体
    go func() {
        body, err := s.getResponseBody(requestID)
        if err == nil {
            s.mu.Lock()
            if req := s.requestMap[requestID]; req != nil {
                req.ResponseBody = body
            }
            s.mu.Unlock()
        }
    }()
}

func (s *CDPSession) getResponseBody(requestID string) (string, error) {
    result, err := s.SendCommand("Network.getResponseBody", map[string]interface{}{
        "requestId": requestID,
    })
    if err != nil {
        return "", err
    }
    
    body, _ := result["body"].(string)
    base64Encoded, _ := result["base64Encoded"].(bool)
    
    if base64Encoded {
        // 处理二进制内容
        decoded, _ := base64.StdEncoding.DecodeString(body)
        return string(decoded), nil
    }
    
    return body, nil
}
```

---

### 2. **内存管理优化**

**问题**：
长时间抓包导致内存无限增长。

**改进**：
```go
// 添加配置项
type SessionConfig struct {
    MaxRequests      int   // 最多保留多少个请求（默认 1000）
    MaxResponseSize  int64 // 单个响应体最大大小（默认 10MB）
    MaxTotalSize     int64 // 所有响应体总大小限制（默认 100MB）
}

// 实现 LRU 淘汰
func (s *CDPSession) addRequest(req *NetworkRequest) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 超过限制时移除最老的请求
    if len(s.networkRequests) >= s.config.MaxRequests {
        s.networkRequests = s.networkRequests[1:]
    }
    
    s.networkRequests = append(s.networkRequests, req)
}
```

---

### 3. **WebSocket 稳定性增强**

**当前问题**：
```go
// session.go - listenEvents
// 连接断开后重连逻辑不够健壮
```

**改进**：
```go
func (s *CDPSession) listenEvents() {
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            var msg CDPMessage
            if err := s.ws.ReadJSON(&msg); err != nil {
                if s.shouldReconnect() {
                    s.attemptReconnect()
                    continue
                }
                return
            }
            s.handleMessage(&msg)
        }
    }
}

func (s *CDPSession) attemptReconnect() error {
    s.reconnectAttempts++
    delay := time.Duration(s.reconnectAttempts) * time.Second
    time.Sleep(delay)
    
    ws, _, err := websocket.DefaultDialer.Dial(s.wsURL, nil)
    if err != nil {
        return err
    }
    
    s.ws.Close()
    s.ws = ws
    s.reconnectAttempts = 0
    
    // 重新启用 domains
    return s.enableDomains()
}
```

---

### 4. **性能优化**

**批量更新 UI**：
```go
// 每秒批量推送一次，而不是每个请求推送一次
func (s *CDPSession) startBatchNotify() {
    ticker := time.NewTicker(1 * time.Second)
    go func() {
        for {
            select {
            case <-ticker.C:
                s.notifySubscribers("batch_update", nil)
            case <-s.ctx.Done():
                ticker.Stop()
                return
            }
        }
    }()
}
```

**前端虚拟滚动**：
```tsx
// 大量请求时使用虚拟列表
import { FixedSizeList } from 'react-window'

<FixedSizeList
  height={600}
  itemCount={filteredRequests.length}
  itemSize={60}
>
  {({ index, style }) => (
    <RequestRow request={filteredRequests[index]} style={style} />
  )}
</FixedSizeList>
```

---

## 📋 实施路线图

### 第一阶段（1-2 周）- 核心功能
- [ ] 完整响应体捕获（含大小限制）
- [ ] 请求详情 Timing 数据完善
- [ ] 内存管理优化（LRU 淘汰）

### 第二阶段（2-3 周）- 高级功能
- [ ] WebSocket 抓包支持
- [ ] 请求重放功能
- [ ] 高级筛选（资源类型/域名/大小/耗时）

### 第三阶段（3-4 周）- 专业功能
- [ ] 请求拦截与修改（Fetch 域）
- [ ] 瀑布图性能分析
- [ ] HAR 导入/编辑功能

### 第四阶段（按需）- 锦上添花
- [ ] Cookie 管理
- [ ] 控制台增强（对象展开）
- [ ] 会话历史与对比

---

## 🎨 UI/UX 改进建议

### 1. **布局优化**

**当前**：单一标签页切换

**改进**：分栏布局
```
┌─────────────────────────────────────┐
│ 控制栏 [实例选择] [连接] [刷新]     │
├──────────┬──────────────────────────┤
│ 左侧面板  │ 右侧详情面板              │
│          │                          │
│ • 请求列表│ [概要] [请求] [响应]      │
│   (可筛选)│                          │
│          │ URL: https://...         │
│ • 统计    │ Method: GET              │
│          │ Status: 200 OK           │
│ • 筛选器  │                          │
│          │ Headers:                 │
│          │ ...                      │
└──────────┴──────────────────────────┘
```

### 2. **快捷操作**

- 右键菜单：复制 URL / 复制为 cURL / 重放请求 / 阻止此域名
- 键盘快捷键：
  - `Ctrl+F` 搜索
  - `Ctrl+R` 刷新
  - `Ctrl+E` 导出 HAR
  - `Ctrl+K` 清空

### 3. **状态指示**

- 实时显示：抓包中 🔴 / 已停止 ⏸️
- 统计卡片：动态更新数字
- 连接状态：WebSocket 连接健康度

---

## 🔍 竞品对比

| 功能 | Ant Browser（当前） | Chrome DevTools | Charles | Fiddler |
|------|-------------------|-----------------|---------|---------|
| HTTP 抓包 | ✅ | ✅ | ✅ | ✅ |
| WebSocket | ❌ | ✅ | ✅ | ✅ |
| 请求重放 | ❌ | ❌ | ✅ | ✅ |
| 请求拦截 | ❌ | ✅（Overrides） | ✅ | ✅ |
| HAR 导出 | ✅（基础） | ✅（完整） | ✅ | ✅ |
| 瀑布图 | ❌ | ✅ | ✅ | ✅ |
| Cookie 管理 | ❌ | ✅ | ✅ | ✅ |
| 移动端抓包 | ❌ | ❌ | ✅ | ✅ |

**优势保持**：
- ✅ 集成在反检测浏览器内（无需代理设置）
- ✅ 通过后端规避 WebSocket Origin 限制
- ✅ 自动规避 Runtime 检测风险

**待补齐**：
- ❌ WebSocket / 请求重放 / 拦截修改（核心差距）
- ❌ 瀑布图 / 性能分析（专业功能）

---

## ✅ 总结

### 🎯 核心推荐（立即实施）

1. **响应体完整捕获** - 修复 HAR 导出不完整问题
2. **WebSocket 抓包** - 覆盖更多应用场景
3. **请求重放** - 大幅提升调试效率
4. **高级筛选** - 快速定位问题请求

### 📊 预期收益

- **功能完整性**：从 60% → 85%（对标 Chrome DevTools）
- **用户体验**：调试效率提升 3-5 倍
- **专业度**：达到商业抓包工具 80% 的能力

### 🚀 快速启动

**第一步**（1-2 天）：
```go
// 实现完整响应体捕获
func (s *CDPSession) handleNetworkLoadingFinished(params map[string]interface{}) {
    requestID := params["requestId"].(string)
    go s.fetchAndStoreResponseBody(requestID)
}
```

**第二步**（3-5 天）：
```typescript
// 前端添加请求重放按钮
<Button onClick={() => replayRequest(selectedReq)}>
  🔁 重放请求
</Button>
```

**第三步**（1-2 周）：
```go
// 实现 WebSocket 抓包
s.SendCommand("Network.enable", map[string]interface{}{
    "maxResourceBufferSize":     10485760,
    "maxPostDataSize":           10485760,
})
// 监听 webSocketCreated/FrameSent/FrameReceived 事件
```

---

**优先级排序**：
1. 🔴 响应体捕获 + 请求重放（基础必备）
2. 🟠 WebSocket + 高级筛选（覆盖更多场景）
3. 🟢 拦截修改 + 瀑布图（专业能力）
4. 🔵 Cookie 管理 + 历史记录（锦上添花）
