package browser

import (
	"strings"
	"sync"
	"time"
)

// SourceRefreshFunc 执行单个订阅源刷新的函数类型
type SourceRefreshFunc func(sourceId string) error

// ProxySourceScheduler 订阅源自动刷新调度器。
// 周期性遍历所有订阅源，对开启自动刷新且已到刷新间隔者触发刷新。
// 与 UI 是否打开无关，关闭代理池页面也能后台刷新。
type ProxySourceScheduler struct {
	dao     ProxySourceDAO
	refresh SourceRefreshFunc
	tick    time.Duration
	stopCh  chan struct{}
	mu      sync.Mutex
	running bool
}

// NewProxySourceScheduler 创建订阅源调度器，tick 为巡检间隔（默认 60s）
func NewProxySourceScheduler(dao ProxySourceDAO, refresh SourceRefreshFunc, tick time.Duration) *ProxySourceScheduler {
	if tick <= 0 {
		tick = 60 * time.Second
	}
	return &ProxySourceScheduler{
		dao:     dao,
		refresh: refresh,
		tick:    tick,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动调度（非阻塞）
func (s *ProxySourceScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || s.dao == nil || s.refresh == nil {
		return
	}
	s.running = true
	go s.loop()
}

// Stop 停止调度
func (s *ProxySourceScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

func (s *ProxySourceScheduler) loop() {
	// 启动后延迟 15s 再跑第一轮，避免影响启动速度
	select {
	case <-time.After(15 * time.Second):
	case <-s.stopCh:
		return
	}
	s.runDue()

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runDue()
		case <-s.stopCh:
			return
		}
	}
}

func (s *ProxySourceScheduler) runDue() {
	sources, err := s.dao.ListSources()
	if err != nil {
		return
	}
	now := time.Now()
	for _, src := range sources {
		if !src.AutoRefresh {
			continue
		}
		interval := src.RefreshIntervalM
		if interval <= 0 {
			interval = 60
		}
		if !sourceDue(src.LastRefreshAt, interval, now) {
			continue
		}
		_ = s.refresh(src.SourceID)
	}
}

// sourceDue 判断订阅源是否已到刷新时间
func sourceDue(lastRefreshAt string, intervalM int, now time.Time) bool {
	if strings.TrimSpace(lastRefreshAt) == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, lastRefreshAt)
	if err != nil {
		return true
	}
	return now.Sub(last) >= time.Duration(intervalM)*time.Minute
}
