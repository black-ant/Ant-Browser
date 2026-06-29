#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_DIR="$ROOT_DIR/publish/output"
STAGING_ROOT="$ROOT_DIR/publish/staging/mac"
ARCH=""
VERSION=""
SKIP_BUILD=0
SKIP_RUNTIME_VERIFY=0
KEEP_STAGING=0

usage() {
  cat <<'EOF'
Usage:
  publish/mac/publish-mac.sh --arch <arm64|amd64> [options]

Options:
  --arch <arm64|amd64>   Target architecture (required)
  --version <ver>        Package version (default: read from wails.json)
  --skip-build           Skip frontend and Wails build steps
  --skip-runtime-verify  Skip runtime hash verification
  --keep-staging         Keep assembled .app bundle in publish/staging/mac
  -h, --help             Show help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --arch)
      ARCH="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    --skip-runtime-verify)
      SKIP_RUNTIME_VERIFY=1
      shift
      ;;
    --keep-staging)
      KEEP_STAGING=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[ERROR] Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$ARCH" ]]; then
  echo "[ERROR] --arch is required" >&2
  usage
  exit 1
fi

if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
  echo "[ERROR] unsupported arch: $ARCH (expected amd64 or arm64)" >&2
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "[ERROR] this script must run on macOS host" >&2
  exit 1
fi

host_arch_raw="$(uname -m)"
case "$host_arch_raw" in
  x86_64) HOST_ARCH="amd64" ;;
  arm64) HOST_ARCH="arm64" ;;
  *)
    echo "[ERROR] unsupported host architecture: $host_arch_raw" >&2
    exit 1
    ;;
esac

if [[ "$HOST_ARCH" != "$ARCH" ]]; then
  echo "[ERROR] host arch is $HOST_ARCH but target arch is $ARCH." >&2
  echo "        Build the first macOS package on a native runner for the same architecture." >&2
  exit 1
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] required command not found: $1" >&2
    exit 1
  fi
}

require_cmd python3
require_cmd curl
require_cmd codesign
require_cmd ditto
require_cmd hdiutil
require_cmd wails
require_cmd xattr

NODE_RUNTIME_SRC="${MAKA_BROWSER_NODE_RUNTIME_SRC:-${NODE_RUNTIME_SRC:-}}"
if [[ -z "$NODE_RUNTIME_SRC" ]] && command -v node >/dev/null 2>&1; then
  NODE_RUNTIME_SRC="$(command -v node)"
fi
if [[ -z "$NODE_RUNTIME_SRC" || ! -f "$NODE_RUNTIME_SRC" ]]; then
  echo "[ERROR] Node runtime source missing. Set MAKA_BROWSER_NODE_RUNTIME_SRC to a portable macOS node binary." >&2
  exit 1
fi

echo "[0/4] Verifying publish contract..."
python3 "$ROOT_DIR/tools/runtime/verify-publish-contract.py"
echo

if [[ -z "$VERSION" ]]; then
  VERSION="$(python3 - "$ROOT_DIR/wails.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)
version = (((data or {}).get("info") or {}).get("productVersion") or "").strip()
if not version:
    raise SystemExit("productVersion missing in wails.json")
print(version)
PY
)"
fi

TARGET="darwin-$ARCH"
RUNTIME_DIR="$ROOT_DIR/bin/$TARGET"
XRAY_SRC="$RUNTIME_DIR/xray"
SINGBOX_SRC="$RUNTIME_DIR/sing-box"
APP_BIN_DIR="$ROOT_DIR/build/bin"
CHROME_README_SRC="$ROOT_DIR/chrome/README.md"
CONFIG_INIT_SRC="$ROOT_DIR/publish/config.init.mac.yaml"
FINGERPRINT_CORE_TAG="142.0.7444.175"
FINGERPRINT_CORE_ASSET="ungoogled-chromium_142.0.7444.175-1.1_macos.dmg"
FINGERPRINT_CORE_URL="https://github.com/adryfish/fingerprint-chromium/releases/download/${FINGERPRINT_CORE_TAG}/${FINGERPRINT_CORE_ASSET}"
ZIP_NAME="MakaBrowser-${VERSION}-macos-${ARCH}.zip"
APP_UPDATE_ZIP_NAME="MakaBrowser-${VERSION}-${TARGET}.zip"
APP_EXPORT="$OUTPUT_DIR/MakaBrowser-${VERSION}-macos-${ARCH}.app"
STAGE_DIR="$STAGING_ROOT/$TARGET"
APP_STAGE="$STAGE_DIR/Ant Browser.app"
APP_UPDATE_MANIFEST="$OUTPUT_DIR/app-update-stable.json"

find_built_app_bundle() {
  python3 - "$APP_BIN_DIR" <<'PY'
from pathlib import Path
import sys

root = Path(sys.argv[1])
if not root.is_dir():
    sys.exit(0)

candidates = [p for p in root.iterdir() if p.is_dir() and p.suffix == ".app"]
if not candidates:
    sys.exit(0)

candidates.sort(key=lambda p: p.stat().st_mtime, reverse=True)
print(candidates[0])
PY
}

manifest_has_target() {
  python3 - "$ROOT_DIR/publish/runtime-manifest.json" "$TARGET" <<'PY'
import json
import sys

manifest_path = sys.argv[1]
target = sys.argv[2]

with open(manifest_path, "r", encoding="utf-8") as f:
    data = json.load(f)

for item in data.get("packages", []):
    if item.get("required") and (item.get("target") or "").strip().lower() == target.lower():
        print("yes")
        raise SystemExit(0)

for item in data.get("files", []):
    if target in (item.get("targets") or []):
        print("yes")
        raise SystemExit(0)

raise SystemExit(1)
PY
}

write_app_update_manifest() {
  local zip_path="$1"
  python3 - "$APP_UPDATE_MANIFEST" "$zip_path" "$TARGET" "$VERSION" <<'PY'
from __future__ import annotations

import hashlib
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

manifest_path = Path(sys.argv[1])
zip_path = Path(sys.argv[2])
target = sys.argv[3]
version = sys.argv[4]

if manifest_path.exists():
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise SystemExit(f"invalid existing app-update manifest: {exc}") from exc
    if int(manifest.get("schemaVersion") or 0) != 1:
        raise SystemExit("existing app-update manifest schemaVersion must be 1")
else:
    manifest = {
        "schemaVersion": 1,
        "channel": "stable",
        "packages": [],
    }

manifest["schemaVersion"] = 1
manifest["channel"] = manifest.get("channel") or "stable"
manifest["version"] = version
manifest["minimumRuntimeResourceVersion"] = version
manifest["minimumAppVersion"] = version
manifest["publishedAt"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
notes = str(manifest.get("notes") or "").strip()
manifest["notes"] = notes if notes and "Ant Browser" not in notes else f"Maka Browser {version}"

digest = hashlib.sha256(zip_path.read_bytes()).hexdigest()
package = {
    "target": target,
    "payloadType": "full",
    "url": zip_path.name,
    "sha256": digest,
    "size": zip_path.stat().st_size,
}

packages = [
    item
    for item in (manifest.get("packages") or [])
    if str(item.get("target") or "").strip().lower() != target.lower()
]
packages.append(package)
packages.sort(key=lambda item: str(item.get("target") or ""))
manifest["packages"] = packages

manifest_path.parent.mkdir(parents=True, exist_ok=True)
manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

manifest_hash = hashlib.sha256(manifest_path.read_bytes()).hexdigest()
(manifest_path.with_suffix(manifest_path.suffix + ".sha256")).write_text(
    f"{manifest_hash}  {manifest_path.name}\n",
    encoding="utf-8",
)
(zip_path.with_suffix(zip_path.suffix + ".sha256")).write_text(
    f"{digest}  {zip_path.name}\n",
    encoding="utf-8",
)
PY
}

echo "========================================"
echo "  Maka Browser macOS Publish"
echo "========================================"
echo "Target : $TARGET"
echo "Version: $VERSION"
echo "Root   : $ROOT_DIR"
echo

if [[ ! -f "$XRAY_SRC" || ! -f "$SINGBOX_SRC" ]]; then
  echo "[ERROR] runtime files missing for $TARGET" >&2
  echo "        expected: $XRAY_SRC and $SINGBOX_SRC" >&2
  exit 1
fi

if [[ ! -f "$CONFIG_INIT_SRC" ]]; then
  echo "[ERROR] mac config template missing: $CONFIG_INIT_SRC" >&2
  exit 1
fi

if [[ "$SKIP_RUNTIME_VERIFY" -ne 1 ]]; then
  if manifest_has_target >/dev/null 2>&1; then
    bash "$ROOT_DIR/tools/runtime/verify-runtime.sh" "$TARGET"
  else
    echo "[WARN] runtime manifest does not yet define $TARGET, skipping hash verification"
  fi
else
  echo "[WARN] runtime verification skipped"
fi

if [[ "$SKIP_BUILD" -ne 1 ]]; then
  echo "[1/4] Installing frontend dependencies..."
  (cd "$ROOT_DIR/frontend" && npm ci --prefer-offline --no-audit --no-fund)

  echo "[2/4] Building frontend assets..."
  (cd "$ROOT_DIR/frontend" && npm run build)

  echo "[3/4] Building macOS app bundle with Wails..."
  (
    cd "$ROOT_DIR"
    wails build -s -platform "darwin/$ARCH" -o ant-chrome
  )
else
  echo "[WARN] skipping build step"
fi

APP_SOURCE="$(find_built_app_bundle)"
if [[ -z "$APP_SOURCE" || ! -d "$APP_SOURCE" ]]; then
  echo "[ERROR] failed to locate built .app bundle under $APP_BIN_DIR" >&2
  exit 1
fi

echo "[4/4] Assembling macOS app bundle..."
rm -rf "$APP_STAGE" "$APP_EXPORT"
mkdir -p "$STAGE_DIR" "$OUTPUT_DIR"
ditto "$APP_SOURCE" "$APP_STAGE"

APP_MACOS_DIR="$APP_STAGE/Contents/MacOS"
if [[ ! -d "$APP_MACOS_DIR" ]]; then
  echo "[ERROR] invalid app bundle layout, missing: $APP_MACOS_DIR" >&2
  exit 1
fi

mkdir -p "$APP_MACOS_DIR/bin"
cp "$XRAY_SRC" "$APP_MACOS_DIR/bin/xray"
cp "$SINGBOX_SRC" "$APP_MACOS_DIR/bin/sing-box"
cp "$CONFIG_INIT_SRC" "$APP_MACOS_DIR/config.yaml"
chmod +x "$APP_MACOS_DIR/bin/xray" "$APP_MACOS_DIR/bin/sing-box"

mkdir -p "$APP_MACOS_DIR/runtime/node/bin" "$APP_MACOS_DIR/apps"
cp "$NODE_RUNTIME_SRC" "$APP_MACOS_DIR/runtime/node/bin/node"
chmod +x "$APP_MACOS_DIR/runtime/node/bin/node"
ditto "$ROOT_DIR/apps/agent" "$APP_MACOS_DIR/apps/agent"

APP_PUBLISH_DIR="$APP_MACOS_DIR/publish"
mkdir -p "$APP_PUBLISH_DIR/bin/$TARGET"
cp "$ROOT_DIR/publish/runtime-manifest.json" "$APP_PUBLISH_DIR/runtime-manifest.json"
if [[ -f "$ROOT_DIR/publish/runtime-sources.json" ]]; then
  cp "$ROOT_DIR/publish/runtime-sources.json" "$APP_PUBLISH_DIR/runtime-sources.json"
fi
cp "$XRAY_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/xray"
cp "$SINGBOX_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/sing-box"
chmod +x "$APP_PUBLISH_DIR/bin/$TARGET/xray" "$APP_PUBLISH_DIR/bin/$TARGET/sing-box"

echo "  - downloading fingerprint core ${FINGERPRINT_CORE_TAG}..."
CORE_TMP_DIR="$(mktemp -d)"
CORE_DMG="$CORE_TMP_DIR/fingerprint-core.dmg"
CORE_MOUNT="$CORE_TMP_DIR/mount"
mkdir -p "$CORE_MOUNT"
curl -L --fail --retry 3 --retry-delay 2 -o "$CORE_DMG" "$FINGERPRINT_CORE_URL"
hdiutil attach -nobrowse -readonly -mountpoint "$CORE_MOUNT" "$CORE_DMG" >/dev/null
CORE_APP="$(find "$CORE_MOUNT" -maxdepth 2 -name '*.app' -type d | head -n 1)"
if [[ -z "$CORE_APP" ]]; then
  hdiutil detach "$CORE_MOUNT" >/dev/null || true
  echo "[ERROR] fingerprint dmg missing .app bundle" >&2
  exit 1
fi
mkdir -p "$APP_MACOS_DIR/chrome/fingerprint-macos"
ditto "$CORE_APP" "$APP_MACOS_DIR/chrome/fingerprint-macos/Chromium.app"
hdiutil detach "$CORE_MOUNT" >/dev/null || true
rm -rf "$CORE_TMP_DIR"

if [[ -f "$CHROME_README_SRC" ]]; then
  mkdir -p "$APP_MACOS_DIR/chrome"
  cp "$CHROME_README_SRC" "$APP_MACOS_DIR/chrome/README.md"
fi

echo "  - normalizing bundle metadata..."
xattr -cr "$APP_STAGE"

echo "  - signing assembled app bundle..."
codesign --force --deep --sign - "$APP_STAGE"
codesign --verify --deep --strict --verbose=2 "$APP_STAGE"

ditto "$APP_STAGE" "$APP_EXPORT"
codesign --verify --deep --strict --verbose=2 "$APP_EXPORT"
rm -f "$OUTPUT_DIR/$ZIP_NAME"
ditto -c -k --sequesterRsrc --keepParent "$APP_EXPORT" "$OUTPUT_DIR/$ZIP_NAME"

APP_UPDATE_ZIP="$OUTPUT_DIR/$APP_UPDATE_ZIP_NAME"
rm -f "$APP_UPDATE_ZIP"
ditto -c -k --sequesterRsrc --keepParent "$APP_STAGE" "$APP_UPDATE_ZIP"
write_app_update_manifest "$APP_UPDATE_ZIP"
python3 "$ROOT_DIR/tools/app-update/verify-app-update-package.py" "$APP_UPDATE_MANIFEST" "$APP_UPDATE_ZIP" "$TARGET"

echo "Artifacts generated:"
echo "  - $APP_EXPORT"
echo "  - $OUTPUT_DIR/$ZIP_NAME"
echo "  - $APP_UPDATE_ZIP"
echo "  - $APP_UPDATE_MANIFEST"

if [[ "$KEEP_STAGING" -ne 1 ]]; then
  rm -rf "$APP_STAGE"
fi

echo "Done."
