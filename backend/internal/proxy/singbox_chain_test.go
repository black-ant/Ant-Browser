package proxy

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

func buildTestHy2Chain(t *testing.T, scheme string) string {
	t.Helper()
	first := scheme + `://password@hy2.example.invalid:443?sni=hy2.example.invalid&insecure=1`
	raw, err := json.Marshal(map[string]interface{}{
		"localPort": 19191,
		"first":     map[string]interface{}{"proxyConfig": first},
		"second": map[string]interface{}{
			"protocol": "socks5",
			"server":   "127.0.0.2",
			"port":     1080,
			"username": "user",
			"password": "pass",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return chainSocks5Prefix + url.QueryEscape(string(raw))
}

func TestHy2ChainUsesSingBoxAndDetour(t *testing.T) {
	for _, scheme := range []string{"hysteria2", "hy2"} {
		t.Run(scheme, func(t *testing.T) {
			chainConfig := buildTestHy2Chain(t, scheme)
			chainCfg, err := ParseChainSocks5Config(chainConfig)
			if err != nil {
				t.Fatalf("ParseChainSocks5Config returned error: %v", err)
			}

			resolution, err := ResolveProxyKernel(chainConfig, nil, "", "")
			if err != nil {
				t.Fatalf("ResolveProxyKernel returned error: %v", err)
			}
			if resolution.Kernel != ProxyKernelSingBox {
				t.Fatalf("kernel = %q, want sing-box; resolution=%+v", resolution.Kernel, resolution)
			}

			outbounds, routeOutbound, err := buildSingBoxChainOutbounds(chainCfg)
			if err != nil {
				t.Fatalf("buildSingBoxChainOutbounds returned error: %v", err)
			}
			if routeOutbound != "second-hop" || len(outbounds) != 2 {
				t.Fatalf("unexpected chain route/outbounds: route=%q len=%d", routeOutbound, len(outbounds))
			}
			first := outbounds[0].(map[string]interface{})
			if first["type"] != "hysteria2" || first["tag"] != "first-hop" {
				t.Fatalf("unexpected HY2 first hop: %#v", first)
			}
			second := outbounds[1].(map[string]interface{})
			if second["type"] != "socks" || second["tag"] != "second-hop" || second["detour"] != "first-hop" {
				t.Fatalf("unexpected second hop: %#v", second)
			}
		})
	}
}

func TestHy2ChainRuntimeConfigRoutesThroughSecondHop(t *testing.T) {
	chainConfig := buildTestHy2Chain(t, "hy2")
	chainCfg, err := ParseChainSocks5Config(chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	outbounds, routeOutbound, err := buildSingBoxChainOutbounds(chainCfg)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = t.TempDir()
	manager := NewSingBoxManager(cfg, t.TempDir())
	defer manager.StopAll()
	cfgPath, err := manager.buildConfigWithOutbounds("hy2-chain-test", outbounds, routeOutbound, chainCfg.LocalPort)
	if err != nil {
		t.Fatalf("buildConfigWithOutbounds returned error: %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var runtime map[string]interface{}
	if err := json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	inbound := runtime["inbounds"].([]interface{})[0].(map[string]interface{})
	if got := int(inbound["listen_port"].(float64)); got != 19191 {
		t.Fatalf("listen_port = %d, want 19191", got)
	}
	route := runtime["route"].(map[string]interface{})
	rules := route["rules"].([]interface{})
	lastRule := rules[len(rules)-1].(map[string]interface{})
	if got := lastRule["outbound"]; got != "second-hop" {
		t.Fatalf("route outbound = %v, want second-hop", got)
	}
	builtOutbounds := runtime["outbounds"].([]interface{})
	second := builtOutbounds[1].(map[string]interface{})
	if got := second["detour"]; got != "first-hop" {
		t.Fatalf("second-hop detour = %v, want first-hop", got)
	}

	binary := filepath.Clean(filepath.Join("..", "..", "..", "bin", "sing-box.exe"))
	if _, err := os.Stat(binary); err == nil {
		if output, err := exec.Command(binary, "check", "-c", cfgPath).CombinedOutput(); err != nil {
			t.Fatalf("bundled sing-box rejected HY2 chain config: %v\n%s", err, string(output))
		}
	}
}
