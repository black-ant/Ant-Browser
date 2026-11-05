# bat

> 脚本入口运行于 Windows。`publish.bat` 支持 Windows 打包，也可通过 Docker Desktop 调用 Linux 发布脚本。

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
```

模式说明：

- `bat\dev.bat`：默认稳定模式。先生成 Wails bindings，再构建 `frontend/dist`，最后以静态资源模式启动 Wails
- `bat\dev.bat live`：显式启动 `frontend/scripts/dev-watcher.mjs`，并通过 `-frontenddevserverurl` 接入 Vite dev server
- `bat\dev.bat limited`：在 `live` 基础上通过 `scripts/run-limited-frontend-dev.ps1` 给 watcher 及其子进程附加 Windows Job Object 内存限制

Linux 或其他 POSIX shell 环境使用 `scripts/dev.sh`：

```bash
./scripts/dev.sh
./scripts/dev.sh live
./scripts/dev.sh help
```

该入口默认使用 `stable` 模式；Windows 开发请继续使用 `bat\dev.bat`。

默认行为：

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
```

日志：

- `live` / `limited` 模式的 watcher 日志会写入仓库根目录：
- `tmp-npm-dev.log`
- `tmp-npm-dev.err.log`

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
bat\publish.bat W -Version 1.8.0
bat\publish.bat B -Version 1.8.0
```

说明：

- `-Version 1.8.0` 会覆盖本次发布使用的版本号；发布其他版本时替换为目标版本。
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
- publish\output\AntBrowser-Setup-<version>.exe
```

说明：

- Windows 安装包包含应用本体、默认配置和代理运行时。
- 如果 `chrome\` 根目录或其一级子目录中检测到有效的 Windows `chrome.exe`，会自动一起打进 EXE 安装包。
- 如果未检测到 Windows 内核，安装包仍会保留 `chrome\README.md` 说明文件。

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
```

Windows 产物：

```text
publish\output\AntBrowser-Setup-<version>.exe
```

### `recover-profiles.ps1`

用于“实例配置丢了，但 `data\<userDataDir>` 目录还在”的恢复场景。

默认只预览，不写数据库：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Ant Browser'
```

确认结果后再写回 `app.db`：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Ant Browser' -Apply
```

如果旧目录来自备份恢复，且怀疑存在跨内核残留状态，可同时为“风险目录”创建一份 `__repair_时间戳` 副本，再将新配置指向副本：

```powershell
pwsh -File bat/recover-profiles.ps1 -AppRoot 'E:\software\Ant Browser' -Apply -RepairRisky
```

说明：

- 脚本会调用 `go run ./backend/cmd/profile-recover`
- `-Apply` 模式会先在 `data\recovery-backups\` 下备份当前数据库文件
- 默认不会删除旧目录，也不会主动清理登录态文件
- 运行 `-Apply` 前应先关闭 Ant Browser，避免并发写库

## 备注

- `generate-bindings.bat` 是辅助脚本，通常由 `build.bat` 调用。
- `generate-bindings.bat`、`build.bat`、`dev.bat` 都假定当前分支是完整源码仓库。
- 如果这些脚本报告缺少 `go.mod`、`main.go`、`wails.json`，应先恢复源码入口，而不是继续复用旧二进制。
