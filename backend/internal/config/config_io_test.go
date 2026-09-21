package config

import "testing"

func TestDefaultConfigRestoresLastSession(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ConfigVersion != CurrentConfigVersion {
		t.Fatalf("config_version = %d, want %d", cfg.ConfigVersion, CurrentConfigVersion)
	}
	if !cfg.Browser.RestoreLastSession {
		t.Fatal("default config should restore the last browser session")
	}
}
