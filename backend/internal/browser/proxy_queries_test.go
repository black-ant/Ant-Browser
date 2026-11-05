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

func TestLoadProxyQueriesReturnDAOErrorsWithoutConfigFallback(t *testing.T) {
	fallback := []Proxy{{ProxyId: "stale", ProxyName: "stale", ProxyConfig: "http://stale:8080", GroupName: "stale-group"}}
	readErr := errors.New("database unavailable")
	dao := &proxyQueryTestDAO{err: readErr}

	list, err := LoadLatestProxiesWithFallback(dao, fallback)
	if !errors.Is(err, readErr) || len(list) != 1 || list[0].ProxyId != directProxyID {
		t.Fatalf("latest proxy query = %#v, err = %v", list, err)
	}

	groups, err := LoadProxyGroupsWithFallback(dao, fallback)
	if !errors.Is(err, readErr) || len(groups) != 0 {
		t.Fatalf("proxy groups query = %#v, err = %v", groups, err)
	}

	byGroup, err := LoadProxiesByGroupWithFallback(dao, "stale-group", fallback)
	if !errors.Is(err, readErr) || len(byGroup) != 0 {
		t.Fatalf("proxy group query = %#v, err = %v", byGroup, err)
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
