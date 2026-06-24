package browser

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"testing"
)

type proxyDAOStub struct {
	list []Proxy
	err  error
}

func (s *proxyDAOStub) List() ([]Proxy, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Proxy{}, s.list...), nil
}

func (s *proxyDAOStub) ListByGroup(string) ([]Proxy, error)        { return nil, nil }
func (s *proxyDAOStub) ListGroups() ([]string, error)              { return nil, nil }
func (s *proxyDAOStub) Upsert(Proxy) error                         { return nil }
func (s *proxyDAOStub) Delete(string) error                        { return nil }
func (s *proxyDAOStub) DeleteAll() error                           { return nil }
func (s *proxyDAOStub) ReplaceAllProxies([]Proxy) error            { return nil }
func (s *proxyDAOStub) ReplaceSourceProxies(string, []Proxy) error { return nil }
func (s *proxyDAOStub) UpdateSpeedResult(string, bool, int64, string) error {
	return nil
}
func (s *proxyDAOStub) UpdateIPHealthResult(string, string) error { return nil }

func TestGetProxyConfigByIdPreferDAO(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "pool-1", ProxyConfig: "http://127.0.0.1:9999"},
	}

	mgr := NewManager(cfg, "")
	mgr.ProxyDAO = &proxyDAOStub{
		list: []Proxy{
			{ProxyId: "pool-1", ProxyConfig: "socks5://127.0.0.1:1080"},
		},
	}

	got, ok := mgr.GetProxyConfigById("pool-1")
	if !ok {
		t.Fatalf("expected proxy to be found")
	}
	if got != "socks5://127.0.0.1:1080" {
		t.Fatalf("expected dao proxy config, got=%q", got)
	}
}

func TestGetProxyConfigByIdFallbackToConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "pool-2", ProxyConfig: "http://127.0.0.1:7890"},
	}

	mgr := NewManager(cfg, "")
	mgr.ProxyDAO = &proxyDAOStub{err: errors.New("dao unavailable")}

	got, ok := mgr.GetProxyConfigById("pool-2")
	if !ok {
		t.Fatalf("expected proxy to be found in config fallback")
	}
	if got != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy config: %q", got)
	}
}
