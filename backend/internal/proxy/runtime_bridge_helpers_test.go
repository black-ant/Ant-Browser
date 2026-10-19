package proxy

import "testing"

func TestComputeBridgeKeyUsesEngineSpecificInputs(t *testing.T) {
	proxyConfig := "  vless://example.com:443  "
	xrayKey := ComputeBridgeKey(proxyConfig, "1.1.1.1", ProxyKernelXray)
	if xrayKey == "" {
		t.Fatal("xray bridge key is empty")
	}
	if xrayKey == ComputeBridgeKey(proxyConfig, "8.8.8.8", ProxyKernelXray) {
		t.Fatal("xray bridge key ignored DNS settings")
	}
	if xrayKey == ComputeBridgeKey(proxyConfig, "1.1.1.1", ProxyKernelSingBox) {
		t.Fatal("different bridge engines shared a key")
	}
	if ComputeBridgeKey(proxyConfig, "", "unknown") != "" {
		t.Fatal("unknown bridge engine returned a key")
	}
}
