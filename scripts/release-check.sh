#!/usr/bin/env bash
# 发布前运行时清单校验：检查打包所需的运行时二进制是否就位，避免 bin 缺失导致
# 桥接代理 / 内核无法启动。
# 用法：bash scripts/release-check.sh
# 退出码：缺少必需项时非零。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 解析平台目录（与 backend 的 resolveBinary 一致：bin/<os>-<arch>）
OS="$(go env GOOS 2>/dev/null || echo unknown)"
ARCH="$(go env GOARCH 2>/dev/null || echo unknown)"
EXE=""
[ "$OS" = "windows" ] && EXE=".exe"
PLATFORM_DIR="bin/${OS}-${ARCH}"

missing=0
warn=0

check_bin() {
  local name="$1"; local required="$2"
  for dir in "$PLATFORM_DIR" "bin"; do
    if [ -f "$dir/${name}${EXE}" ]; then
      echo "  ✅ ${name}: $dir/${name}${EXE}"
      return 0
    fi
  done
  if command -v "$name" >/dev/null 2>&1; then
    echo "  ✅ ${name}: $(command -v "$name") (PATH)"
    return 0
  fi
  if [ "$required" = "required" ]; then
    echo "  ❌ ${name}: 未找到（${PLATFORM_DIR}/ 或 bin/ 或 PATH）"
    missing=$((missing+1))
  else
    echo "  ⚠️  ${name}: 未找到（可选；hysteria2/tuic 等协议需要）"
    warn=$((warn+1))
  fi
}

echo "==> 运行时二进制清单校验（平台 ${OS}-${ARCH}）"
# xray：vmess/vless/trojan/ss 桥接所需
check_bin "xray" "required"
# sing-box：hysteria2/tuic 桥接所需（可选）
check_bin "sing-box" "optional"

echo "==> Chrome 内核目录"
if [ -d "chrome" ] && [ -n "$(ls -A chrome 2>/dev/null)" ]; then
  echo "  ✅ chrome/ 存在且非空（$(ls chrome | wc -l | tr -d ' ') 个条目）"
else
  echo "  ⚠️  chrome/ 缺失或为空（用户也可在应用内下载内核）"
  warn=$((warn+1))
fi

echo ""
if [ "$missing" -gt 0 ]; then
  echo "❌ 校验未通过：缺少 ${missing} 个必需运行时文件（另有 ${warn} 个告警）"
  exit 1
fi
echo "✅ 必需运行时文件齐备（${warn} 个可选告警）"
