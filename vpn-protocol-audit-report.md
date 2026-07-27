# VPN / 代理协议审计报告(按优先级)

审计日期:2026-07-26
范围:`backend/internal/proxy/`(解析 + 内核路由 + 桥接运行时)+ 前端导入
架构:三内核路由 —— **Xray** / **sing-box** / **mihomo(clash-meta)** / **native**

> 关键背景(决定修复成本):mihomo 节点是 **YAML 直通**——`buildMihomoNode()`
> (`mihomo_bridge.go:357`)把 Clash 节点原样透传给 mihomo 内核。因此“补协议”
> 大多不是写解析器,而是在 `SupportedKernelsForProtocol`(`kernel_resolver.go:118`)
> 和 `mihomoOnlyProtocolType`(`mihomo_protocol.go:14`)两处白名单里放行。

---

## P0 — 必须优先(导致连接失败 / 高频缺失)

### P0-1　VLESS Reality 的 `vless://` URI 解析被降级 【Bug】
- 位置:`parser_uri.go:112` `buildOutboundVless`
- 现象:`security=reality` 被强制写成 `security:"tls"`,且 `pbk`(publicKey)、
  `sid`(shortId)、`spx`(spiderX)**完全没读取**。
- 后果:通过分享链接导入的 Reality 节点在 Xray 下**必然握手失败**;Reality 目前
  仅在 Clash YAML 路径(`parser_clash_v_protocols.go:38` reality-opts)可用。
- 影响面:Reality 是当前最主流的抗封锁协议,URI 分享链接极其常见。
- 修复:URI 解析补 `security:"reality"` + `realitySettings{publicKey/shortId/spiderX}`。
- 成本:中

### P0-2　TUIC 缺 `tuic://` URI 解析器 【缺失】
- 位置:`DetectProxyProtocol`(`kernel_resolver.go:110`)已识别 `tuic://` 前缀,
  但 `BuildSingBoxOutbound`(`singbox_parser.go:29`)**没有 tuic 分支**,只有
  Clash YAML 能解析 TUIC。
- 后果:TUIC 分享链接导入报“不支持的格式”。
- 修复:仿照已有 `buildSingBoxTUICFromClash` 写 `parseTUICURI`。
- 成本:低(高性价比)

---

## P1 — 尽快(隐蔽 Bug / 高价值协议)

### P1-1　Hysteria v1 被错误映射为 Hysteria2 【Bug】
- 位置:`singbox_parser.go:52`,`hysteria://` 被直接改写成 `hysteria2://`;
  `IsSingBoxProtocol`、`parseClashSingBoxNode` 也把 v1 归到 v2 分支。
- 后果:v1/v2 是不同协议(sing-box `type` 分别为 `hysteria`/`hysteria2`,
  认证字段 `auth_str` vs `password`、obfs 结构、带宽字段要求都不同),真 v1
  节点会被生成成非法的 v2 配置,静默连不上。
- 修复:正确实现 v1,或明确拒绝并提示“请使用 Hysteria2”。
- 成本:低

### P1-2　Shadowsocks plugin 被静默丢弃 【Bug】
- 位置:`parser_clash_other_protocols.go:77`,`plugin`/`plugin-opts` 读出后
  `_ = pluginOpts` 直接丢弃。
- 后果:`obfs`(simple-obfs)、`v2ray-plugin`、`shadow-tls` 等被忽略,带插件的
  SS 以“裸 SS”连接 → 失败。
- 修复:检测到 plugin 时改走 mihomo(原生支持这些插件),或明确报错,禁止静默降级。
- 成本:低

### P1-3　WireGuard 未放行 【缺失】
- 位置:`SupportedKernelsForProtocol` 无 `wireguard` 分支,default 里
  `IsMihomoOnlyProtocol` 只认 `mieru`,故 `type: wireguard` 节点被判“不支持的代理协议”。
- 说明:mihomo/sing-box 均原生支持,mihomo 直通即可。使用面广(WARP、自建、机场落地)。
- 修复:放行到 mihomo(mihomo-only 直通)。
- 成本:低

---

## P2 — 计划内(增强 / 抗封锁传输)

### P2-1　ShadowTLS(v3)放行 【缺失】
主流机场抗封锁传输层,mihomo/sing-box 支持,当前不识别。放行到 mihomo 直通;
与 P1-2 的 SS plugin=shadow-tls 联动。成本:低

### P2-2　Snell(v3/v4)放行 【缺失】
Surge 生态协议,mihomo 支持。mihomo 直通放行即可。成本:低

### P2-3　VMess `vmess://` URI 传输层对齐 Clash 【增强】
- 位置:`parser_uri.go:11` `buildOutboundVmess` 只处理 `net=ws`,不支持
  grpc/h2/quic,且声明了 `Alpn` 却未使用。
- 后果:同一 VMess 节点,URI 导入与 YAML 导入能力不一致。
- 成本:中

---

## P3 — 按需 / 长尾(产品取舍)

- **SSR**:`parser.go:133`、`parser_clash_entry.go:42` 显式拒绝。Xray 确实不支持,
  但 mihomo 原生支持;若有存量用户,可改为路由 mihomo 而非报错。
- **SSH**:mihomo/sing-box 支持,偶作出口跳板。
- **Juicity**:较新 QUIC 协议,sing-box 支持,用户面窄。

---

## 汇总表

| 优先级 | 项目 | 类型 | 成本 |
|:---:|------|:---:|:---:|
| P0 | VLESS Reality 的 URI 解析(pbk/sid/spx + reality) | Bug | 中 |
| P0 | TUIC `tuic://` URI 解析器 | 缺失 | 低 |
| P1 | Hysteria v1 正确处理或明确拒绝 | Bug | 低 |
| P1 | SS plugin 不再静默丢弃 | Bug | 低 |
| P1 | WireGuard 放行 mihomo 直通 | 缺失 | 低 |
| P2 | ShadowTLS 放行 | 缺失 | 低 |
| P2 | Snell 放行 | 缺失 | 低 |
| P2 | VMess URI 支持 grpc/alpn | 增强 | 中 |
| P3 | SSR 改路由 mihomo / SSH / Juicity | 取舍 | 中 |

---

## 当前协议支持矩阵(现状)

| 协议 | URI | Clash | 内核 | 状态 |
|------|:---:|:---:|------|------|
| HTTP/HTTPS/SOCKS5 | ✓ | ✓ | native / xray(带鉴权桥接) | 完整 |
| chain+socks5(自研) | ✓ | - | xray | 完整 |
| VMess | ✓ | ✓ | xray/mihomo | 部分(URI 仅 ws) |
| VLESS | ✓ | ✓ | xray/mihomo | 部分(Reality URI 有 Bug) |
| Trojan | ✓ | ✓ | xray/mihomo | 完整 |
| Shadowsocks | ✓ | ✓ | xray/mihomo | 部分(plugin 丢弃) |
| Hysteria2 | ✓ | ✓ | sing-box/mihomo | 完整 |
| Hysteria v1 | △ | △ | sing-box/mihomo | 有缺陷(误当 v2) |
| TUIC | ✗ | ✓ | sing-box/mihomo | 部分(无 URI) |
| AnyTLS | ✓ | ✓ | sing-box/mihomo | 完整 |
| Mieru | ✗ | ✓ | mihomo only | 完整 |
| SSR | ✗ 拒绝 | ✗ 拒绝 | - | 不支持 |
| WireGuard / ShadowTLS / Snell / SSH | ✗ | ✗ | - | 不支持(mihomo 可直通) |

✓ 完整 / △ 有缺陷 / ✗ 缺失

---

## 一句话结论

主流协议框架已齐备,但有 **两处会直接导致连不上的 Bug(P0 Reality URI、P1 SS 插件丢弃)** 和 **一处隐蔽 Bug(P1 Hysteria v1 误当 v2)** 应优先处理;而 **WireGuard / ShadowTLS / Snell 等“不支持”其实是白名单没放开**,借助 mihomo YAML 直通架构,补充成本很低。
