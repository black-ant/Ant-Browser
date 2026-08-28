package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupConfigSaveKeepsChannelSettingsPrivateToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Backup.Channels.OpenList.BaseURL = "http://127.0.0.1:5244/dav"
	cfg.Backup.Channels.OpenList.RemotePath = "ant-chrome/backups"
	cfg.Backup.Channels.OpenList.Token = "secret-token"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "channels:") || !strings.Contains(text, "base_url: http://127.0.0.1:5244/dav") {
		t.Fatalf("saved backup channel settings are missing: %s", text)
	}
	if strings.Contains(text, "secret-token") {
		t.Fatal("backup token must not be written to config.yaml")
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Backup.Channels.OpenList.Token != "" {
		t.Fatalf("loaded token = %q, want empty after sanitization", loaded.Backup.Channels.OpenList.Token)
	}
}

func TestBackupConfigLoadsLegacyOpenListSection(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	legacy := []byte(`backup:
  openlist:
    base_url: http://127.0.0.1:5244/dav
    remote_path: ant-chrome/backups
    token: legacy-token
  schedule:
    enabled: true
    daily_time: "03:15"
`)
	if err := os.WriteFile(configPath, legacy, 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	openList := loaded.Backup.Channels.OpenList
	if openList.BaseURL != "http://127.0.0.1:5244/dav" || openList.RemotePath != "ant-chrome/backups" || openList.Token != "legacy-token" {
		t.Fatalf("legacy OpenList config = %+v", openList)
	}
	if !loaded.Backup.Schedule.Enabled || loaded.Backup.Schedule.DailyTime != "03:15" {
		t.Fatalf("legacy schedule = %+v", loaded.Backup.Schedule)
	}
}
