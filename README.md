# Maka Browser

> 面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具（Windows / Linux）。

[![Release](https://img.shields.io/github/v/release/black-ant/Ant-Browser?sort=semver)](https://github.com/black-ant/Ant-Browser/releases)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-blue)](https://github.com/black-ant/Ant-Browser/releases)
[![Issues](https://img.shields.io/github/issues/black-ant/Ant-Browser)](https://github.com/black-ant/Ant-Browser/issues)

## 推荐内核项目

Maka Browser 当前推荐配套使用的浏览器内核，来源于开源项目 [fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium)。

如果你正在寻找可直接下载和维护的指纹内核版本，建议先查看它的 Releases 页面：

- <https://github.com/adryfish/fingerprint-chromium/releases>

这个项目为 Maka Browser 的内核准备提供了直接可用的基础来源，这里先对原项目做明确推荐与致谢。

Maka Browser 的目标很明确：在一台桌面设备上，帮助用户稳定管理多个彼此隔离的浏览器实例，并配合代理池、浏览器内核和快捷启动能力完成日常运营或测试工作。

## 目录

- [项目简介](#项目简介)
- [近期更新](#近期更新)
- [更新日志](CHANGELOG.md)
- [核心特性](#核心特性)
- [界面预览](#界面预览)
- [快速开始](#快速开始)
- [常用操作](#常用操作)
- [常见问题](#常见问题)
- [Roadmap](#roadmap)
- [贡献](#贡献)
- [支持与反馈](#支持与反馈)
- [License](#license)

## 项目简介

Maka Browser 适合以下场景：

- 多账号环境隔离
- 跨境电商与社媒账号运营
- 需要独立代理出口的本地测试
- 需要统一管理浏览器内核和实例配置的团队

这个项目当前提供的核心价值是：

- 给每个账号分配独立浏览器实例
- 给每个实例绑定独立代理
- 统一管理浏览器内核、标签、关键字和快捷打开码
- 在本地保存配置和运行数据，便于自主控制

## 近期更新

### 1.1.0 · 2026-03-19

- 完善 Linux 支持：补齐 Linux 环境下的开发、打包、安装、启动与运行链路，并持续修复安装版启动与退出稳定性问题
- 新增 SOCKS 代理测试支持：SOCKS 代理能力已进入测试阶段，后续会继续验证稳定性与兼容性
- 实验性支持接口触发浏览器：支持通过接口启动浏览器实例，便于后续接入自动化流程

完整历史版本记录见 [CHANGELOG.md](CHANGELOG.md)。

## 源码分支说明

- `master`：面向开发者的干净基线分支，不提交 `data/app.db`、实例目录或其他用户数据。首次启动时会自动初始化空数据库。
- `user_data`：在 `master` 基础上额外提交一份 `data/app.db` 测试快照，便于演示、联调和复现问题。
- 代理运行时 `bin/xray.exe`、`bin/sing-box.exe` 已随源码仓库提供；开发和发布打包不需要再单独下载这些运行时文件。

## 核心特性

- 实例隔离管理：支持创建、编辑、启动、停止、重启、克隆和删除浏览器实例
- 代理池配置：支持统一维护代理节点，并将代理分配到具体实例
- 多协议支持：支持常见代理配置方式，并支持导入 Clash
- 内核管理：支持维护多个 Chrome 内核版本，并设置默认内核
- 快捷启动：支持通过实例 Code 和 `Ctrl + K` 快速打开目标实例
- 标签与检索：支持按标签、关键字、状态、代理、内核、分组进行筛选
- 本地化存储：配置和实例数据保存在本地，适合长期使用和备份

## 界面预览

### 1. 控制台

<img src="images/readme/001-首页.png" alt="控制台" width="100%" />

对应功能点：

- 查看实例总数、运行中实例、代理节点数量和内核版本
- 从首页快速进入 `实例列表`、`代理池配置`、`内核管理`、`系统设置`
- 查看客户端版本、运行环境、数据存储和当前实例运行状态

### 2. 实例列表

<img src="images/readme/002-实例列表.png" alt="实例列表" width="100%" />

对应功能点：

- 统一查看和管理所有浏览器实例
- 按状态、代理、内核、分组、关键字筛选实例
- 支持 `新建配置`、启动、停止、重启、配置、克隆、删除
- 给实例分配快捷打开码，后续可以直接快速启动

### 3. 代理池配置

<img src="images/readme/003-设置代理池.png" alt="代理池配置" width="100%" />

对应功能点：

- 统一管理代理节点
- 支持按协议、分组筛选代理
- 支持手动维护代理和导入 Clash
- 支持查看延迟、IP 健康并挑选可用节点

### 4. 代理生效验证

<img src="images/readme/004-自定义代理.png" alt="代理生效验证" width="100%" />

对应功能点：

- 启动实例后访问 IP 检测网站验证代理是否真正生效
- 检查 IP 地区、ASN、运营商和风险值等信息
- 用于确认当前实例是否已经走目标代理出口

## 快速开始

Windows 发布 / 安装 / 更新回归内部说明见：

- [docs/release/windows-packaging-and-update-runbook.md](docs/release/windows-packaging-and-update-runbook.md)
- [docs/reports/2026-05-16-windows-release-stability-phase-report.md](docs/reports/2026-05-16-windows-release-stability-phase-report.md)

### 环境要求

- 操作系统：
  - Windows 10 / 11（64 位）
  - Linux（amd64 / arm64）
- 建议内存：8 GB 及以上
- 建议磁盘空间：2 GB 以上

### 下载与运行

1. 前往 Releases 页面下载最新版本：<https://github.com/black-ant/Ant-Browser/releases>
2. 安装版直接运行 `MakaBrowser-Setup-*.exe`
3. 便携版解压后运行 `ant-chrome.exe`
4. Linux 包下载后可直接安装 `ant-browser_<version>_<arch>.deb`，或解压 `tar.gz` 后运行 `ant-chrome`

### 从源码运行

1. 开发默认使用 `master` 分支；该分支不带测试用户数据，适合作为日常开发基线。
2. 如需带测试库的演示环境，请切换到 `user_data` 分支。
3. Windows 统一执行 `bat\dev.bat`；默认是稳定模式，如需前端 HMR 联调使用 `bat\dev.bat live`，如需受限内存复现使用 `bat\dev.bat limited`。脚本会自动注入 `ANT_BROWSER_WORKSPACE_INSTALL_ROOT`，优先级为环境变量 > 位置参数 > 默认路径 `%USERPROFILE%\Codex\1688shopManager\desktop-repos\1688shop-desktop`；若 `http://127.0.0.1:4174/api/health` 不可达，还会优先从 `ANT_BROWSER_WORKSPACE_SERVER_ROOT` / `WORKSPACE_SERVER_ROOT` / install root 推导出的主仓库自动拉起 `node --experimental-sqlite server/index.mjs`。
4. macOS 开发可执行 `scripts/dev-mac.sh`；默认 `stable` 模式会自动注入 `ANT_BROWSER_WORKSPACE_INSTALL_ROOT`，默认尝试 `$HOME/Codex/1688shopManager/desktop-repos/1688shop-desktop`，也可手动传入 install root 参数或预先设置环境变量覆盖。若本地 `4174` 未启动，脚本也会尝试自动补起 workspace server。
5. Windows 运行时使用 `bin/xray.exe`、`bin/sing-box.exe`；Linux 运行时使用 `bin/linux-<arch>/xray`、`bin/linux-<arch>/sing-box`。
6. 运行时文件采用“仓库固定 + 哈希校验”，校验清单在 `publish/runtime-manifest.json`，固定来源清单在 `publish/runtime-sources.json`。
7. 启动时更新检查支持多层清单来源：
   - `runtimeDir/config/release-update.json`
   - 环境变量 `DESKTOP_UPDATE_MANIFEST_URL`
   - `config.yaml -> release.update_manifest_url`
   - 默认无远端更新
7. 如需刷新 Linux 运行时，执行 `python3 tools/runtime/sync-runtime.py`（会按固定来源下载、校验归档并更新 manifest）。

开发模式说明：

- `bat\dev.bat`：默认稳定模式，先自动解析 workspace install root、必要时自动补起本地 workspace server，再构建 `frontend/dist`，以静态资源模式启动 Wails，不依赖外部 Vite dev server
- `bat\dev.bat live`：显式启动 Vite watcher，并通过 `-frontenddevserverurl` 接入桌面壳
- `bat\dev.bat limited`：在 `live` 基础上为 watcher 与其子进程附加 Windows Job Object 内存限制
- `scripts/dev-mac.sh`：macOS 开发入口；会自动解析 workspace install root，并在本地 workspace server 未健康时优先尝试自动补起。`stable` 模式先构建 `frontend/dist` 再启动 Wails，`live` 模式会先起 Vite dev server 再通过 `-frontenddevserverurl` 接入桌面壳
- 如需为依赖下载配置代理，可在启动前设置 `DEV_PROXY_URL`、`DEV_NO_PROXY`、`DEV_GOPROXY`
- 如需覆盖自动发现的 workspace 主仓库，可设置 `ANT_BROWSER_WORKSPACE_SERVER_ROOT` 或 `WORKSPACE_SERVER_ROOT`
- Workspace 接入参数现在可写入 `config.yaml > workspace`：
  - `install_root`：外部 `1688shop-desktop` 安装根
  - `agent_base_url`：本地 agent 基础地址
  - `server_origin`：workspace server 地址
  - `runtime_dir`：agent runtime 目录
- 优先级始终是：环境变量 > `config.yaml` > 内置默认值

### Linux 发布打包（源码）

Linux 发布脚本位于 `publish/linux/`。

```bash
bash publish/linux/publish-linux.sh --arch amd64
bash publish/linux/publish-linux.sh --arch arm64
```

详细说明见 [publish/linux/README.md](publish/linux/README.md)。

### 准备浏览器内核

代理运行时已经随仓库提供，你只需要准备浏览器内核。

1. 打开应用，进入 `指纹浏览器 > 内核管理`
2. 优先使用应用内下载功能准备内核
3. 如果手动准备内核，请确保目录下存在 `chrome.exe`

建议目录结构：

```text
chrome/
  chrom-142/
    chrome.exe
    ...
```

### 第一次使用建议流程

1. 在 `代理池配置` 中先导入或新增可用代理节点
2. 在 `实例列表` 中点击 `新建配置`
3. 选择实例名称、内核、代理、标签和需要的启动参数
4. 返回实例列表，点击启动按钮运行实例
5. 打开 IP 检测网站，确认代理结果是否符合预期

## 常用操作

| 目标 | 入口 | 说明 |
| --- | --- | --- |
| 新建浏览器实例 | `实例列表 > 新建配置` | 创建一个新的独立浏览器环境 |
| 配置代理池 | `代理池配置` | 维护代理节点并检查延迟、健康状态 |
| 绑定实例代理 | `实例编辑页` | 给指定实例分配目标代理节点 |
| 启动实例 | `实例列表` | 单击启动按钮即可运行目标实例 |
| 快速打开实例 | `Ctrl + K` | 可按 Code、实例名、标签、关键字快速检索 |
| 管理浏览器内核 | `内核管理` | 新增、编辑、删除和设置默认内核 |
| 验证代理结果 | 启动实例后访问 IP 检测网站 | 核对 IP、地区、ASN、风险值 |

## 常见问题

### 1. 应用无法启动怎么办？

先检查浏览器内核路径是否有效，并确认目标目录下存在 `chrome.exe`。

### 2. 实例启动了但代理没有生效怎么办？

先检查代理节点本身是否可用，再确认该实例已经正确绑定代理。建议启动后访问 IP 检测网站复核当前出口。

### 3. 实例太多，怎么快速找到目标实例？

可以在 `实例列表` 中按状态、代理、内核、分组、关键字筛选，也可以通过 `Ctrl + K` 使用实例 Code 或名称快速启动。

### 4. 多个账号怎么避免串号？

建议采用一账号一实例、一实例一稳定代理的方式，不要混用浏览器环境，也不要频繁切换同一实例的出口 IP。

## Roadmap

- 完善自动化模块能力
- 持续补充使用文档和接口说明
- 增强实例模板、批量管理和检索体验

## 贡献

欢迎通过 Issue 和 Pull Request 参与改进。

- Bug 反馈：请附带版本号、系统版本、复现步骤和截图
- 功能建议：请说明业务场景、预期行为和现有问题
- 文档优化：欢迎直接提交 README、教程和截图说明相关改进

如果是较大改动，建议先开 Issue 对齐需求再提交 PR。

## 支持与反馈

- Releases：<https://github.com/black-ant/Ant-Browser/releases>
- Issues：<https://github.com/black-ant/Ant-Browser/issues>

## License

当前仓库暂未附带独立的 `LICENSE` 文件，后续会补充。
