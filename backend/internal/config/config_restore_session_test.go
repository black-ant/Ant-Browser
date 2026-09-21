package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesRestoreLastSessionDefaultWhenFieldMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("browser:\n  user_data_root: data\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Browser.RestoreLastSession {
		t.Fatal("restore_last_session should inherit the default true value when omitted")
	}
}

func TestLoadRespectsExplicitRestoreLastSessionFalseForCurrentConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("config_version: 2\nbrowser:\n  restore_last_session: false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Browser.RestoreLastSession {
		t.Fatal("explicit restore_last_session=false must remain disabled")
	}
}

func TestMigrateLegacyConfigEnablesRestoreForPreVersionedConfig(t *testing.T) {
	cfg := &Config{}
	cfg.Browser.RestoreLastSession = false
	if !MigrateLegacyConfig(cfg) {
		t.Fatal("legacy config should require migration")
	}
	if cfg.ConfigVersion != CurrentConfigVersion {
		t.Fatalf("config_version = %d, want %d", cfg.ConfigVersion, CurrentConfigVersion)
	}
	if !cfg.Browser.RestoreLastSession {
		t.Fatal("legacy restore_last_session=false should migrate to true")
	}
}

func TestMigrateLegacyConfigPreservesCurrentExplicitDisable(t *testing.T) {
	cfg := &Config{ConfigVersion: CurrentConfigVersion}
	cfg.Browser.RestoreLastSession = false
	if MigrateLegacyConfig(cfg) {
		t.Fatal("current config must not be migrated again")
	}
	if cfg.Browser.RestoreLastSession {
		t.Fatal("explicit current restore_last_session=false must remain disabled")
	}
}
