package browser

import (
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	_ "modernc.org/sqlite"
)

// 新建内存库并跑迁移，返回 DAO。
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

// 复制实例(Manager.Copy 真实路径)必须继承源实例的保活/静音设置,
// 而不是回退成 Profile{} 的 zero value(=false/false,反转了"默认开"的要求)。
// 覆盖开/关两种取值,确保是"继承"而非硬编码常量。
func TestCopyProfileInheritsLiveKeepAliveAndMuteAudio(t *testing.T) {
	cases := []struct {
		name      string
		keepAlive bool
		muteAudio bool
	}{
		{"both_on_default", true, true},
		{"both_off", false, false},
		{"mixed", true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewManager(&config.Config{}, t.TempDir())
			src := &Profile{
				ProfileId:            "src",
				ProfileName:          "源实例",
				LiveKeepAliveEnabled: tc.keepAlive,
				MuteAudio:            tc.muteAudio,
			}
			manager.Profiles[src.ProfileId] = src

			copied, err := manager.Copy(src.ProfileId, "")
			if err != nil {
				t.Fatalf("Copy: %v", err)
			}
			if copied.LiveKeepAliveEnabled != tc.keepAlive {
				t.Fatalf("LiveKeepAliveEnabled = %v, want %v", copied.LiveKeepAliveEnabled, tc.keepAlive)
			}
			if copied.MuteAudio != tc.muteAudio {
				t.Fatalf("MuteAudio = %v, want %v", copied.MuteAudio, tc.muteAudio)
			}
		})
	}
}
