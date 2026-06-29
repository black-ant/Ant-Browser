#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DEFAULT_WORKSPACE_INSTALL_ROOT="${HOME}/Codex/1688shopManager/desktop-repos/1688shop-desktop"
DEFAULT_MODE="stable"
DEFAULT_FRONTEND_PORT="5218"
DEFAULT_WORKSPACE_SERVER_URL="http://127.0.0.1:4174/api/health"

DEV_MODE="${DEFAULT_MODE}"
INSTALL_ROOT_ARG=""
WATCHER_PID=""
WORKSPACE_SERVER_PID=""

cleanup() {
  if [[ -n "${WATCHER_PID}" ]]; then
    kill "${WATCHER_PID}" >/dev/null 2>&1 || true
    wait "${WATCHER_PID}" >/dev/null 2>&1 || true
    WATCHER_PID=""
  fi

  if [[ -n "${WORKSPACE_SERVER_PID}" ]]; then
    kill "${WORKSPACE_SERVER_PID}" >/dev/null 2>&1 || true
    wait "${WORKSPACE_SERVER_PID}" >/dev/null 2>&1 || true
    WORKSPACE_SERVER_PID=""
  fi
}

trap cleanup EXIT INT TERM

usage() {
  cat <<'EOF'
Usage:
  scripts/dev-mac.sh [stable|live] [workspace-install-root]

Examples:
  scripts/dev-mac.sh
  scripts/dev-mac.sh live
  scripts/dev-mac.sh stable /path/to/1688shop-desktop

Config priority:
  1. ANT_BROWSER_WORKSPACE_INSTALL_ROOT
  2. WORKSPACE_INSTALL_ROOT
  3. workspace-install-root argument
  4. $HOME/Codex/1688shopManager/desktop-repos/1688shop-desktop
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "${1}" in
      stable|live)
        DEV_MODE="${1}"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        if [[ -n "${INSTALL_ROOT_ARG}" ]]; then
          printf '[ERROR] Unsupported extra argument: %s\n' "${1}" >&2
          usage >&2
          exit 1
        fi
        INSTALL_ROOT_ARG="${1}"
        shift
        ;;
    esac
  done
}

resolve_install_root() {
  if [[ -n "${ANT_BROWSER_WORKSPACE_INSTALL_ROOT:-}" ]]; then
    printf '%s\n' "${ANT_BROWSER_WORKSPACE_INSTALL_ROOT}"
    return 0
  fi

  if [[ -n "${WORKSPACE_INSTALL_ROOT:-}" ]]; then
    printf '%s\n' "${WORKSPACE_INSTALL_ROOT}"
    return 0
  fi

  if [[ -n "${INSTALL_ROOT_ARG}" ]]; then
    printf '%s\n' "${INSTALL_ROOT_ARG}"
    return 0
  fi

  if [[ -d "${DEFAULT_WORKSPACE_INSTALL_ROOT}" ]]; then
    printf '%s\n' "${DEFAULT_WORKSPACE_INSTALL_ROOT}"
    return 0
  fi

  return 1
}

wait_for_frontend_dev_server() {
  local port="${1}"
  local attempt

  for attempt in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${port}/" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  printf '[ERROR] Frontend dev server did not become ready on port %s\n' "${port}" >&2
  return 1
}

wait_for_http_health() {
  local url="${1}"
  local attempt

  for attempt in $(seq 1 60); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  printf '[ERROR] HTTP endpoint did not become ready: %s\n' "${url}" >&2
  return 1
}

resolve_wails_bin() {
  if [[ -n "${WAILS_BIN:-}" ]]; then
    printf '%s\n' "${WAILS_BIN}"
    return 0
  fi

  if command -v wails >/dev/null 2>&1; then
    command -v wails
    return 0
  fi

  local gopath_bin="${HOME}/go/bin/wails"
  if [[ -x "${gopath_bin}" ]]; then
    printf '%s\n' "${gopath_bin}"
    return 0
  fi

  return 1
}

workspace_server_root_looks_valid() {
  local candidate="${1}"
  [[ -n "${candidate}" ]] &&
    [[ -f "${candidate}/server/index.mjs" ]] &&
    [[ -f "${candidate}/package.json" ]]
}

resolve_workspace_server_root() {
  local candidate

  for candidate in \
    "${ANT_BROWSER_WORKSPACE_SERVER_ROOT:-}" \
    "${WORKSPACE_SERVER_ROOT:-}" \
    "$(cd "${INSTALL_ROOT}/../.." >/dev/null 2>&1 && pwd || true)"; do
    if workspace_server_root_looks_valid "${candidate}"; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  return 1
}

ensure_workspace_server() {
  if curl -fsS "${DEFAULT_WORKSPACE_SERVER_URL}" >/dev/null 2>&1; then
    printf 'Workspace server: already healthy on %s\n' "${DEFAULT_WORKSPACE_SERVER_URL}"
    return 0
  fi

  local workspace_server_root
  if ! workspace_server_root="$(resolve_workspace_server_root)"; then
    printf '[WARN] Workspace server is not reachable at %s and no local server root was found.\n' "${DEFAULT_WORKSPACE_SERVER_URL}" >&2
    printf '       Set ANT_BROWSER_WORKSPACE_SERVER_ROOT or start the server manually with `npm run server`.\n' >&2
    return 0
  fi

  printf 'Workspace server: starting from %s\n' "${workspace_server_root}"
  (
    cd "${workspace_server_root}"
    npm run server
  ) &
  WORKSPACE_SERVER_PID="$!"

  wait_for_http_health "${DEFAULT_WORKSPACE_SERVER_URL}"
}

parse_args "$@"

if ! INSTALL_ROOT="$(resolve_install_root)"; then
  usage
  exit 1
fi

AGENT_ENTRY="${INSTALL_ROOT}/apps/agent/src/server/index.mjs"
if [[ ! -f "${AGENT_ENTRY}" ]]; then
  printf '[ERROR] Invalid workspace install root: %s\n' "${INSTALL_ROOT}" >&2
  printf '        Missing: %s\n' "${AGENT_ENTRY}" >&2
  exit 1
fi

if ! WAILS_BIN_PATH="$(resolve_wails_bin)"; then
  printf '[ERROR] Cannot find Wails CLI. Set WAILS_BIN or install wails into PATH.\n' >&2
  exit 1
fi

export ANT_BROWSER_WORKSPACE_INSTALL_ROOT="${INSTALL_ROOT}"
export ANT_BROWSER_DEBUG_STARTUP="${ANT_BROWSER_DEBUG_STARTUP:-1}"

printf '== Maka Browser mac dev ==\n'
printf 'Repo root: %s\n' "${REPO_ROOT}"
printf 'Mode: %s\n' "${DEV_MODE}"
printf 'Workspace install root: %s\n' "${ANT_BROWSER_WORKSPACE_INSTALL_ROOT}"
printf 'Wails CLI: %s\n' "${WAILS_BIN_PATH}"
printf '\n'

cd "${REPO_ROOT}"

ensure_workspace_server

if [[ "${DEV_MODE}" == "stable" ]]; then
  npm --prefix frontend run build
  exec "${WAILS_BIN_PATH}" dev -m -nogorebuild -noreload -s -skipbindings -assetdir frontend/dist
fi

if [[ "${DEV_MODE}" == "live" ]]; then
  export FRONTEND_PORT="${FRONTEND_PORT:-${DEFAULT_FRONTEND_PORT}}"
  printf 'Frontend port: %s\n' "${FRONTEND_PORT}"
  printf '\n'

  npm --prefix frontend run dev &
  WATCHER_PID="$!"

  wait_for_frontend_dev_server "${FRONTEND_PORT}"
  exec "${WAILS_BIN_PATH}" dev -m -s -skipbindings -frontenddevserverurl "http://127.0.0.1:${FRONTEND_PORT}" -viteservertimeout 60
fi

printf '[ERROR] Unsupported dev mode: %s\n' "${DEV_MODE}" >&2
exit 1
