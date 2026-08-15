#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="${1:-stable}"

usage() {
  cat <<'EOF'
Usage:
  ./dev.sh [stable|live|help]

Modes:
  stable   Default. Build frontend static assets and start Wails without Vite dev server.
  live     Start the frontend dev server and connect Wails to it.
  help     Show this help.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[ERROR] Missing required command: $cmd" >&2
    exit 1
  fi
}

is_tcp_port_busy() {
  local port="$1"
  # 跨平台端口占用检测:macOS 用 lsof,Linux 用 ss;都没有则保守认为未占用。
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
  elif command -v ss >/dev/null 2>&1; then
    ss -ltn "( sport = :$port )" | tail -n +2 | grep -q .
  else
    return 1
  fi
}

resolve_wails_devserver() {
  local start_port="${WAILS_DEVSERVER_PORT:-34115}"
  local host="${WAILS_DEVSERVER_HOST:-127.0.0.1}"
  local port="$start_port"

  while is_tcp_port_busy "$port"; do
    port=$((port + 1))
  done

  WAILS_DEVSERVER_ADDRESS="$host:$port"
  export WAILS_DEVSERVER_ADDRESS
}

prepare_env() {
  require_cmd node
  require_cmd npm
  require_cmd go
  # 确保 `go install` 安装的 wails 可被找到(GOPATH/bin 常未加入 PATH)。
  export PATH="$PATH:$(go env GOPATH)/bin"
  require_cmd wails

  if [[ -n "${DEV_PROXY_URL:-}" ]]; then
    export HTTP_PROXY="$DEV_PROXY_URL"
    export HTTPS_PROXY="$DEV_PROXY_URL"
    export http_proxy="$DEV_PROXY_URL"
    export https_proxy="$DEV_PROXY_URL"
  fi

  if [[ -n "${DEV_NO_PROXY:-}" ]]; then
    export NO_PROXY="$DEV_NO_PROXY"
    export no_proxy="$DEV_NO_PROXY"
  fi

  if [[ -n "${DEV_GOPROXY:-}" ]]; then
    export GOPROXY="$DEV_GOPROXY"
  elif [[ -z "${GOPROXY:-}" ]]; then
    export GOPROXY="https://goproxy.cn,direct"
  fi
}

install_frontend_deps() {
  echo "Installing frontend dependencies..."
  npm install
}

build_frontend() {
  echo "Building frontend assets..."
  npm run build:clean
}

run_stable() {
  echo "========================================"
  echo "  Ant Chrome - Dev Launcher"
  echo "========================================"
  echo
  echo "Current workdir: $ROOT_DIR"
  echo "Mode: stable"
  echo "Frontend mode: stable static assets"
  echo "Wails frontend dev server: disabled"
  echo

  prepare_env
  resolve_wails_devserver
  cd "$ROOT_DIR/frontend"
  install_frontend_deps
  build_frontend

  cd "$ROOT_DIR"
  echo "Starting Wails dev..."
  echo "Wails dev server: http://$WAILS_DEVSERVER_ADDRESS"
  exec wails dev -m -nogorebuild -noreload -s -skipbindings -assetdir frontend/dist -devserver "$WAILS_DEVSERVER_ADDRESS"
}

run_live() {
  local frontend_port="${FRONTEND_PORT:-5218}"
  local frontend_pid=""

  trap 'if [[ -n "$frontend_pid" ]] && kill -0 "$frontend_pid" >/dev/null 2>&1; then kill "$frontend_pid" >/dev/null 2>&1 || true; fi' EXIT

  echo "========================================"
  echo "  Ant Chrome - Dev Launcher"
  echo "========================================"
  echo
  echo "Current workdir: $ROOT_DIR"
  echo "Mode: live"
  echo "Frontend URL: http://127.0.0.1:$frontend_port"
  echo

  prepare_env
  resolve_wails_devserver

  cd "$ROOT_DIR/frontend"
  install_frontend_deps
  npm run dev:raw -- --host 127.0.0.1 --port "$frontend_port" &
  frontend_pid="$!"

  cd "$ROOT_DIR"
  echo "Starting Wails dev..."
  echo "Wails dev server: http://$WAILS_DEVSERVER_ADDRESS"
  exec wails dev -m -s -skipbindings -frontenddevserverurl "http://127.0.0.1:$frontend_port" -viteservertimeout 60 -devserver "$WAILS_DEVSERVER_ADDRESS"
}

case "$MODE" in
  stable)
    run_stable
    ;;
  live)
    run_live
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    echo "[ERROR] Unsupported mode: $MODE" >&2
    echo >&2
    usage >&2
    exit 1
    ;;
esac
