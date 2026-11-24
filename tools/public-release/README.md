Public release tools.

- `publish-public.bat`: one-click publish to public `master`
- `publish-public.ps1`: manual entrypoint

发布前请确认 `wails.json`、`frontend/package.json` 和 `frontend/package-lock.json` 使用同一个版本号。当前版本为 `1.9.8`。

Release snapshot safety:

- replaces `config.yaml` with `publish/config.init.yaml`
- excludes `docs/`, `DEPLOYMENT_GUIDE.md`, `plan.md`, `pic/`, `data/`, `build/bin/`, local IDE folders

Usage:

```bat
tools\public-release\publish-public.bat
tools\public-release\publish-public.bat -Version 1.9.0
```

Custom commit message:

```bat
tools\public-release\publish-public.bat -CommitMessage "release: public snapshot 1.9.0"
```

Or load a multi-line message from a file:

```powershell
.\tools\public-release\publish-public.ps1 -CommitMessageFile .\publish-message.txt
```

Behavior:

- public `master` keeps history and appends one aggregated commit per publish
- interactive mode supports overriding the publish version before deciding whether to publish `release/<version>` and `v<version>`
- the script shows console options for publish scope: `master` / `master+release` / `master+tag` / `master+release+tag`
- running the script will publish directly; use `-DryRun` only when you explicitly want a no-push preview
- command line switches are kept only for automation/manual override

## 版本通道（正式 / 预览）

应用内「检测更新」默认只看正式版；预览版按 GitHub Release 的 `prerelease` 标记识别。发布与分支无关，单分支即可：

| 目标 | 操作 | 结果 |
|---|---|---|
| 预览版 | `git tag v1.9.8-beta.1 && git push origin v1.9.8-beta.1` | 自动标记 prerelease；正式用户看不到，预览通道可见 |
| 正式版 | `git tag v1.9.8 && git push origin v1.9.8` | 正式通道可见；`1.9.8 > 1.9.8-beta.1`，beta 用户会收到正式版 |

要点：

- 版本号以 tag（或 workflow_dispatch 的 `version` 输入）为准。CI 在构建前把该版本写入 `wails.json` 的 `productVersion`，所以 `master` 上的版本号只需要是“下一个开发版本”，发预览不需要为了版本号单独提交一次。
- 打 tag 前保持工作区干净，tag 指向 `master` 上的提交；预览版修完 bug 直接打 `-beta.2`，同一版本号不能重复发，只能递增后缀。
- 预览版本号必须大于当前已发布的正式版（如 `1.9.8-beta.1 > 1.9.7`），否则新老用户都检测不到。
- 只有给**已发布的旧版本**打补丁（例如 1.9.7 的 hotfix）时，才需要从旧 tag 拉分支；这是唯一需要分支的场景。

