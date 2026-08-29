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

func TestBackupLocalConfigStoresFullS3Config(t *testing.T) {
	path := filepath.Join(t.TempDir(), backupLocalConfigFileName)
	base := config.DefaultConfig().Backup
	base.Channels.S3.Endpoint = "https://s3.example.com"
	base.Channels.S3.Region = "us-west-2"
	base.Channels.S3.Bucket = "backup-bucket"
	base.Channels.S3.Prefix = "ant-chrome/backups"
	base.Channels.S3.AccessKeyID = "access-key"
	base.Channels.S3.SecretAccessKey = "secret-key"
	base.Channels.S3.SessionToken = "session-token"
	base.Channels.S3.ForcePathStyle = true

	if err := saveBackupLocalConfig(path, base); err != nil {
		t.Fatalf("save S3 local config: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read S3 local config: %v", err)
	}
	text := string(data)
	for _, publicValue := range []string{"s3.example.com", "us-west-2", "backup-bucket", "ant-chrome/backups", "force_path_style: true"} {
		if !strings.Contains(text, publicValue) {
			t.Fatalf("local S3 config must contain %q: %s", publicValue, text)
		}
	}
	for _, secret := range []string{"access-key", "secret-key", "session-token"} {
		if !strings.Contains(text, secret) {
			t.Fatalf("local S3 config must contain %q: %s", secret, text)
		}
	}

	loaded, exists, err := loadBackupLocalConfig(path, config.DefaultConfig().Backup)
	if err != nil {
		t.Fatalf("load S3 local config: %v", err)
	}
	if !exists {
		t.Fatal("S3 local config should be detected")
	}
	if loaded.Channels.S3.Endpoint != "https://s3.example.com" || loaded.Channels.S3.Region != "us-west-2" || loaded.Channels.S3.Bucket != "backup-bucket" || loaded.Channels.S3.Prefix != "ant-chrome/backups" ||
		loaded.Channels.S3.AccessKeyID != "access-key" || loaded.Channels.S3.SecretAccessKey != "secret-key" || loaded.Channels.S3.SessionToken != "session-token" || !loaded.Channels.S3.ForcePathStyle {
		t.Fatalf("loaded S3 credentials = %+v", loaded.Channels.S3)
	}
}

func TestPrepareBackupLocalConfigMovesS3ConfigOutOfConfigFile(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Backup.Channels.S3.Endpoint = "https://s3.example.com"
	cfg.Backup.Channels.S3.Region = "us-west-2"
	cfg.Backup.Channels.S3.Bucket = "backup-bucket"
	cfg.Backup.Channels.S3.Prefix = "s3-only-prefix"
	cfg.Backup.Channels.S3.AccessKeyID = "access-key"
	cfg.Backup.Channels.S3.SecretAccessKey = "secret-key"
	cfg.Backup.Channels.S3.SessionToken = "session-token"
	cfg.Backup.Channels.S3.ForcePathStyle = true

	app := NewApp(root)
	app.config = cfg
	if err := app.prepareBackupLocalConfig(); err != nil {
		t.Fatalf("prepare backup local config: %v", err)
	}

	configData, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatalf("read sanitized config: %v", err)
	}
	configText := string(configData)
	for _, value := range []string{
		"https://s3.example.com",
		"us-west-2",
		"backup-bucket",
		"s3-only-prefix",
		"access-key",
		"secret-key",
		"session-token",
		"s3:",
	} {
		if strings.Contains(configText, value) {
			t.Fatalf("S3 value %q leaked into config.yaml: %s", value, configText)
		}
	}

	localData, err := os.ReadFile(filepath.Join(root, backupLocalConfigFileName))
	if err != nil {
		t.Fatalf("read local S3 config: %v", err)
	}
	localText := string(localData)
	for _, value := range []string{
		"https://s3.example.com",
		"us-west-2",
		"backup-bucket",
		"s3-only-prefix",
		"access-key",
		"secret-key",
		"session-token",
		"force_path_style: true",
	} {
		if !strings.Contains(localText, value) {
			t.Fatalf("S3 value %q missing from backup.local.yaml: %s", value, localText)
		}
	}
}
