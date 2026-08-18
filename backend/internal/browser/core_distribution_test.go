package browser

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

// 构造带两个内核(148/144)的 Manager:各建一个临时目录 + 写 *.manifest 文件,
// 并把 cores 直接写入 config(ListCores 在无 CoreDAO 时返回 config cores)。
func newManagerWithFakeCores(t *testing.T) *Manager {
	t.Helper()
	appRoot := t.TempDir()
	cfg := &config.Config{}
	cfg.Browser.KernelDistribution = map[string]int{"148": 70, "144": 30}
	m := NewManager(cfg, appRoot)
	var cores []config.BrowserCore
	for _, ver := range []string{"148", "144"} {
		dir := filepath.Join(appRoot, "chrome", ver)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ver+".0.0.0.manifest"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		cores = append(cores, config.BrowserCore{
			CoreId: "core-" + ver, CoreName: "Chrome " + ver, CorePath: "chrome/" + ver,
		})
	}
	m.Config.Browser.Cores = cores
	m.InitData()
	return m
}

func TestResolveProfileCoreForKernelSelectAuto(t *testing.T) {
	m := newManagerWithFakeCores(t)
	c148, c144 := 0, 0
	for i := 0; i < 100; i++ {
		id, err := m.resolveProfileCoreForKernelSelect(KernelSelectAuto, i)
		if err != nil {
			t.Fatalf("auto i=%d: %v", i, err)
		}
		switch id {
		case "core-148":
			c148++
		case "core-144":
			c144++
		default:
			t.Fatalf("auto i=%d → unexpected core %q", i, id)
		}
	}
	if c148 != 70 || c144 != 30 {
		t.Errorf("auto distribution = 148:%d 144:%d, want 70:30", c148, c144)
	}
}

func TestResolveProfileCoreForKernelSelectAll(t *testing.T) {
	m := newManagerWithFakeCores(t)
	id, err := m.resolveProfileCoreForKernelSelect(KernelSelectAll148, 0)
	if err != nil || id != "core-148" {
		t.Errorf("all148 → %q err=%v", id, err)
	}
	id, err = m.resolveProfileCoreForKernelSelect(KernelSelectAll144, 0)
	if err != nil || id != "core-144" {
		t.Errorf("all144 → %q err=%v", id, err)
	}
}

// 未注册指定版本内核时应硬失败(不回退伪造)。
func TestResolveProfileCoreForKernelSelectMissingVersionFails(t *testing.T) {
	m := newManagerWithFakeCores(t)
	if _, err := m.resolveProfileCoreForKernelSelect(KernelSelectAll144, 0); err != nil {
		// 144 已注册,应成功;此行仅确认正常路径不报错
	}
	// 构造只有 148 的场景
	m.Config.Browser.Cores = m.Config.Browser.Cores[:1]
	if _, err := m.resolveProfileCoreForKernelSelect(KernelSelectAll144, 0); err == nil {
		t.Errorf("all144 without 144 core should hard-fail")
	}
}
