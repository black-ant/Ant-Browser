#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import sys
import zipfile
from pathlib import Path


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def normalized_zip_names(zip_path: Path) -> set[str]:
    with zipfile.ZipFile(zip_path) as zf:
        return {name.replace("\\", "/").lstrip("./") for name in zf.namelist()}


def required_paths_for_target(target: str) -> set[str]:
    if target == "windows-amd64":
        return {
            "ant-chrome.exe",
            "publish/runtime-manifest.json",
            "bin/xray.exe",
            "bin/sing-box.exe",
            "apps/agent/src/server/index.mjs",
            "runtime/node/node.exe",
        }
    if target in {"darwin-arm64", "darwin-amd64"}:
        return {
            "Ant Browser.app/Contents/Info.plist",
            "Ant Browser.app/Contents/MacOS/ant-chrome",
            "Ant Browser.app/Contents/MacOS/publish/runtime-manifest.json",
            "Ant Browser.app/Contents/MacOS/bin/xray",
            "Ant Browser.app/Contents/MacOS/bin/sing-box",
        }
    fail(f"unsupported target: {target}")


def forbidden_mutable_user_data(names: set[str], target: str) -> list[str]:
    forbidden = []
    for name in names:
        lower = name.lower()
        segments = [segment for segment in lower.split("/") if segment]
        if (
            lower == "data/"
            or lower.startswith("data/")
            or "user data" in segments
            or lower.endswith(".db")
            or lower.endswith(".sqlite")
            or lower.endswith(".sqlite3")
        ):
            forbidden.append(name)
            continue
        if target in {"darwin-arm64", "darwin-amd64"} and any(
            lower == f"{app_root}/contents/macos/data/"
            or lower.startswith(f"{app_root}/contents/macos/data/")
            for app_root in ("ant browser.app", "maka browser.app")
        ):
            forbidden.append(name)
    return sorted(forbidden)


def main() -> None:
    if len(sys.argv) != 4:
        fail("usage: verify-app-update-package.py <manifest> <zip> <target>")

    manifest_path = Path(sys.argv[1])
    zip_path = Path(sys.argv[2])
    target = sys.argv[3].strip().lower()

    if not manifest_path.is_file():
        fail(f"manifest not found: {manifest_path}")
    if not zip_path.is_file():
        fail(f"zip not found: {zip_path}")

    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schemaVersion") != 1:
        fail("app-update manifest schemaVersion must be 1")

    packages = manifest.get("packages") or []
    package = next((p for p in packages if str(p.get("target", "")).strip().lower() == target), None)
    if package is None:
        fail(f"manifest missing package for {target}")
    if package.get("payloadType") != "full":
        fail("Phase 1 package payloadType must be full")

    expected_hash = str(package.get("sha256") or "").strip().lower()
    actual_hash = sha256(zip_path)
    if expected_hash != actual_hash:
        fail(f"sha256 mismatch: expected {expected_hash}, got {actual_hash}")

    expected_size = int(package.get("size") or 0)
    actual_size = zip_path.stat().st_size
    if expected_size > 0 and expected_size != actual_size:
        fail(f"size mismatch: expected {expected_size}, got {actual_size}")

    names = normalized_zip_names(zip_path)
    required = required_paths_for_target(target)
    missing = sorted(required - names)
    if missing:
        fail("zip missing required files: " + ", ".join(missing))

    forbidden = forbidden_mutable_user_data(names, target)
    if forbidden:
        fail("zip contains mutable user data: " + ", ".join(sorted(forbidden)[:10]))

    print("[OK] app update package verified")


if __name__ == "__main__":
    main()
