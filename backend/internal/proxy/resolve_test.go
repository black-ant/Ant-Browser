package proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestResolveProxyConfigPrefersExplicitConfig(t *testing.T) {
	proxyID := "proxy-1"
	got := resolveProxyConfig(
		"http://temporary.example.com:8080",
		[]config.BrowserProxy{{ProxyId: proxyID, ProxyConfig: "http://saved.example.com:8080"}},
		proxyID,
	)
	if got != "http://temporary.example.com:8080" {
		t.Fatalf("resolveProxyConfig = %q, want explicit config", got)
	}
}

func TestResolveProxyConfigFallsBackToProxyID(t *testing.T) {
	proxyID := "proxy-1"
	got := resolveProxyConfig(
		"",
		[]config.BrowserProxy{{ProxyId: proxyID, ProxyConfig: "http://saved.example.com:8080"}},
		proxyID,
	)
	if got != "http://saved.example.com:8080" {
		t.Fatalf("resolveProxyConfig = %q, want saved config", got)
	}
}

func TestWaitSocks5ReadyContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	startedAt := time.Now()
	err := waitSocks5ReadyContext(ctx, "127.0.0.1", 1, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitSocks5ReadyContext error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("waitSocks5ReadyContext took %s after cancellation", elapsed)
	}
}

func TestBuildProxyHTTPClientContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := buildProxyHTTPClientContext(
		ctx,
		"vmess://temporary.example.com",
		"proxy-1",
		nil,
		nil,
		nil,
		nil,
		"xray",
		time.Second,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("buildProxyHTTPClientContext error = %v, want context.Canceled", err)
	}
}
