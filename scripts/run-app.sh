#!/usr/bin/env bash
# 从项目目录启动 Ant Browser（项目版 / dev 构建），数据走 <项目>/data/app.db
#
# 用法:
#   scripts/run-app.sh            已在运行就只报状态；端口被安装版占用则提示；否则启动
#   scripts/run-app.sh --restart  关掉占用 19876 的实例（项目版或安装版），再重新启动
#   scripts/run-app.sh --force    同 --restart（保留旧名字）
#   scripts/run-app.sh --check    只检查，不启动、不改动任何进程
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT_DIR/build/bin/ant-chrome"
PORT=19876
LOCK="$ROOT_DIR/app-instance.lock"
# 安装版（旧构建）把根目录落在 XDG 数据目录，锁文件在这里
XDG_LOCK="/data/antblack/app-data/ant-browser/app-instance.lock"

MODE="start"
case "${1:-}" in
  ""|start)      MODE="start" ;;
  --restart|restart|--force|force) MODE="restart" ;;
  --check|check) MODE="check" ;;
  -h|--help|help)
    sed -n '2,9p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0 ;;
  *) echo "未知参数：$1（可用 --check / --restart）" >&2; exit 2 ;;
esac

port_busy() { ss -ltn 2>/dev/null | awk -v p=":$PORT" 'NR>1 && $4 ~ p"$" {f=1} END{exit !f}'; }
lock_pid() { sed -n 's/.*"pid":\([0-9]*\).*/\1/p' "$1" 2>/dev/null | head -1; }

show_state() {
  echo "项目根    : $ROOT_DIR"
  if [[ -x "$BIN" ]]; then
    echo "可执行    : $BIN（构建于 $(stat -c %y "$BIN" | cut -d. -f1)）"
  else
    echo "可执行    : [缺失] 先执行 wails build -s -skipbindings -tags webkit2_41 -o ant-chrome"
  fi
  echo "数据库    : $ROOT_DIR/data/app.db（$(stat -c %s "$ROOT_DIR/data/app.db" 2>/dev/null || echo 0) 字节）"
  if port_busy; then echo "端口 $PORT : 被占用"; else echo "端口 $PORT : 空闲"; fi
  if [[ -f "$LOCK" ]]; then
    echo "项目版进程: $(cat "$LOCK")"
  else
    echo "项目版进程: 未运行"
  fi
  if [[ -f "$XDG_LOCK" ]]; then
    echo "安装版进程: $(cat "$XDG_LOCK")"
  fi
  if port_busy; then
    local n
    n="$(curl -s -m 5 "http://127.0.0.1:$PORT/api/profiles" 2>/dev/null | sed -n 's/.*"count":\([0-9]*\).*/\1/p' | head -1)"
    [[ -n "${n:-}" ]] && echo "接口实例数: $n（37=项目版 / 110=安装版）"
  fi
}

wait_port_free() {
  for _ in $(seq 1 30); do
    port_busy || return 0
    sleep 0.5
  done
  return 1
}

# 先礼后兵：SIGTERM 等 10 秒；仍不退出说明卡在后台任务上（例如代理测速扫描），
# 连同它拉起的 sing-box/xray 内核一起 SIGKILL。
stop_pid() {
  local p="$1"
  kill "$p" 2>/dev/null || return 0
  for _ in $(seq 1 20); do
    kill -0 "$p" 2>/dev/null || return 0
    sleep 0.5
  done
  echo "pid=$p 10 秒内未退出，升级为 SIGKILL（连同子进程）"
  local kids
  kids="$(pgrep -P "$p" 2>/dev/null | tr '\n' ' ' || true)"
  kill -9 "$p" 2>/dev/null || true
  [[ -n "${kids// /}" ]] && kill -9 $kids 2>/dev/null || true
  for _ in $(seq 1 10); do
    kill -0 "$p" 2>/dev/null || break
    sleep 0.5
  done
}

# 顺序：先看状态，再决定要不要动进程
show_state
echo

if [[ "$MODE" == "check" ]]; then
  exit 0
fi

if port_busy; then
  if [[ -f "$LOCK" && "$MODE" == "start" ]]; then
    echo "项目版已经在运行（$(cat "$LOCK")），不需要重复启动。"
    echo "要重启就加参数：scripts/run-app.sh --restart"
    exit 0
  fi
  # 占着端口的是别的实例（通常是 /opt 安装版），或用户显式要求重启
  killed=0
  for lf in "$LOCK" "$XDG_LOCK"; do
    [[ -f "$lf" ]] || continue
    p="$(lock_pid "$lf")"
    if [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null; then
      echo "关闭占用端口 $PORT 的实例：pid=$p（$lf）"
      stop_pid "$p"
      killed=1
    fi
  done
  if [[ "$killed" == "0" ]]; then
    echo "端口 $PORT 被占用，但没找到可关闭的实例锁（可能在别的用户/命名空间里）。" >&2
    exit 1
  fi
  if ! wait_port_free; then
    echo "端口 $PORT 仍未释放，Abort。" >&2
    exit 1
  fi
  # 清理被强杀后残留的锁文件，避免新进程误判已有实例
  for lf in "$LOCK" "$XDG_LOCK"; do
    [[ -f "$lf" ]] || continue
    p="$(lock_pid "$lf")"
    if [[ -z "$p" ]] || ! kill -0 "$p" 2>/dev/null; then
      rm -f "$lf" && echo "清理残留锁：$lf"
    fi
  done
fi

if [[ ! -x "$BIN" ]]; then
  echo "找不到可执行文件：$BIN" >&2
  exit 1
fi

cd "$ROOT_DIR"
echo "启动：$BIN（工作目录 $ROOT_DIR → appRoot 取当前目录，数据 data/app.db）"
setsid nohup "$BIN" >"$ROOT_DIR/data/logs/dev-stdout.log" 2>&1 &
disown 2>/dev/null || true

for _ in $(seq 1 20); do
  [[ -f "$LOCK" ]] && break
  sleep 0.5
done
sleep 1
echo
show_state
echo
if [[ -f "$LOCK" ]]; then
  echo "已启动。核对实例数："
  echo "  curl -s http://127.0.0.1:$PORT/api/profiles | head -c 120"
else
  echo "没等到 app-instance.lock，启动可能失败，看日志："
  echo "  tail -20 $ROOT_DIR/data/logs/dev-stdout.log"
  exit 1
fi
