# WebGL 指纹异常修复方案

## 问题背景

browserscan.net 检测到两个问题：
1. **WebGL 异常** - WebGL vendor/renderer 参数不一致或与实际渲染不符
2. **指纹浏览器特征** - 多个浏览器 API 返回值不一致，被识别为反检测浏览器

## 根本原因

fingerprint-chromium **在 Chrome 144+ 版本中已废弃自定义 GPU 参数**：
- ~~`--fingerprint-webgl-vendor`~~ （已废弃）
- ~~`--fingerprint-webgl-renderer`~~ （已废弃）
- ~~`--fingerprint-canvas-noise`~~ （已废弃）
- ~~`--fingerprint-audio-noise`~~ （已废弃）

旧版本前端 UI 仍然提供这些选项，但内核会忽略它们，导致：
- 用户设置的 WebGL vendor/renderer 与实际渲染不符
- Canvas/Audio/WebGL 指纹之间不一致
- 被检测系统识别为异常

## 解决方案

### Chrome 144+ 新机制

使用新的 `--disable-spoofing` 参数来**选择性禁用**指纹混淆：

```bash
--disable-spoofing=font,audio,canvas,clientrects,gpu
```

可选模块：
- `font` - 字体指纹
- `audio` - 音频指纹
- `canvas` - Canvas 指纹
- `clientrects` - ClientRects 指纹
- `gpu` - GPU/WebGL 指纹

### 实现的三种策略

#### 策略 1：自动混淆（推荐，默认）

**原理**：让 fingerprint-chromium 内核基于 `--fingerprint` seed 自动生成一致的指纹。

**配置**：
```yaml
default_fingerprint_args:
  - --fingerprint=<random-seed>  # 必需
  - --fingerprint-brand=Chrome
  - --fingerprint-platform=windows
  - --fingerprint-hardware-concurrency=8
  - --webrtc-ip-handling-policy=disable_non_proxied_udp
```

**优点**：
- ✅ 所有指纹特征（Canvas/Audio/WebGL）基于同一个 seed，相互一致
- ✅ 不会出现 vendor/renderer 不匹配
- ✅ 每个 profile 使用不同 seed，指纹唯一
- ✅ 内核自动处理，无需手动配置

**缺点**：
- ❌ 无法精确控制 WebGL vendor/renderer

**适用场景**：
- 大多数多账号管理场景
- 需要每个账号有独立指纹
- 希望避免被检测为异常

---

#### 策略 2：使用真实硬件

**原理**：禁用 GPU 指纹混淆，使用本机真实的 GPU 信息。

**配置**：
```yaml
default_fingerprint_args:
  - --fingerprint=<random-seed>  # 仍需要，用于 Canvas/Audio
  - --fingerprint-brand=Chrome
  - --fingerprint-platform=windows
  - --disable-spoofing=gpu  # 禁用 GPU 混淆
  - --webrtc-ip-handling-policy=disable_non_proxied_udp
```

**优点**：
- ✅ 完全消除 WebGL 不一致
- ✅ 真实硬件信息更可信
- ✅ 简化配置

**缺点**：
- ❌ 所有 profile 共享相同的 GPU 指纹（同一台机器）
- ❌ 暴露真实 GPU 信息

**适用场景**：
- 单机少量账号
- 不需要隔离 GPU 指纹
- 追求最大真实性

---

#### 策略 3：自定义（已废弃，不推荐）

**原理**：尝试使用旧的 `--fingerprint-webgl-vendor` 等参数。

**问题**：
- ⚠️ Chrome 144+ 内核会**忽略这些参数**
- ⚠️ 设置的 vendor/renderer 与实际渲染不符
- ⚠️ 容易被检测为异常

**仅用于**：
- 向后兼容旧配置
- 测试目的

---

## 代码修改

### 1. 更新 FingerprintConfig 接口

添加 `disableSpoofing` 字段：

```typescript
export interface FingerprintConfig {
  // ...现有字段...

  // Chrome 144+ 新参数：选择性禁用指纹混淆
  disableSpoofing?: string[]    // --disable-spoofing=font,audio,canvas,clientrects,gpu

  // 已废弃字段（保留用于向后兼容）
  canvasNoise?: boolean         // --fingerprint-canvas-noise=（已废弃）
  webglVendor?: string          // --fingerprint-webgl-vendor=（已废弃）
  webglRenderer?: string        // --fingerprint-webgl-renderer=（已废弃）
  audioNoise?: boolean          // --fingerprint-audio-noise=（已废弃）
}
```

### 2. 更新序列化函数

```typescript
export function serialize(config: FingerprintConfig): string[] {
  const args: string[] = []
  
  // ...现有字段序列化...

  // Chrome 144+ 新参数
  if (config.disableSpoofing && config.disableSpoofing.length > 0) {
    args.push(`--disable-spoofing=${config.disableSpoofing.join(',')}`)
  }

  // 废弃参数（保留用于向后兼容）
  if (config.canvasNoise !== undefined) args.push(`--fingerprint-canvas-noise=${config.canvasNoise}`)
  if (config.webglVendor) args.push(`--fingerprint-webgl-vendor=${config.webglVendor}`)
  if (config.webglRenderer) args.push(`--fingerprint-webgl-renderer=${config.webglRenderer}`)
  if (config.audioNoise !== undefined) args.push(`--fingerprint-audio-noise=${config.audioNoise}`)

  return [...args, ...(config.unknownArgs ?? [])]
}
```

### 3. UI 更新

在 FingerprintPanel.tsx 中添加策略选择器：

- **WebGL 指纹策略**下拉菜单
  - 自动混淆（推荐）
  - 使用真实硬件
  - 自定义（已废弃）

- 根据选择的策略动态显示/隐藏相关字段
- 添加说明文字解释每种策略的作用

### 4. 更新预设配置

移除所有预设中的废弃参数：
- 删除 `canvasNoise`
- 删除 `audioNoise`
- 删除 `webglVendor`
- 删除 `webglRenderer`

让内核基于 `--fingerprint` seed 自动生成一致的指纹。

---

## 验证方法

### 1. 新建 Profile 测试

1. 新建一个 profile
2. 指纹策略选择 **"自动混淆（推荐）"**
3. 确保有唯一的指纹种子（自动生成）
4. 启动窗口

**预期结果**：
- 日志显示：`--fingerprint=<seed>`
- 日志**不应**显示：`--fingerprint-webgl-vendor` 或 `--fingerprint-webgl-renderer`

### 2. browserscan.net 检测

访问 https://www.browserscan.net/zh/

**检查项目**：
- ✅ **WebGL** - 应为绿色，无异常
- ✅ **Canvas** - 指纹正常
- ✅ **Audio** - 指纹正常
- ✅ **机器人检测** - 不应被识别为指纹浏览器

**关键指标**：
- WebGL Vendor/Renderer 与实际渲染一致
- Canvas/Audio/WebGL 指纹相互匹配
- 可信度评分提升

### 3. 对比测试

创建两个 profile，使用不同的指纹种子：

**Profile A**:
```yaml
--fingerprint=123456789
```

**Profile B**:
```yaml
--fingerprint=987654321
```

**预期结果**：
- 两个 profile 的 WebGL 指纹应不同
- 每个 profile 内部的 Canvas/Audio/WebGL 应一致
- browserscan.net 检测均无异常

---

## 升级指南

### 对于现有用户

1. **旧 profile 保持兼容**：现有配置中的 `webglVendor`/`webglRenderer` 参数会被保留，但内核会忽略它们。

2. **推荐操作**：
   - 编辑现有 profile
   - 将 WebGL 策略改为 **"自动混淆（推荐）"**
   - 保存后重新启动窗口

3. **批量更新**：
   - 可以通过编辑 `config.yaml` 中的 profile 配置
   - 删除 `fingerprint_args` 中的废弃参数
   - 确保每个 profile 有唯一的 `--fingerprint=<seed>`

### 对于新用户

- 直接使用新 UI，默认就是 **"自动混淆"** 策略
- 系统会自动生成唯一的指纹种子
- 无需手动配置 WebGL vendor/renderer

---

## 技术参考

- **fingerprint-chromium 仓库**: https://github.com/adryfish/fingerprint-chromium
- **Chrome 144+ 变更**: 移除自定义 GPU 参数，改用 `--disable-spoofing`
- **WebGL 指纹检测原理**: https://blog.browserscan.net/zh/docs/webgl-fingerprinting
- **BrowserScan 检测工具**: https://www.browserscan.net/zh/

---

## 常见问题

### Q: 为什么不再支持自定义 WebGL vendor/renderer？

A: fingerprint-chromium 在 Chrome 144+ 版本中移除了这些参数，因为：
1. 维护成本高
2. 容易造成不一致（vendor 与实际渲染不符）
3. 新的自动混淆机制更可靠

### Q: 自动混淆会生成什么样的 GPU 信息？

A: 内核会基于 `--fingerprint` seed 和 `--fingerprint-platform` 生成合理的 GPU 信息，确保与平台一致（例如 Windows 不会生成 Apple GPU）。

### Q: 使用真实硬件会暴露我的机器信息吗？

A: 是的，使用 `--disable-spoofing=gpu` 会暴露真实的 GPU vendor/renderer。如果同一台机器运行多个 profile，它们会共享相同的 GPU 指纹。

### Q: 旧的 Canvas/Audio 噪声参数还能用吗？

A: 可以设置，但内核会忽略。Chrome 144+ 的 Canvas/Audio 噪声由 `--fingerprint` seed 统一控制，无需单独设置。

### Q: 如何确认我的内核版本支持新参数？

A: 检查内核版本号是否 >= Chrome 144。运行 `chrome --version` 或在浏览器中访问 `chrome://version`。

---

## 总结

**推荐配置（新建 profile）**：

```yaml
fingerprint_args:
  - --fingerprint=<auto-generated-unique-seed>
  - --fingerprint-brand=Chrome
  - --fingerprint-platform=windows
  - --fingerprint-hardware-concurrency=8
  - --webrtc-ip-handling-policy=disable_non_proxied_udp
  
# 不再需要：
# - --fingerprint-webgl-vendor=Intel
# - --fingerprint-webgl-renderer=Intel(R) UHD Graphics 630
# - --fingerprint-canvas-noise=true
# - --fingerprint-audio-noise=true
```

**关键点**：
1. ✅ 每个 profile 使用唯一的 `--fingerprint` seed
2. ✅ 让内核自动生成一致的 Canvas/Audio/WebGL 指纹
3. ✅ 删除所有废弃的自定义 GPU 参数
4. ✅ 通过 browserscan.net 验证无异常

这样可以有效消除 **WebGL 异常** 和 **指纹浏览器特征** 检测。
