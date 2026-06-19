#!/usr/bin/env python3

from __future__ import annotations

import json
import sys
from pathlib import Path


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def load_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"missing required file: {path}")
    except json.JSONDecodeError as exc:
        fail(f"invalid json in {path}: {exc}")


def assert_contains(text: str, needle: str, context: str) -> None:
    if needle not in text:
        fail(f"{context} is missing required text: {needle}")


def collect_required_packages(manifest: dict, target: str) -> list[dict]:
    packages = manifest.get("packages") or []
    normalized = target.strip().lower()
    return [
        pkg
        for pkg in packages
        if bool(pkg.get("required")) and str(pkg.get("target") or "").strip().lower() == normalized
    ]


def main() -> None:
    repo_root = Path(__file__).resolve().parents[2]

    manifest_path = repo_root / "publish" / "runtime-manifest.json"
    sources_path = repo_root / "publish" / "runtime-sources.json"
    installer_path = repo_root / "publish" / "installer.nsi"
    windows_publish_path = repo_root / "bat" / "publish.ps1"
    mac_publish_path = repo_root / "publish" / "mac" / "publish-mac.sh"
    release_readme_path = repo_root / "tools" / "public-release" / "README.md"
    windows_e2e_path = repo_root / "tools" / "app-update" / "windows-app-update-e2e.ps1"
    windows_e2e_verifier_path = repo_root / "tools" / "app-update" / "verify-windows-e2e-script.py"

    manifest = load_json(manifest_path)
    sources = load_json(sources_path)

    if int(manifest.get("schemaVersion") or 0) != 2:
        fail("publish/runtime-manifest.json must use schemaVersion 2")
    if int(sources.get("schemaVersion") or 0) != 2:
        fail("publish/runtime-sources.json must use schemaVersion 2")

    for target in ("windows-amd64", "darwin-amd64", "darwin-arm64"):
        packages = collect_required_packages(manifest, target)
        if not packages:
            fail(f"runtime manifest is missing required packages for {target}")

    installer_text = installer_path.read_text(encoding="utf-8-sig")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\publish\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'SetOutPath "$INSTDIR\\publish"', "publish/installer.nsi")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\apps\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\runtime\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'CreateDirectory "$INSTDIR\\data"', "publish/installer.nsi")
    assert_contains(installer_text, '!define INSTALL_DIR     "$LOCALAPPDATA\\Programs\\Maka Browser"', "publish/installer.nsi")
    assert_contains(installer_text, "RequestExecutionLevel user", "publish/installer.nsi")
    assert_contains(installer_text, 'InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"', "publish/installer.nsi")

    windows_publish_text = windows_publish_path.read_text(encoding="utf-8-sig")
    assert_contains(windows_publish_text, 'Copy-Item -LiteralPath $runtimeManifestSource -Destination (Join-Path $stagingPublishDir "runtime-manifest.json") -Force', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-WindowsChromePayload -ChromeRoot $chromeRoot -StagingDir $stagingDir', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-RequiredDirectoryContents `', "bat/publish.ps1")
    assert_contains(windows_publish_text, '-DisplayName "workspace agent 运行时"', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-WindowsWorkspaceNodeRuntime `', "bat/publish.ps1")
    assert_contains(windows_publish_text, '$zipName = "MakaBrowser-$script:Version-windows-amd64.zip"', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'product = "Maka Browser"', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'New-WindowsAppUpdateArtifacts -StagingDir $stagingDir', "bat/publish.ps1")

    windows_e2e_text = windows_e2e_path.read_text(encoding="utf-8-sig")
    assert_contains(windows_e2e_text, "CurrentExePath: currentExe", "tools/app-update/windows-app-update-e2e.ps1")
    assert_contains(windows_e2e_text, "DESKTOP_APP_UPDATE_MANIFEST_URL", "tools/app-update/windows-app-update-e2e.ps1")
    assert_contains(windows_e2e_text, "localAppVersion", "tools/app-update/windows-app-update-e2e.ps1")
    assert_contains(windows_e2e_text, "data\\app.db", "tools/app-update/windows-app-update-e2e.ps1")

    windows_e2e_verifier_text = windows_e2e_verifier_path.read_text(encoding="utf-8")
    assert_contains(windows_e2e_verifier_text, "Windows app-update e2e script contract verified", "tools/app-update/verify-windows-e2e-script.py")

    mac_publish_text = mac_publish_path.read_text(encoding="utf-8")
    assert_contains(mac_publish_text, 'cp "$ROOT_DIR/publish/runtime-manifest.json" "$APP_PUBLISH_DIR/runtime-manifest.json"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'cp "$XRAY_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/xray"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'cp "$SINGBOX_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/sing-box"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'cp "$NODE_RUNTIME_SRC" "$APP_MACOS_DIR/runtime/node/bin/node"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'ditto "$ROOT_DIR/apps/agent" "$APP_MACOS_DIR/apps/agent"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'APP_UPDATE_ZIP_NAME="MakaBrowser-${VERSION}-${TARGET}.zip"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'tools/app-update/verify-app-update-package.py', "publish/mac/publish-mac.sh")

    mac_config_text = (repo_root / "publish" / "config.init.mac.yaml").read_text(encoding="utf-8")
    assert_contains(mac_config_text, "app_update_manifest_url:", "publish/config.init.mac.yaml")

    readme_text = release_readme_path.read_text(encoding="utf-8")
    assert_contains(readme_text, "packaged builds must include `publish/runtime-manifest.json`", "tools/public-release/README.md")
    assert_contains(readme_text, "`runtime/current.json` is created in the writable state root", "tools/public-release/README.md")

    print("[OK] publish contract verified")


if __name__ == "__main__":
    main()
