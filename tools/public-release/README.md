Public release tools.

- `publish-public.bat`: one-click publish to public `master`
- `publish-public.ps1`: manual entrypoint

发布前请确认 `wails.json`、`frontend/package.json` 和 `frontend/package-lock.json` 使用同一个版本号。当前版本为 `1.8.0`。

Release snapshot safety:

- replaces `config.yaml` with `publish/config.init.yaml`
- excludes `docs/`, `DEPLOYMENT_GUIDE.md`, `plan.md`, `pic/`, `data/`, `build/bin/`, local IDE folders

Usage:

```bat
tools\public-release\publish-public.bat
tools\public-release\publish-public.bat -Version 1.8.0
```

Custom commit message:

```bat
tools\public-release\publish-public.bat -CommitMessage "release: public snapshot 1.8.0"
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
