#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="$ROOT_DIR/publish/output"
SSH_TARGET=""
SSH_PORT=""
REMOTE_ROOT="/opt/1688shop"
CHANNEL="stable"
TARGET="windows-amd64"
VERSION=""
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage:
  scripts/upload-local-release.sh --ssh <user@host> --version <version> [options]

Options:
  --ssh <target>              SSH target used by rsync, for example ubuntu@1.2.3.4
  --ssh-port <port>           SSH port, for example 2222 for JumpServer
  --version <version>         Release version, for example 1.1.5
  --target <target>           Package target: windows-amd64, darwin-arm64, darwin-amd64 (default: windows-amd64)
  --channel <channel>         Release channel directory under releases/<platform>/ (default: stable)
  --remote-root <path>        Remote deployment root (default: /opt/1688shop)
  --dry-run                   Print rsync changes without uploading
  -h, --help                  Show help

Examples:
  scripts/upload-local-release.sh --ssh ubuntu@192.168.210.169 --version 1.1.5
  scripts/upload-local-release.sh --ssh JMS-xxx@jump.example.com --ssh-port 2222 --version 1.1.5
  scripts/upload-local-release.sh --ssh ubuntu@192.168.210.169 --version 1.1.5 --target windows-amd64 --channel test
EOF
}

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ssh)
      SSH_TARGET="${2:-}"
      shift 2
      ;;
    --ssh-port)
      SSH_PORT="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --target)
      TARGET="${2:-}"
      shift 2
      ;;
    --channel)
      CHANNEL="${2:-}"
      shift 2
      ;;
    --remote-root)
      REMOTE_ROOT="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

[[ -n "$SSH_TARGET" ]] || fail "--ssh is required"
[[ -n "$VERSION" ]] || fail "--version is required"
[[ "$CHANNEL" =~ ^[A-Za-z0-9._-]+$ ]] || fail "Invalid --channel: $CHANNEL"
if [[ -n "$SSH_PORT" && ! "$SSH_PORT" =~ ^[0-9]+$ ]]; then
  fail "Invalid --ssh-port: $SSH_PORT"
fi

case "$TARGET" in
  windows-amd64)
    PLATFORM="windows"
    UPDATE_ZIP="$OUTPUT_DIR/MakaBrowser-${VERSION}-windows-amd64.zip"
    INSTALLER="$OUTPUT_DIR/MakaBrowser-Setup-${VERSION}.exe"
    EXTRA_FILES=("$INSTALLER")
    ;;
  darwin-arm64)
    PLATFORM="mac"
    UPDATE_ZIP="$OUTPUT_DIR/MakaBrowser-${VERSION}-darwin-arm64.zip"
    EXTRA_FILES=("$OUTPUT_DIR/MakaBrowser-${VERSION}-macos-arm64.app")
    ;;
  darwin-amd64)
    PLATFORM="mac"
    UPDATE_ZIP="$OUTPUT_DIR/MakaBrowser-${VERSION}-darwin-amd64.zip"
    EXTRA_FILES=("$OUTPUT_DIR/MakaBrowser-${VERSION}-macos-amd64.app")
    ;;
  *)
    fail "Unsupported --target: $TARGET"
    ;;
esac

MANIFEST="$OUTPUT_DIR/app-update-stable.json"
FILES=(
  "$MANIFEST"
  "$MANIFEST.sha256"
  "$UPDATE_ZIP"
  "$UPDATE_ZIP.sha256"
)

for file in "${EXTRA_FILES[@]}"; do
  [[ -e "$file" ]] && FILES+=("$file")
done

for file in "${FILES[@]}"; do
  [[ -e "$file" ]] || fail "Missing local artifact: $file"
done

REMOTE_RELEASE_DIR="${REMOTE_ROOT%/}/releases/$PLATFORM/$CHANNEL"
RSYNC_ARGS=(-av --chmod=F644,D755)
SSH_ARGS=()
if [[ -n "$SSH_PORT" ]]; then
  SSH_ARGS=(-p "$SSH_PORT")
  RSYNC_ARGS+=(-e "ssh -p $SSH_PORT")
fi
if [[ "$DRY_RUN" -eq 1 ]]; then
  RSYNC_ARGS+=(--dry-run)
fi

ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "mkdir -p '$REMOTE_RELEASE_DIR'"
rsync "${RSYNC_ARGS[@]}" "${FILES[@]}" "$SSH_TARGET:$REMOTE_RELEASE_DIR/"

echo "[OK] uploaded $TARGET $VERSION artifacts to $SSH_TARGET:$REMOTE_RELEASE_DIR"
echo "[INFO] manifest URL: http://<server-ip>:18080/releases/$PLATFORM/$CHANNEL/app-update-stable.json"
