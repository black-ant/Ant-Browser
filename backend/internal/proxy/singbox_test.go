package proxy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSingBoxRegisterBridgeStoresNewBridge(t *testing.T) {
	manager := &SingBoxManager{
		Bridges: make(map[string]*SingBoxBridge),
	}
	bridge := &SingBoxBridge{
		NodeKey: "node-a",
		Port:    21001,
		Running: true,
	}

	socksURL, reused := manager.registerBridge("node-a", bridge, false)
	if reused {
		t.Fatalf("expected new bridge registration, got reused with %q", socksURL)
	}
	if socksURL != "" {
		t.Fatalf("expected empty socksURL for new bridge registration, got %q", socksURL)
	}
	if manager.Bridges["node-a"] != bridge {
		t.Fatalf("bridge was not stored in manager")
	}
}

func TestSingBoxRegisterBridgeIgnoresSamePointer(t *testing.T) {
	manager := &SingBoxManager{
		Bridges: make(map[string]*SingBoxBridge),
	}
	bridge := &SingBoxBridge{
		NodeKey: "node-a",
		Port:    21001,
		Running: true,
	}
	manager.Bridges["node-a"] = bridge

	socksURL, reused := manager.registerBridge("node-a", bridge, false)
	if reused {
		t.Fatalf("same bridge pointer must not be treated as duplicate, got reused with %q", socksURL)
	}
	if socksURL != "" {
		t.Fatalf("expected empty socksURL when registering same pointer, got %q", socksURL)
	}
	if manager.Bridges["node-a"] != bridge {
		t.Fatalf("bridge mapping changed unexpectedly")
	}
	if bridge.Stopping {
		t.Fatalf("same bridge pointer should not be marked as stopping")
	}
}

func TestSingBoxRegisterBridgePinsLongLivedBridge(t *testing.T) {
	t.Parallel()

	manager := &SingBoxManager{Bridges: make(map[string]*SingBoxBridge)}
	bridge := &SingBoxBridge{NodeKey: "node-a", Port: 21001, Running: true}

	_, reused := manager.registerBridge("node-a", bridge, true)
	if reused {
		t.Fatalf("new bridge should not be reported as reused")
	}
	if bridge.RefCount != 1 {
		t.Fatalf("pinned bridge refcount = %d, want 1", bridge.RefCount)
	}

	manager.ReleaseBridge("node-a")
	if bridge.RefCount != 0 {
		t.Fatalf("released bridge refcount = %d, want 0", bridge.RefCount)
	}
}

func TestSingBoxRecycleIdleBridgesSkipsPinnedBridge(t *testing.T) {
	t.Parallel()

	manager := &SingBoxManager{Bridges: make(map[string]*SingBoxBridge)}
	pinned := &SingBoxBridge{NodeKey: "pinned-node-key", LastUsedAt: time.Now().Add(-2 * singBoxBridgeIdleTTL), RefCount: 1}
	idle := &SingBoxBridge{NodeKey: "idle-node-key", LastUsedAt: time.Now().Add(-2 * singBoxBridgeIdleTTL)}
	manager.Bridges[pinned.NodeKey] = pinned
	manager.Bridges[idle.NodeKey] = idle

	manager.recycleIdleBridges()

	if manager.Bridges[pinned.NodeKey] != pinned {
		t.Fatalf("pinned bridge should not be recycled")
	}
	if _, ok := manager.Bridges[idle.NodeKey]; ok {
		t.Fatalf("idle bridge should be recycled")
	}
}

func TestSingBoxLaunchErrorRetryClassification(t *testing.T) {
	t.Parallel()

	if isRetryableSingBoxLaunchError(&singBoxLaunchError{err: fmt.Errorf("config invalid"), retryable: false}) {
		t.Fatalf("non-retryable sing-box launch error was classified as retryable")
	}
	if !isRetryableSingBoxLaunchError(&singBoxLaunchError{err: fmt.Errorf("port race"), retryable: true}) {
		t.Fatalf("retryable sing-box launch error was classified as non-retryable")
	}
	if !isRetryableSingBoxLaunchError(fmt.Errorf("legacy error")) {
		t.Fatalf("plain errors should remain retryable for backward compatibility")
	}
}

func TestSingBoxBridgeReadyErrorRetryPolicy(t *testing.T) {
	t.Parallel()

	manager := &SingBoxManager{}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "singbox-config.json")
	stderrPath := filepath.Join(dir, "singbox-stderr.log")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if manager.isRetryableBridgeReadyError(fmt.Errorf("sing-box 进程提前退出: config invalid"), cfgPath, stderrPath) {
		t.Fatalf("early process exit without bind evidence should not be retried")
	}
	if err := os.WriteFile(stderrPath, []byte("listen tcp 127.0.0.1:10001: bind: address already in use"), 0o644); err != nil {
		t.Fatalf("write stderr failed: %v", err)
	}
	if !manager.isRetryableBridgeReadyError(fmt.Errorf("sing-box 进程提前退出"), cfgPath, stderrPath) {
		t.Fatalf("bind conflict should be retried with another port")
	}
}

func TestSingBoxRestartBridgeNotNeededWhenBridgeChanged(t *testing.T) {
	t.Parallel()

	manager := &SingBoxManager{Bridges: make(map[string]*SingBoxBridge)}
	oldBridge := &SingBoxBridge{NodeKey: "node-a", Port: 21001, Outbound: map[string]interface{}{"type": "direct"}}
	manager.Bridges["node-a"] = &SingBoxBridge{NodeKey: "node-a", Port: 21002}

	err := manager.restartBridgeOnSamePort(nil, "node-a", oldBridge, oldBridge.RefCount)
	if !errors.Is(err, errSingBoxBridgeRestartNotNeeded) {
		t.Fatalf("restartBridgeOnSamePort() error = %v, want restart-not-needed", err)
	}
}

func TestSingBoxRestartBridgeRequiresContext(t *testing.T) {
	t.Parallel()

	manager := &SingBoxManager{Bridges: make(map[string]*SingBoxBridge)}
	bridge := &SingBoxBridge{NodeKey: "node-a", Port: 21001}
	manager.Bridges["node-a"] = bridge

	err := manager.restartBridgeOnSamePort(nil, "node-a", bridge, bridge.RefCount)
	if err == nil {
		t.Fatalf("restartBridgeOnSamePort() returned nil, want missing context error")
	}
	if errors.Is(err, errSingBoxBridgeRestartNotNeeded) {
		t.Fatalf("missing restart context should not be treated as restart-not-needed")
	}
}
