# Windows 小Q协作手册

## 目标

小Q负责 Windows 侧发布验证，不负责设计服务端架构，也不直接修改生产 stable manifest。

小Q的核心产出是：

- Windows 客户端安装包
- Windows app-update zip
- Windows app-update manifest
- Windows app-update e2e 验证报告
- Windows 真机登录/工作台/授权流程验收结论

## 固定仓库与分支

Maka Browser 仓库：

```text
https://github.com/robbin810130/Ant-Browser.git
```

默认协作分支：

```text
codex/windows-app-update-validation
```

每次开始前必须执行：

```powershell
git fetch origin
git checkout codex/windows-app-update-validation
git pull --ff-only origin codex/windows-app-update-validation
git rev-parse --short HEAD
git status --short
```

如果工作树不干净，先汇报，不要擅自清理未知改动。

## 当前服务器参数

业务服务端：

```text
http://192.168.210.169:4174
```

服务端健康检查：

```powershell
curl.exe -fsS http://192.168.210.169:4174/api/health
curl.exe -fsS http://192.168.210.169:4174/api/client/health
```

Windows 更新包分发：

```text
http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
```

Nginx 静态分发健康检查：

```powershell
curl.exe -fsS http://192.168.210.169:18080/healthz
```

## 小Q职责边界

小Q应该做：

- 拉取最新 Maka Browser 分支。
- 在 Windows 机器上打包。
- 跑 Windows app-update e2e。
- 安装真实客户端，登录页填服务端地址。
- 验证店铺资料、工作台、授权、打开后台。
- 将测试产物和报告反馈给 Robbin / Vera。

小Q不应该做：

- 不直接改服务器 stable manifest。
- 不跳过 e2e 门禁。
- 不删除旧版本 zip、installer、manifest。
- 不改 Mac 签名、公证流程。
- 不把密码、token、cookie 写进日志或提交。

## 当前稳定基线

截至 2026-05-29，Windows 自动更新链路的已验证稳定基线是：

```text
baseline: 1.1.0
target: 1.1.7
HEAD: b4c393d fix: preserve desktop server connection across updates
```

该轮验证结论：

- `1.1.0 -> 1.1.7` 真实客户端更新通过。
- 更新后不手工修改 `server-connection.json`，可直接登录 `http://192.168.210.169:4174`。
- `%ProgramData%\1688shop-agent\runtime\config\server-connection.json` 升级前后内容与 mtime 保持不变。
- `%LOCALAPPDATA%\Programs\Maka Browser\runtime\config\server-connection.json` 从旧版升级场景下可以不存在；它只在新版 GUI 登录页执行 `SaveDesktopServerConnection` 后产生。

以后验证高版本时，不允许只跑 harness 后收口，必须重复“真实客户端升级后不手工改配置直接登录”的场景。

## Windows 打包命令

在 Maka Browser 仓库根目录执行：

```powershell
bat\publish.bat W -Version <target-version>
```

期望产物：

```text
publish\output\MakaBrowser-Setup-<target-version>.exe
publish\output\MakaBrowser-<target-version>-windows-amd64.zip
publish\output\MakaBrowser-<target-version>-windows-amd64.zip.sha256
publish\output\app-update-stable.json
publish\output\app-update-stable.json.sha256
```

如果缺任何一个文件，停止并汇报。

## Release Factory 后的小Q职责

小Q机器现在作为 GitHub Actions self-hosted Windows runner。默认情况下，小Q不再手工拉代码和打包。

小Q仍负责：

- 保持 runner 在线。
- 维护 Windows 工具链。
- 在 workflow 失败时提供机器侧日志。
- 在 release 到达 test 通道后做真实客户端业务验收。

小Q不再默认负责：

- 手工 `git pull` 后打包。
- 手工运行 `bat\publish.bat`。
- 手工运行 app-update e2e。
- 手工改 stable manifest。

Release Factory workflow 的标准输入：

```text
target_version=<目标版本>
baseline_version=<基线版本>
channel=test
run_e2e=true
upload_to_server=true
```

在 workflow 未正式启用前，本节前面的手工命令仍可作为 fallback；启用后，手工命令只用于排障和一次性复现。

## Windows 自动更新门禁

每次发版必须跑：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools\app-update\windows-app-update-e2e.ps1 -BaselineVersion <baseline-version> -TargetVersion <target-version>
```

通过标准：

- baseline 安装成功。
- target 打包成功。
- manifest 和 zip 校验成功。
- baseline -> target 自动更新成功。
- `state.json.status = idle`。
- `localAppVersion = <target-version>`。
- 安装目录 `ant-chrome.exe` SHA256 等于 target zip 内 exe SHA256。
- `data/`、`runtime/`、`diagnostics/`、`config.yaml` 保留。
- `lastError.code` 和 `lastError.message` 为空。

失败时必须报告：

- 当前分支和 HEAD。
- BaselineVersion / TargetVersion。
- 失败步骤。
- `state.json` 内容。
- `app-update-debug.log` 关键错误。
- 是否有 `ant-chrome.exe`、`xray.exe`、`sing-box.exe` 残留进程。

### PowerShell 找不到 Go 的处理

如果 e2e 脚本在 PowerShell 中报 `go` 不在 PATH，不要跳过门禁，也不要重新打包。先在同一个 PowerShell 会话显式定位并注入 Go：

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
& $GoExe version
go version
```

确认 `go version` 后，在同一个 PowerShell 会话继续执行 e2e。

## 远端服务端登录验收

安装 target 客户端后，在登录页填写：

```text
http://192.168.210.169:4174
```

必须验证：

| 编号 | 验证项 | 通过标准 |
|---|---|---|
| V1 | 保存服务端地址 | 重启客户端后仍显示同一地址 |
| V2 | 登录打到远端 4174 | 登录成功，无默认 127.0.0.1 不可达错误 |
| V3 | 店铺资料加载 | 店铺资料页有真实店铺数据 |
| V4 | 工作台加载 | 工作台可显示店铺执行状态 |
| V5 | 授权/打开后台 | 至少一个可操作店铺完成授权或打开后台流程 |

如果出现：

```text
ENV-WORKSPACE-HOST-DEFAULT-UNREACHABLE
```

先做三件事：

1. 确认登录页服务端地址不是空，也不是 `127.0.0.1`。
2. 重启客户端，让 Wails backend 重新读取 `server-connection.json`。
3. 检查本机 runtime config 是否存在：

```powershell
Get-ChildItem -Recurse "$env:ProgramData\1688shop-agent\runtime\config" -ErrorAction SilentlyContinue
```

若仍失败，把错误截图、runtime config 路径和日志一起回传。

## 升级后服务端配置证据

每次真实客户端 app-update 验收完成后，必须采集这两个路径。该证据用于确认升级没有让客户端回退到默认 `127.0.0.1:4174` 或旧服务器地址。

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

通过标准：

- ProgramData 路径存在，且 `serverOrigin` 是当前业务服务器。
- 从旧版升级上来时，安装目录 runtime 路径可以不存在。
- 如果两个路径都存在，以最新 mtime 的有效配置为准。
- 升级后必须不手工改配置，直接重启并登录远端业务服务器。

## 产物上传约定

当前由 Mac/Vera 侧负责上传到服务器。如果需要小Q协助上传，先上传到 test 通道，不直接上 stable。

目标结构：

```text
/opt/1688shop/releases/windows/test/<target-version>/
  app-update.json
  app-update-stable.json
  app-update-stable.json.sha256
  MakaBrowser-<target-version>-windows-amd64.zip
  MakaBrowser-<target-version>-windows-amd64.zip.sha256
  MakaBrowser-Setup-<target-version>.exe
```

稳定发布由 Vera / 服务器侧执行 promote：

```bash
scripts/release/promote-release.sh --root /opt/1688shop/releases --platform windows --version <target-version>
```

### JumpServer 上传红线

JumpServer 的 SFTP subsystem 可能写入隔离文件系统。**SFTP stat 成功不代表 Nginx 能读取。**

上传后必须从普通 HTTP 客户端验证：

```powershell
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/test/<target-version>/app-update-stable.json
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/test/<target-version>/MakaBrowser-<target-version>-windows-amd64.zip
```

promote 到 stable 后必须验证：

```powershell
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/MakaBrowser-<target-version>-windows-amd64.zip
curl.exe -fsSI http://192.168.210.169:18080/releases/windows/stable/MakaBrowser-Setup-<target-version>.exe
```

只有 `HTTP/1.1 200 OK` 才算发布面可用。HTTP 404 必须打回重传，不允许继续真实客户端更新验证。

## 汇报格式

小Q每次汇报用这个格式：

```text
## Windows 发布验证报告

分支：
HEAD：
BaselineVersion：
TargetVersion：
服务端地址：
Manifest URL：

### 打包产物
- Setup exe: PASS/FAIL，路径，大小
- App update zip: PASS/FAIL，路径，大小
- Manifest: PASS/FAIL，version/channel/url/sha256

### e2e
- Check: PASS/FAIL
- Download: PASS/FAIL
- Apply: PASS/FAIL
- Final state: idle/localAppVersion/lastError
- Data preserved: PASS/FAIL

### 真机验收
- V1 保存服务端地址：PASS/FAIL
- V2 远端登录：PASS/FAIL
- V3 店铺资料：PASS/FAIL
- V4 工作台：PASS/FAIL
- V5 授权/打开后台：PASS/FAIL

### server-connection 证据
- ProgramData path: EXISTS/MISSING，mtime，内容
- Install runtime path: EXISTS/MISSING，mtime，内容
- 更新后是否手工改过配置：是/否
- 不改配置直接登录远端 4174：PASS/FAIL

### 问题
- 无 / 具体错误

### 结论
- 允许进入 test/stable / 不允许，原因
```

## 小Q收到任务后的第一句话

小Q开始工作前，应先确认：

```text
我会按 docs/release/windows-agent-q-playbook.md 执行。
先拉取 Maka Browser 的 codex/windows-app-update-validation 分支，确认 HEAD，然后跑 Windows 打包和 e2e。
不会直接改 stable manifest，不会删除旧产物。
```

## 关联文档

- `docs/release/agent-operated-cicd.md`
- `docs/desktop/release-handover-2026-05-28-remote-server.md`
- `docs/release/windows-packaging-and-update-runbook.md`（Maka Browser 仓库）
