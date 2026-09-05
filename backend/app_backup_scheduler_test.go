package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestBackupScheduleMatchesTime(t *testing.T) {
	now := time.Date(2026, time.August, 26, 2, 3, 0, 0, time.Local)
	if !backupScheduleMatchesTime("02:03", now) {
		t.Fatal("schedule should match the configured minute")
	}
	if backupScheduleMatchesTime("02:04", now) {
		t.Fatal("schedule should not match a different minute")
	}
	if backupScheduleMatchesTime("2:03", now) {
		t.Fatal("schedule should require HH:MM format")
	}
}

func TestBackupScheduledSaveSettingsStoresTokenLocally(t *testing.T) {
	appRoot := t.TempDir()
	app := NewApp(appRoot)
	app.config = config.DefaultConfig()
	scheduler := newBackupScheduler(app)

	result, err := scheduler.saveOpenList(map[string]string{
		"baseURL":    "http://127.0.0.1:5244/dav",
		"remotePath": "ant-chrome/backups",
		"token":      "secret-token",
	})
	if err != nil {
		t.Fatalf("save returned error: %v", err)
	}
	if configured, ok := result["tokenConfigured"].(bool); !ok || !configured {
		t.Fatalf("tokenConfigured = %#v, want true", result["tokenConfigured"])
	}
	if _, ok := result["token"]; ok {
		t.Fatal("scheduled settings response must not include token")
	}
	result, err = scheduler.saveSchedule(map[string]string{
		"enabled":   "true",
		"dailyTime": "03:15",
	})
	if err != nil {
		t.Fatalf("schedule save returned error: %v", err)
	}

	localData, err := os.ReadFile(filepath.Join(appRoot, backupLocalConfigFileName))
	if err != nil {
		t.Fatalf("read local backup config: %v", err)
	}
	if string(localData) == "" || !containsString(string(localData), "secret-token") {
		t.Fatal("OpenList token must be written to the local backup config")
	}
	if containsString(string(localData), "127.0.0.1:5244") || containsString(string(localData), "03:15") {
		t.Fatal("local backup config must only contain the OpenList token")
	}

	configData, err := os.ReadFile(filepath.Join(appRoot, "config.yaml"))
	if err == nil {
		if !containsString(string(configData), "127.0.0.1:5244") {
			t.Fatal("OpenList address must be written to config.yaml")
		}
		if containsString(string(configData), "secret-token") {
			t.Fatal("OpenList token must not be written to config.yaml")
		}
	}

	persistedConfig, err := config.Load(filepath.Join(appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	reloadedScheduler := newBackupScheduler(&App{appRoot: appRoot, config: persistedConfig})
	if err := reloadedScheduler.loadLocalConfig(); err != nil {
		t.Fatalf("reload local backup config: %v", err)
	}
	if reloadedScheduler.settings.Channels.OpenList.Token != "secret-token" {
		t.Fatalf("reloaded token = %q, want persisted token", reloadedScheduler.settings.Channels.OpenList.Token)
	}

	if _, err := scheduler.saveSchedule(map[string]string{
		"enabled":   "true",
		"dailyTime": "03:15",
	}); err != nil {
		t.Fatalf("saving with an empty token should reuse the stored token: %v", err)
	}
}

func TestBackupScheduledSettingsDefaults(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	settings := backupScheduledSettingsSnapshot(app, nil)
	if settings["dailyTime"] != "02:00" {
		t.Fatalf("dailyTime = %#v, want 02:00", settings["dailyTime"])
	}
	if settings["tokenConfigured"] != false {
		t.Fatalf("tokenConfigured = %#v, want false", settings["tokenConfigured"])
	}
	if _, ok := settings["baseURL"]; ok {
		t.Fatal("scheduled settings must not include OpenList base URL")
	}
	if _, ok := settings["remotePath"]; ok {
		t.Fatal("scheduled settings must not include OpenList remote path")
	}
}

func TestBackupScheduledSettingsExposeRecentBackupTimes(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	app.config = config.DefaultConfig()
	scheduler := newBackupScheduler(app)
	scheduler.settings = app.config.Backup

	for _, timestamp := range []string{
		"2026-09-02T01:00:00Z",
		"2026-09-03T01:00:00Z",
		"2026-09-04T01:00:00Z",
		"2026-09-05T01:00:00Z",
	} {
		scheduler.recordSuccessAt(timestamp, "backup.zip")
	}

	want := []string{
		"2026-09-05T01:00:00Z",
		"2026-09-04T01:00:00Z",
		"2026-09-03T01:00:00Z",
	}
	settings := scheduler.snapshot()
	got, ok := settings["recentBackupTimes"].([]string)
	if !ok {
		t.Fatalf("recentBackupTimes = %#v, want []string", settings["recentBackupTimes"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recentBackupTimes = %#v, want %#v", got, want)
	}

	persisted, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !reflect.DeepEqual(persisted.Backup.Schedule.RecentBackupTimes, want) {
		t.Fatalf("persisted recentBackupTimes = %#v, want %#v", persisted.Backup.Schedule.RecentBackupTimes, want)
	}

	reloadedScheduler := newBackupScheduler(&App{appRoot: root, config: persisted})
	if err := reloadedScheduler.loadLocalConfig(); err != nil {
		t.Fatalf("reload scheduler settings: %v", err)
	}
	if reloadedScheduler.state.Status != backupScheduleStatusSuccess || reloadedScheduler.state.LastSuccessAt != want[0] {
		t.Fatalf("reloaded scheduler state = %+v, want successful state at %s", reloadedScheduler.state, want[0])
	}
}

func TestBackupLocalDirectoryStaysInSyncWithSchedulerSettings(t *testing.T) {
	appRoot := t.TempDir()
	backupDirectory := filepath.Join(appRoot, "backups")
	if err := os.MkdirAll(backupDirectory, 0o755); err != nil {
		t.Fatalf("create backup directory: %v", err)
	}

	app := NewApp(appRoot)
	app.config = config.DefaultConfig()
	scheduler := newBackupScheduler(app)
	app.backupScheduler = scheduler

	if err := app.backupSetLocalDirectoryLocked(backupDirectory); err != nil {
		t.Fatalf("save local backup directory: %v", err)
	}
	if scheduler.settings.LocalDirectory != backupDirectory {
		t.Fatalf("scheduler local directory = %q, want %q", scheduler.settings.LocalDirectory, backupDirectory)
	}

	if _, err := scheduler.saveOpenList(map[string]string{
		"baseURL": "http://127.0.0.1:5244/dav",
		"token":   "secret-token",
	}); err != nil {
		t.Fatalf("save OpenList settings: %v", err)
	}

	persisted, err := config.Load(filepath.Join(appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if persisted.Backup.LocalDirectory != backupDirectory {
		t.Fatalf("persisted local directory = %q, want %q", persisted.Backup.LocalDirectory, backupDirectory)
	}
}

func TestOpenListRemotePathCanBeClearedToUseRoot(t *testing.T) {
	next := config.DefaultConfig().Backup
	next.Channels.OpenList.RemotePath = "ant-chrome/backups"

	if err := applyOpenListInput(&next, map[string]string{"remotePath": ""}); err != nil {
		t.Fatalf("apply OpenList input: %v", err)
	}
	if next.Channels.OpenList.RemotePath != "" {
		t.Fatalf("remote path = %q, want empty root path", next.Channels.OpenList.RemotePath)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.Backup.Channels.OpenList.BaseURL = "http://127.0.0.1:5244/dav"
	app.config.Backup.Channels.OpenList.Token = "secret-token"
	resolved, err := app.backupResolvedOpenListConfig(map[string]string{"remotePath": ""})
	if err != nil {
		t.Fatalf("resolve OpenList config: %v", err)
	}
	if resolved.RemotePath != "" {
		t.Fatalf("resolved remote path = %q, want empty root path", resolved.RemotePath)
	}
}

func containsString(value, target string) bool {
	for index := 0; index+len(target) <= len(value); index++ {
		if value[index:index+len(target)] == target {
			return true
		}
	}
	return false
}
