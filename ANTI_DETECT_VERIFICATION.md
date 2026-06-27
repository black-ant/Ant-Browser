# Ant-Browser 反检测改进验证指南

本次改动覆盖 **默认值加固、代理一致性、CDP 痕迹最小化、pipe 传输**四大方向,以降低被 CreepJS / PixelScan / IPHey / BrowserLeaks 等检测站点识别为"指纹浏览器"的概率。

---

## 改动摘要

### ✅ Tier 1: 默认值 + 代理一致性(投入产出最高)

1. **默认指纹参数加固** ([config.go](backend/internal/config/config.go) / 各 YAML)
   - 新增 `--webrtc-ip-handling-policy=disable_non_proxied_udp`(默认阻止 WebRTC 泄露真实 IP)
   - 新增 `--disable-blink-features=AutomationControlled`(防御 `navigator.webdriver`)
   - 去除启动时强制打开的三个 IP 检测站(`ippure/iplark/ping0`),改为可配 `default_start_urls`(默认空)

2. **按代理出口 IP 自动推导时区/语言** ([geoderive.go](backend/internal/browser/geoderive.go) 新增)
   - 新建 profile 挂代理时,启动前自动从代理出口地理信息(IPPure 缓存)推导 `--timezone` / `--lang`
   - 若用户显式设置则尊重;若代理无缓存则后台补查
   - 消除"时区≠代理 IP 地理位置""语言≠地区"等秒级穿帮

### ✅ Tier 3b: CDP 传输(pipe 模式,实验性)

**配置**: `browser.cdp_transport: "port"(默认) | "pipe"`
- `port`: 沿用 `--remote-debugging-port`(仅绑 127.0.0.1),全平台稳定
- `pipe`: 用 `--remote-debugging-pipe`(fd 3/4 通信,不开本地端口);**仅 Linux/macOS**,Windows 自动回退端口

**改动**:
- 新增 `backend/internal/cdp/pipe*.go` + `pipe_session.go`(多路复用、目标发现、就绪探测)
- `app_instance.go` 启动流程自动分支(pipe/端口)
- Cookie / 用户名扫描 / DevTools 全部适配 `profileCDP`(统一分派层)

---

## 验证范围说明

**本次验证专注于后端改动的核心功能**:
- ✅ 默认指纹参数加固（WebRTC、AutomationControlled）
- ✅ 代理一致性（时区/语言自动推导）
- ✅ CDP 传输模式（pipe/port）
- ✅ 默认启动行为（不再自动外联检测站）

**明确排除以下功能**（不在本次验证范围内）:
- ❌ WebGL 异常 (-5% 选项)
- ❌ 指纹浏览器 (-5% 选项)

这两项为独立的指纹混淆功能，与本次后端改动无关。

---

## 端到端验证步骤

### 1. 基础功能验证(默认端口模式)

```bash
# 构建(已通过)
go build ./...

# 启动应用
wails dev  # 或双击 exe
```

**新建 profile 测试**:
1. 新建一个 profile,**挂任意代理**(如美国 socks5)
2. 启动该窗口 → 日志应显示:
   ```
   注入代理一致性指纹参数: --timezone=America/New_York --lang=en-US
   cdp_transport=port
   ```
3. 在窗口内访问 `https://browserleaks.com/webrtc` → 确认:
   - **WebRTC IP 栏**:仅显示代理 IP,无真实公网/内网 IP 泄露
   - 页面时区 = 代理地理位置(美国东部)
4. 访问 `https://abrahamjuliot.github.io/creepjs` → 检查:
   - `navigator.webdriver` = `false`(绿色)
   - `Timezone` 与代理国一致
   - 整体可信度评分提升

**默认启动行为**:
- 无指定网址启动 → 落到浏览器新标签页(`about:blank`),**不再自动外联三个 IP 检测站**

### 2. pipe 模式验证(仅 Linux/macOS)

编辑 `config.yaml`:
```yaml
browser:
  cdp_transport: pipe  # 从默认 "" 改为 "pipe"
```

重启应用,新建 profile 并启动 → 日志显示:
```
cdp_transport=pipe
```

验证功能完整:
- 启动/停止正常
- **Cookie 读写**(右键菜单 → Cookie 管理)
- **用户名扫描**(若启用该功能)
- **DevTools 面板**(网络/控制台/存储/执行 JS/截图)

若 pipe 模式异常,日志会显示:
```
pipe 模式不支持，回退调试端口
```
此时自动退回端口模式,功能不受影响。

### 3. 高级检测站验证

| 站点 | 检测项 | 预期结果(改进后) |
|------|--------|------------------|
| **iphey.com** | 时区≠IP / WebRTC 泄露 | ✅ 绿色(时区与代理一致,无 IP 泄露) |
| **pixelscan.net** | 时区/语言/WebRTC/webdriver | ✅ 分数提升,无明显异常标红 |
| **browserleaks.com/webrtc** | WebRTC 本地/公网 IP | ✅ 仅显示代理 IP |
| **CreepJS** | navigator.webdriver / Runtime | ✅ webdriver=false;Runtime 泄露需前端配合(Tier 3a 待完成) |

---

## 已知限制与后续

1. **fingerprint-chromium 内核固有特征**:噪声算法签名、内核版本落后等属内核层,无法在本仓库消除;需定期更新内核 + 保证 `--fingerprint-brand` 版本号与实际一致。

2. **pipe 模式运行时验证**:本机 Windows 仅编译通过,Linux/macOS 实际运行需目标平台测试。若遇问题,`cdp_transport: port` 立即回退。

3. **Runtime.enable 惰性启用(Tier 3a)**:需前端配合(控制台标签激活时才启用 Runtime 域),暂未实现。当前 DevTools 连接时仍全量 enable。

4. **地理定位注入(Tier 1c)**:可选功能(经 CDP `Emulation.setGeolocationOverride`),默认关闭;站点需授权才生效,时区+IP 一致已覆盖绝大多数检测。

---

## 配置参考

**新增配置项**(`config.yaml`):
```yaml
browser:
  cdp_transport: ""  # "" 或 "port"(默认端口) / "pipe"(实验性,仅 Linux/macOS)
  default_start_urls: []  # 默认起始页(空=新标签页),避免每次启动都外联检测站
  default_fingerprint_args:
    - --fingerprint-brand=Chrome
    - --fingerprint-platform=windows
    - --webrtc-ip-handling-policy=disable_non_proxied_udp  # 新增
  default_launch_args:
    - --disable-sync
    - --no-first-run
    - --disable-blink-features=AutomationControlled  # 新增
```

**profile 级开关**(待前端 UI,当前后端已支持):
- `FollowProxyGeo bool`:是否按代理出口 IP 自动注入时区/语言(默认 true)

---

## 故障排查

**pipe 模式启动失败**(Linux/macOS):
```
错误: pipe 连接在 3s 内未就绪
→ 检查内核 chrome 是否支持 --remote-debugging-pipe(需 Chrome 63+)
→ 回退: config.yaml 改 cdp_transport: port
```

**时区/语言未自动注入**:
```
→ 确认代理已挂且 ProxyId 非空
→ 查看日志是否显示 "注入代理一致性指纹参数"
→ 首次使用某代理需等待后台 IP 查询完成(~5s),下次启动生效
```

**WebRTC 仍泄露 IP**:
```
→ 确认 profile 指纹参数里没有显式覆盖 --webrtc-ip-handling-policy
→ 旧 profile 需手动加该参数,或删除重建(新 profile 自动含)
```
