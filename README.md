# Ant Browser

> 面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具（Windows / Linux / macOS unsigned）。

[![Release](https://img.shields.io/github/v/release/black-ant/Ant-Browser?sort=semver)](https://github.com/black-ant/Ant-Browser/releases)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)](https://github.com/black-ant/Ant-Browser/releases)
[![Issues](https://img.shields.io/github/issues/black-ant/Ant-Browser)](https://github.com/black-ant/Ant-Browser/issues)

## 推荐内核项目

Ant Browser 当前推荐配套使用的浏览器内核，来源于开源项目 [fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium)。

如果你正在寻找可直接下载和维护的指纹内核版本，建议先查看它的 Releases 页面：

- <https://github.com/adryfish/fingerprint-chromium/releases>

这个项目为 Ant Browser 的内核准备提供了直接可用的基础来源，这里先对原项目做明确推荐与致谢。

Ant Browser 的目标很明确：在一台桌面设备上，帮助用户稳定管理多个彼此隔离的浏览器窗口，并配合代理池、浏览器内核和快捷启动能力完成日常运营或测试工作。

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

Ant Browser 适合以下场景：

- 多账号环境隔离
- 跨境电商与社媒账号运营
- 需要独立代理出口的本地测试
- 需要统一管理浏览器内核和窗口配置的团队

这个项目当前提供的核心价值是：

- 给每个账号分配独立浏览器窗口
- 给每个窗口绑定独立代理
- 统一管理浏览器内核、标签、关键字和快捷打开码
- 在本地保存配置和运行数据，便于自主控制

## 近期更新

### 1.1.0 · 2026-03-19

- 完善 Linux 支持：补齐 Linux 环境下的开发、打包、安装、启动与运行链路，并持续修复安装版启动与退出稳定性问题
- 补齐 macOS unsigned 内测构建链路：支持在原生 macOS 主机上打包 `.app` / `.zip`，并将用户状态目录放到 `~/Library/Application Support/ant-browser`
- 新增 SOCKS 代理测试支持：SOCKS 代理能力已进入测试阶段，后续会继续验证稳定性与兼容性
- 实验性支持接口触发浏览器：支持通过接口启动浏览器窗口，便于后续接入自动化流程

完整历史版本记录见 [CHANGELOG.md](CHANGELOG.md)。

## 源码分支说明

- `master`：面向开发者的干净基线分支，不提交 `data/app.db`、窗口目录或其他用户数据。首次启动时会自动初始化空数据库。
- `user_data`：在 `master` 基础上额外提交一份 `data/app.db` 测试快照，便于演示、联调和复现问题。
- 代理运行时 `bin/xray.exe`、`bin/sing-box.exe` 已随源码仓库提供；开发和发布打包不需要再单独下载这些运行时文件。

## 核心特性

- 窗口隔离管理：支持创建、编辑、启动、停止、重启、克隆和删除浏览器窗口
- 代理池配置：支持统一维护代理节点，并将代理分配到具体窗口
- 多协议支持：支持常见代理配置方式，并支持导入 Clash
- 内核管理：支持维护多个 Chrome 内核版本，并设置默认内核
- 快捷启动：支持通过窗口 Code 和 `Ctrl + K` 快速打开目标窗口
- 标签与检索：支持按标签、关键字、状态、代理、内核、分组进行筛选
- 本地化存储：配置和窗口数据保存在本地，适合长期使用和备份

## 界面预览

### 1. 控制台

<img src="images/readme/001-首页.png" alt="控制台" width="100%" />

对应功能点：

- 查看窗口总数、运行中窗口、代理节点数量和内核版本
- 从首页快速进入 `窗口列表`、`代理池配置`、`内核管理`、`系统设置`
- 查看客户端版本、运行环境、数据存储和当前窗口运行状态

### 2. 窗口列表

<img src="images/readme/002-窗口列表.png" alt="窗口列表" width="100%" />

对应功能点：

- 统一查看和管理所有浏览器窗口
- 按状态、代理、内核、分组、关键字筛选窗口
- 支持 `新建配置`、启动、停止、重启、配置、克隆、删除
- 给窗口分配快捷打开码，后续可以直接快速启动

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

- 启动窗口后访问 IP 检测网站验证代理是否真正生效
- 检查 IP 地区、ASN、运营商和风险值等信息
- 用于确认当前窗口是否已经走目标代理出口

## 快速开始

### 环境要求

- 操作系统：
  - Windows 10 / 11（64 位）
  - Linux（amd64 / arm64）
  - macOS（amd64 / arm64，当前为 unsigned 内测包）
- 建议内存：8 GB 及以上
- 建议磁盘空间：2 GB 以上

### 下载与运行

1. 前往 Releases 页面下载最新版本：<https://github.com/black-ant/Ant-Browser/releases>
2. 安装版直接运行 `AntBrowser-Setup-*.exe`
3. 便携版解压后运行 `ant-chrome.exe`
4. Linux 包下载后可直接安装 `ant-browser_<version>_<arch>.deb`，或解压 `tar.gz` 后运行 `ant-chrome`
5. macOS unsigned 包解压后运行 `AntBrowser-<version>-macos-<arch>.app`；如被 Gatekeeper 拦截，请对本机测试包执行 `xattr -dr com.apple.quarantine <app路径>` 后再打开

### 从源码运行

1. 开发默认使用 `master` 分支；该分支不带测试用户数据，适合作为日常开发基线。
2. 如需带测试库的演示环境，请切换到 `user_data` 分支。
3. Windows 统一执行 `bat\dev.bat`；默认是稳定模式，如需前端 HMR 联调使用 `bat\dev.bat live`，如需受限内存复现使用 `bat\dev.bat limited`。
4. Windows 运行时使用 `bin/xray.exe`、`bin/sing-box.exe`；Linux 运行时使用 `bin/linux-<arch>/xray`、`bin/linux-<arch>/sing-box`；macOS 运行时使用 `bin/darwin-<arch>/xray`、`bin/darwin-<arch>/sing-box`。
5. 运行时文件采用“仓库固定 + 哈希校验”，校验清单在 `publish/runtime-manifest.json`，固定来源清单在 `publish/runtime-sources.json`。
6. 如需刷新 Linux / macOS 运行时，执行 `python3 tools/runtime/sync-runtime.py --target <target>`（会按固定来源下载、校验归档并更新 manifest）。

开发模式说明：

- `bat\dev.bat`：默认稳定模式，先构建 `frontend/dist`，再以静态资源模式启动 Wails，不依赖外部 Vite dev server
- `bat\dev.bat live`：显式启动 Vite watcher，并通过 `-frontenddevserverurl` 接入桌面壳
- `bat\dev.bat limited`：在 `live` 基础上为 watcher 与其子进程附加 Windows Job Object 内存限制
- 如需为依赖下载配置代理，可在启动前设置 `DEV_PROXY_URL`、`DEV_NO_PROXY`、`DEV_GOPROXY`

### Linux 发布打包（源码）

Linux 发布脚本位于 `publish/linux/`。

```bash
bash publish/linux/publish-linux.sh --arch amd64
bash publish/linux/publish-linux.sh --arch arm64
```

详细说明见 [publish/linux/README.md](publish/linux/README.md)。

### macOS unsigned 发布打包（源码）

macOS 发布脚本位于 `publish/mac/`，必须在原生 macOS 主机上执行，且目标架构需与主机架构一致。

```bash
bash publish/mac/publish-mac.sh --arch amd64
bash publish/mac/publish-mac.sh --arch arm64
```

脚本会生成 unsigned `.app` 和 `.zip`，适合 PR 验证与内部测试。详细说明见 [publish/mac/README.md](publish/mac/README.md)。

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
2. 在 `窗口列表` 中点击 `新建配置`
3. 选择窗口名称、内核、代理、标签和需要的启动参数
4. 返回窗口列表，点击启动按钮运行窗口
5. 打开 IP 检测网站，确认代理结果是否符合预期

## 常用操作

| 目标 | 入口 | 说明 |
| --- | --- | --- |
| 新建浏览器窗口 | `窗口列表 > 新建配置` | 创建一个新的独立浏览器环境 |
| 配置代理池 | `代理池配置` | 维护代理节点并检查延迟、健康状态 |
| 绑定窗口代理 | `窗口编辑页` | 给指定窗口分配目标代理节点 |
| 启动窗口 | `窗口列表` | 单击启动按钮即可运行目标窗口 |
| 快速打开窗口 | `Ctrl + K` | 可按 Code、窗口名、标签、关键字快速检索 |
| 管理浏览器内核 | `内核管理` | 新增、编辑、删除和设置默认内核 |
| 验证代理结果 | 启动窗口后访问 IP 检测网站 | 核对 IP、地区、ASN、风险值 |

## 常见问题

### 1. 应用无法启动怎么办？

先检查浏览器内核路径是否有效，并确认目标目录下存在 `chrome.exe`。

### 2. 窗口启动了但代理没有生效怎么办？

先检查代理节点本身是否可用，再确认该窗口已经正确绑定代理。建议启动后访问 IP 检测网站复核当前出口。

### 3. 窗口太多，怎么快速找到目标窗口？

可以在 `窗口列表` 中按状态、代理、内核、分组、关键字筛选，也可以通过 `Ctrl + K` 使用窗口 Code 或名称快速启动。

### 4. 多个账号怎么避免串号？

建议采用一账号一窗口、一窗口一稳定代理的方式，不要混用浏览器环境，也不要频繁切换同一窗口的出口 IP。

## Roadmap

- 完善自动化模块能力
- 持续补充使用文档和接口说明
- 增强窗口模板、批量管理和检索体验

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
