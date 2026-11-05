package proxy

import (
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestXrayReclaimBridgeIfIdle(t *testing.T) {
	mgr := NewXrayManager(config.DefaultConfig(), t.TempDir())
	key := "xray-reclaim-key"
	mgr.Bridges[key] = &XrayBridge{NodeKey: key, Running: true, LastUsedAt: time.Now()}

	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.Bridges[key]; exists {
		t.Fatal("idle xray bridge was not reclaimed")
	}

	pinned := &XrayBridge{NodeKey: key, Running: true, RefCount: 1, LastUsedAt: time.Now()}
	mgr.Bridges[key] = pinned
	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.Bridges[key]; !exists {
		t.Fatal("pinned xray bridge was reclaimed while in use")
	}
}

func TestSingBoxReclaimBridgeIfIdle(t *testing.T) {
	mgr := NewSingBoxManager(config.DefaultConfig(), t.TempDir())
	key := "singbox-reclaim-key"
	mgr.Bridges[key] = &SingBoxBridge{NodeKey: key, Running: true, LastUsedAt: time.Now()}

	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.Bridges[key]; exists {
		t.Fatal("idle sing-box bridge was not reclaimed")
	}

	mgr.Bridges[key] = &SingBoxBridge{NodeKey: key, Running: true, RefCount: 1, LastUsedAt: time.Now()}
	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.Bridges[key]; !exists {
		t.Fatal("pinned sing-box bridge was reclaimed while in use")
	}
}

func TestMihomoReclaimBridgeIfIdle(t *testing.T) {
	mgr := NewClashManager(config.DefaultConfig(), t.TempDir())
	key := "mihomo-reclaim-key"
	if mgr.NodeBridges == nil {
		mgr.NodeBridges = map[string]*MihomoNodeBridge{}
	}
	mgr.NodeBridges[key] = &MihomoNodeBridge{NodeKey: key, Running: true, LastUsedAt: time.Now()}

	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.NodeBridges[key]; exists {
		t.Fatal("idle mihomo bridge was not reclaimed")
	}

	mgr.NodeBridges[key] = &MihomoNodeBridge{NodeKey: key, Running: true, RefCount: 1, LastUsedAt: time.Now()}
	mgr.ReclaimBridgeIfIdle(key)
	if _, exists := mgr.NodeBridges[key]; !exists {
		t.Fatal("pinned mihomo bridge was reclaimed while in use")
	}
}
