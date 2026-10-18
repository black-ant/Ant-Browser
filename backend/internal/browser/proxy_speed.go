package browser

import (
	"strings"
	"sync"
	"time"
)

// SpeedTestFunc 执行单个代理测速的函数类型
type SpeedTestFunc func(proxyId string) (ok bool, latencyMs int64, err string)
type ProxySpeedTestFunc func(proxy Proxy) (ok bool, latencyMs int64, err string)
type ProxySpeedResultCurrentFunc func(proxy Proxy, proxies []Proxy) bool

// ProxySpeedScheduler 代理测速定时调度器
type ProxySpeedScheduler struct {
	dao           ProxyDAO
	testFn        SpeedTestFunc
	proxyTestFn   ProxySpeedTestFunc
	resultCurrent ProxySpeedResultCurrentFunc
	persistResult bool
	interval      time.Duration
	failureRetry  time.Duration
	concLimit     int
	stopCh        chan struct{}
	doneCh        chan struct{}
	mu            sync.Mutex
	running       bool
	testing       bool
	stopped       bool
	runWG         sync.WaitGroup
}

const (
	DefaultProxySpeedInterval     = 30 * time.Minute
	DefaultProxySpeedInitialDelay = 2 * time.Minute
	DefaultProxySpeedConcurrency  = 2
	DefaultProxySpeedFailureRetry = 5 * time.Minute
)

func isProxySpeedResultDue(testedAt string, checkedAt time.Time, interval time.Duration) bool {
	if interval <= 0 {
		interval = DefaultProxySpeedInterval
	}
	testedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(testedAt))
	if err != nil {
		return true
	}
	age := checkedAt.Sub(testedTime)
	return age < 0 || age >= interval
}

// NewProxySpeedScheduler 创建调度器，interval 为测速间隔，concLimit 为并发数
func NewProxySpeedScheduler(dao ProxyDAO, testFn SpeedTestFunc, interval time.Duration, concLimit int) *ProxySpeedScheduler {
	if interval <= 0 {
		interval = DefaultProxySpeedInterval
	}
	if concLimit <= 0 {
		concLimit = DefaultProxySpeedConcurrency
	}
	return &ProxySpeedScheduler{
		dao:           dao,
		testFn:        testFn,
		interval:      interval,
		failureRetry:  DefaultProxySpeedFailureRetry,
		concLimit:     concLimit,
		stopCh:        make(chan struct{}),
		persistResult: true,
	}
}

func NewProxySpeedSchedulerWithRetry(dao ProxyDAO, testFn SpeedTestFunc, interval time.Duration, failureRetry time.Duration, concLimit int) *ProxySpeedScheduler {
	s := NewProxySpeedScheduler(dao, testFn, interval, concLimit)
	if failureRetry > 0 {
		s.failureRetry = failureRetry
	}
	return s
}

func (s *ProxySpeedScheduler) SetProxySpeedTestFunc(testFn ProxySpeedTestFunc) {
	s.mu.Lock()
	s.proxyTestFn = testFn
	s.mu.Unlock()
}

func (s *ProxySpeedScheduler) SetResultCurrentFunc(check ProxySpeedResultCurrentFunc) {
	s.mu.Lock()
	s.resultCurrent = check
	s.mu.Unlock()
}

func (s *ProxySpeedScheduler) SetResultPersistence(enabled bool) {
	s.mu.Lock()
	s.persistResult = enabled
	s.mu.Unlock()
}

// Start 启动定时任务（非阻塞）
func (s *ProxySpeedScheduler) Start() {
	s.mu.Lock()
	if s.running || s.stopped {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.doneCh = make(chan struct{})
	doneCh := s.doneCh
	s.mu.Unlock()
	go s.loop(doneCh)
}

// Stop 停止定时任务
func (s *ProxySpeedScheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		doneCh := s.doneCh
		s.mu.Unlock()
		if doneCh != nil {
			<-doneCh
		}
		s.runWG.Wait()
		return
	}
	s.stopped = true
	s.running = false
	doneCh := s.doneCh
	close(s.stopCh)
	s.mu.Unlock()

	if doneCh != nil {
		<-doneCh
	}
	s.runWG.Wait()
}

// RunOnce 立即执行一轮测速（可手动触发）
func (s *ProxySpeedScheduler) RunOnce() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.runWG.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.runWG.Done()
		s.runAll()
	}()
}

func (s *ProxySpeedScheduler) loop(doneCh chan struct{}) {
	defer close(doneCh)

	// 启动后延迟一段时间跑第一轮，避免启动阶段频繁拉起代理内核。
	select {
	case <-time.After(DefaultProxySpeedInitialDelay):
	case <-s.stopCh:
		return
	}
	s.runTracked()

	tickInterval := s.interval
	if s.failureRetry > 0 && s.failureRetry < tickInterval {
		tickInterval = s.failureRetry
	}
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runTracked()
		case <-s.stopCh:
			return
		}
	}
}

func (s *ProxySpeedScheduler) runTracked() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.runWG.Add(1)
	s.mu.Unlock()
	defer s.runWG.Done()
	s.runAll()
}

func (s *ProxySpeedScheduler) runAll() {
	if !s.beginRun() {
		return
	}
	defer s.finishRun()

	proxies, err := s.dao.List()
	if err != nil || len(proxies) == 0 {
		return
	}

	sem := make(chan struct{}, s.concLimit)
	var wg sync.WaitGroup
	s.mu.Lock()
	testFn := s.testFn
	proxyTestFn := s.proxyTestFn
	resultCurrent := s.resultCurrent
	persistResult := s.persistResult
	s.mu.Unlock()

	for _, p := range proxies {
		// 跳过直连（无意义测速）
		if p.ProxyConfig == "direct://" {
			continue
		}
		nextInterval := s.interval
		if !p.LastTestOk {
			nextInterval = s.failureRetry
		}
		due := isProxySpeedResultDue(p.LastTestedAt, time.Now(), nextInterval)
		if !due && resultCurrent != nil && !resultCurrent(p, proxies) {
			due = true
		}
		if !due || testFn == nil && proxyTestFn == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(proxy Proxy) {
			defer wg.Done()
			defer func() { <-sem }()
			proxyId := proxy.ProxyId

			var ok bool
			var latencyMs int64
			var testErr string
			if proxyTestFn != nil {
				ok, latencyMs, testErr = proxyTestFn(proxy)
			} else {
				ok, latencyMs, testErr = testFn(proxyId)
			}
			testedAt := time.Now().Format(time.RFC3339)
			if persistResult {
				_ = s.dao.UpdateSpeedResult(proxyId, ok, latencyMs, testedAt)
			}
			_ = testErr
		}(p)
	}
	wg.Wait()
}

func (s *ProxySpeedScheduler) beginRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.testing {
		return false
	}
	s.testing = true
	return true
}

func (s *ProxySpeedScheduler) finishRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.testing = false
}
