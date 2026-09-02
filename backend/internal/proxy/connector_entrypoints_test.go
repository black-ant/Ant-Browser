package proxy

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestBuildProxyDiagnosticUsesConfiguredConnectorStack(t *testing.T) {
	result := BuildProxyDiagnostic(
		"vless://00000000-0000-0000-0000-000000000000@example.com:443",
		nil,
		"",
		BuildDiagnosticOptions{ConnectorType: config.BrowserConnectorMihomo},
	)
	if result.Engine != ProxyKernelMihomo {
		t.Fatalf("diagnostic engine = %q, want %q; result=%+v", result.Engine, ProxyKernelMihomo, result)
	}
}

func TestProbeBrowserPageConnectivityUsesConfiguredConnectorStack(t *testing.T) {
	result := ProbeBrowserPageConnectivityWithConnector(
		"p1",
		[]config.BrowserProxy{{
			ProxyId:     "p1",
			ProxyConfig: "vless://00000000-0000-0000-0000-000000000000@example.com:443",
		}},
		nil,
		nil,
		nil,
		config.BrowserConnectorMihomo,
		&BrowserPageProbeConfig{URLs: []string{"http://127.0.0.1"}},
	)
	if !strings.Contains(result.Error, "Mihomo 管理器未初始化") {
		t.Fatalf("probe error = %q, want Mihomo initialization error; result=%+v", result.Error, result)
	}
}
