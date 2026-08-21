package backend

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestSpeedAutoTestDefaultOff(t *testing.T) {
	if speedAutoTestEnabled(&config.Config{}) {
		t.Fatal("auto speed test should default OFF for live farm")
	}
	if speedAutoTestEnabled(nil) {
		t.Fatal("nil config should default OFF")
	}
}

func TestSpeedAutoTestExplicitOptIn(t *testing.T) {
	on := true
	cfg := &config.Config{}
	cfg.Browser.SpeedAutoTestEnabled = &on
	if !speedAutoTestEnabled(cfg) {
		t.Fatal("explicit true should enable")
	}

	off := false
	cfg.Browser.SpeedAutoTestEnabled = &off
	if speedAutoTestEnabled(cfg) {
		t.Fatal("explicit false should disable")
	}
}
