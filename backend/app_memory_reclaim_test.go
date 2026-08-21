package backend

import (
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestMemoryReclaimDefaults(t *testing.T) {
	a := &App{config: &config.Config{}}
	if !a.memoryReclaimEnabled() {
		t.Error("should default on")
	}
	if got := a.memoryReclaimInterval(); got != 10*time.Minute {
		t.Errorf("default interval got %v, want 10m", got)
	}
	if got := a.memoryReclaimLevel(); got != "moderate" {
		t.Errorf("default level got %q, want moderate", got)
	}
}

func TestMemoryReclaimNilSafe(t *testing.T) {
	var a *App
	if !a.memoryReclaimEnabled() {
		t.Error("nil App should default on without panicking")
	}
	if got := a.memoryReclaimInterval(); got != 10*time.Minute {
		t.Errorf("nil App interval got %v", got)
	}
}

func TestMemoryReclaimDisabled(t *testing.T) {
	off := false
	a := &App{config: &config.Config{}}
	a.config.Browser.MemoryReclaimEnabled = &off
	if a.memoryReclaimEnabled() {
		t.Fatal("explicit false should disable")
	}
}

// 间隔过小会让上百实例频繁回收,反而制造 CPU 尖峰,必须有下限。
func TestMemoryReclaimIntervalFloor(t *testing.T) {
	a := &App{config: &config.Config{}}
	a.config.Browser.MemoryReclaimIntervalMs = 1000
	if got := a.memoryReclaimInterval(); got < time.Minute {
		t.Errorf("interval must be floored to >=1m, got %v", got)
	}
}

func TestMemoryReclaimLevelWhitelist(t *testing.T) {
	a := &App{config: &config.Config{}}

	a.config.Browser.MemoryReclaimLevel = "nuke"
	if got := a.memoryReclaimLevel(); got != "moderate" {
		t.Errorf("invalid level must fall back to moderate, got %q", got)
	}

	a.config.Browser.MemoryReclaimLevel = "critical"
	if got := a.memoryReclaimLevel(); got != "critical" {
		t.Errorf("critical should be accepted, got %q", got)
	}
}

// 只对运行中且调试就绪、端口有效的实例回收。
func TestMemoryReclaimShouldReclaim(t *testing.T) {
	cases := []struct {
		name    string
		running bool
		ready   bool
		port    int
		want    bool
	}{
		{"全部就绪", true, true, 9222, true},
		{"未运行", false, true, 9222, false},
		{"调试未就绪", true, false, 9222, false},
		{"无端口", true, true, 0, false},
	}
	for _, c := range cases {
		if got := memoryReclaimShouldReclaim(c.running, c.ready, c.port); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
