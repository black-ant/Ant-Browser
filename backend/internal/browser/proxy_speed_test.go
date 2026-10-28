package browser

import (
	"sync"
	"testing"
	"time"
)

type blockingProxySpeedDAO struct {
	started  chan struct{}
	released chan struct{}
	finished chan struct{}
}

type proxySpeedSchedulerTestDAO struct {
	proxies []Proxy
	updates chan string
}

func (dao *proxySpeedSchedulerTestDAO) List() ([]Proxy, error)              { return dao.proxies, nil }
func (dao *proxySpeedSchedulerTestDAO) ListByGroup(string) ([]Proxy, error) { return nil, nil }
func (dao *proxySpeedSchedulerTestDAO) ListGroups() ([]string, error)       { return nil, nil }
func (dao *proxySpeedSchedulerTestDAO) Upsert(Proxy) error                  { return nil }
func (dao *proxySpeedSchedulerTestDAO) ReplaceAll([]Proxy) error            { return nil }
func (dao *proxySpeedSchedulerTestDAO) Delete(string) error                 { return nil }
func (dao *proxySpeedSchedulerTestDAO) DeleteAll() error                    { return nil }
func (dao *proxySpeedSchedulerTestDAO) UpdateSpeedResult(proxyID string, _ bool, _ int64, _ string) error {
	dao.updates <- proxyID
	return nil
}
func (dao *proxySpeedSchedulerTestDAO) UpdateIPHealthResult(string, string) error { return nil }

func TestProxySpeedSchedulerRetriesFailedAndStaleResults(t *testing.T) {
	testedAt := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	dao := &proxySpeedSchedulerTestDAO{
		proxies: []Proxy{{ProxyId: "failed", ProxyConfig: "http://127.0.0.1:8080", LastTestedAt: testedAt}},
		updates: make(chan string, 2),
	}
	var testMu sync.Mutex
	var tested []string
	scheduler := NewProxySpeedSchedulerWithRetry(dao, nil, 30*time.Minute, 5*time.Minute, 1)
	scheduler.SetProxySpeedTestFunc(func(item Proxy) (bool, int64, string) {
		testMu.Lock()
		tested = append(tested, item.ProxyId)
		testMu.Unlock()
		return true, 20, ""
	})
	scheduler.SetResultCurrentFunc(func(item Proxy, _ []Proxy) bool { return item.LastTestOk })
	scheduler.RunOnce()
	select {
	case proxyID := <-dao.updates:
		if proxyID != "failed" {
			t.Fatalf("updated proxy = %q, want failed", proxyID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("failed result was not retried")
	}

	if len(tested) != 1 {
		t.Fatalf("tested proxies = %#v, want one retry", tested)
	}
	scheduler.Stop()

	dao.proxies = []Proxy{{ProxyId: "stale", ProxyConfig: "http://127.0.0.1:8080", LastTestOk: true, LastTestedAt: time.Now().Add(-time.Minute).Format(time.RFC3339)}}
	staleScheduler := NewProxySpeedSchedulerWithRetry(dao, nil, 30*time.Minute, 5*time.Minute, 1)
	staleScheduler.SetProxySpeedTestFunc(func(item Proxy) (bool, int64, string) {
		return true, 20, ""
	})
	staleScheduler.SetResultCurrentFunc(func(Proxy, []Proxy) bool { return false })
	staleScheduler.RunOnce()
	select {
	case proxyID := <-dao.updates:
		if proxyID != "stale" {
			t.Fatalf("updated proxy = %q, want stale", proxyID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stale result was not retested")
	}
	staleScheduler.Stop()
}

// 分组成员应优先探测，并按自定义间隔提前到期。
func TestProxySpeedSchedulerAppliesPriorityAndInterval(t *testing.T) {
	dao := &proxySpeedSchedulerTestDAO{
		proxies: []Proxy{
			{ProxyId: "recent-pool", ProxyConfig: "http://pool:8080", LastTestOk: true, LastTestedAt: time.Now().Format(time.RFC3339)},
			{ProxyId: "recent-member", ProxyConfig: "http://member:8080", LastTestOk: true, LastTestedAt: time.Now().Add(-5 * time.Minute).Format(time.RFC3339)},
			{ProxyId: "old-pool", ProxyConfig: "http://old:8080", LastTestOk: true, LastTestedAt: time.Now().Add(-40 * time.Minute).Format(time.RFC3339)},
		},
		updates: make(chan string, 8),
	}
	var mu sync.Mutex
	tested := make([]string, 0, 3)
	scheduler := NewProxySpeedSchedulerWithRetry(dao, nil, 30*time.Minute, 5*time.Minute, 1)
	scheduler.SetProxySpeedTestFunc(func(item Proxy) (bool, int64, string) {
		mu.Lock()
		tested = append(tested, item.ProxyId)
		mu.Unlock()
		return true, 20, ""
	})
	scheduler.SetResultCurrentFunc(func(Proxy, []Proxy) bool { return true })
	scheduler.SetPriorityFunc(func(item Proxy) int {
		if item.ProxyId == "recent-member" {
			return 100
		}
		return 0
	})
	scheduler.SetIntervalFunc(func(item Proxy) time.Duration {
		if item.ProxyId == "recent-member" {
			return time.Minute
		}
		return 0
	})
	scheduler.RunOnce()

	deadline := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-dao.updates:
		case <-deadline:
			t.Fatalf("只完成了 %d 个节点探测", i)
		}
	}
	scheduler.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(tested) != 2 {
		t.Fatalf("tested = %#v, want 2 个（recent-pool 未到期）", tested)
	}
	if tested[0] != "recent-member" {
		t.Fatalf("首个探测节点 = %q, want recent-member（分组成员优先）", tested[0])
	}
	if tested[1] != "old-pool" {
		t.Fatalf("第二个探测节点 = %q, want old-pool", tested[1])
	}
}

func (dao *blockingProxySpeedDAO) List() ([]Proxy, error) {
	return []Proxy{{ProxyId: "proxy-1", ProxyConfig: "http://127.0.0.1:8080"}}, nil
}

func (dao *blockingProxySpeedDAO) ListByGroup(string) ([]Proxy, error) { return nil, nil }

func (dao *blockingProxySpeedDAO) ListGroups() ([]string, error) { return nil, nil }

func (dao *blockingProxySpeedDAO) Upsert(Proxy) error { return nil }

func (dao *blockingProxySpeedDAO) ReplaceAll([]Proxy) error { return nil }

func (dao *blockingProxySpeedDAO) Delete(string) error { return nil }

func (dao *blockingProxySpeedDAO) DeleteAll() error { return nil }

func (dao *blockingProxySpeedDAO) UpdateSpeedResult(string, bool, int64, string) error {
	close(dao.finished)
	return nil
}

func (dao *blockingProxySpeedDAO) UpdateIPHealthResult(string, string) error { return nil }

func TestProxySpeedSchedulerStopWaitsForRunningTest(t *testing.T) {
	dao := &blockingProxySpeedDAO{
		started:  make(chan struct{}),
		released: make(chan struct{}),
		finished: make(chan struct{}),
	}
	scheduler := NewProxySpeedScheduler(dao, func(string) (bool, int64, string) {
		close(dao.started)
		<-dao.released
		return true, 10, ""
	}, time.Hour, 1)
	scheduler.Start()
	scheduler.RunOnce()

	select {
	case <-dao.started:
	case <-time.After(2 * time.Second):
		t.Fatal("speed test did not start")
	}

	stopped := make(chan struct{})
	go func() {
		scheduler.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("scheduler returned before speed test finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(dao.released)
	select {
	case <-dao.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("speed test did not finish")
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler stop timed out")
	}
}

func TestProxySpeedSchedulerDoesNotRunAfterStop(t *testing.T) {
	dao := &blockingProxySpeedDAO{
		started:  make(chan struct{}),
		released: make(chan struct{}),
		finished: make(chan struct{}),
	}
	scheduler := NewProxySpeedScheduler(dao, func(string) (bool, int64, string) {
		close(dao.started)
		return true, 10, ""
	}, time.Hour, 1)
	scheduler.Start()
	scheduler.Stop()
	scheduler.RunOnce()

	select {
	case <-dao.started:
		t.Fatal("stopped scheduler started a speed test")
	case <-time.After(100 * time.Millisecond):
	}
}
