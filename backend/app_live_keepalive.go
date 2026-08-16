package backend

import (
	"math/rand"
	"time"

	"ant-chrome/backend/internal/logger"
)

// 直播/视频「防挂机」保活。
//
// 背景:抖音等直播/视频网页有自己的**挂机检测**——一段时间(约 2–3 分钟)没有真实
// 鼠标/键盘活动,就暂停播放"节能"并弹「长时间无操作,已暂停播放」。这不是浏览器后台
// 节流,是页面级 JS 在监听真实用户输入。多开养号时会被逐个暂停。
//
// 方案:定时用 CDP `Input.dispatchMouseEvent`(mouseMoved)向每个运行中的实例注入一次
// **可信输入**(isTrusted=true,与真人无法区分,不能用页面 JS 的 dispatchEvent),重置挂机
// 计时器。配合启动参数(禁用后台/遮挡节流)一起,解决多开直播被暂停的问题。注入的是
// mouseMoved,不会点击、不动真实 OS 光标,对页面无副作用。

// liveKeepAliveEnabledResolved 解析开关:未配置(nil)默认开启。
func (a *App) liveKeepAliveEnabledResolved() bool {
	if a == nil || a.config == nil {
		return true
	}
	if p := a.config.Browser.LiveKeepAliveEnabled; p != nil {
		return *p
	}
	return true
}

// liveKeepAliveInterval 保活间隔:默认 75s,下限 15s(避免过于频繁)。
func (a *App) liveKeepAliveInterval() time.Duration {
	ms := 75000
	if a != nil && a.config != nil && a.config.Browser.LiveKeepAliveIntervalMs > 0 {
		ms = a.config.Browser.LiveKeepAliveIntervalMs
	}
	if ms < 15000 {
		ms = 15000
	}
	return time.Duration(ms) * time.Millisecond
}

// startLiveKeepAlive 启动后台保活循环(每次启动调用一次)。随 app 上下文取消而退出。
func (a *App) startLiveKeepAlive() {
	if a == nil {
		return
	}
	log := logger.New("KeepAlive")
	var done <-chan struct{}
	if a.ctx != nil {
		done = a.ctx.Done()
	}
	go func() {
		ticker := time.NewTicker(a.liveKeepAliveInterval())
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if a.liveKeepAliveEnabledResolved() {
					a.tickLiveKeepAlive(log)
				}
			}
		}
	}()
}

// tickLiveKeepAlive 给所有 DebugReady 的运行中实例各注入一次可信鼠标移动。
// 停止/异常实例(连接失败)自动跳过,不影响其他实例。
func (a *App) tickLiveKeepAlive(log *logger.Logger) {
	if a.browserMgr == nil {
		return
	}
	for _, p := range a.browserMgr.List() {
		if !p.Running || !p.DebugReady || p.DebugPort <= 0 {
			continue
		}
		// 轻微抖动坐标,更接近真人;mouseMoved 不点击、不干扰页面。
		x := 40 + rand.Intn(220)
		y := 40 + rand.Intn(160)
		if _, err := cdpCall(p.DebugPort, "Input.dispatchMouseEvent", map[string]any{
			"type": "mouseMoved",
			"x":    x,
			"y":    y,
		}); err != nil {
			log.Debug("保活注入跳过", logger.F("profile", p.ProfileId), logger.F("port", p.DebugPort), logger.F("error", err.Error()))
		}
	}
}
