# 标签/分组/直播养号/批量创建 六项功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为多开养号补齐六项:标签显示/行内/批量优化、每环境保活开关、默认硬静音、分组操作进批量工具条、批量创建按平台生成且永不重复、修复标签管理页滚动与条数。

**Architecture:** 后端在既有 `browser_profiles` 表加 v16 两列(`live_keepalive_enabled`、`mute_audio`,INTEGER 0/1)并透传到 `Profile`/`ProfileInput`/DAO;身份池新增按 `platform` 过滤采样并从 `CreateBatch` 透传;静音在启动参数注入、保活循环改读每实例开关。前端标签/分组几乎纯前端接线(后端已具批量标签与分组 CRUD),批量弹窗加平台/保活/静音,标签管理页修 flex 滚动。

**Tech Stack:** Go 1.2x(纯 Go sqlite,`CGO_ENABLED=0`)、Wails v2、React + TS + Vite + Tailwind(CSS 变量主题)、`lucide-react` 图标。

## Global Constraints

- 交叉编译零 CGO:`CGO_ENABLED=0`;后端改动必须 `go build ./...` 且 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./backend/...` 通过。
- 身份**永不重复**由 DB 强制(`browser_identities` 的 `fingerprint_hash`/`seed` 部分唯一索引 + 运行时 `Seen`)——平台过滤不得绕过这条链路。
- 身份池 `platform` 仅 `windows`(1470)/ `macos`(822),**无 linux**;平台下拉只出「全部 / Windows / macOS」。
- 静音默认**开**(muted):v16 列 `DEFAULT 1`,已有环境下次启动也静音——符合"默认都关闭声音"。
- 保活默认**开**:v16 列 `DEFAULT 1`;全局 `a.config.Browser.LiveKeepAliveEnabled` 作总闸。
- Wails 绑定为**手动**重生成(dev 用 `-skipbindings`):凡改 App 方法签名/`ProfileInput` 字段,必须 `wails generate module` 并提交生成物 `frontend/src/wailsjs/`。
- 每完成一个可交付改动即 `git commit`;提交尾注 `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`;分支 `feature/fingerprint-self-consistency`。
- 全部实现合并后按既定流程重打含内核 Windows 安装包交付(服务器 Docker + 内核缓存)。
- 前端仓库无单元测试框架:前端任务以 `npm run build`(tsc+vite)+ 明确的手动验证为"测试";后端逻辑一律 Go TDD。

---

## 文件结构(改动地图)

**后端(Go):**
- `backend/internal/database/sqlite.go` — 迁移切片末尾加 v16(两列)。
- `backend/internal/browser/types.go` — `Profile` 加 2 字段;`ProfileInput` 加 3 字段(2 开关 + 1 透传平台)。
- `backend/internal/browser/profile_dao.go` — 7 处 SELECT、Upsert(INSERT/VALUES/SET/args)、`scanProfile` 传播两列;加 `boolToInt`。
- `backend/internal/browser/profile_create.go` — `createProfileLocked` 映射两开关默认值 + 用 `RegenerateForPlatform`。
- `backend/internal/browser/profile_update.go` — `Update` 仅在 input 指针非 nil 时改两开关。
- `backend/internal/browser/profile_create_batch.go` — `CreateBatch` 加 `platform` 形参,写入 `item.IdentityPlatform`。
- `backend/internal/browser/identity_service.go` — 加 `GenerateUniqueForPlatform` / `RegenerateForPlatform`。
- `backend/internal/identity/pool.go` — 加 `Pool.Filter`。
- `backend/app_browser_profile_api.go` — `BrowserProfileCreateBatch` 加 `platform` 形参。
- `backend/app_instance_start_prepare.go` — `prepareBrowserStartPlan` 注入 `--mute-audio`。
- `backend/app_live_keepalive.go` — `runKeepAliveDue` 加每实例开关判断。
- 新增测试:`backend/internal/identity/pool_filter_test.go`、`backend/internal/browser/profile_live_columns_test.go`、`backend/internal/browser/batch_platform_test.go`、`backend/internal/identity/platform_consistency_test.go`。

**前端(React/TS):**
- `frontend/src/modules/browser/api/profiles.ts` — `createBrowserProfileBatch` 加 `platform`。
- `frontend/src/modules/browser/components/BatchCreateModal.tsx` — 加 平台下拉 + 保活 + 静音。
- `frontend/src/modules/browser/pages/BrowserListPage.tsx` — `handleBatchCreate` 透传平台/开关;接线批量标签、分组工具条动作。
- `frontend/src/modules/browser/components/TagInlineCell.tsx`(新) — 列表标签紧凑显示 + 悬停展开 + 行内加删。
- `frontend/src/modules/browser/components/BrowserProfilesPanel.tsx` — 表格/卡片标签块换成 `TagInlineCell`。
- `frontend/src/modules/browser/components/BrowserListWidgets.tsx` — `BatchToolbar` 加「标签」「分组」入口。
- `frontend/src/modules/browser/components/GroupOpsModal.tsx`(新) — 移动到分组 + 管理分组(增删改)。
- `frontend/src/modules/browser/components/BatchTagModal.tsx`(新) — 批量加/删标签。
- `frontend/src/modules/browser/pages/BrowserEditPage.tsx` — 加「直播养号」区块(保活 + 静音开关)。
- `frontend/src/modules/browser/pages/TagManagementPage.tsx` — 修 flex 滚动 + 加条数行。

---

## 后端任务(Go TDD)

### Task 1: v16 迁移 + Profile/ProfileInput 两列 + DAO 传播

**Files:**
- Modify: `backend/internal/database/sqlite.go:240`(v15 之后追加 v16)
- Modify: `backend/internal/browser/types.go:11-42`(Profile)、`:45-58`(ProfileInput)
- Modify: `backend/internal/browser/profile_dao.go`(SELECT×7、Upsert、scanProfile)
- Modify: `backend/internal/browser/profile_create.go:57-102`
- Modify: `backend/internal/browser/profile_update.go:28-53`
- Test: `backend/internal/browser/profile_live_columns_test.go`(新)

**Interfaces:**
- Produces:
  - `Profile.LiveKeepAliveEnabled bool`(json `liveKeepaliveEnabled`)、`Profile.MuteAudio bool`(json `muteAudio`)
  - `ProfileInput.LiveKeepAliveEnabled *bool`、`ProfileInput.MuteAudio *bool`(json `...,omitempty`;nil=不改/取默认)
  - `boolToInt(b bool) int`(profile_dao.go 包内)

- [ ] **Step 1: 写失败测试** — `backend/internal/browser/profile_live_columns_test.go`

```go
package browser

import (
	"database/sql"
	"testing"

	"ant-chrome/backend/internal/database"
	_ "modernc.org/sqlite"
)

// 新建内存库并跑迁移,返回 DAO。
func newTestProfileDAO(t *testing.T) *SQLiteProfileDAO {
	t.Helper()
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLiteProfileDAO(db.GetConn())
}

func TestProfileLiveColumnsDefaultOnAndRoundTrip(t *testing.T) {
	dao := newTestProfileDAO(t)

	// 未显式设置 → 默认都开(保活+静音)。
	p := &Profile{ProfileId: "p1", ProfileName: "e1", LiveKeepAliveEnabled: true, MuteAudio: true}
	if err := dao.Upsert(p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := dao.GetById("p1")
	if err != nil {
		t.Fatalf("GetById: %v", err)
	}
	if !got.LiveKeepAliveEnabled || !got.MuteAudio {
		t.Fatalf("默认应为开: keepalive=%v mute=%v", got.LiveKeepAliveEnabled, got.MuteAudio)
	}

	// 关掉保活,静音保留 → 往返一致。
	got.LiveKeepAliveEnabled = false
	if err := dao.Upsert(got); err != nil {
		t.Fatalf("Upsert2: %v", err)
	}
	again, err := dao.GetById("p1")
	if err != nil {
		t.Fatalf("GetById2: %v", err)
	}
	if again.LiveKeepAliveEnabled {
		t.Fatalf("保活应为关")
	}
	if !again.MuteAudio {
		t.Fatalf("静音应仍为开")
	}
}

// 直接插入不含新列的历史行,读出应回退默认 1/1(COALESCE)。
func TestProfileLiveColumnsLegacyRowDefaults(t *testing.T) {
	dao := newTestProfileDAO(t)
	_, err := dao.db.Exec(`INSERT INTO browser_profiles
	  (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config,
	   memory_limit_mb, launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session, deleted_at)
	  VALUES ('old','oldname','old','', '[]','','',0,'[]','[]','[]','','2026-01-01','2026-01-01','','')`)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	got, err := dao.GetById("old")
	if err != nil {
		t.Fatalf("GetById: %v", err)
	}
	if !got.LiveKeepAliveEnabled || !got.MuteAudio {
		t.Fatalf("历史行应回退默认开: keepalive=%v mute=%v", got.LiveKeepAliveEnabled, got.MuteAudio)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/browser/ -run TestProfileLive -v`
Expected: 编译失败(`Profile` 无 `LiveKeepAliveEnabled`/`MuteAudio` 字段)或断言失败。

- [ ] **Step 3: 加 v16 迁移** — `backend/internal/database/sqlite.go`,在 v15 项(`:240` 的 `},`)之后、注释 `// ── 新版本在此追加` 之前插入:

```go
	{
		version: 16,
		desc:    "实例表添加直播保活与静音开关(默认开)",
		stmts: []string{
			`ALTER TABLE browser_profiles ADD COLUMN live_keepalive_enabled INTEGER NOT NULL DEFAULT 1`,
			`ALTER TABLE browser_profiles ADD COLUMN mute_audio INTEGER NOT NULL DEFAULT 1`,
		},
	},
```

- [ ] **Step 4: Profile / ProfileInput 加字段** — `backend/internal/browser/types.go`

`Profile` 结构体在 `GroupId` 字段(`:29`)之后加:
```go
	LiveKeepAliveEnabled bool `json:"liveKeepaliveEnabled"` // 直播保活开关,默认开
	MuteAudio            bool `json:"muteAudio"`            // 硬静音(--mute-audio),默认开
```
`ProfileInput` 结构体在 `GroupId` 字段(`:57`)之后加:
```go
	LiveKeepAliveEnabled *bool  `json:"liveKeepaliveEnabled,omitempty"` // nil=不改/取默认(开)
	MuteAudio            *bool  `json:"muteAudio,omitempty"`            // nil=不改/取默认(开)
	IdentityPlatform     string `json:"identityPlatform,omitempty"`     // 仅批量创建用:限定身份平台(""=全部),不落库
```

- [ ] **Step 5: DAO 传播两列** — `backend/internal/browser/profile_dao.go`

(a) **每一处 SELECT 列表**(共 7 处:`List`、`ListDeleted`、`GetById`、`ListByGroup` 的两个分支、`ListExpiredDeleted`;都以 `COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')` 结尾)——把这段结尾替换为:
```
COALESCE(restore_last_session, ''), COALESCE(deleted_at, ''),
COALESCE(live_keepalive_enabled, 1), COALESCE(mute_audio, 1)
```

(b) **Upsert**:INSERT 列清单(`:127`)在 `restore_last_session, deleted_at)` 前把结尾改为 `restore_last_session, deleted_at, live_keepalive_enabled, mute_audio)`;VALUES 占位符(`:128`)从 20 个 `?` 增加到 22 个;ON CONFLICT SET(`:146` 的 `deleted_at = excluded.deleted_at,` 之后)加:
```
			  live_keepalive_enabled = excluded.live_keepalive_enabled,
			  mute_audio       = excluded.mute_audio,
```
参数列表(`:152` 的 `... profile.DeletedAt,` 之后)加:
```go
			boolToInt(profile.LiveKeepAliveEnabled), boolToInt(profile.MuteAudio),
```

(c) **scanProfile**(`:313`):在函数顶部 var 块加两个 int 局部:
```go
	var liveKA, mute int
```
在 `Scan(...)` 参数末尾(`&p.RestoreLastSession, &p.DeletedAt,` 之后)加 `&liveKA, &mute,`;在 `return &p, nil` 前加:
```go
	p.LiveKeepAliveEnabled = liveKA != 0
	p.MuteAudio = mute != 0
```

(d) 文件末尾加辅助函数:
```go
// boolToInt 把 bool 存成 SQLite 的 0/1(driver 不支持 int64→bool 直扫,统一用 int 列)。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

- [ ] **Step 6: create 映射默认值** — `backend/internal/browser/profile_create.go`

在 `createProfileLocked` 组装 `profile := &Profile{...}` 时,`GroupId` 之后加两字段(默认开,input 指针非 nil 时覆盖):
```go
		LiveKeepAliveEnabled: input.LiveKeepAliveEnabled == nil || *input.LiveKeepAliveEnabled,
		MuteAudio:            input.MuteAudio == nil || *input.MuteAudio,
```
(即 nil→true,显式 false→false。)

- [ ] **Step 7: update 仅按需改** — `backend/internal/browser/profile_update.go`,在 `profile.GroupId = buildProfileGroupID(input.GroupId)`(`:52`)之后加:
```go
	if input.LiveKeepAliveEnabled != nil {
		profile.LiveKeepAliveEnabled = *input.LiveKeepAliveEnabled
	}
	if input.MuteAudio != nil {
		profile.MuteAudio = *input.MuteAudio
	}
```

- [ ] **Step 8: 跑测试确认通过**

Run: `cd backend && go test ./internal/browser/ -run TestProfileLive -v`
Expected: PASS(两个用例)。

- [ ] **Step 9: 提交**

```bash
cd /Users/damoguyansi/Documents/wrokspaces/workspaces-lc/zwbrowser/Ant-Browser-Plus
git add backend/internal/database/sqlite.go backend/internal/browser/types.go backend/internal/browser/profile_dao.go backend/internal/browser/profile_create.go backend/internal/browser/profile_update.go backend/internal/browser/profile_live_columns_test.go
git commit -m "feat(profile): v16 迁移加每实例保活/静音列(默认开)+ DAO/创建/更新传播

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: 启动注入 `--mute-audio`

**Files:**
- Modify: `backend/app_instance_start_prepare.go:166-168`
- Test: `backend/app_mute_inject_test.go`(新)

**Interfaces:**
- Consumes: `Profile.MuteAudio`(Task 1)

- [ ] **Step 1: 写失败测试** — `backend/app_mute_inject_test.go`

```go
package backend

import (
	"strings"
	"testing"
)

// buildBrowserLaunchArgs 已把 sanitizedExtraLaunchArgs 追加到命令行;
// 静音注入应把 --mute-audio 放进 extra 分支。这里直接验证 helper 组装结果包含它。
func TestBuildBrowserLaunchArgsCarriesMuteFromExtra(t *testing.T) {
	args := buildBrowserLaunchArgs("/tmp/ud", 9222, "", nil, nil, nil, []string{"--mute-audio"}, nil, false)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--mute-audio") {
		t.Fatalf("命令行应包含 --mute-audio,实际: %s", joined)
	}
}
```

- [ ] **Step 2: 跑测试确认通过前先确认基线** — 该测试验证既有 `buildBrowserLaunchArgs` 行为(extra 会进命令行),应直接 PASS,作为注入路径的回归护栏。

Run: `cd backend && go test . -run TestBuildBrowserLaunchArgsCarriesMute -v`
Expected: PASS。

- [ ] **Step 3: 注入静音** — `backend/app_instance_start_prepare.go`,在 MemorySaver 块(`:166-168`)之后、`return &browserStartPlan{` 之前加:

```go
	// 直播养号:默认硬静音,避免多开时上百路音频抢占声卡。--mute-audio 是启动参数,
	// 取消静音需重启该实例(前端「取消静音并重启」)。
	if profile.MuteAudio {
		sanitizedExtraLaunchArgs = append(sanitizedExtraLaunchArgs, "--mute-audio")
	}
```

- [ ] **Step 4: 编译 + 跑测试**

Run: `cd backend && go build ./... && go test . -run TestBuildBrowserLaunchArgsCarriesMute -v`
Expected: 编译通过,PASS。

- [ ] **Step 5: 提交**

```bash
git add backend/app_instance_start_prepare.go backend/app_mute_inject_test.go
git commit -m "feat(live): 每实例默认硬静音,启动注入 --mute-audio

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: 保活循环改读每实例开关

**Files:**
- Modify: `backend/app_live_keepalive.go:114-118`(`runKeepAliveDue` 循环体)
- Test: `backend/app_live_keepalive_perprofile_test.go`(新)

**Interfaces:**
- Consumes: `Profile.LiveKeepAliveEnabled`(Task 1);`a.browserMgr.List()` 返回完整 `Profile`。

- [ ] **Step 1: 写失败测试** — `backend/app_live_keepalive_perprofile_test.go`

```go
package backend

import "testing"

// keepAliveShouldInject 抽出"是否对该实例注入"的判定,便于单测(纯函数,不触发 CDP)。
func TestKeepAliveShouldInjectRespectsPerProfileSwitch(t *testing.T) {
	cases := []struct {
		name    string
		running bool
		ready   bool
		port    int
		enabled bool
		want    bool
	}{
		{"正常且开", true, true, 9222, true, true},
		{"每实例关", true, true, 9222, false, false},
		{"未就绪", true, false, 9222, true, false},
		{"未运行", false, true, 9222, true, false},
		{"无端口", true, true, 0, true, false},
	}
	for _, c := range cases {
		got := keepAliveShouldInject(c.running, c.ready, c.port, c.enabled)
		if got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test . -run TestKeepAliveShouldInject -v`
Expected: 编译失败(`keepAliveShouldInject` 未定义)。

- [ ] **Step 3: 抽出判定并在循环里使用每实例开关** — `backend/app_live_keepalive.go`

在文件加纯函数:
```go
// keepAliveShouldInject 判定某运行中实例本轮是否应注入保活:必须运行中、调试就绪、
// 有端口,且该实例保活开关为开。
func keepAliveShouldInject(running, ready bool, debugPort int, enabled bool) bool {
	return running && ready && debugPort > 0 && enabled
}
```
把 `runKeepAliveDue` 循环体开头的运行判断(`:115` 的 `if !p.Running || !p.DebugReady || p.DebugPort <= 0 { continue }`)替换为:
```go
		if !keepAliveShouldInject(p.Running, p.DebugReady, p.DebugPort, p.LiveKeepAliveEnabled) {
			continue
		}
```
(全局 `liveKeepAliveEnabledResolved()` 总闸在 tick 层 `:98` 保持不变。)

- [ ] **Step 4: 编译 + 跑测试**

Run: `cd backend && go build ./... && go test . -run TestKeepAliveShouldInject -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add backend/app_live_keepalive.go backend/app_live_keepalive_perprofile_test.go
git commit -m "feat(live): 保活改读每实例开关(默认开),全局配置降级为总闸

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: 身份池平台过滤采样 + CreateBatch 透传平台

**Files:**
- Modify: `backend/internal/identity/pool.go`(加 `Filter`)
- Modify: `backend/internal/browser/identity_service.go`(加 `GenerateUniqueForPlatform` / `RegenerateForPlatform`)
- Modify: `backend/internal/browser/profile_create.go:95`(改用 `RegenerateForPlatform`)
- Modify: `backend/internal/browser/profile_create_batch.go:22,45-49`(加 `platform` 形参)
- Modify: `backend/app_browser_profile_api.go:47-49`(绑定加 `platform`)
- Test: `backend/internal/identity/pool_filter_test.go`(新)、`backend/internal/browser/batch_platform_test.go`(新)

**Interfaces:**
- Produces:
  - `func (p *Pool) Filter(pred func(PoolRecord) bool) *Pool`
  - `func (s *IdentityService) GenerateUniqueForPlatform(platform string) (identity.Identity, error)`
  - `func (s *IdentityService) RegenerateForPlatform(profile *Profile, platform string) error`
  - `func (m *Manager) CreateBatch(prefix string, count, startIndex int, platform string, template ProfileInput) ([]*Profile, error)`
  - `func (a *App) BrowserProfileCreateBatch(prefix string, count int, startIndex int, platform string, template BrowserProfileInput) ([]*BrowserProfile, error)`
- Consumes: `ProfileInput.IdentityPlatform`(Task 1)、`identity.GenerateUnique`、`Pool.NewIdentity`。

- [ ] **Step 1: 写失败测试(pool.Filter)** — `backend/internal/identity/pool_filter_test.go`

```go
package identity

import (
	"math/rand"
	"testing"
)

func TestPoolFilterByPlatformOnlyYieldsTarget(t *testing.T) {
	recs := []PoolRecord{
		{Platform: "windows", UAFull: "w1", Weight: 1},
		{Platform: "macos", UAFull: "m1", Weight: 1},
		{Platform: "windows", UAFull: "w2", Weight: 1},
	}
	pool := NewPool(recs)
	win := pool.Filter(func(r PoolRecord) bool { return r.Platform == "windows" })
	if win.Len() != 2 {
		t.Fatalf("windows 子池应有 2 条,实际 %d", win.Len())
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		if got := win.Sample(r).Platform; got != "windows" {
			t.Fatalf("子池采样应只出 windows,得到 %s", got)
		}
	}
}

func TestPoolFilterEmptyResult(t *testing.T) {
	pool := NewPool([]PoolRecord{{Platform: "windows", Weight: 1}})
	got := pool.Filter(func(r PoolRecord) bool { return r.Platform == "linux" })
	if got.Len() != 0 {
		t.Fatalf("无匹配应得空池,实际 %d", got.Len())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/identity/ -run TestPoolFilter -v`
Expected: 编译失败(`Filter` 未定义)。

- [ ] **Step 3: 实现 Pool.Filter** — `backend/internal/identity/pool.go`,在 `Sample` 之前加:

```go
// Filter 返回只含满足 pred 的记录的新采样池(重算加权 total)。用于按平台等维度限定采样域;
// 唯一性登记仍由上层全局强制,过滤只改"从哪些真机模板里采"。
func (p *Pool) Filter(pred func(PoolRecord) bool) *Pool {
	filtered := make([]PoolRecord, 0, len(p.records))
	for _, r := range p.records {
		if pred(r) {
			filtered = append(filtered, r)
		}
	}
	return NewPool(filtered)
}
```

- [ ] **Step 4: 跑 pool 测试确认通过**

Run: `cd backend && go test ./internal/identity/ -run TestPoolFilter -v`
Expected: PASS。

- [ ] **Step 5: IdentityService 平台方法** — `backend/internal/browser/identity_service.go`

在 `GenerateUnique`(`:66`)之后加:
```go
// GenerateUniqueForPlatform 与 GenerateUnique 相同,但仅从指定平台的真机模板采样。
// platform 为空则等价 GenerateUnique(全平台)。目标平台无模板时返回错误。
func (s *IdentityService) GenerateUniqueForPlatform(platform string) (identity.Identity, error) {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return s.GenerateUnique()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pool := s.poolStore.Pool().Filter(func(r identity.PoolRecord) bool { return r.Platform == platform })
	if pool.Len() == 0 {
		return identity.Identity{}, fmt.Errorf("身份池中没有平台 %q 的模板", platform)
	}
	return identity.GenerateUnique(s.store, func() identity.Identity {
		return pool.NewIdentity(s.rng)
	}, 100)
}
```
把 `Regenerate`(`:99`)改为委托,并加平台版本:
```go
// Regenerate 全平台重生成(等价 RegenerateForPlatform(profile, ""))。
func (s *IdentityService) Regenerate(profile *Profile) error {
	return s.RegenerateForPlatform(profile, "")
}

// RegenerateForPlatform 强制为 profile 生成一套唯一身份(可限定平台),存库并刷新 FingerprintArgs。
func (s *IdentityService) RegenerateForPlatform(profile *Profile, platform string) error {
	if profile == nil {
		return nil
	}
	id, err := s.GenerateUniqueForPlatform(platform)
	if err != nil {
		return err
	}
	s.mu.Lock()
	err = s.store.Save(profile.ProfileId, id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	profile.FingerprintArgs = id.LaunchArgs()
	return nil
}
```
确认文件已 import `"fmt"`(未 import 则加)。

- [ ] **Step 6: createProfileLocked 用平台版本** — `backend/internal/browser/profile_create.go:95`,把
`if err := m.IdentityService.Regenerate(profile); err != nil {` 改为
`if err := m.IdentityService.RegenerateForPlatform(profile, input.IdentityPlatform); err != nil {`。

- [ ] **Step 7: CreateBatch 加 platform 形参** — `backend/internal/browser/profile_create_batch.go`

签名(`:22`)改为:
```go
func (m *Manager) CreateBatch(prefix string, count, startIndex int, platform string, template ProfileInput) ([]*Profile, error) {
```
在循环体设置 `item` 时(`:47` 附近,`item.UserDataDir = ""` 之后)加:
```go
		item.IdentityPlatform = platform // 限定该批身份平台(""=全部)
```

- [ ] **Step 8: 绑定加 platform** — `backend/app_browser_profile_api.go:47-49`:
```go
func (a *App) BrowserProfileCreateBatch(prefix string, count int, startIndex int, platform string, template BrowserProfileInput) ([]*BrowserProfile, error) {
	return a.browserMgr.CreateBatch(prefix, count, startIndex, platform, template)
}
```

- [ ] **Step 9: 写批量平台测试** — `backend/internal/browser/batch_platform_test.go`

```go
package browser

import (
	"testing"

	"ant-chrome/backend/internal/database"
	_ "modernc.org/sqlite"
)

// 组一个仅含 IdentityService 的最小 Manager,批量生成后校验平台一致且不重复。
func TestCreateBatchPlatformFilterUniqueAndConsistent(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	idSvc, err := NewIdentityService(db.GetConn(), "") // 空 overlay=内嵌池
	if err != nil {
		t.Fatalf("NewIdentityService: %v", err)
	}

	// 直接驱动 IdentityService:对每个 profile RegenerateForPlatform("macos")。
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		p := &Profile{ProfileId: "b" + string(rune('a'+i))}
		if err := idSvc.RegenerateForPlatform(p, "macos"); err != nil {
			t.Fatalf("RegenerateForPlatform: %v", err)
		}
		var hasMac bool
		for _, a := range p.FingerprintArgs {
			if a == "--fingerprint-platform=macos" {
				hasMac = true
			}
			if a == "--fingerprint-platform=windows" {
				t.Fatalf("macOS 批不应出现 windows 平台参数: %v", p.FingerprintArgs)
			}
		}
		if !hasMac {
			t.Fatalf("应含 --fingerprint-platform=macos: %v", p.FingerprintArgs)
		}
		key := p.FingerprintArgs[0] // --fingerprint=<seed> 在首位,seed 全局唯一
		if seen[key] {
			t.Fatalf("身份重复: %s", key)
		}
		seen[key] = true
	}
}
```

- [ ] **Step 10: 编译 + 跑测试**

Run: `cd backend && go build ./... && go test ./internal/browser/ -run TestCreateBatchPlatform -v && go test ./internal/identity/ -run TestPoolFilter -v`
Expected: PASS。

- [ ] **Step 11: 提交**

```bash
git add backend/internal/identity/pool.go backend/internal/identity/pool_filter_test.go backend/internal/browser/identity_service.go backend/internal/browser/profile_create.go backend/internal/browser/profile_create_batch.go backend/internal/browser/batch_platform_test.go backend/app_browser_profile_api.go
git commit -m "feat(identity): 按平台过滤采样 + CreateBatch 透传平台(全局去重不变)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: macOS 平台自洽守卫(锁定测试)

**Files:**
- Test: `backend/internal/identity/platform_consistency_test.go`(新)

**Interfaces:**
- Consumes: `EmbeddedPoolRecords`、`BuildIdentity`、`Identity.LaunchArgs`。

- [ ] **Step 1: 写测试** — `backend/internal/identity/platform_consistency_test.go`

```go
package identity

import (
	"strings"
	"testing"
)

// 锁定:每条 macos 模板生成的身份,LaunchArgs 恰有一个 --fingerprint-platform 且为 macos,
// 且 UA 含 Mac 标记 —— 平台/UA 自洽,不会与全局 windows 混淆。
func TestMacIdentitiesArePlatformConsistent(t *testing.T) {
	recs, err := EmbeddedPoolRecords()
	if err != nil {
		t.Fatalf("EmbeddedPoolRecords: %v", err)
	}
	checked := 0
	for _, r := range recs {
		if r.Platform != "macos" {
			continue
		}
		checked++
		args := BuildIdentity(r, 12345).LaunchArgs()
		var platCount int
		var uaFull string
		for _, a := range args {
			if strings.HasPrefix(a, "--fingerprint-platform=") {
				platCount++
				if a != "--fingerprint-platform=macos" {
					t.Fatalf("macos 模板平台参数错误: %s", a)
				}
			}
			if strings.HasPrefix(a, "--user-agent=") {
				uaFull = a
			}
		}
		if platCount != 1 {
			t.Fatalf("应恰有 1 个平台参数,实际 %d(rec=%s)", platCount, r.UAFull)
		}
		if uaFull != "" && !strings.Contains(uaFull, "Mac") {
			t.Fatalf("macos 身份 UA 应含 Mac: %s", uaFull)
		}
	}
	if checked == 0 {
		t.Skip("内嵌池无 macos 模板,跳过")
	}
}
```

- [ ] **Step 2: 跑测试**

Run: `cd backend && go test ./internal/identity/ -run TestMacIdentitiesArePlatformConsistent -v`
Expected: PASS(若失败,说明存在 UA↔平台不一致的历史 macos 模板,需在 `data/pool.json` 修正该条,而非改序列化)。

- [ ] **Step 3: 提交**

```bash
git add backend/internal/identity/platform_consistency_test.go
git commit -m "test(identity): 锁定 macOS 身份平台/UA 自洽(平台过滤守卫)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: 全量后端验证 + 重生成 Wails 绑定

**Files:**
- Modify(生成物): `frontend/src/wailsjs/go/main/App.{d.ts,js}`、`frontend/src/wailsjs/go/models.ts`

- [ ] **Step 1: 后端全量构建/测试/交叉编译**

Run:
```bash
cd backend && go vet ./... && go test ./... -count=1
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...
```
Expected: 全绿。

- [ ] **Step 2: 重生成绑定**

Run(在项目根):
```bash
cd /Users/damoguyansi/Documents/wrokspaces/workspaces-lc/zwbrowser/Ant-Browser-Plus
wails generate module
```
Expected: `frontend/src/wailsjs/go/main/App.d.ts` 中 `BrowserProfileCreateBatch` 变为 5 参(含 `platform string`);`models.ts` 的 `ProfileInput` 出现 `liveKeepaliveEnabled?`、`muteAudio?`、`identityPlatform?`,`Profile` 出现 `liveKeepaliveEnabled`、`muteAudio`。

- [ ] **Step 3: 提交生成物**

```bash
git add frontend/src/wailsjs
git commit -m "chore(bindings): 重生成 Wails 绑定(CreateBatch 平台参数 + 保活/静音字段)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## 前端任务(build + 手动验证)

### Task 7: api 层 createBrowserProfileBatch 加 platform

**Files:**
- Modify: `frontend/src/modules/browser/api/profiles.ts:196-217`(`createBrowserProfileBatch`)

**Interfaces:**
- Produces: `createBrowserProfileBatch(prefix, count, startIndex, platform, template)`

- [ ] **Step 1: 读现状再改** — 打开 `frontend/src/modules/browser/api/profiles.ts`,定位 `createBrowserProfileBatch`。它当前形如 `(prefix, count, startIndex, template)` 调 `bindings.BrowserProfileCreateBatch(prefix, count, startIndex, template)`。改为在两处都插入 `platform`:

```ts
export async function createBrowserProfileBatch(
  prefix: string,
  count: number,
  startIndex: number,
  platform: string,
  template: BrowserProfileInput,
): Promise<BrowserProfile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileCreateBatch) {
    return (await bindings.BrowserProfileCreateBatch(prefix, count, startIndex, platform, template)) || []
  }
  return []
}
```
(若 mock 分支/返回类型与现状不同,保持其风格,仅插入 `platform` 形参与实参。)

- [ ] **Step 2: 构建校验(会因调用点未改而报类型错——预期,下个任务修)**

Run: `cd frontend && npx tsc --noEmit`
Expected: 仅 `BrowserListPage.tsx` 调用 `createBrowserProfileBatch` 处参数不匹配报错(Task 9 修)。

- [ ] **Step 3: 暂不提交**,与 Task 8/9 一起提交(同一编译单元)。

---

### Task 8: 批量弹窗加 平台 / 保活 / 静音

**Files:**
- Modify: `frontend/src/modules/browser/components/BatchCreateModal.tsx`

**Interfaces:**
- Produces: `onSubmit(prefix, count, startIndex, platform, liveKeepaliveEnabled, muteAudio)`

- [ ] **Step 1: 改 props + 表单** — 用下面整体替换 `BatchCreateModal.tsx`:

```tsx
import { useMemo, useState } from 'react'

import { Button, FormItem, Input, Modal } from '../../../shared/components'

const MAX_BATCH = 200

type IdentityPlatform = '' | 'windows' | 'macos'

interface BatchCreateModalProps {
  open: boolean
  loading: boolean
  onClose: () => void
  onSubmit: (
    prefix: string,
    count: number,
    startIndex: number,
    platform: string,
    liveKeepaliveEnabled: boolean,
    muteAudio: boolean,
  ) => void
}

const pad3 = (n: number) => String(n).padStart(3, '0')

export function BatchCreateModal({ open, loading, onClose, onSubmit }: BatchCreateModalProps) {
  const [prefix, setPrefix] = useState('env')
  const [count, setCount] = useState(10)
  const [startIndex, setStartIndex] = useState(1)
  const [platform, setPlatform] = useState<IdentityPlatform>('')
  const [liveKeepalive, setLiveKeepalive] = useState(true)
  const [muteAudio, setMuteAudio] = useState(true)

  const trimmedPrefix = prefix.trim()
  const safeCount = Math.min(Math.max(1, Math.floor(count) || 0), MAX_BATCH)
  const safeStart = Math.max(1, Math.floor(startIndex) || 1)

  const preview = useMemo(() => {
    if (!trimmedPrefix) return ''
    const first = `${trimmedPrefix}-${pad3(safeStart)}`
    if (safeCount === 1) return first
    return `${first} ~ ${trimmedPrefix}-${pad3(safeStart + safeCount - 1)}`
  }, [trimmedPrefix, safeCount, safeStart])

  const canSubmit = trimmedPrefix.length > 0 && safeCount >= 1 && !loading

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="批量新建配置"
      width="480px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={loading}>取消</Button>
          <Button
            onClick={() => onSubmit(trimmedPrefix, safeCount, safeStart, platform, liveKeepalive, muteAudio)}
            loading={loading}
            disabled={!canSubmit}
          >
            创建 {safeCount} 个
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="名称前缀" required>
          <Input value={prefix} onChange={(event) => setPrefix(event.target.value)} placeholder="env" />
        </FormItem>
        <div className="grid grid-cols-2 gap-4">
          <FormItem label="数量" hint={`最多 ${MAX_BATCH} 个`}>
            <Input
              type="number"
              min={1}
              max={MAX_BATCH}
              value={count}
              onChange={(event) => setCount(Number(event.target.value))}
            />
          </FormItem>
          <FormItem label="起始编号">
            <Input
              type="number"
              min={1}
              value={startIndex}
              onChange={(event) => setStartIndex(Number(event.target.value))}
            />
          </FormItem>
        </div>

        <FormItem label="身份平台" hint="选定后本批全部生成该平台身份;任何情况都不重复">
          <select
            value={platform}
            onChange={(event) => setPlatform(event.target.value as IdentityPlatform)}
            className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]"
          >
            <option value="">全部平台</option>
            <option value="windows">Windows</option>
            <option value="macos">macOS</option>
          </select>
        </FormItem>

        <div className="flex items-center gap-6">
          <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
            <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={liveKeepalive} onChange={(e) => setLiveKeepalive(e.target.checked)} />
            开启直播保活
          </label>
          <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
            <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={muteAudio} onChange={(e) => setMuteAudio(e.target.checked)} />
            默认静音
          </label>
        </div>

        <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm">
          <div className="mb-1 text-[var(--color-text-muted)]">名称预览(编号 3 位)</div>
          <div className="font-mono text-[var(--color-text-primary)]">{preview || '请输入前缀'}</div>
          <div className="mt-2 text-xs text-[var(--color-text-muted)]">
            每个环境都会获得独立、唯一、自洽的指纹身份。大批量会占用较多内存,请按需创建。
          </div>
        </div>
      </div>
    </Modal>
  )
}
```

- [ ] **Step 2: 暂不单独构建**,随 Task 9 一起校验。

---

### Task 9: 列表页透传平台/开关到批量创建

**Files:**
- Modify: `frontend/src/modules/browser/pages/BrowserListPage.tsx:540-554`(`handleBatchCreate`)、`:619-623`(挂载 `BatchCreateModal` 的 props)

**Interfaces:**
- Consumes: `createBrowserProfileBatch(...5 参...)`(Task 7)、`BatchCreateModal.onSubmit`(Task 8)

- [ ] **Step 1: 读现状再改** — 打开 `BrowserListPage.tsx`,定位 `handleBatchCreate`(约 `:540`)。它当前形如 `(prefix, count, startIndex) => { const template = {...空..., proxyId: directProxyID}; await createBrowserProfileBatch(prefix, count, startIndex, template) }`。改成接收平台/两开关,写进 template 并透传 platform:

```tsx
  const handleBatchCreate = async (
    prefix: string,
    count: number,
    startIndex: number,
    platform: string,
    liveKeepaliveEnabled: boolean,
    muteAudio: boolean,
  ) => {
    // ...保持原有 setBatchCreating(true)/try 结构...
    const template: BrowserProfileInput = {
      // ...保持原有其余字段(proxyId: directProxyID 等)...
      liveKeepaliveEnabled,
      muteAudio,
    }
    const created = await createBrowserProfileBatch(prefix, count, startIndex, platform, template)
    // ...保持原有成功提示 / 刷新列表 / 关闭弹窗...
  }
```
(仅新增 `platform`/两开关形参与 template 两字段;其余保留。若 `BrowserProfileInput` 类型未含新字段,确认 Task 6 已重生成 `models.ts`。)

- [ ] **Step 2: 全量前端类型检查 + 构建**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过(Task 7/8/9 闭合)。

- [ ] **Step 3: 提交(Task 7+8+9 合并)**

```bash
git add frontend/src/modules/browser/api/profiles.ts frontend/src/modules/browser/components/BatchCreateModal.tsx frontend/src/modules/browser/pages/BrowserListPage.tsx
git commit -m "feat(batch): 批量创建加平台选择 + 全部保活/静音默认

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: 列表标签紧凑显示 + 悬停展开 + 行内加删

**Files:**
- Create: `frontend/src/modules/browser/components/TagInlineCell.tsx`
- Modify: `frontend/src/modules/browser/components/BrowserProfilesPanel.tsx:476-481`(表格标签块)、`:335-339`(卡片标签块)

**Interfaces:**
- Consumes: `batchSetProfileTags(ids, tags, replace)`、`batchRemoveProfileTags(ids, tags)`、`fetchAllTags()`(均在 `../api`)
- Produces: `<TagInlineCell tags={string[]} profileId={string} allTags={string[]} onChanged={() => void} />`

- [ ] **Step 1: 新建 TagInlineCell** — `frontend/src/modules/browser/components/TagInlineCell.tsx`

```tsx
import { useState } from 'react'
import { Plus, X } from 'lucide-react'

import { toast } from '../../../shared/components'
import { batchRemoveProfileTags, batchSetProfileTags } from '../api'

interface TagInlineCellProps {
  tags: string[]
  profileId: string
  allTags: string[]
  onChanged: () => void
  maxVisible?: number
}

// 列表内标签单元:等高显示前 maxVisible 个,其余悬停 +N 展开;支持行内加/删。
export function TagInlineCell({ tags, profileId, allTags, onChanged, maxVisible = 2 }: TagInlineCellProps) {
  const [open, setOpen] = useState(false)
  const [adding, setAdding] = useState(false)
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)

  const list = tags ?? []
  const visible = list.slice(0, maxVisible)
  const hiddenCount = Math.max(0, list.length - maxVisible)
  const suggestions = allTags.filter((t) => !list.includes(t) && t.includes(input.trim())).slice(0, 8)

  const addTag = async (raw: string) => {
    const tag = raw.trim()
    if (!tag || list.includes(tag)) { setInput(''); setAdding(false); return }
    setBusy(true)
    try {
      await batchSetProfileTags([profileId], [tag], false)
      setInput('')
      setAdding(false)
      onChanged()
    } catch (e: any) {
      toast.error(e?.message || '添加标签失败')
    } finally { setBusy(false) }
  }

  const removeTag = async (tag: string) => {
    setBusy(true)
    try {
      await batchRemoveProfileTags([profileId], [tag])
      onChanged()
    } catch (e: any) {
      toast.error(e?.message || '删除标签失败')
    } finally { setBusy(false) }
  }

  return (
    <div className="relative flex items-center gap-1 min-w-0" onMouseLeave={() => setOpen(false)}>
      <div className="flex items-center gap-1 overflow-hidden">
        {visible.map((t) => (
          <span key={t} className="inline-flex items-center rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)] max-w-[96px] truncate">{t}</span>
        ))}
        {hiddenCount > 0 && (
          <button
            type="button"
            onMouseEnter={() => setOpen(true)}
            onClick={() => setOpen((v) => !v)}
            className="rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-accent)] shrink-0"
          >+{hiddenCount}</button>
        )}
        {list.length === 0 && <span className="text-xs text-[var(--color-text-muted)]">无标签</span>}
      </div>

      <button
        type="button"
        title="添加标签"
        onClick={() => setAdding((v) => !v)}
        disabled={busy}
        className="shrink-0 p-0.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-accent)] disabled:opacity-50"
      ><Plus className="w-3.5 h-3.5" /></button>

      {/* 悬停/点击展开:全部标签 + 逐个删除 */}
      {open && list.length > 0 && (
        <div className="absolute z-20 top-full left-0 mt-1 max-w-[280px] rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl">
          <div className="flex flex-wrap gap-1">
            {list.map((t) => (
              <span key={t} className="inline-flex items-center gap-1 rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)]">
                {t}
                <button type="button" onClick={() => removeTag(t)} disabled={busy} className="hover:text-red-500 disabled:opacity-50"><X className="w-3 h-3" /></button>
              </span>
            ))}
          </div>
        </div>
      )}

      {/* 行内新增输入 */}
      {adding && (
        <div className="absolute z-20 top-full left-0 mt-1 w-56 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl">
          <input
            autoFocus
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') addTag(input); if (e.key === 'Escape') setAdding(false) }}
            placeholder="输入标签,回车添加"
            className="w-full px-2 py-1 text-xs rounded border border-[var(--color-accent)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none"
          />
          {suggestions.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-1">
              {suggestions.map((s) => (
                <button key={s} type="button" onClick={() => addTag(s)} className="rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-accent)]">{s}</button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: 表格标签块换成组件** — `BrowserProfilesPanel.tsx`,把标签列渲染(约 `:476-481` 的 `record.tags.slice(0, 2)` + `+N` 那段)替换为:
```tsx
<TagInlineCell
  tags={record.tags}
  profileId={record.profileId}
  allTags={allTags}
  onChanged={onReloadProfiles}
/>
```
`allTags` / `onReloadProfiles` 若组件当前无此 prop:在 `BrowserProfilesPanel` 的 props 接口加 `allTags: string[]` 与 `onReloadProfiles: () => void`,并从父级 `BrowserListPage` 传入(列表已有全部标签派生值 `allTags` 与列表刷新函数,直接透传;找不到现成刷新函数时用加载 profiles 的那个)。文件顶部 `import { TagInlineCell } from './TagInlineCell'`。

- [ ] **Step 3: 卡片标签块同样收敛** — 把卡片视图标签块(约 `:335-339` 全量渲染 `tags`)替换为同一个 `<TagInlineCell .../>`(props 同上)。

- [ ] **Step 4: 构建校验**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/modules/browser/components/TagInlineCell.tsx frontend/src/modules/browser/components/BrowserProfilesPanel.tsx frontend/src/modules/browser/pages/BrowserListPage.tsx
git commit -m "feat(tags): 列表标签紧凑显示+悬停展开全部+行内加删

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: 批量工具条加「标签」批量加删

**Files:**
- Create: `frontend/src/modules/browser/components/BatchTagModal.tsx`
- Modify: `frontend/src/modules/browser/components/BrowserListWidgets.tsx`(`BatchToolbar` 加按钮 + props)
- Modify: `frontend/src/modules/browser/pages/BrowserListPage.tsx`(挂载弹窗 + 传 `selectedIds`/`allTags`/刷新)

**Interfaces:**
- Consumes: `batchSetProfileTags`、`batchRemoveProfileTags`、`fetchAllTags`
- Produces: `<BatchTagModal open profileIds allTags onClose onDone />`;`BatchToolbar` 新增 `onOpenTags: () => void`

- [ ] **Step 1: 新建 BatchTagModal** — `frontend/src/modules/browser/components/BatchTagModal.tsx`

```tsx
import { useState } from 'react'

import { Button, Modal, toast } from '../../../shared/components'
import { batchRemoveProfileTags, batchSetProfileTags } from '../api'

interface BatchTagModalProps {
  open: boolean
  profileIds: string[]
  allTags: string[]
  onClose: () => void
  onDone: () => void
}

export function BatchTagModal({ open, profileIds, allTags, onClose, onDone }: BatchTagModalProps) {
  const [addInput, setAddInput] = useState('')
  const [removeTag, setRemoveTag] = useState('')
  const [busy, setBusy] = useState(false)

  const doAdd = async () => {
    const tags = addInput.split(/[,，\s]+/).map((t) => t.trim()).filter(Boolean)
    if (!tags.length) return
    setBusy(true)
    try {
      await batchSetProfileTags(profileIds, tags, false)
      toast.success(`已为 ${profileIds.length} 个实例添加标签`)
      setAddInput('')
      onDone()
    } catch (e: any) { toast.error(e?.message || '添加失败') } finally { setBusy(false) }
  }

  const doRemove = async () => {
    if (!removeTag) return
    setBusy(true)
    try {
      await batchRemoveProfileTags(profileIds, [removeTag])
      toast.success(`已从 ${profileIds.length} 个实例移除标签`)
      setRemoveTag('')
      onDone()
    } catch (e: any) { toast.error(e?.message || '移除失败') } finally { setBusy(false) }
  }

  return (
    <Modal open={open} onClose={onClose} title={`批量标签(已选 ${profileIds.length} 个)`} width="420px"
      footer={<Button variant="secondary" onClick={onClose} disabled={busy}>关闭</Button>}>
      <div className="space-y-4">
        <div>
          <div className="mb-1 text-sm text-[var(--color-text-secondary)]">添加标签(逗号分隔)</div>
          <div className="flex gap-2">
            <input value={addInput} onChange={(e) => setAddInput(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && doAdd()}
              placeholder="如:养号,抖音" className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]" />
            <Button size="sm" onClick={doAdd} loading={busy} disabled={!addInput.trim()}>添加</Button>
          </div>
        </div>
        {allTags.length > 0 && (
          <div>
            <div className="mb-1 text-sm text-[var(--color-text-secondary)]">移除标签</div>
            <div className="flex gap-2">
              <select value={removeTag} onChange={(e) => setRemoveTag(e.target.value)}
                className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]">
                <option value="">选择要移除的标签</option>
                {allTags.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
              <Button size="sm" variant="secondary" onClick={doRemove} loading={busy} disabled={!removeTag}>移除</Button>
            </div>
          </div>
        )}
      </div>
    </Modal>
  )
}
```

- [ ] **Step 2: BatchToolbar 加「标签」按钮** — `BrowserListWidgets.tsx`,在 `BatchToolbarProps` 加 `onOpenTags: () => void`,解构参数加 `onOpenTags`,并在「导出」按钮之前插入:
```tsx
        <Button size="sm" variant="secondary" onClick={onOpenTags} title="批量加/删标签">
          <Tag className="w-3.5 h-3.5" />标签
        </Button>
```
文件顶部 `lucide-react` import 加 `Tag`。

- [ ] **Step 3: 列表页挂载** — `BrowserListPage.tsx`:加 `const [tagModalOpen, setTagModalOpen] = useState(false)`;给 `BatchToolbar` 传 `onOpenTags={() => setTagModalOpen(true)}`;在页面 JSX 末尾挂 `<BatchTagModal open={tagModalOpen} profileIds={Array.from(selectedIds)} allTags={allTags} onClose={() => setTagModalOpen(false)} onDone={() => { setTagModalOpen(false); /*调用现有列表刷新*/ }} />`(`selectedIds` 若是 Set 用 `Array.from`;是数组直接用)。顶部 import `BatchTagModal`。

- [ ] **Step 4: 构建校验**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/modules/browser/components/BatchTagModal.tsx frontend/src/modules/browser/components/BrowserListWidgets.tsx frontend/src/modules/browser/pages/BrowserListPage.tsx
git commit -m "feat(tags): 批量工具条加标签批量加删

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: 批量工具条加「分组」(移动 + 管理)

**Files:**
- Create: `frontend/src/modules/browser/components/GroupOpsModal.tsx`
- Modify: `frontend/src/modules/browser/components/BrowserListWidgets.tsx`(`BatchToolbar` 加「分组」按钮 + prop)
- Modify: `frontend/src/modules/browser/pages/BrowserListPage.tsx`(挂载弹窗)

**Interfaces:**
- Consumes: `fetchGroups`、`createGroup`、`updateGroup`、`deleteGroup`、`moveInstancesToGroup`(`../api`)
- Produces: `<GroupOpsModal open profileIds onClose onDone />`;`BatchToolbar` 新增 `onOpenGroups: () => void`

- [ ] **Step 1: 新建 GroupOpsModal** — `frontend/src/modules/browser/components/GroupOpsModal.tsx`

```tsx
import { useEffect, useState } from 'react'
import { Plus, Trash2, Check, FolderInput } from 'lucide-react'

import { Button, Modal, toast } from '../../../shared/components'
import type { BrowserGroupWithCount } from '../types'
import { createGroup, deleteGroup, fetchGroups, moveInstancesToGroup, updateGroup } from '../api'

interface GroupOpsModalProps {
  open: boolean
  profileIds: string[]
  onClose: () => void
  onDone: () => void
}

// 分组入口:上半"移动选中环境到某分组/未分组/新建",下半"管理分组(改名/删除/新建)"。
export function GroupOpsModal({ open, profileIds, onClose, onDone }: GroupOpsModalProps) {
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [busy, setBusy] = useState(false)
  const [newName, setNewName] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')

  const load = async () => { setGroups(await fetchGroups()) }
  useEffect(() => { if (open) void load() }, [open])

  const move = async (groupId: string) => {
    if (!profileIds.length) { toast.error('请先勾选环境'); return }
    setBusy(true)
    try {
      await moveInstancesToGroup(profileIds, groupId)
      toast.success(groupId ? '已移动到分组' : '已移出分组')
      onDone()
    } catch (e: any) { toast.error(e?.message || '移动失败') } finally { setBusy(false) }
  }

  const addGroup = async (moveAfter: boolean) => {
    const name = newName.trim()
    if (!name) return
    setBusy(true)
    try {
      const g = await createGroup({ groupName: name, parentId: '', sortOrder: 0 })
      setNewName('')
      await load()
      if (moveAfter && g?.groupId) await move(g.groupId)
      else toast.success('分组已创建')
    } catch (e: any) { toast.error(e?.message || '创建失败') } finally { setBusy(false) }
  }

  const rename = async (groupId: string) => {
    const name = editName.trim()
    if (!name) { setEditingId(null); return }
    setBusy(true)
    try {
      await updateGroup(groupId, { groupName: name, parentId: '', sortOrder: 0 })
      setEditingId(null)
      await load()
    } catch (e: any) { toast.error(e?.message || '重命名失败') } finally { setBusy(false) }
  }

  const remove = async (groupId: string) => {
    setBusy(true)
    try {
      await deleteGroup(groupId)
      await load()
    } catch (e: any) { toast.error(e?.message || '删除失败') } finally { setBusy(false) }
  }

  return (
    <Modal open={open} onClose={onClose} title={`分组(已选 ${profileIds.length} 个)`} width="480px"
      footer={<Button variant="secondary" onClick={onClose} disabled={busy}>关闭</Button>}>
      <div className="space-y-5">
        <div>
          <div className="mb-2 text-sm font-medium text-[var(--color-text-secondary)]">移动选中环境到</div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="secondary" onClick={() => move('')} disabled={busy || !profileIds.length}>
              <FolderInput className="w-3.5 h-3.5" />未分组(移出)
            </Button>
            {groups.map((g) => (
              <Button key={g.groupId} size="sm" variant="secondary" onClick={() => move(g.groupId)} disabled={busy || !profileIds.length}>
                {g.groupName} <span className="opacity-60">({g.instanceCount})</span>
              </Button>
            ))}
          </div>
          <div className="mt-2 flex gap-2">
            <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="新建分组名"
              className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]" />
            <Button size="sm" onClick={() => addGroup(true)} loading={busy} disabled={!newName.trim()}>
              <Plus className="w-3.5 h-3.5" />新建并移入
            </Button>
          </div>
        </div>

        <div className="border-t border-[var(--color-border-default)] pt-3">
          <div className="mb-2 text-sm font-medium text-[var(--color-text-secondary)]">管理分组</div>
          <div className="space-y-1.5 max-h-56 overflow-y-auto">
            {groups.length === 0 && <div className="text-xs text-[var(--color-text-muted)]">暂无分组</div>}
            {groups.map((g) => (
              <div key={g.groupId} className="flex items-center gap-2">
                {editingId === g.groupId ? (
                  <input autoFocus value={editName} onChange={(e) => setEditName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') rename(g.groupId); if (e.key === 'Escape') setEditingId(null) }}
                    className="flex-1 px-2 py-1 text-xs rounded border border-[var(--color-accent)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none" />
                ) : (
                  <button className="flex-1 text-left text-sm text-[var(--color-text-primary)]" onClick={() => { setEditingId(g.groupId); setEditName(g.groupName) }}>
                    {g.groupName} <span className="text-xs opacity-60">({g.instanceCount})</span>
                  </button>
                )}
                {editingId === g.groupId ? (
                  <button onClick={() => rename(g.groupId)} className="p-1 text-[var(--color-accent)]"><Check className="w-4 h-4" /></button>
                ) : (
                  <button onClick={() => remove(g.groupId)} disabled={busy} className="p-1 text-[var(--color-text-muted)] hover:text-red-500 disabled:opacity-50"><Trash2 className="w-4 h-4" /></button>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </Modal>
  )
}
```

- [ ] **Step 2: BatchToolbar 加「分组」按钮** — `BrowserListWidgets.tsx`,`BatchToolbarProps` 加 `onOpenGroups: () => void`,解构加 `onOpenGroups`,在「标签」按钮之后插入:
```tsx
        <Button size="sm" variant="secondary" onClick={onOpenGroups} title="移动到分组 / 管理分组">
          <FolderInput className="w-3.5 h-3.5" />分组
        </Button>
```
`lucide-react` import 加 `FolderInput`。

- [ ] **Step 3: 列表页挂载** — `BrowserListPage.tsx`:加 `const [groupModalOpen, setGroupModalOpen] = useState(false)`;`BatchToolbar` 传 `onOpenGroups={() => setGroupModalOpen(true)}`;JSX 末尾挂 `<GroupOpsModal open={groupModalOpen} profileIds={Array.from(selectedIds)} onClose={() => setGroupModalOpen(false)} onDone={() => { setGroupModalOpen(false); /*刷新列表+分组筛选数据*/ }} />`。移动/新建分组后需刷新顶部分组筛选来源(调用 `useBrowserListData` 暴露的分组重载,如无则同时刷新 profiles 使计数更新)。顶部 import `GroupOpsModal`。

- [ ] **Step 4: 构建校验**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/modules/browser/components/GroupOpsModal.tsx frontend/src/modules/browser/components/BrowserListWidgets.tsx frontend/src/modules/browser/pages/BrowserListPage.tsx
git commit -m "feat(group): 批量工具条加分组入口(移动/移出/新建/改名/删除)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: 编辑页「直播养号」区块(保活 + 静音)

**Files:**
- Modify: `frontend/src/modules/browser/pages/BrowserEditPage.tsx:665-681`(标签/分组字段附近)

**Interfaces:**
- Consumes: 表单 state 需含 `liveKeepaliveEnabled`、`muteAudio`,并在提交的 `BrowserProfileInput` 里带上。

- [ ] **Step 1: 读现状再改** — 打开 `BrowserEditPage.tsx`,定位表单 state 初始化(会从 `profile` 载入各字段)与提交时组装 `BrowserProfileInput` 的位置。加入两个布尔 state,默认取 `profile?.liveKeepaliveEnabled ?? true` / `profile?.muteAudio ?? true`。在标签/分组字段(`:665-681`)附近插入一段:

```tsx
<div className="rounded-lg border border-[var(--color-border-default)] p-3 space-y-2">
  <div className="text-sm font-medium text-[var(--color-text-secondary)]">直播养号</div>
  <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
    <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={liveKeepalive} onChange={(e) => setLiveKeepalive(e.target.checked)} />
    开启直播保活(防"长时间无操作已暂停")
  </label>
  <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
    <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={muteAudio} onChange={(e) => setMuteAudio(e.target.checked)} />
    静音(取消静音需重启该实例)
  </label>
</div>
```
提交组装 `BrowserProfileInput` 处加 `liveKeepaliveEnabled: liveKeepalive, muteAudio: muteAudio`。

- [ ] **Step 2: 构建校验**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 3: 提交**

```bash
git add frontend/src/modules/browser/pages/BrowserEditPage.tsx
git commit -m "feat(live): 环境编辑页加直播养号区块(保活/静音开关)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 14: 修复标签管理页滚动 + 条数(#6)

**Files:**
- Modify: `frontend/src/modules/browser/pages/TagManagementPage.tsx:332,343,67`

**Interfaces:** 纯样式/展示,无接口变更。

- [ ] **Step 1: 右侧内容列 + 表格 Card 加 `min-h-0`**
  - `:332` 右侧内容列 `className="flex-1 flex flex-col overflow-hidden pl-5 gap-4"` → 末尾加 `min-h-0`,即 `"... gap-4 min-h-0"`。
  - `:343` 表格 Card `className="flex-1 overflow-hidden"` → 改为 `"flex-1 overflow-hidden min-h-0"`。

- [ ] **Step 2: 左侧 TagPanel 滚动列表加 `min-h-0`** — `:67` `<div className="flex-1 overflow-y-auto py-2">` → `"flex-1 overflow-y-auto py-2 min-h-0"`。

- [ ] **Step 3: 表格上方加条数行** — 在 ActionBar 与表格 Card 之间(约 `:341`)插入:
```tsx
<div className="text-xs text-[var(--color-text-muted)]">
  共 {displayProfiles.length} 个实例 · 已选 {selectedIds.size}
</div>
```

- [ ] **Step 4: 构建校验**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/modules/browser/pages/TagManagementPage.tsx
git commit -m "fix(tags): 标签管理页表格可滚动 + 显示实例条数(min-h-0)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: 全量验证 + 端到端 + 交付

**Files:** 无(验证与打包)

- [ ] **Step 1: 后端全绿**

Run: `cd backend && go vet ./... && go test ./... -count=1 && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`
Expected: 全绿。

- [ ] **Step 2: 前端全绿**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: 通过。

- [ ] **Step 3: 重启 app 端到端手测(逐项)**
  - 标签:列表标签等高、`+N` 悬停展开全部、行内 `＋`/`×` 加删即时生效;批量工具条「标签」批量加删;标签管理页可上下滚动、显示"共 N 个实例"。
  - 保活:编辑页关某实例保活 → 该实例不再被注入(日志 KeepAlive 无该 profile);批量创建"开启直播保活"生效。
  - 静音:新建默认静音;编辑页关静音 + 重启该实例 → 出声。
  - 分组:工具条「分组」新建/改名/删除、批量移动到分组、移出分组;顶部分组筛选联动。
  - 批量创建:选 Windows / macOS 各建 5 个;CDP 抽验(`/api/launch` → CDP `Network.getUserAgent` 或页面 `navigator.userAgent` + `--fingerprint-platform`)平台/UA/品牌自洽;身份互不重复;已有环境不变。

- [ ] **Step 4: 打含内核 Windows 安装包并交付**(按 `docs/superpowers/specs` 交付配方 / 记忆 `zwbrowser-delivery-workflow`):mac 重建 exe → 服务器 Docker makensis(内核缓存命中)→ scp 回 `dist/` → `SendUserFile` 交付,附 sha256。

- [ ] **Step 5: push**

```bash
cd /Users/damoguyansi/Documents/wrokspaces/workspaces-lc/zwbrowser/Ant-Browser-Plus
git push
```

---

## Self-Review(计划对照 spec)

**1. Spec coverage**
- ① 标签显示/行内/批量 → Task 10(显示+行内)、Task 11(批量);标签管理页 → Task 14。✅
- ② 每环境保活开关 + 批量总开关 → Task 1(列)、Task 3(循环)、Task 8/9(批量)、Task 13(编辑页)。✅
- ③ 默认硬静音 + 可重启取消 → Task 1(列)、Task 2(注入)、Task 8/9(批量默认)、Task 13(编辑页 + 重启说明)。✅
- ④ 分组操作进工具条(新建/改名/删/批量移动/移出) → Task 12;顶部筛选保留(不改)。✅
- ⑤ 批量平台选择 + 永不重复 + 自洽守卫 → Task 4(过滤+透传)、Task 5(守卫测试)、Task 8/9(前端)。✅
- ⑥ 标签管理页滚动+条数 → Task 14。✅
- 绑定重生成 → Task 6;交付 → Task 15。✅

**2. Placeholder scan**:后端步骤均含完整代码/命令;前端 3 处"读现状再改"(profiles.ts、BrowserListPage.handleBatchCreate、BrowserEditPage)给了目标代码与精确锚点,非空泛占位。✅

**3. Type consistency**:`liveKeepaliveEnabled`/`muteAudio`(Profile bool / ProfileInput *bool)、`identityPlatform`、`CreateBatch(...platform...)`、`createBrowserProfileBatch(...platform...)`、`RegenerateForPlatform`、`GenerateUniqueForPlatform`、`Pool.Filter`、`boolToInt`、`keepAliveShouldInject`、组件 `TagInlineCell`/`BatchTagModal`/`GroupOpsModal` 命名跨任务一致。✅

**风险**:前端 `BrowserListPage`/`BrowserProfilesPanel` 的 prop 线路(`allTags`、刷新函数、`selectedIds` 是 Set 还是数组)需执行时按实际代码接线——已在对应步骤标注"读现状"。分组移动后顶部筛选计数刷新依赖 `useBrowserListData` 的重载,执行时确认其暴露方式。
