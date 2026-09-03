package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestResetManagedSettingsResetsManagedConfigAndSecrets(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	cfg := config.DefaultConfig()
	cfg.App.Name = "custom app"
	cfg.Browser.UserDataRoot = filepath.Join(root, "browser-data")
	cfg.Automation.Enabled = true
	cfg.Automation.NodeSource = config.AutomationNodeSourceSystem
	cfg.LaunchServer.Port = 23456
	cfg.LaunchServer.Auth.Enabled = true
	cfg.LaunchServer.Auth.APIKey = "launch-key"
	cfg.Backup.Channels.OpenList.BaseURL = "https://openlist.example.com/dav"
	cfg.Backup.Channels.OpenList.Token = "openlist-token"
	cfg.Backup.Channels.S3.Endpoint = "https://s3.example.com"
	cfg.Backup.Channels.S3.Bucket = "backup-bucket"
	cfg.Backup.Channels.S3.AccessKeyID = "access-key"
	cfg.Backup.Channels.S3.SecretAccessKey = "secret-key"
	cfg.Backup.Schedule.Enabled = true
	cfg.Backup.Schedule.DailyTime = "03:15"
	app.config = cfg

	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := saveBackupLocalConfig(filepath.Join(root, backupLocalConfigFileName), cfg.Backup); err != nil {
		t.Fatalf("save local backup config: %v", err)
	}

	scheduler := newBackupScheduler(app)
	scheduler.settings = cfg.Backup
	scheduler.configurationError = "old configuration error"
	scheduler.state = backupScheduleState{
		Status:         backupScheduleStatusSuccess,
		LastRunAt:      "2026-09-02T01:00:00Z",
		LastSuccessAt:  "2026-09-02T01:01:00Z",
		LastRemoteName: "backup.zip",
	}
	scheduler.lastDate = "2026-09-02"
	app.backupScheduler = scheduler

	if err := app.ResetManagedSettings(); err != nil {
		t.Fatalf("reset managed settings: %v", err)
	}

	defaults := config.DefaultConfig()
	if app.config.Automation != defaults.Automation {
		t.Fatalf("automation config = %+v, want %+v", app.config.Automation, defaults.Automation)
	}
	if app.config.Backup != defaults.Backup {
		t.Fatalf("backup config = %+v, want %+v", app.config.Backup, defaults.Backup)
	}
	if app.config.LaunchServer != defaults.LaunchServer {
		t.Fatalf("LaunchServer config = %+v, want %+v", app.config.LaunchServer, defaults.LaunchServer)
	}
	if app.config.App.Name != "custom app" {
		t.Fatalf("unmanaged app settings changed: %q", app.config.App.Name)
	}
	if app.config.Browser.UserDataRoot != filepath.Join(root, "browser-data") {
		t.Fatalf("unmanaged browser settings changed: %q", app.config.Browser.UserDataRoot)
	}

	if _, err := os.Stat(filepath.Join(root, backupLocalConfigFileName)); !os.IsNotExist(err) {
		t.Fatalf("backup local config still exists, stat error: %v", err)
	}

	if scheduler.settings != defaults.Backup {
		t.Fatalf("scheduler settings = %+v, want %+v", scheduler.settings, defaults.Backup)
	}
	if scheduler.configurationError != "" || scheduler.state.Status != backupScheduleStatusNever || scheduler.lastDate != "" || scheduler.running || scheduler.resetting {
		t.Fatalf("scheduler state was not reset: error=%q state=%+v lastDate=%q running=%v resetting=%v", scheduler.configurationError, scheduler.state, scheduler.lastDate, scheduler.running, scheduler.resetting)
	}

	persisted, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if persisted.Automation != defaults.Automation || persisted.Backup != defaults.Backup || persisted.LaunchServer != defaults.LaunchServer {
		t.Fatalf("persisted managed settings were not reset: automation=%+v backup=%+v launchServer=%+v", persisted.Automation, persisted.Backup, persisted.LaunchServer)
	}
	if persisted.App.Name != "custom app" || persisted.Browser.UserDataRoot != filepath.Join(root, "browser-data") {
		t.Fatalf("persisted unmanaged settings changed: app=%+v browser.userDataRoot=%q", persisted.App, persisted.Browser.UserDataRoot)
	}
}

func TestResetManagedSettingsRequiresInitializedConfig(t *testing.T) {
	app := NewApp(t.TempDir())
	if err := app.ResetManagedSettings(); err == nil {
		t.Fatal("reset should fail when config is not initialized")
	}
}
