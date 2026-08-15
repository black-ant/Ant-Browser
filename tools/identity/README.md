# 指纹自洽引擎 · 构建期数据产物

运行期完全离线;以下产物需要联网**生成一次**(可在有网机器或海外服务器上做),之后随仓库分发。

## 1. 完整指纹池 `pool.json`

```bash
cd tools/identity
npm init -y && npm i fingerprint-generator
node gen-pool.mjs 10000
```

输出到 `backend/internal/identity/data/pool.json`(覆盖引导池)。schema 与引导池相同,`go:embed` 自动生效,无需改 Go 代码。

## 2. 离线 GeoIP 库 `dbip-city-lite.mmdb`

下载 **DB-IP City Lite**(CC-BY-4.0,免费可商用可再分发,含经纬度+时区)或 **MaxMind GeoLite2-City**(需 license key),放到:

```
data/geoip/dbip-city-lite.mmdb
```

启动时若该文件存在,`IdentityService` 自动启用代理地理对齐(`backend/app_startup.go` 已接线);缺文件则优雅降级为不做地理对齐。

> DB-IP: https://db-ip.com/db/download/ip-to-city-lite  ·  需保留 CC-BY 署名。
