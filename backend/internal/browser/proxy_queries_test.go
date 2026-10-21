package browser

import (
	"errors"
	"testing"
)

type proxyQueryTestDAO struct {
	list []Proxy
	err  error
}

func (d *proxyQueryTestDAO) List() ([]Proxy, error) { return d.list, d.err }

func (d *proxyQueryTestDAO) ListByGroup(string) ([]Proxy, error) { return d.list, d.err }

func (d *proxyQueryTestDAO) ListGroups() ([]string, error) { return nil, d.err }

func (d *proxyQueryTestDAO) Upsert(Proxy) error { return nil }

func (d *proxyQueryTestDAO) ReplaceAll([]Proxy) error { return nil }

func (d *proxyQueryTestDAO) Delete(string) error { return nil }

func (d *proxyQueryTestDAO) DeleteAll() error { return nil }

func (d *proxyQueryTestDAO) UpdateSpeedResult(string, bool, int64, string) error { return nil }

func (d *proxyQueryTestDAO) UpdateIPHealthResult(string, string) error { return nil }

func TestProxyQueriesDoNotUseFallbackWhenDAOIsAvailable(t *testing.T) {
	fallback := []Proxy{{ProxyId: "stale", ProxyName: "stale", ProxyConfig: "http://stale:8080"}}
	dao := &proxyQueryTestDAO{err: errors.New("database unavailable")}

	list := ListProxiesWithFallback(dao, fallback)
	if len(list) != 1 || list[0].ProxyId != directProxyID {
		t.Fatalf("proxy list after DAO failure = %#v, want direct proxy only", list)
	}
	if got := ListProxiesByGroupWithFallback(dao, "stale", fallback); len(got) != 0 {
		t.Fatalf("group list after DAO failure = %#v, want empty", got)
	}
	latest := LatestProxiesWithFallback(dao, fallback)
	if len(latest) != 1 || latest[0].ProxyId != directProxyID {
		t.Fatalf("latest proxy list after DAO failure = %#v, want direct proxy only", latest)
	}
}

func TestProxyQueriesDoNotUseFallbackWhenDatabaseIsEmpty(t *testing.T) {
	fallback := []Proxy{{ProxyId: "stale", ProxyName: "stale", ProxyConfig: "http://stale:8080"}}
	dao := &proxyQueryTestDAO{}
	list := ListProxiesWithFallback(dao, fallback)
	if len(list) != 1 || list[0].ProxyId != directProxyID {
		t.Fatalf("proxy list for empty DAO = %#v, want direct proxy only", list)
	}
}
