package backend

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
)

func TestStopAutomationTargetProfileCleansStaleLocks(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	userDataDir := t.TempDir()
	profile := &browser.Profile{
		ProfileId:   "profile-cleanup",
		ProfileName: "cleanup",
		UserDataDir: userDataDir,
		Running:     false,
	}
	app.browserMgr.Profiles[profile.ProfileId] = profile
	for _, name := range chromiumSingletonLockNames {
		if err := os.WriteFile(filepath.Join(userDataDir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	ok, detail := app.stopAutomationTargetProfile(profile.ProfileId)
	if !ok {
		t.Fatalf("已停止实例的收尾不应失败：%s", detail)
	}
	for _, name := range chromiumSingletonLockNames {
		if _, err := os.Stat(filepath.Join(userDataDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s 应被清理，stat err = %v", name, err)
		}
	}
}

func TestStopAutomationTargetProfileEmptyID(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	ok, detail := app.stopAutomationTargetProfile("   ")
	if !ok || detail != "" {
		t.Fatalf("空 profile id 应直接通过，got ok=%v detail=%q", ok, detail)
	}
}

func TestAppendAutomationCleanupError(t *testing.T) {
	if got := appendAutomationCleanupError("", "实例仍在运行"); got != "实例清理失败：实例仍在运行" {
		t.Fatalf("empty existing = %q", got)
	}
	if got := appendAutomationCleanupError("脚本超时", "实例仍在运行"); got != "脚本超时；实例清理失败：实例仍在运行" {
		t.Fatalf("existing = %q", got)
	}
	if got := appendAutomationCleanupError("脚本超时", "  "); got != "脚本超时" {
		t.Fatalf("empty detail = %q", got)
	}
}
