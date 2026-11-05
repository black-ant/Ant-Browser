package proxy

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestXrayRegisterBridgeStoresNewBridge(t *testing.T) {
	t.Parallel()

	manager := &XrayManager{
		Bridges: make(map[string]*XrayBridge),
	}
	bridge := &XrayBridge{
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

func TestXrayRegisterBridgeIgnoresSamePointer(t *testing.T) {
	t.Parallel()

	manager := &XrayManager{
		Bridges: make(map[string]*XrayBridge),
	}
	bridge := &XrayBridge{
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

func TestXrayLaunchErrorRetryClassification(t *testing.T) {
	t.Parallel()

	if isRetryableXrayLaunchError(&xrayLaunchError{err: fmt.Errorf("config invalid"), retryable: false}) {
		t.Fatalf("non-retryable xray launch error was classified as retryable")
	}
	if !isRetryableXrayLaunchError(&xrayLaunchError{err: fmt.Errorf("port race"), retryable: true}) {
		t.Fatalf("retryable xray launch error was classified as non-retryable")
	}
	if !isRetryableXrayLaunchError(fmt.Errorf("legacy error")) {
		t.Fatalf("plain errors should remain retryable for backward compatibility")
	}
}

func TestXrayBridgeReadyErrorRetryPolicy(t *testing.T) {
	t.Parallel()

	manager := &XrayManager{}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "xray-config.json")
	stderrPath := filepath.Join(dir, "xray-stderr.log")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if manager.isRetryableBridgeReadyError(fmt.Errorf("xray 进程提前退出: config invalid"), cfgPath, stderrPath) {
		t.Fatalf("early process exit without bind evidence should not be retried")
	}
	if err := os.WriteFile(stderrPath, []byte("listen tcp 127.0.0.1:10001: bind: address already in use"), 0o644); err != nil {
		t.Fatalf("write stderr failed: %v", err)
	}
	if !manager.isRetryableBridgeReadyError(fmt.Errorf("xray 进程提前退出"), cfgPath, stderrPath) {
		t.Fatalf("bind conflict should be retried with another port")
	}
}

func TestXrayRegisterBridgeTransfersRestartingRefCount(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	manager := &XrayManager{Bridges: make(map[string]*XrayBridge)}
	oldBridge := &XrayBridge{
		NodeKey:    "node-a",
		Port:       21001,
		Running:    false,
		RefCount:   2,
		Restarting: true,
	}
	manager.Bridges["node-a"] = oldBridge
	newBridge := &XrayBridge{NodeKey: "node-a", Port: port, Running: true}

	socksURL, reused := manager.registerBridge("node-a", newBridge, false)
	if reused {
		t.Fatalf("expected new restarted bridge registration, got reused %q", socksURL)
	}
	if manager.Bridges["node-a"] != newBridge {
		t.Fatalf("new bridge was not registered")
	}
	if newBridge.RefCount != 2 {
		t.Fatalf("restart refcount = %d, want 2", newBridge.RefCount)
	}
}
