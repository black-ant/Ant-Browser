package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBackupLocalConfigMigratesLegacySettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), backupLocalConfigFileName)
	legacy := []byte(`backup:
  channels:
    openlist:
      base_url: http://127.0.0.1:5244/dav
      remote_path: legacy/backups
      token: legacy-token
  schedule:
    enabled: true
    daily_time: "03:15"
`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy local config: %v", err)
	}

	settings, exists, err := loadBackupLocalConfig(path, config.DefaultConfig().Backup)
	if err != nil {
		t.Fatalf("load legacy local config: %v", err)
	}
	if !exists {
		t.Fatal("legacy local config should be detected")
	}
	if settings.Channels.OpenList.BaseURL != "http://127.0.0.1:5244/dav" ||
		settings.Channels.OpenList.RemotePath != "legacy/backups" ||
		settings.Channels.OpenList.Token != "legacy-token" {
		t.Fatalf("legacy OpenList settings = %+v", settings.Channels.OpenList)
	}
	if !settings.Schedule.Enabled || settings.Schedule.DailyTime != "03:15" {
		t.Fatalf("legacy schedule = %+v", settings.Schedule)
	}

	if err := saveBackupLocalConfig(path, settings); err != nil {
		t.Fatalf("rewrite local config: %v", err)
	}
	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten local config: %v", err)
	}
	text := string(rewritten)
	if !strings.Contains(text, "legacy-token") {
		t.Fatal("rewritten local config must keep the token")
	}
	if strings.Contains(text, "127.0.0.1:5244") || strings.Contains(text, "03:15") || strings.Contains(text, "legacy/backups") {
		t.Fatal("rewritten local config must only contain the token")
	}
}
