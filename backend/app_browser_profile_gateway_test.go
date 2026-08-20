package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestBrowserProfileDeleteRejectsRunningProfile(t *testing.T) {
	manager := browser.NewManager(config.DefaultConfig(), t.TempDir())
	manager.InitData()
	manager.Profiles["running-profile"] = &BrowserProfile{
		ProfileId:   "running-profile",
		ProfileName: "Running",
		Running:     true,
	}
	app := &App{browserMgr: manager}

	err := app.BrowserProfileDelete("running-profile")
	if err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("BrowserProfileDelete error = %v, want running-profile rejection", err)
	}
	if manager.Profiles["running-profile"] == nil {
		t.Fatal("running profile was deleted")
	}
}
