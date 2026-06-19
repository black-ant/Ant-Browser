package backend

import (
	"strings"
	"testing"
)

const sampleClashYAML = `proxies:
  - {name: hk-1, type: vmess, server: a.example.com, port: 443}
  - {name: us-2, type: trojan, server: b.example.com, port: 8443}
`

func TestNormalizeClashSubscriptionContent(t *testing.T) {
	content, payload, err := normalizeClashSubscriptionContent([]byte(sampleClashYAML))
	if err != nil {
		t.Fatalf("normalizeClashSubscriptionContent error: %v", err)
	}
	if !strings.Contains(content, "hk-1") {
		t.Errorf("content missing node name, got: %q", content)
	}
	if n := clashProxyCount(payload); n != 2 {
		t.Errorf("clashProxyCount = %d, want 2", n)
	}
}

func TestExtractClashProxyNodesAndConfig(t *testing.T) {
	_, payload, err := normalizeClashSubscriptionContent([]byte(sampleClashYAML))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	nodes := extractClashProxyNodes(payload)
	if len(nodes) != 2 {
		t.Fatalf("extractClashProxyNodes len = %d, want 2", len(nodes))
	}
	if name := getMapString(nodes[0], "name"); name != "hk-1" {
		t.Errorf("node[0] name = %q, want hk-1", name)
	}

	cfg, err := proxyNodeToConfig(nodes[0])
	if err != nil {
		t.Fatalf("proxyNodeToConfig error: %v", err)
	}
	// proxyConfig 为单元素 YAML 数组，应包含节点名与协议类型
	if !strings.Contains(cfg, "hk-1") || !strings.Contains(cfg, "vmess") {
		t.Errorf("proxyNodeToConfig output missing fields: %q", cfg)
	}
}

func TestNormalizeClashSubscriptionContentRejectsEmpty(t *testing.T) {
	if _, _, err := normalizeClashSubscriptionContent([]byte("   ")); err == nil {
		t.Error("expected error for empty subscription content")
	}
}
