package browser

import (
	"testing"
	"time"
)

type blockingProxySpeedDAO struct {
	started  chan struct{}
	released chan struct{}
	finished chan struct{}
}

func (dao *blockingProxySpeedDAO) List() ([]Proxy, error) {
	return []Proxy{{ProxyId: "proxy-1", ProxyConfig: "http://127.0.0.1:8080"}}, nil
}

func (dao *blockingProxySpeedDAO) ListByGroup(string) ([]Proxy, error) { return nil, nil }

func (dao *blockingProxySpeedDAO) ListGroups() ([]string, error) { return nil, nil }

func (dao *blockingProxySpeedDAO) Upsert(Proxy) error { return nil }

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
