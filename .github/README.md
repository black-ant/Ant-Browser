# Actions 构建与自动发布

统一入口为 **Actions → Build and Release**（`build-release.yml`）。其余三个 workflow 为仅供该入口调用的可复用平台构建，不再各自触发或抢写 Release。

## 触发与版本

| 触发 | 版本来源 | 发布行为 |
| --- | --- | --- |
| push 到 `master` / `main` | `wails.json` 的 `info.productVersion` | 发布 `ci-<run_number>-<commit前7位>` 预发行版，不替换正式 Latest |
| push `v1.9.0` 等标签 | 标签去掉 `v` | 所有平台成功后发布正式 Release，标为 Latest |
| push `v1.9.0-beta.1` 等标签 | 标签去掉 `v` | 发布预发行版，不替换正式 Latest |
| Run workflow | 可选 `version`，默认读 `wails.json` | 发布当前所选分支/标签的 Release；分支运行仍是预发行版 |
| Pull request | `wails.json` | 构建与验证，**不发布** |

版本接受 `1.9.0` / `1.9.0-beta.1` 格式。标签优先，手动覆盖值与标签冲突时直接失败。构建副本中的 Wails、前端和 Arch 包版本统一写入；不自动回写 Git 提交。Arch 的 prerelease `-` 转换为 `_`，以满足 makepkg 规则。

## 流水线

1. 校验版本、运行发布脚本单元测试；Node 24 下 `npm ci`、前端生产构建一次，上传共享前端资源。
2. 并行构建 Linux amd64/arm64、Arch x86_64、macOS Intel/Apple Silicon。
3. 每个平台校验固定运行时 SHA256、安装包结构和可执行权限；Linux/Arch 检查动态链接库。Linux amd64 额外编译所有 Go 包与测试文件。
4. 所有平台成功后，专用发布任务收集 **7 个安装包**，严格检查名称和非空文件，生成 `SHA256SUMS`。
5. 显式设置 `GH_REPO`；先创建草稿 Release，再上传安装包及校验文件，最后公开。失败时不公开新建的残缺 Release。重跑可以替换同一提交的资产，但拒绝覆盖指向其他提交的标签。

发布资产：

- `ant-browser_<version>_{amd64,arm64}.deb`
- `AntBrowser-<version>-linux-{amd64,arm64}.tar.gz`
- `ant-browser-<arch-version>-1-x86_64.pkg.tar.zst`
- `AntBrowser-<version>-macos-{amd64,arm64}.zip`
- `SHA256SUMS`

只上传 macOS ZIP，以保留 `.app` 内的可执行权限；不直接上传原始 `.app` 目录。当前自动发布覆盖现有 Linux/Arch/macOS 构建，Windows 仍使用仓库原有本地发布脚本。

## 稳定性与权限

- npm 锁文件统一使用官方 registry，保留原版本和 integrity；不通过放开任意远程 URL 下载来掩盖 `EALLOWREMOTE`。
- 前端仅允许锁定版本的 `@swc/core`、`esbuild` 安装脚本，兼容新版 npm。
- 各平台复用已构建前端；Wails 使用 `-s -m -skipbindings -skipembedcreate`，避免重复安装、修改 Go 模块或扫描已存在的嵌入目录。
- Ubuntu/macOS 使用 `setup-go` 的 stable 工具链（`go.mod` 的 1.22 是项目最低声明，而不是 CI 固定工具链）；Arch 使用完整系统升级后的 Go。
- 全局仅有 `contents: read`；只有发布任务有 `contents: write`，使用内置 `GITHUB_TOKEN`，无需新增发布密钥。
- 同一分支新提交取消旧构建；标签构建不自动取消。压缩过的安装包不再重复压缩，Artifacts 保留 7 天。

## 本地验证

```bash
python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v
actionlint
bash -n scripts/ci/publish-release.sh publish/linux/publish-linux.sh publish/mac/publish-mac.sh
(cd frontend && npm ci --prefer-offline --no-audit --no-fund && npm run build:clean)
go test -tags webkit2_41 -run '^$' ./...
```

完整业务测试另行运行 `go test -tags webkit2_41 ./...`。本次检查发现现有测试涉及备份内核导入、平台运行时路径和启动服务恢复的失败/不稳定结果；它们不是本次 npm 构建失败的原因，也未被标记为通过。构建流程只把发布工具单元测试与 Go 编译检查作为新增门禁，不擅自改动业务行为。

macOS 包未签名、未公证；包结构和链接检查不代表真实桌面启动、浏览器内核运行或两套代理连接栈验收通过。
