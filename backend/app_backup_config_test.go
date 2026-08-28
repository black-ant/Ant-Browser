package backend

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestBackupApplyIncomingConfigKeepsCurrentBackupSettings(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.App.Name = ""
	app.config.Backup.Channels.OpenList.BaseURL = "http://current.example/dav"
	app.config.Backup.Channels.OpenList.RemotePath = "current/backups"
	app.config.Backup.Schedule.Enabled = true
	app.config.Backup.Schedule.DailyTime = "03:15"

	incoming := config.DefaultConfig()
	incoming.App.Name = "Imported App"
	incoming.Backup.Channels.OpenList.BaseURL = "http://imported.example/dav"
	incoming.Backup.Channels.OpenList.RemotePath = "imported/backups"
	incoming.Backup.Schedule.Enabled = false

	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}
	if app.config.App.Name != "Imported App" {
		t.Fatalf("app name = %q, want imported app name", app.config.App.Name)
	}
	backupConfig := app.config.Backup
	if backupConfig.Channels.OpenList.BaseURL != "http://current.example/dav" || backupConfig.Channels.OpenList.RemotePath != "current/backups" {
		t.Fatalf("OpenList settings = %+v, want current settings", backupConfig.Channels.OpenList)
	}
	if !backupConfig.Schedule.Enabled || backupConfig.Schedule.DailyTime != "03:15" {
		t.Fatalf("schedule = %+v, want current schedule", backupConfig.Schedule)
	}
}
