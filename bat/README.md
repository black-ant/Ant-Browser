# bat

> 脚本入口运行于 Windows。`publish.bat` 支持 Windows 打包，也可通过 Docker Desktop 调用 Linux 发布脚本。

Windows 发布 / 安装 / 更新回归请优先查看：

- [`docs/release/windows-packaging-and-update-runbook.md`](../docs/release/windows-packaging-and-update-runbook.md)

## 用途

- `dev.bat`：统一的本地开发入口
- `build.bat`：本地构建可执行文件
- `publish.bat`：发布打包入口（Windows / Linux / 两者）
- `recover-profiles.ps1`：从现有 `user_data_root` 目录补回丢失的实例配置

## 用法

### `dev.bat`

统一入口，按参数切换开发模式，避免多个 bat 文件误导使用者。

```bat
bat\dev.bat
bat\dev.bat live
bat\dev.bat limited
bat\dev.bat stable C:\Users\you\Codex\1688shopManager\desktop-repos\1688shop-desktop
```

模式说明：

- `bat\dev.bat`：默认稳定模式。会先解析 workspace install root、必要时自动补起本地 workspace server，再生成 Wails bindings、构建 `frontend/dist`，最后以静态资源模式启动 Wails
- `bat\dev.bat live`：显式启动 `frontend/scripts/dev-watcher.mjs`，并通过 `-frontenddevserverurl` 接入 Vite dev server
- `bat\dev.bat limited`：在 `live` 基础上通过 `scripts/run-limited-frontend-dev.ps1` 给 watcher 及其子进程附加 Windows Job Object 内存限制

默认行为：

- workspace install root 优先级：`ANT_BROWSER_WORKSPACE_INSTALL_ROOT` > `WORKSPACE_INSTALL_ROOT` > 位置参数 > `%USERPROFILE%\Codex\1688shopManager\desktop-repos\1688shop-desktop`
- 当 `http://127.0.0.1:4174/api/health` 不可达时，脚本会优先从 `ANT_BROWSER_WORKSPACE_SERVER_ROOT`、`WORKSPACE_SERVER_ROOT`、install root 向上两级推导出的主仓库、默认 `%USERPROFILE%\Codex\1688shopManager` 中自动寻找 workspace server 根并执行 `node --experimental-sqlite server/index.mjs`
- 如果脚本自己拉起了 workspace server，退出 `dev.bat` 时会一并清理该进程树
- 稳定模式不依赖外部 Vite dev server，因此不会因为 watcher 或 `5218` 端口异常直接白屏
- `live` 模式默认优先使用 `5218`，若端口被其他程序占用，会自动切换到下一个可用端口
- watcher 默认 `FRONTEND_NODE_RSS_HARD_LIMIT_MB=0`，即只告警，不默认 RSS 强杀
- `limited` 模式默认 `FRONTEND_PROCESS_MEMORY_LIMIT_MB=512`

常用内存控制变量：

```text
FRONTEND_PROCESS_MEMORY_LIMIT_MB
FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB
FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB
FRONTEND_NODE_RSS_WARN_MB
FRONTEND_NODE_RSS_HARD_LIMIT_MB
FRONTEND_NODE_RSS_HARD_LIMIT_HITS
FRONTEND_NODE_RSS_AUTO_RESTART
FRONTEND_NODE_RSS_RESTART_DELAY_MS
FRONTEND_NODE_RSS_RESTART_MAX_COUNT
FRONTEND_NODE_RSS_RESTART_WINDOW_MS
FRONTEND_NODE_MEMORY_POLL_MS
FRONTEND_DISABLE_HMR
```

开发代理相关变量：

```text
DEV_PROXY_URL   -> 为 npm / Node / Go 下载流量注入 HTTP(S) 代理
DEV_NO_PROXY    -> 设置 NO_PROXY / no_proxy
DEV_GOPROXY     -> 覆盖 GOPROXY；未设置时默认使用 https://goproxy.cn,direct
ANT_BROWSER_WORKSPACE_INSTALL_ROOT / WORKSPACE_INSTALL_ROOT -> 覆盖 workspace install root 自动发现
ANT_BROWSER_WORKSPACE_SERVER_ROOT / WORKSPACE_SERVER_ROOT   -> 覆盖 workspace server 根自动发现
ANT_BROWSER_DEBUG_STARTUP                                   -> 启动时额外输出 workspace 调试日志；默认自动设为 1
```

日志：

- `live` / `limited` 模式的 watcher 日志会写入仓库根目录：
- `tmp-npm-dev.log`
- `tmp-npm-dev.err.log`
- 如果 `dev.bat` 自动拉起了 workspace server，对应日志也会写入仓库根目录：
- `tmp-workspace-server.log`
- `tmp-workspace-server.err.log`

FAQ：

- 为什么默认模式没有 HMR：因为默认入口优先保证桌面壳可用性，不依赖外部 Vite
- 什么情况下用 `bat\dev.bat live`：页面样式、交互、接口联调需要快速热更新时
- 什么情况下用 `bat\dev.bat limited`：低内存机器、复现 Vite 内存膨胀、或需要显式进程级内存约束时

### `build.bat`

构建 `build\bin\ant-chrome.exe`。

```bat
bat\build.bat
```

说明：

- 开发分支默认按完整源码构建
- 若缺少 `go.mod`、`main.go`、`wails.json` 等核心入口文件，脚本会直接失败，避免复用旧产物掩盖问题

### `publish.bat`

发布打包入口，启动后会提示选择：

- `W`：仅 Windows
- `L`：仅 Linux（通过 Docker Desktop）
- `B`：Windows + Linux

```bat
bat\publish.bat
```

也支持无交互参数（适合脚本调用）：

```bat
bat\publish.bat W
bat\publish.bat L
bat\publish.bat B
bat\publish.bat W -Version 1.1.0
bat\publish.bat B -Version 1.1.0
```

说明：

- `-Version 1.1.0` 会覆盖本次发布使用的版本号。
- Windows / Linux 包名、NSIS 安装包版本号，以及本次构建期间读取到的 `wails.json productVersion` 会统一使用该值。

Windows 打包依赖 NSIS，默认查找顺序：

```text
MAKENSIS_PATH -> 直接指向 makensis.exe
NSIS_PATH     -> NSIS 目录或 makensis.exe
NSIS_HOME     -> NSIS 安装目录
PATH          -> where makensis.exe
```

默认兜底目录：

```text
C:\Program Files (x86)\NSIS\makensis.exe
C:\Program Files\NSIS\makensis.exe
```

Windows 分支使用的项目路径：

```text
输入：
- build\bin\ant-chrome.exe
- publish\config.init.yaml
- bin\xray.exe
- bin\sing-box.exe

临时目录：
- publish\staging\

输出：
- publish\output\MakaBrowser-Setup-<version>.exe
```

说明：

- Windows 安装包包含应用本体、默认配置和代理运行时。
- 如果 `chrome\` 根目录或其一级子目录中检测到有效的 Windows `chrome.exe`，会自动一起打进 EXE 安装包。
- 如果未检测到 Windows 内核，安装包仍会保留 `chrome\README.md` 说明文件。
- 打包前会自动执行 `python3 tools/runtime/verify-publish-contract.py`，确认 `publish/runtime-manifest.json`、`publish/runtime-sources.json`、NSIS staging 规则和 release README 的 runtime contract 仍保持一致。

Linux 分支会通过 Docker Desktop 调用：

```text
docker build -f publish/linux/linux-builder.Dockerfile -t ant-browser-linux-builder:local publish/linux
docker run --rm -v <repo>:/workspace -w /workspace ant-browser-linux-builder:local ^
  bash -c "bash publish/linux/publish-linux.sh --arch <Docker当前架构>"
```

要求：Docker Desktop 已安装并启动，且 Linux 容器引擎可用。

Linux 产物输出目录：

```text
publish\output\
```

常用环境变量：

```text
NO_PAUSE=1  -> 运行结束不 pause（适合 CI 或脚本调用）
CI=1        -> 同样不 pause
DESKTOP_UPDATE_MANIFEST_URL -> 启动时更新清单地址，可为 file://、本地路径或 http(s) URL
```

Windows 产物：

```text
publish\output\MakaBrowser-Setup-<version>.exe
```

完整发布与真机回归步骤见：

- [`docs/release/windows-packaging-and-update-runbook.md`](../docs/release/windows-packaging-and-update-runbook.md)

### `recover-profiles.ps1`

用于“实例配置丢了，但 `data\<userDataDir>` 目录还在”的恢复场景。

默认只预览，不写数据库：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Maka Browser'
```

确认结果后再写回 `app.db`：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Maka Browser' -Apply
```

如果旧目录来自备份恢复，且怀疑存在跨内核残留状态，可同时为“风险目录”创建一份 `__repair_时间戳` 副本，再将新配置指向副本：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Maka Browser' -Apply -RepairRisky
```

说明：

- 脚本会调用 `go run ./backend/cmd/profile-recover`
- `-Apply` 模式会先在 `data\recovery-backups\` 下备份当前数据库文件
- 默认不会删除旧目录，也不会主动清理登录态文件
- 运行 `-Apply` 前应先关闭 Maka Browser，避免并发写库

## 备注

- `generate-bindings.bat` 是辅助脚本，通常由 `build.bat` 调用。
- `generate-bindings.bat`、`build.bat`、`dev.bat` 都假定当前分支是完整源码仓库。
- 如果这些脚本报告缺少 `go.mod`、`main.go`、`wails.json`，应先恢复源码入口，而不是继续复用旧二进制。
