# VPN / 代理协议审计报告

审计日期：2026-09-02
范围：`backend/internal/proxy/`（解析、内核路由、桥接运行时）及代理相关前端入口。
连接栈：`xray` = Xray + sing-box 组合栈；`mihomo` = 独立 Mihomo 栈。

## 已关闭项目

### P0-1 VLESS Reality URI：已修复

`vless://` 的 `security=reality` 现在生成 `realitySettings`，读取 `pbk`、`sid`、`spx`、SNI 和 fingerprint，不再降级为普通 TLS。

验证：`backend/internal/proxy/protocol_regression_test.go`、`backend/internal/proxy/parser_uri.go`。

### P0-2 TUIC URI：已修复

已增加 `tuic://` URI 解析，支持 UUID、密码、SNI、ALPN、fingerprint、insecure 和拥塞控制参数，并统一生成 sing-box TUIC outbound。

验证：`backend/internal/proxy/singbox_tuic_test.go`、`backend/internal/proxy/singbox_parser.go`。

### P1-1 Hysteria v1 与 Hysteria2：已修复

`hysteria://` 和 `hysteria2://` 现在分别生成 sing-box `hysteria` 与 `hysteria2`，Clash YAML 的 v1 节点也不再误生成 v2 字段。

验证：`backend/internal/proxy/protocol_regression_test.go`、`backend/internal/proxy/singbox_parser.go`。

### P1-2 Shadowsocks plugin：已修复静默降级

带 `plugin` 的 Shadowsocks 节点被识别为 Mihomo-only；在 Xray 路径明确拒绝，不再丢弃 plugin 后按裸 SS 连接。

验证：`backend/internal/proxy/protocol_regression_test.go`、`backend/internal/proxy/mihomo_protocol.go`、`backend/internal/proxy/parser_clash_other_protocols.go`。

### P1-3 WireGuard：已放行 Mihomo

Clash `type: wireguard` 节点被识别为 Mihomo-only，通过 YAML 直通 Mihomo，不再被误判为未知协议或进入 Xray。

验证：`backend/internal/proxy/protocol_regression_test.go`、`backend/internal/proxy/mihomo_protocol.go`。

## 运行时专项结论

- `xray` 组合栈只使用 Xray、sing-box 或 native；不会自动回退到 Mihomo。
- `mihomo` 栈的需桥接协议统一使用 Mihomo；跨栈指定会明确报错。
- 实例启动、测速、真实连通性、IP 健康、桥接预热、订阅拉取、插件下载和内核下载均经过统一连接栈解析或 HTTP 客户端入口。
- 临时 `proxyConfig` 在应用层、内核解析层和桥接层均优先于同一 `proxyId` 的已保存配置。
- 测速准备超时会向 Xray、sing-box、Mihomo 建桥流程传递取消信号；取消后不会继续注册迟到的桥接进程。
- LaunchServer 切换失败会尝试恢复旧服务；恢复也失败时，错误中同时保留两段失败上下文。

## 未关闭项目

以下项目属于原报告中的 P2/P3 长尾协议增强，不是本轮 P0/P1 修复的阻塞项：

- ShadowTLS：尚未作为独立协议入口放行。
- Snell：尚未作为独立协议入口放行。
- SSH：尚未作为独立协议入口放行。
- Juicity：尚未作为独立协议入口放行。
- SSR：当前仍明确拒绝；如需支持，应单独设计 Mihomo 路由和兼容性测试。
- VMess URI：仍主要覆盖现有 URI 传输字段；gRPC、H2、QUIC 等完整对齐属于后续增强。

## 当前支持矩阵

| 协议 | URI | Clash | 当前内核 | 状态 |
|---|:---:|:---:|---|---|
| HTTP / HTTPS / SOCKS5 | ✓ | ✓ | native / Xray / Mihomo | 已覆盖 |
| chain+socks5 | ✓ | - | Xray | 已覆盖 |
| VMess | ✓ | ✓ | Xray / Mihomo | URI 传输字段仍有增强空间 |
| VLESS | ✓ | ✓ | Xray / Mihomo | Reality URI 已覆盖 |
| Trojan | ✓ | ✓ | Xray / Mihomo | 已覆盖 |
| Shadowsocks | ✓ | ✓ | Xray / Mihomo | plugin 节点走 Mihomo |
| Hysteria2 | ✓ | ✓ | sing-box / Mihomo | 已覆盖 |
| Hysteria v1 | ✓ | ✓ | sing-box / Mihomo | 已区分 v1/v2 |
| TUIC | ✓ | ✓ | sing-box / Mihomo | URI 与 Clash 均覆盖 |
| AnyTLS | ✓ | ✓ | sing-box / Mihomo | 已覆盖 |
| Mieru | ✗ | ✓ | Mihomo | 已覆盖 |
| WireGuard | ✗ | ✓ | Mihomo | 已放行 |
| SSR / ShadowTLS / Snell / SSH / Juicity | 部分或 ✗ | 部分或 ✗ | - | 后续范围 |

## 结论

原报告列出的 P0/P1 高风险协议问题已由当前代码和回归测试覆盖。若本轮目标是完成当前 `origin/master` 对比中的主流协议、连接栈隔离、运行时生命周期和代理下载修复，可以进入收尾验证；若目标是覆盖全部长尾协议，则不能称为全部完成，需另立任务处理 P2/P3。
