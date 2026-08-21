package backend

import (
	"math/rand"
	"time"

	"ant-chrome/backend/internal/logger"
)

// 直播实例长时运行会缓慢累积图片缓存、脚本缓存与已解码资源。
//
// 做法:定时向每个运行中实例发送一次 CDP 内存压力通知,触发 Chromium 自身的回收链路 ——
// 它本就会**优先丢缓存类内存、保留正在播放的媒体缓冲**,所以不需要我们自己实现优先级。
//
// 错峰:与保活同理,各实例首次触发时间在一个完整周期内独立随机,避免上百实例
// 在同一刻一起回收造成 CPU 尖峰。
//
// 级别:默认 moderate。critical 回收更狠,但可能让播放器出现短暂卡顿 —— 需在真机
// 观察直播无异常后再开。

const (
	memoryReclaimTickResolution = 30 * time.Second // 调度粒度;间隔在 10min 量级,30s 足够
	memoryReclaimFloor          = time.Minute      // 间隔下限,避免过于频繁
	memoryReclaimDefault        = 10 * time.Minute
)

// memoryReclaimEnabled 解析开关:未配置(nil)默认开启。
func (a *App) memoryReclaimEnabled() bool {
	if a == nil || a.config == nil {
		return true
	}
	if p := a.config.Browser.MemoryReclaimEnabled; p != nil {
		return *p
	}
	return true
}

// memoryReclaimInterval 返回回收间隔,低于下限时抬到下限。
func (a *App) memoryReclaimInterval() time.Duration {
	interval := memoryReclaimDefault
	if a != nil && a.config != nil && a.config.Browser.MemoryReclaimIntervalMs > 0 {
		interval = time.Duration(a.config.Browser.MemoryReclaimIntervalMs) * time.Millisecond
	}
	if interval < memoryReclaimFloor {
		interval = memoryReclaimFloor
	}
	return interval
}

// memoryReclaimLevel 返回压力级别,白名单外一律回退 moderate。
func (a *App) memoryReclaimLevel() string {
	if a != nil && a.config != nil && a.config.Browser.MemoryReclaimLevel == "critical" {
		return "critical"
	}
	return "moderate"
}

// memoryReclaimShouldReclaim 判定某实例本轮是否参与回收。
func memoryReclaimShouldReclaim(running, ready bool, debugPort int) bool {
	return running && ready && debugPort > 0
}

// startMemoryReclaim 启动后台回收循环(每次启动调用一次)。随 app 上下文取消而退出。
// 单 goroutine 维护每实例的下次触发时间,无需加锁。
func (a *App) startMemoryReclaim() {
	if a == nil {
		return
	}
	log := logger.New("MemReclaim")
	var done <-chan struct{}
	if a.ctx != nil {
		done = a.ctx.Done()
	}
	next := map[string]time.Time{} // profileId -> 下次触发时间

	go func() {
		ticker := time.NewTicker(memoryReclaimTickResolution)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				if !a.memoryReclaimEnabled() {
					continue
				}
				a.runMemoryReclaimDue(log, next, now)
			}
		}
	}()
}

// runMemoryReclaimDue 对到期实例发一次内存压力通知,并安排下一次。
// 实例首次出现时给一个 [0,interval) 的随机首触发 → 各实例天然错峰。
func (a *App) runMemoryReclaimDue(log *logger.Logger, next map[string]time.Time, now time.Time) {
	if a.browserMgr == nil {
		return
	}
	interval := a.memoryReclaimInterval()
	level := a.memoryReclaimLevel()
	live := map[string]struct{}{}

	for _, p := range a.browserMgr.List() {
		if !memoryReclaimShouldReclaim(p.Running, p.DebugReady, p.DebugPort) {
			continue
		}
		live[p.ProfileId] = struct{}{}

		t, ok := next[p.ProfileId]
		if !ok {
			next[p.ProfileId] = now.Add(time.Duration(rand.Int63n(int64(interval) + 1)))
			continue
		}
		if now.Before(t) {
			continue
		}
		a.reclaimProfileMemory(log, p.ProfileId, p.DebugPort, level)
		next[p.ProfileId] = now.Add(interval)
	}

	// 清理已停实例的调度,避免 map 泄漏
	for id := range next {
		if _, ok := live[id]; !ok {
			delete(next, id)
		}
	}
}

// reclaimProfileMemory 触发一次实例内存回收。
// 优先用浏览器级 Memory.simulatePressureNotification(走 Chromium 自己的回收优先级);
// 若该实例的内核不支持该命令,退回逐标签 HeapProfiler.collectGarbage。
func (a *App) reclaimProfileMemory(log *logger.Logger, profileID string, debugPort int, level string) {
	err := cdpBrowserCall(debugPort, "Memory.simulatePressureNotification", map[string]any{"level": level})
	if err == nil {
		return
	}
	log.Debug("内存压力通知失败,回退 GC",
		logger.F("profile", profileID),
		logger.F("error", err.Error()),
	)
	if _, gcErr := cdpCall(debugPort, "HeapProfiler.collectGarbage", nil); gcErr != nil {
		log.Debug("回退 GC 也失败",
			logger.F("profile", profileID),
			logger.F("error", gcErr.Error()),
		)
	}
}
