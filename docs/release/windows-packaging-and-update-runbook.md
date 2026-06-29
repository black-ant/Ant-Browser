# Windows 发布 / 安装 / 更新回归 Runbook

## 小Q协作入口

Windows 智能体小Q执行发布验证时，先读：

- `docs/release/windows-agent-q-playbook.md`

该手册定义了小Q的职责边界、服务器地址、打包命令、app-update e2e、远端 4174 登录验收和标准汇报格式。

## 目标

这份 runbook 只服务内部发布、真机回归与排障，不面向终端用户。

当前 Windows 线分两层：

1. **安装包 / 首启稳定性**
   - 生成 `publish/output/MakaBrowser-Setup-<version>.exe`
   - 安装后首启 Gate、runtime pointer repair、workspace host 检查正常
2. **运行时更新**
   - 启动检查更新
   - `soft / required / manifest load fail` 三类场景可回归
3. **应用本体自更新**
   - 生成 `publish/output/app-update-stable.json`
   - 生成 `publish/output/MakaBrowser-<version>-windows-amd64.zip`
   - 客户端内执行下载、hash 校验、staging、runner 替换与重启

## 当前边界

**已完成：**

- runtime 更新检查与弹窗链路
- `runtime/current.json` 切换
- 更新失败在弹窗内展示完整错误
- 应用本体自更新
- 安装包级别的客户端下载、替换与重启
- 首个 Maka Browser 桥接版本继续使用
  `%LOCALAPPDATA%\Programs\Ant Browser` 和旧卸载注册表键，以保证现有
  Ant Browser 用户原位升级；产品名称和发布产物名称已经切换为 Maka Browser

结论：

- 现在可以做到“更新运行时资源”
- 现在也具备“更新客户端本体”的代码与发布产物链路
- 真机发布前仍必须跑完本 runbook 的应用本体自更新回归场景

## 发布前检查

Windows 打包前，操作者应先确认：

1. 当前分支已对齐目标发布分支
2. 工作区干净，或至少没有会污染发布产物的临时改动
3. 已安装：
   - `node`
   - `npm`
   - `go`
   - `wails`
   - `python3`
   - `NSIS / makensis.exe`
4. 仓库中以下文件存在：
   - `publish/runtime-manifest.json`
   - `publish/runtime-sources.json`
   - `bin/xray.exe`
   - `bin/sing-box.exe`
5. 如要把浏览器内核一并打包，`chrome/` 目录下存在可识别的 Windows `chrome.exe`

## Windows 打包命令

交互式：

```bat
bat\publish.bat
```

非交互：

```bat
bat\publish.bat W
bat\publish.bat W -Version 1.1.0
```

## 打包脚本实际会做什么

`bat/publish.ps1` 会按这个顺序执行：

1. 校验版本号
2. 检测 NSIS (`makensis.exe`)
3. 运行 `python3 tools/runtime/verify-publish-contract.py`
4. 校验 `publish/runtime-manifest.json` 对应的 runtime hash
5. 执行 `wails build`
6. 组装 `publish/staging/`
7. 调用 `publish/installer.nsi`
8. 输出 `publish/output/MakaBrowser-Setup-<version>.exe`
9. 生成应用本体更新 zip 与 manifest
10. 运行 `tools/app-update/verify-app-update-package.py`
11. 清理 `publish/staging/`

## Windows 打包成功判定

同时满足下面条件，才算打包成功：

1. `bat\publish.bat W` 退出码为 `0`
2. 控制台出现：
   - `发布契约校验通过`
   - `运行时哈希校验通过`
   - `Windows 安装包生成成功`
   - `应用本体更新包生成成功`
3. 产物存在：
   - `publish/output/MakaBrowser-Setup-<version>.exe`
   - `publish/output/app-update-stable.json`
   - `publish/output/app-update-stable.json.sha256`
   - `publish/output/MakaBrowser-<version>-windows-amd64.zip`
   - `publish/output/MakaBrowser-<version>-windows-amd64.zip.sha256`

## Windows 应用本体自更新自动化门禁

应用本体自更新必须用同一开发分支构造两个版本做闭环：

- baseline：低版本安装包，例如 `1.1.0`
- target：高版本更新包，例如 `1.1.5`

一键回归命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools\app-update\windows-app-update-e2e.ps1 -BaselineVersion 1.1.0 -TargetVersion 1.1.5
```

脚本会执行：

1. 分别打包 baseline 与 target。
2. 校验 target `MakaBrowser-<version>-windows-amd64.zip` 与 `app-update-stable.json`。
3. 静默安装 baseline 到 `%LOCALAPPDATA%\Programs\Maka Browser`。
4. 使用 Windows 本地绝对路径配置 `DESKTOP_APP_UPDATE_MANIFEST_URL`。
5. 通过临时 Go harness 调用 `Check -> Download -> Apply`。
6. 显式设置 `WindowsBackend.CurrentExePath` 为已安装的 `ant-chrome.exe`，避免 `go run` 的临时程序被复制成 runner。
7. 等待 runner 完成替换。
8. 校验安装目录 `ant-chrome.exe` 的 SHA256 等于 target zip 内的 `ant-chrome.exe`。
9. 校验 `state.json` 中 `localAppVersion` 等于 target 版本且没有 `lastError`。
10. 校验 `data/app.db`、`runtime/`、`diagnostics/` 与 `config.yaml` 保留。

如果 baseline 与 target 产物已提前生成并复制到测试目录，可跳过打包：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools\app-update\windows-app-update-e2e.ps1 -BaselineVersion 1.1.0 -TargetVersion 1.1.5 -SkipPublish
```

默认测试目录：

```text
C:\MakaBrowserUpdateTest
```

成功判定：

```text
[OK] Windows app-update e2e passed
```

### PowerShell / PATH 排障

如果 Windows agent 在 PowerShell 中报 `go` 不在 PATH，先在同一个 PowerShell 会话显式定位 `go.exe`：

```powershell
$Candidates = @(
  "C:\Program Files\Go\bin\go.exe",
  "C:\Go\bin\go.exe",
  "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"
)

$GoExe = $Candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $GoExe) {
  Get-ChildItem -Path "C:\Program Files", "C:\", "$env:LOCALAPPDATA\Programs" -Filter go.exe -Recurse -ErrorAction SilentlyContinue | Select-Object -First 20 FullName
  throw "go.exe not found"
}

$env:PATH = "$(Split-Path $GoExe);$env:PATH"
go version
```

确认后继续执行 e2e。不要因为 PATH 问题跳过 app-update 门禁。

## Windows 真机安装回归

### 场景 1：安装包能正常安装

检查项：

1. 安装程序可启动
2. 安装流程可完成
3. 新安装默认目录为 `%LOCALAPPDATA%\Programs\Maka Browser`
4. 安装目录存在：
   - `ant-chrome.exe`
   - `publish/runtime-manifest.json`
   - `publish/runtime-sources.json`

### 场景 2：首启 pointer 缺失

预期：

1. 首启 Gate 先卡在 pointer 缺失
2. 点击“自动修复”后生成 `runtime/current.json`
3. 然后进入 workspace host 连通性检查

### 场景 3：workspace host 不可达

预期：

1. Gate blocked
2. 错误码按来源分层
3. details / recommendedAction 正确展示

### 场景 4：workspace host 可达

预期：

1. Gate 放行
2. 应用进入登录页或后续正常流程

### 场景 5：app-update 后 server connection 保持

这是 2026-05-29 `1.1.7` 验证周期新增的必测项。

构造规则：

- baseline 使用旧版，例如 `1.1.0`
- target 使用包含 `b4c393d` 或后续修复的版本
- 登录页服务端地址为 `http://192.168.210.169:4174`
- 完成 `baseline -> target` 应用本体自更新

预期：

1. 更新完成后不手工修改任何 `server-connection.json`
2. 重启客户端后可直接登录远端 `4174`
3. `%ProgramData%\1688shop-agent\runtime\config\server-connection.json` 存在，且 `serverOrigin` 指向当前业务服务器
4. `%LOCALAPPDATA%\Programs\Maka Browser\runtime\config\server-connection.json` 从旧版升级场景下可以不存在
5. 如果两个路径都存在，客户端使用最新 mtime 的有效配置

采集命令：

```powershell
$Paths = @(
  "$env:ProgramData\1688shop-agent\runtime\config\server-connection.json",
  "$env:LOCALAPPDATA\Programs\Maka Browser\runtime\config\server-connection.json"
)

foreach ($p in $Paths) {
  Write-Host "==== $p ===="
  if (Test-Path $p) {
    Get-Item $p | Select-Object FullName, Length, LastWriteTime
    Get-Content $p -Raw
  } else {
    Write-Host "MISSING"
  }
}
```

## 更新回归场景

本节只覆盖 runtime/resource 更新，不替换 `ant-chrome.exe`。

### A：soft update

构造规则：

- `remote appVersion > local appVersion`
- `minimumResourceVersion == 当前 runtime resourceVersion`

预期：

1. 弹窗出现
2. 显示正确 `manifestSource / manifestUrl`
3. 有“稍后再说”
4. 关闭后继续进入正常流程

### B：required update

构造规则：

- `minimumResourceVersion > 当前 runtime resourceVersion`
- manifest 中的 package path 必须指向真实 payload

预期：

1. 弹窗显示 required 样式
2. 没有“稍后再说”
3. 点击“立即更新并继续”后成功切换 `runtime/current.json`

### C：manifest load fail

构造规则：

- 设置一个不存在的 manifest 路径

预期：

1. 弹窗出现
2. 错误直接显示在弹窗里
3. 文案里能看到：
   - 来源（如 `env:DESKTOP_UPDATE_MANIFEST_URL`）
    - manifest 路径
    - 文件不存在 / load failed 信息

## 应用本体自更新回归场景

应用本体更新与 runtime 更新分开验证。runtime 更新只切换 `runtime/current.json`；应用本体更新会替换用户态安装目录中的 `ant-chrome.exe` 与随包 payload。

### 已验证稳定基线

截至 2026-05-29：

```text
baseline: 1.1.0
target: 1.1.7
HEAD: b4c393d fix: preserve desktop server connection across updates
manifest: http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
server: http://192.168.210.169:4174
```

验证结论：

- app-update `Check -> Download -> Apply` 通过。
- 真实客户端 `1.1.0 -> 1.1.7` 更新通过。
- 更新后 `state.json` 为 `succeeded`，`localAppVersion` 为 `1.1.7`。
- 安装目录 `ant-chrome.exe` SHA256 与 target zip 内 exe 一致。
- 更新后不手工改 `server-connection.json`，重启后可直接登录远端 `4174`。
- ProgramData 下的 `server-connection.json` 升级前后内容与 mtime 不变。

### 前置条件

1. 新安装默认目录为 `%LOCALAPPDATA%\Programs\Maka Browser`
2. `publish/output/app-update-stable.json` 存在
3. `publish/output/MakaBrowser-<version>-windows-amd64.zip` 存在
4. manifest 中的 `sha256` 与 zip 文件一致
5. 执行以下命令通过：

```powershell
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json publish/output/MakaBrowser-<version>-windows-amd64.zip windows-amd64
```

### A：soft app update success

构造规则：

- 本地 app version 小于 manifest `version`
- 本地 app version 大于等于 manifest `minimumAppVersion`
- payload 为 `payloadType: full`

预期：

1. 弹窗显示客户端更新
2. 用户可稍后处理
3. 点击更新后应用先下载、校验并 staging
4. 应用退出，runner 替换用户态安装目录
5. 应用自动重启
6. 新版本号等于 manifest `version`
7. `stateRoot/app-update/state.json` 最终为 `succeeded`

### B：required app update success

构造规则：

- 本地 app version 小于 manifest `minimumAppVersion`
- manifest `version` 大于本地 app version

预期：

1. 弹窗显示 required 客户端更新
2. 没有“稍后再说”
3. 登录恢复和主工作台被阻断
4. 点击更新后完成下载、staging、替换与重启
5. 新版本通过 `--post-update-check`

### C：unsupported install

构造规则：

- 当前安装目录位于 `C:\Program Files` 或其他不可写目录
- app-update manifest 指向可用的新版本

预期：

1. 弹窗显示当前安装位置不支持自动更新
2. 不执行下载与替换
3. 提示迁移到用户态安装目录
4. 诊断包包含：
   - `appUpdateRoot`
   - `appUpdateStatePath`
   - `appUpdatePlanPath`

### C2：remote release hosting HTTP gate

发布到远端服务器后，不能以 SFTP 或 SSH 文件列表作为最终依据。JumpServer 的 SFTP subsystem 可能写入隔离文件系统。

test 通道必须验证：

```powershell
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/test/<target-version>/app-update-stable.json
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/test/<target-version>/MakaBrowser-<target-version>-windows-amd64.zip
```

stable 通道必须验证：

```powershell
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/MakaBrowser-<target-version>-windows-amd64.zip
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/MakaBrowser-Setup-<target-version>.exe
```

预期：

1. 所有 URL 返回 `HTTP/1.1 200 OK`
2. manifest `Content-Length` 与远端文件大小一致
3. zip / exe `Content-Length` 与本地产物大小一致
4. 任何 404 都必须停止验证并重新上传到真实 Nginx 文件系统

### D：manifest load fail

构造规则：

- 设置 `DESKTOP_APP_UPDATE_MANIFEST_URL` 为不存在的路径

预期：

1. 弹窗显示客户端更新检查失败
2. 错误码为 `APP-UPDATE-MANIFEST-LOAD-FAILED`
3. 不影响 runtime 更新 API 的状态

### E：checksum mismatch

构造规则：

- manifest package `sha256` 与 zip 实际 hash 不一致

预期：

1. 下载后拒绝进入 staging
2. `state.json` 保留错误码 `APP-UPDATE-DOWNLOAD-FAILED`
3. 不启动 apply runner
4. 安装目录没有变化

## macOS Application Self-Update Regression

### Scope

macOS app-update uses the same app-update manifest and shared backend contract as Windows. The platform target must be `darwin-arm64` or `darwin-amd64`.

This phase supports full package updates only. Delta patching and release channel rollout are out of scope.

### Supported Install Location

Supported:

```text
~/Applications/Maka Browser.app
```

Unsupported for automatic update:

```text
/Applications/Maka Browser.app
/System/Applications/...
```

Unsupported installs must return `unsupported_install` and must not delete or replace any bundle files.

### Required Payload Shape

The macOS app-update zip must contain:

```text
Ant Browser.app/
  Contents/
    Info.plist
    MacOS/
      ant-chrome
      publish/runtime-manifest.json
      bin/xray
      bin/sing-box
```

The legacy archive root is intentional for the first Maka Browser bridge
release. The installed bundle may still be named `Maka Browser.app`; the
updater copies the staged bundle contents into the current install root.

The payload must not contain `data/`, `User Data/`, `.db`, `.sqlite`, or `.sqlite3` files.

### Package Verification

Run:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/MakaBrowser-${VERSION}-darwin-arm64.zip" darwin-arm64
```

or:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/MakaBrowser-${VERSION}-darwin-amd64.zip" darwin-amd64
```

Expected:

```text
[OK] app update package verified
```

### Regression Matrix

1. Local file manifest smoke test: PASS.
2. HTTP manifest smoke test: PASS.
3. Soft update from `~/Applications/Maka Browser.app`: covered by shared UI path and non-required prompt behavior; no separate manual pass in this phase.
4. Required update from `~/Applications/Maka Browser.app`: PASS.
5. Unsupported install at `/Applications/Maka Browser.app`: PASS by backend rejection regression.
6. Checksum mismatch: PASS by macOS target regression.
7. Invalid `.app` payload: PASS by payload contract and tampered-stage regression.
8. Replace failure rollback: PASS by Darwin backend rollback regression.
9. Post-check version mismatch rollback/manual-repair: PASS by Darwin post-check regression.
10. Manual repair state after rollback failure: covered by existing manual-repair state path; destructive full manual pass deferred to formal distribution readiness if needed.

### Real Manual Regression Evidence

Latest real macOS manual regression:

- Date: 2026-05-22
- Report: `docs/reports/2026-05-22-macos-app-update-manual-regression.md`
- Phase closeout: `docs/reports/2026-05-22-cross-platform-app-update-phase-closeout.md`
- Baseline: `1.0.0`
- Target: `1.1.0`
- Install shape: user-writable `~/Applications/Maka Browser.app` style sandbox
- Manifest source: runtime config with local `file://` manifest
- UI action: clicked `更新并重启`
- State progression: `verifying -> succeeded -> idle`
- Relaunched UI version: `1.1.0`
- Installed binary hash matched the `1.1.0` artifact
- macOS Chromium Framework symlink preserved in user state:

```text
Resources -> Versions/Current/Resources
```

Notes:

- `idle` after `succeeded` is expected after relaunch, because the new app rechecks the same manifest and finds no pending update.
- This pass does not cover Developer ID signing, notarization, Gatekeeper quarantine, or public distribution hosting.

### Internal macOS Deployment Readiness

For the current internal-only rollout, the goal is to make a small number of trusted Macs install, update, roll back, and verify versions reliably. Formal distribution checks are not required for this rollout.

Detailed internal deployment runbook:

```text
docs/release/macos-internal-deployment-runbook.md
```

Internal rollout checklist:

1. Install to a user-writable location, preferably:

```text
~/Applications/Maka Browser.app
```

2. Keep `/Applications/Maka Browser.app` unsupported for automatic updates.
3. Point `DESKTOP_APP_UPDATE_MANIFEST_URL` or runtime config at the internal manifest.
4. Run one update from the internal manifest and payload.
5. Confirm the UI client version after relaunch.
6. Confirm `Contents/Info.plist` reports the expected version.
7. Confirm the installed `Contents/MacOS/ant-chrome` hash matches the intended artifact.
8. Confirm failed updates leave a readable `state.json` under the user state root.
9. Keep cleanup commands for old build artifacts and `/private/tmp` smoke/regression sandboxes.

### Release Readiness Checks

Before distributing a macOS release candidate:

1. Confirm the app bundle launches before packaging.
2. Confirm the app-update verifier passes for the target.
3. Confirm signing status for the release candidate.
4. Confirm notarization status for the release candidate.
5. Confirm Gatekeeper and quarantine behavior for the distributed artifact.

Signing, notarization, and Gatekeeper checks are release readiness checks. They are not runtime backend gates in this phase.

## 小Q 的 Windows 安装包任务

在 Windows 真机上，统一按 `docs/release/windows-agent-q-playbook.md` 执行。默认发布验证分支是：

```text
codex/windows-app-update-validation
```

开始前执行：

```powershell
git fetch --all
git checkout codex/windows-app-update-validation
git pull --ff-only origin codex/windows-app-update-validation
git rev-parse --short HEAD
git status --short
```

然后按目标版本执行打包、e2e、远端 HTTP 200、真实客户端升级和 server-connection 证据采集。

## Windows Release Factory MVP

Windows release builds should now run through GitHub Actions on XiaoQ's
self-hosted runner.

Manual workflow:

```text
Windows Release Factory
```

Required inputs:

- `target_version`
- `baseline_version`
- `channel=test`
- `run_e2e=true`
- `upload_to_server=true`

The workflow must pass:

1. runner preflight
2. `bat\publish.bat W -Version <target_version>`
3. artifact validation
4. `tools\app-update\windows-app-update-e2e.ps1`
5. release report generation
6. server test-channel upload

Do not promote to stable from a failed run. Stable promotion is a separate
manual operation.

## Release Factory MVP Acceptance Checklist

- [ ] XiaoQ runner is registered with `self-hosted`, `windows`, `ant-browser-release`.
- [ ] `windows-release-preflight.ps1` passes on the runner.
- [ ] `Windows Release Factory` workflow runs manually.
- [ ] Workflow builds `MakaBrowser-Setup-<version>.exe`.
- [ ] Workflow builds `MakaBrowser-<version>-windows-amd64.zip`.
- [ ] Workflow builds `app-update-stable.json`.
- [ ] Workflow runs app-update e2e successfully.
- [ ] Workflow uploads GitHub artifacts.
- [ ] Workflow uploads test-channel server artifacts.
- [ ] Server test manifest is reachable.
- [ ] Package SHA256 matches manifest SHA256.
- [ ] Manual stable promote publishes tested artifacts without rebuilding.
- [ ] A real installed client updates from baseline to target.
- [ ] XiaoQ no longer runs manual packaging commands for normal release validation.

## 常见失败点

### 1. `makensis.exe` 缺失

处理：

- 配置 `MAKENSIS_PATH`
- 或安装 NSIS 到默认目录

### 2. runtime hash 校验失败

处理：

- 检查 `publish/runtime-manifest.json`
- 检查 `publish/` 下实际 payload 是否被人工替换过

### 3. required update 点击后失败

优先检查：

- manifest 的 `path` 是否对应真实 payload
- payload 是否在安装目录 `publish/...` 下存在
- `runtime/current.json` 是否被回滚

### 4. 更新检查失败只显示通用文案

已在提交 `4ebc8e0 Preserve Wails string update errors` 修复。

若再复发，优先检查：

- 前端是否跑的是旧 bundle
- Wails 绑定是否重新生成
