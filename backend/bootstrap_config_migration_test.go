package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMigratesLegacyRestoreSetting(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	legacy := "browser:\n  user_data_root: data\n  restore_last_session: false\n"
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.ConfigVersion != 2 {
		t.Fatalf("config_version = %d, want 2", cfg.ConfigVersion)
	}
	if !cfg.Browser.RestoreLastSession {
		t.Fatal("legacy restore_last_session=false should migrate to true")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "config_version: 2") {
		t.Fatalf("migrated config missing config_version: %s", text)
	}
	if !strings.Contains(text, "restore_last_session: true") {
		t.Fatalf("migrated config did not persist restore=true: %s", text)
	}
}
