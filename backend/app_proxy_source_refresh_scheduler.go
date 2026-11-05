package backend

import (
	"sync"
	"time"

	"ant-chrome/backend/internal/logger"
)

const (
	// proxySourceRefreshTickInterval 调度巡检粒度；到期的源在同一轮内串行刷新。
	proxySourceRefreshTickInterval = time.Minute
	// proxySourceRefreshFailureRetry 单个源拉取失败后的重试冷却，避免每分钟都去撞失效的订阅地址。
	proxySourceRefreshFailureRetry = 5 * time.Minute
	// proxySourceRefreshInitialDelay 启动后先让测速/桥接初始化，避免启动阶段抢带宽。
	proxySourceRefreshInitialDelay = 2 * time.Minute
	// defaultProxySourceRefreshIntervalM 订阅行未配置间隔时的默认值。
	defaultProxySourceRefreshIntervalM = 60
	// minProxySourceRefreshIntervalM 最小刷新间隔（与前端 normalizeRefreshIntervalM 一致）。
	minProxySourceRefreshIntervalM = 5
	// maxProxySourceRefreshIntervalM 最大刷新间隔（与 internal/proxy 的 normalize 上限一致）。
	maxProxySourceRefreshIntervalM = 24 * 60
)

// proxySourceRefreshScheduler 后端订阅刷新调度器。
// 数据源是代理池里带 source_url 的行；拉取、匹配、保存全部复用 app_proxy_source_refresh.go 的实现，
// 与前端“刷新订阅”按钮走同一条链路，保证匹配语义一致。
type proxySourceRefreshScheduler struct {
	app           *App
	mu            sync.Mutex
	running       bool
	nextAttemptAt map[string]time.Time
	stopCh        chan struct{}
	done          chan struct{}
	stopOnce      sync.Once
}

func newProxySourceRefreshScheduler(app *App) *proxySourceRefreshScheduler {
	return &proxySourceRefreshScheduler{
		app:           app,
		nextAttemptAt: make(map[string]time.Time),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
	}
}

func (a *App) startupInitProxySourceRefreshScheduler() {
	if a == nil || a.proxySourceRefreshScheduler != nil {
		return
	}
	a.proxySourceRefreshScheduler = newProxySourceRefreshScheduler(a)
	a.proxySourceRefreshScheduler.start()
}

func (a *App) stopProxySourceRefreshScheduler() {
	if a == nil || a.proxySourceRefreshScheduler == nil {
		return
	}
	a.proxySourceRefreshScheduler.stop()
}

func (s *proxySourceRefreshScheduler) start() {
	go s.loop()
}

func (s *proxySourceRefreshScheduler) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.done
	})
}

func (s *proxySourceRefreshScheduler) loop() {
	defer close(s.done)

	select {
	case <-time.After(proxySourceRefreshInitialDelay):
	case <-s.stopCh:
		return
	}

	ticker := time.NewTicker(proxySourceRefreshTickInterval)
	defer ticker.Stop()
	s.check(time.Now())
	for {
		select {
		case <-ticker.C:
			s.check(time.Now())
		case <-s.stopCh:
			return
		}
	}
}

func (s *proxySourceRefreshScheduler) check(now time.Time) {
	s.mu.Lock()
	if s.running || s.app == nil {
		s.mu.Unlock()
		return
	}
	if !s.app.proxySourceRefreshEnabled() {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	s.refreshDueSources(now)
}

// refreshDueSources 找出到期的订阅源并串行刷新（订阅请求较重，不做并发）。
func (s *proxySourceRefreshScheduler) refreshDueSources(now time.Time) {
	app := s.app
	due := selectDueProxySources(app.getLatestProxies(), now)

	for _, meta := range due {
		if attemptAt, ok := s.nextAttempt(meta.SourceID); ok && now.Before(attemptAt) {
			continue
		}
		if _, err := app.refreshProxySourceWithMeta(app.getLatestProxies(), meta, meta.SourceFetchProxy); err != nil {
			s.setNextAttempt(meta.SourceID, now.Add(proxySourceRefreshFailureRetry))
			logger.New("ProxySource").Warn("订阅源定时刷新失败",
				logger.F("source_id", meta.SourceID),
				logger.F("source_url", meta.SourceURL),
				logger.F("retry_at", now.Add(proxySourceRefreshFailureRetry).Format(time.RFC3339)),
				logger.F("error", err.Error()),
			)
			continue
		}
		s.clearNextAttempt(meta.SourceID)
	}
}

// selectDueProxySources 只挑出"开启了自动更新"且到期的订阅源。
// 导入弹窗的"自动更新订阅"开关写入行上的 source_auto_refresh；关掉的源不参与定时拉取。
func selectDueProxySources(pool []BrowserProxy, now time.Time) []proxySourceMeta {
	due := make([]proxySourceMeta, 0)
	for _, meta := range collectProxySourceMetas(pool) {
		if !meta.AutoRefresh {
			continue
		}
		intervalM := meta.IntervalM
		if intervalM <= 0 {
			intervalM = defaultProxySourceRefreshIntervalM
		}
		if intervalM < minProxySourceRefreshIntervalM {
			intervalM = minProxySourceRefreshIntervalM
		}
		last := time.UnixMilli(parseProxyTimestampMs(meta.LastRefreshAt))
		if !last.IsZero() && now.Sub(last) < time.Duration(intervalM)*time.Minute {
			continue
		}
		due = append(due, meta)
	}
	return due
}

func (s *proxySourceRefreshScheduler) nextAttempt(sourceID string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.nextAttemptAt[sourceID]
	return at, ok
}

func (s *proxySourceRefreshScheduler) setNextAttempt(sourceID string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAttemptAt[sourceID] = at
}

func (s *proxySourceRefreshScheduler) clearNextAttempt(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nextAttemptAt, sourceID)
}
