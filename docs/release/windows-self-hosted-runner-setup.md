# Windows Self-Hosted Runner Setup

This machine builds and validates Maka Browser Windows releases. It replaces the
manual XiaoQ packaging loop with a repeatable GitHub Actions release path that
can compile, package, smoke-check, and publish Windows artifacts from a
controlled runner host.

## Runner Labels

Register the runner with all of these labels:

- `self-hosted`
- `windows`
- `ant-browser-release`

Release workflows should target:

```yaml
runs-on: [self-hosted, windows, ant-browser-release]
```

## Required Tools

Install and verify these tools on the runner before registration:

- Git
- Go
- Node.js / npm
- Python 3
- Wails CLI
- NSIS / `makensis.exe`
- PowerShell
- GitHub Actions runner service

Recommended quick checks:

```powershell
git --version
go version
node --version
npm --version
python --version
wails version
makensis.exe /VERSION
pwsh --version
```

## Required Release Resources

The release runner must have the Windows fingerprint browser core available
outside the git worktree. Do not commit the browser core into this repository.

The bundled workspace agent source is part of this repository under
`apps/agent`. The bundled Node.js runtime is copied from
`ANT_BROWSER_WORKSPACE_NODE_ROOT` when configured, or from the runner's
`node.exe` on `PATH`.

Recommended layout:

```powershell
C:\AntBrowserReleaseResources\chrome\<core-name>\chrome.exe
C:\AntBrowserReleaseResources\runtime\node\node.exe   # optional override
```

Register these paths as GitHub repository variables when using a non-default
layout:

```text
ANT_BROWSER_WINDOWS_CHROME_ROOT=C:\AntBrowserReleaseResources\chrome
ANT_BROWSER_WORKSPACE_AGENT_ROOT=C:\custom\apps\agent      # optional
ANT_BROWSER_WORKSPACE_NODE_ROOT=C:\custom\runtime\node     # optional
```

If `ANT_BROWSER_WINDOWS_CHROME_ROOT` is not set, the release script uses
`C:\AntBrowserReleaseResources\chrome` by default while running in GitHub
Actions. If the workspace agent variable is not set, the release script uses
the checked-out `apps/agent` source. If the Node.js variable is not set, the
release script first checks `runtime\node`, then
`C:\AntBrowserReleaseResources\runtime\node`, then falls back to copying
`node.exe` from `PATH`.

The Windows release script treats GitHub Actions builds as if this flag is set:

```text
ANT_BROWSER_REQUIRE_WINDOWS_CHROME=1
```

With that gate enabled, packaging fails if no valid Windows `chrome.exe` is
found under `ANT_BROWSER_WINDOWS_CHROME_ROOT`. This prevents producing a green
release that cannot open managed 1688 shop windows with the packaged Ant
fingerprint core.

Packaging also fails if the bundled workspace agent entry or Node.js runtime is
missing. This prevents producing an installer that starts but blocks at
`ENV-WORKSPACE-AGENT-PAYLOAD-MISSING`.

## Required Network Access

The runner must reach the internal server and static update host used by release
validation:

- `http://192.168.210.169:4174/api/health`
- `http://192.168.210.169:4174/api/client/health`
- `http://192.168.210.169:18080/healthz`

Preflight checks should fail fast when any endpoint is unreachable.

## Runner User

Use a dedicated Windows user for the GitHub Actions runner service. Do not reuse
a personal desktop account.

The runner user must be able to:

- Install Maka Browser to `%LOCALAPPDATA%\Programs\Maka Browser`
- Write Maka Browser runtime data under `%LOCALAPPDATA%\Maka Browser`
- Write workspace agent state under `%ProgramData%\1688shop-agent`
- Stop stale release-test processes:
  - `ant-chrome.exe`
  - `xray.exe`
  - `sing-box.exe`

Grant only the permissions needed for packaging and release validation. Avoid
turning the runner into a general-purpose administrator workstation.

## Secrets

Never store Robbin's jump password on the runner, in GitHub Actions, or in this
repository.

Use a deploy-only SSH key for release upload or server-side release operations.
Store the connection details as GitHub Actions secrets:

- `WINDOWS_RELEASE_SSH_HOST`
- `WINDOWS_RELEASE_SSH_PORT`
- `WINDOWS_RELEASE_SSH_USER`
- `WINDOWS_RELEASE_SSH_KEY`

The private key should be scoped to release deployment only and should be
rotatable without affecting personal SSH access.

## Registration

Download the GitHub Actions runner package from GitHub, extract it into a stable
directory owned by the dedicated runner user, then register it with the release
labels:

```powershell
config.cmd --url <repo-url> --token <registration-token> --labels self-hosted,windows,ant-browser-release
```

Install and start the runner as a Windows service:

```powershell
svc install
svc start
```

After startup, confirm the runner appears online in GitHub with the labels
`self-hosted`, `windows`, and `ant-browser-release`.

## Local Preflight

Before enabling release workflows, run the local Windows release preflight from
the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File tools\release\windows-release-preflight.ps1
```

The preflight should verify toolchain availability, internal endpoint access,
write permissions, and cleanup permissions for stale Maka Browser release-test
processes.
