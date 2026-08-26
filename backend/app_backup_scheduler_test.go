package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
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

func TestBackupScheduledSaveSettingsDoesNotPersistPassword(t *testing.T) {
	appRoot := t.TempDir()
	app := NewApp(appRoot)
	app.config = config.DefaultConfig()
	scheduler := newBackupScheduler(app)

	result, err := scheduler.save(map[string]string{
		"enabled":    "true",
		"dailyTime":  "03:15",
		"baseURL":    "http://127.0.0.1:5244/dav",
		"remotePath": "ant-chrome/backups",
		"username":   "user",
		"password":   "secret-password",
	})
	if err != nil {
		t.Fatalf("save returned error: %v", err)
	}
	if configured, ok := result["passwordConfigured"].(bool); !ok || !configured {
		t.Fatalf("passwordConfigured = %#v, want true", result["passwordConfigured"])
	}
	if _, ok := result["password"]; ok {
		t.Fatal("scheduled settings response must not include password")
	}

	data, err := os.ReadFile(filepath.Join(appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) == "" || containsString(string(data), "secret-password") {
		t.Fatal("OpenList password must not be written to config.yaml")
	}

	if _, err := scheduler.save(map[string]string{
		"enabled":    "true",
		"dailyTime":  "03:15",
		"baseURL":    "http://127.0.0.1:5244/dav",
		"remotePath": "ant-chrome/backups",
		"username":   "another-user",
		"password":   "",
	}); err == nil {
		t.Fatal("changing the username without a new password must be rejected")
	}
}

func TestBackupScheduledSettingsDefaults(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	settings := backupScheduledSettingsSnapshot(app, nil)
	if settings["dailyTime"] != "02:00" {
		t.Fatalf("dailyTime = %#v, want 02:00", settings["dailyTime"])
	}
	if settings["remotePath"] != "ant-chrome/backups" {
		t.Fatalf("remotePath = %#v, want default path", settings["remotePath"])
	}
	if settings["passwordConfigured"] != false {
		t.Fatalf("passwordConfigured = %#v, want false", settings["passwordConfigured"])
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
