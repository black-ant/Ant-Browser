package backend

import (
	"sync"
	"time"
)

// instanceLaunchGate 实例并发启动闸门（安全控制）：
// limit<=0 时不限制；否则超出"同时运行实例数"的启动请求排队等待，
// 直到有实例停止腾出名额后自动放行。等待期间不持有任何其它锁，
// 停止实例不会被排队阻塞，避免"等名额→但名额只能靠停止释放→停止被锁"的死锁。
//
// 占用计数统计"经本闸门获准启动或已恢复的运行实例"；释放统一挂在
// markProfileStoppedLocked（全部停止路径的总汇：手动停止/状态巡检/备份恢复/关机）。
type instanceLaunchGate struct {
	mu        sync.Mutex
	admitted  map[string]int // profileId -> 获准且未释放的占用数
	releaseCh chan struct{}  // 关闭以唤醒排队中的启动请求
	waiting   int            // 当前排队中的启动请求数
}

func newLaunchGate() *instanceLaunchGate {
	return &instanceLaunchGate{
		admitted:  map[string]int{},
		releaseCh: make(chan struct{}),
	}
}

func (g *instanceLaunchGate) occupied() int {
	total := 0
	for _, n := range g.admitted {
		if n > 0 {
			total += n
		}
	}
	return total
}

func (g *instanceLaunchGate) broadcast() {
	close(g.releaseCh)
	g.releaseCh = make(chan struct{})
}

func (g *instanceLaunchGate) release(profileId string) {
	g.mu.Lock()
	if n := g.admitted[profileId]; n > 1 {
		g.admitted[profileId] = n - 1
	} else {
		delete(g.admitted, profileId)
	}
	g.broadcast()
	g.mu.Unlock()
}

func (g *instanceLaunchGate) retain(profileId string) {
	g.mu.Lock()
	if g.admitted[profileId] == 0 {
		g.admitted[profileId] = 1
	}
	g.mu.Unlock()
}

func (a *App) instanceLaunchLimit() int {
	if a == nil || a.config == nil {
		return 0
	}
	if a.config.Browser.MaxConcurrentInstances <= 0 {
		return 0 // 0/负数 = 不限制
	}
	return a.config.Browser.MaxConcurrentInstances
}

func (a *App) launchGate() *instanceLaunchGate {
	a.instanceGateOnce.Do(func() {
		a.instanceGate = newLaunchGate()
	})
	return a.instanceGate
}

// releaseInstanceLaunchSlot 供停止路径调用：实例停止即归还名额并唤醒排队请求。
func (a *App) releaseInstanceLaunchSlot(profileId string) {
	if a == nil || a.instanceGate == nil {
		return
	}
	a.instanceGate.release(profileId)
}

// retainInstanceLaunchSlot 将已在运行的恢复实例计入并发上限。
func (a *App) retainInstanceLaunchSlot(profileId string) {
	if a == nil || profileId == "" || a.instanceLaunchLimit() <= 0 {
		return
	}
	a.launchGate().retain(profileId)
}

func (a *App) reconcileInstanceLaunchSlots() {
	if a == nil || a.browserMgr == nil || a.instanceLaunchLimit() <= 0 {
		return
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	for profileId, profile := range a.browserMgr.Profiles {
		if profile != nil && profile.Running {
			a.retainInstanceLaunchSlot(profileId)
		}
	}
}

// acquireInstanceLaunchSlot 返回名额释放函数。
// queued=true 表示该请求经历 过排队（前端据此提示"已进入启动队列"）。
// 同一实例重复的启动请求不重复占名额（启动逻辑内部会按"已在运行"处理）。
func (a *App) acquireInstanceLaunchSlot(profileId string) (release func(), queued bool) {
	gate := a.launchGate()
	noop := func() {}
	queued = false
	for {
		limit := a.instanceLaunchLimit()
		if limit <= 0 {
			if queued {
				gate.mu.Lock()
				gate.waiting--
				gate.mu.Unlock()
			}
			return noop, queued
		}
		gate.mu.Lock()
		if gate.admitted[profileId] > 0 {
			gate.mu.Unlock()
			return noop, queued
		}
		if gate.occupied() < limit {
			gate.admitted[profileId]++
			if queued {
				gate.waiting--
			}
			gate.mu.Unlock()
			a.emitRuntimeEvent("instance:launch:admitted", map[string]interface{}{"profileId": profileId})
			return func() { gate.release(profileId) }, queued
		}
		if !queued {
			queued = true
			gate.waiting++
			position := gate.waiting
			gate.mu.Unlock()
			a.emitRuntimeEvent("instance:launch:queued", map[string]interface{}{
				"profileId": profileId,
				"position":  position,
				"limit":     limit,
			})
		} else {
			gate.mu.Unlock()
		}
		select {
		case <-gate.releaseCh:
		case <-time.After(300 * time.Millisecond):
			// 轮询兜底：limit 被调大等外部变化也能及时放行。
		}
	}
}
