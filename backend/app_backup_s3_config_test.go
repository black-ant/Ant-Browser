package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupS3SaveSettingsKeepsFullConfigLocal(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	defer app.stopBackupScheduler()

	settings, err := app.BackupS3SaveSettings(map[string]string{
		"endpoint":        "https://s3.example.com",
		"region":          "us-west-2",
		"bucket":          "backup-bucket",
		"prefix":          "s3-only-prefix",
		"forcePathStyle":  "true",
		"accessKeyID":     "access-key",
		"secretAccessKey": "secret-key",
		"sessionToken":    "session-token",
	})
	if err != nil {
		t.Fatalf("save S3 settings: %v", err)
	}
	if settings["credentialsConfigured"] != true || settings["sessionTokenConfigured"] != true {
		t.Fatalf("S3 settings result = %+v, want configured credentials", settings)
	}

	publicData, err := os.ReadFile(filepath.Join(app.appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("read public config: %v", err)
	}
	publicText := string(publicData)
	for _, secret := range []string{"access-key", "secret-key", "session-token"} {
		if strings.Contains(publicText, secret) {
			t.Fatalf("public config contains S3 secret %q: %s", secret, publicText)
		}
	}
	for _, publicValue := range []string{"s3.example.com", "us-west-2", "backup-bucket", "s3-only-prefix", "force_path_style"} {
		if strings.Contains(publicText, publicValue) {
			t.Fatalf("public config contains S3 value %q: %s", publicValue, publicText)
		}
	}
	if strings.Contains(publicText, "s3:") {
		t.Fatalf("public config contains S3 channel: %s", publicText)
	}

	localData, err := os.ReadFile(filepath.Join(app.appRoot, backupLocalConfigFileName))
	if err != nil {
		t.Fatalf("read local config: %v", err)
	}
	localText := string(localData)
	for _, secret := range []string{"access-key", "secret-key", "session-token"} {
		if !strings.Contains(localText, secret) {
			t.Fatalf("local config is missing S3 secret %q: %s", secret, localText)
		}
	}
	for _, publicValue := range []string{"s3.example.com", "us-west-2", "backup-bucket", "s3-only-prefix", "force_path_style: true"} {
		if !strings.Contains(localText, publicValue) {
			t.Fatalf("local config is missing S3 value %q: %s", publicValue, localText)
		}
	}
}

func TestBackupS3RevealCredentialReturnsStoredValue(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	defer app.stopBackupScheduler()

	if _, err := app.BackupS3SaveSettings(map[string]string{
		`endpoint`:        `https://s3.example.com`,
		`region`:          `us-west-2`,
		`bucket`:          `backup-bucket`,
		`accessKeyID`:     `access-key`,
		`secretAccessKey`: `secret-key`,
		`sessionToken`:    `session-token`,
	}); err != nil {
		t.Fatalf(`save S3 settings: %v`, err)
	}

	tests := map[string]string{
		`accessKeyID`:     `access-key`,
		`secretAccessKey`: `secret-key`,
		`sessionToken`:    `session-token`,
	}
	for field, want := range tests {
		got, err := app.BackupS3RevealCredential(field)
		if err != nil {
			t.Fatalf(`reveal %s: %v`, field, err)
		}
		if got != want {
			t.Fatalf(`reveal %s = %q, want %q`, field, got, want)
		}
	}

	if _, err := app.BackupS3RevealCredential(`unknown`); err == nil {
		t.Fatal(`reveal unknown field succeeded`)
	}
}
