#!/usr/bin/env python3
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PUBLISH_SCRIPT = ROOT / "bat" / "publish.ps1"
WORKFLOW = ROOT / ".github" / "workflows" / "windows-release-factory.yml"
PREFLIGHT = ROOT / "tools" / "release" / "windows-release-preflight.ps1"
RUNNER_SETUP = ROOT / "docs" / "release" / "windows-self-hosted-runner-setup.md"
INSTALLER = ROOT / "publish" / "installer.nsi"


def require_contains(path: Path, needle: str) -> None:
    text = path.read_text(encoding="utf-8-sig")
    if needle not in text:
        raise AssertionError(f"{path.relative_to(ROOT)} missing expected text: {needle}")


def main() -> None:
    require_contains(PUBLISH_SCRIPT, "ANT_BROWSER_WINDOWS_CHROME_ROOT")
    require_contains(PUBLISH_SCRIPT, "ANT_BROWSER_REQUIRE_WINDOWS_CHROME")
    require_contains(PUBLISH_SCRIPT, "C:\\AntBrowserReleaseResources\\chrome")
    require_contains(PUBLISH_SCRIPT, "GITHUB_ACTIONS")
    require_contains(PUBLISH_SCRIPT, "Resolve-WindowsChromeRoot")
    require_contains(PUBLISH_SCRIPT, "Resolve-WindowsChromeRequirement")
    require_contains(PUBLISH_SCRIPT, "Resolve-WindowsWorkspaceAgentRoot")
    require_contains(PUBLISH_SCRIPT, "Resolve-WindowsWorkspaceNodeRoot")
    require_contains(PUBLISH_SCRIPT, "ANT_BROWSER_WORKSPACE_AGENT_ROOT")
    require_contains(PUBLISH_SCRIPT, "ANT_BROWSER_WORKSPACE_NODE_ROOT")
    require_contains(PUBLISH_SCRIPT, "C:\\AntBrowserReleaseResources\\apps\\agent")
    require_contains(PUBLISH_SCRIPT, "C:\\AntBrowserReleaseResources\\runtime\\node")
    require_contains(PUBLISH_SCRIPT, "src\\server\\index.mjs")
    require_contains(PUBLISH_SCRIPT, "node.exe")
    require_contains(PUBLISH_SCRIPT, "缺少可打包的 Windows 浏览器内核")
    require_contains(PUBLISH_SCRIPT, "throw $message")
    require_contains(PUBLISH_SCRIPT, "publish/runtime-manifest.json")
    require_contains(PUBLISH_SCRIPT, "复制运行时清单")
    require_contains(PREFLIGHT, "Check Windows browser core")
    require_contains(PREFLIGHT, "Check bundled workspace agent payload")
    require_contains(PREFLIGHT, "C:\\AntBrowserReleaseResources\\chrome")
    require_contains(PREFLIGHT, "C:\\AntBrowserReleaseResources\\apps\\agent")
    require_contains(PREFLIGHT, "C:\\AntBrowserReleaseResources\\runtime\\node")
    require_contains(PREFLIGHT, "chrome.exe")
    require_contains(RUNNER_SETUP, "ANT_BROWSER_WINDOWS_CHROME_ROOT")
    require_contains(RUNNER_SETUP, "ANT_BROWSER_REQUIRE_WINDOWS_CHROME")
    require_contains(RUNNER_SETUP, "ANT_BROWSER_WORKSPACE_AGENT_ROOT")
    require_contains(RUNNER_SETUP, "ANT_BROWSER_WORKSPACE_NODE_ROOT")
    require_contains(WORKFLOW, "runs-on: [self-hosted, windows, ant-browser-release]")
    require_contains(INSTALLER, '!define PRODUCT_NAME    "Maka Browser"')
    require_contains(INSTALLER, '!define UNINSTALL_KEY   "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\AntBrowser"')
    require_contains(INSTALLER, '!define INSTALL_DIR     "$LOCALAPPDATA\\Programs\\Ant Browser"')
    require_contains(INSTALLER, 'RMDir /r "$SMPROGRAMS\\Ant Browser"')
    require_contains(INSTALLER, 'Delete /REBOOTOK "$DESKTOP\\Ant Browser.lnk"')
    print("[OK] windows publish script chrome resource contract verified")


if __name__ == "__main__":
    main()
