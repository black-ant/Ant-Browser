package backend

import (
	"encoding/json"
	"math/rand"
	"time"

	"ant-chrome/backend/internal/logger"
)

// 直播/视频「防挂机」保活。
//
// 背景:抖音等直播/视频网页有自己的挂机检测——一段时间(约 2–3 分钟)没有真实鼠标/键盘
// 活动,就暂停播放并弹「长时间无操作,已暂停播放」。多开养号时会被逐个暂停。
//
// 方案:定时用 CDP `Input.dispatchMouseEvent`(mouseMoved)向每个运行中的实例注入一次
// 可信输入(isTrusted=true,与真人无法区分,不能用页面 JS 的 dispatchEvent),重置挂机计时器。
//
// 反检测:间隔不是固定值(固定周期本身就是机器人特征),而是每个实例在 [min,max](默认
// 60–90s)内**独立随机**、彼此**错峰**——避免"同一账号固定 75s 一跳"被单站识别,也避免
// "100 个账号同一毫秒一起动"被养号风控按同源关联。坐标同样带随机抖动;mouseMoved 不点击、
// 不动真实 OS 光标、对页面无副作用。

const (
	keepAliveTickResolution = 5 * time.Second // 调度粒度;间隔在 60–90s 量级,5s 足够
	keepAliveFloorMs        = 15000           // 间隔下限,避免过于频繁
)

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

// keepAliveBoundsMs 返回随机间隔的下/上界(ms)。
// 若显式设置 live_keepalive_interval_ms>0 → 固定间隔(min==max);否则用 min/max(默认 60000–90000)。
func (a *App) keepAliveBoundsMs() (int, int) {
	minMs, maxMs := 60000, 90000
	if a != nil && a.config != nil {
		b := a.config.Browser
		if b.LiveKeepAliveIntervalMs > 0 { // 固定间隔覆盖
			f := b.LiveKeepAliveIntervalMs
			if f < keepAliveFloorMs {
				f = keepAliveFloorMs
			}
			return f, f
		}
		if b.LiveKeepAliveIntervalMinMs > 0 {
			minMs = b.LiveKeepAliveIntervalMinMs
		}
		if b.LiveKeepAliveIntervalMaxMs > 0 {
			maxMs = b.LiveKeepAliveIntervalMaxMs
		}
	}
	if minMs < keepAliveFloorMs {
		minMs = keepAliveFloorMs
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	return minMs, maxMs
}

// randKeepAliveDelay 返回一次 [min,max] 内的随机间隔。
func (a *App) randKeepAliveDelay() time.Duration {
	minMs, maxMs := a.keepAliveBoundsMs()
	ms := minMs
	if span := maxMs - minMs; span > 0 {
		ms += rand.Intn(span + 1)
	}
	return time.Duration(ms) * time.Millisecond
}

// startLiveKeepAlive 启动后台保活循环(每次启动调用一次)。随 app 上下文取消而退出。
// 单 goroutine 维护每实例的下次触发时间,无需加锁。
func (a *App) startLiveKeepAlive() {
	if a == nil {
		return
	}
	log := logger.New("KeepAlive")
	var done <-chan struct{}
	if a.ctx != nil {
		done = a.ctx.Done()
	}
	next := map[string]time.Time{} // profileId -> 下次触发时间
	go func() {
		ticker := time.NewTicker(keepAliveTickResolution)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				if !a.liveKeepAliveEnabledResolved() {
					continue
				}
				a.runKeepAliveDue(log, next, now)
			}
		}
	}()
}

// keepAliveShouldInject 判定某运行中实例本轮是否应注入保活:必须运行中、调试就绪、
// 有端口,且该实例保活开关为开。
func keepAliveShouldInject(running, ready bool, debugPort int, enabled bool) bool {
	return running && ready && debugPort > 0 && enabled
}

// runKeepAliveDue 对到期的运行中实例注入一次可信输入,并为其安排下一次随机时间。
// 实例首次出现时给一个 [0,max) 的随机首触发 → 各实例天然错峰。
func (a *App) runKeepAliveDue(log *logger.Logger, next map[string]time.Time, now time.Time) {
	if a.browserMgr == nil {
		return
	}
	live := map[string]struct{}{}
	for _, p := range a.browserMgr.List() {
		if !keepAliveShouldInject(p.Running, p.DebugReady, p.DebugPort, p.LiveKeepAliveEnabled) {
			continue
		}
		live[p.ProfileId] = struct{}{}
		t, ok := next[p.ProfileId]
		if !ok {
			_, maxMs := a.keepAliveBoundsMs()
			next[p.ProfileId] = now.Add(time.Duration(rand.Intn(maxMs+1)) * time.Millisecond)
			continue
		}
		if now.Before(t) {
			continue
		}
		a.injectKeepAlive(log, p.ProfileId, p.DebugPort)
		next[p.ProfileId] = now.Add(a.randKeepAliveDelay())
	}
	// 清理已停止实例的调度,避免 map 泄漏
	for id := range next {
		if _, ok := live[id]; !ok {
			delete(next, id)
		}
	}
}

// injectKeepAlive 向该实例的**所有** page 标签各注入一次带随机坐标的可信鼠标移动。
// 抖音等挂机检测是逐标签(每个页面自己的 JS 空闲计时器),只喂第一个标签会漏掉其他标签
// (包括可能正在放直播的那个),故遍历全部 page target 逐个注入;CDP 合成输入对后台标签同样送达。
// 某个标签失败(崩溃/正在导航)静默跳过,不影响其他标签。
func (a *App) injectKeepAlive(log *logger.Logger, profileID string, debugPort int) {
	body, err := cdpGetEndpointBody(debugPort, "/json")
	if err != nil {
		log.Debug("保活获取标签列表失败", logger.F("profile", profileID), logger.F("port", debugPort), logger.F("error", err.Error()))
		return
	}
	wsURLs := pageTargetWSURLs(body)
	sent := 0
	for _, ws := range wsURLs {
		x := 40 + rand.Intn(220)
		y := 40 + rand.Intn(160)
		if err := cdpSendOne(ws, "Input.dispatchMouseEvent", map[string]any{
			"type": "mouseMoved",
			"x":    x,
			"y":    y,
		}); err == nil {
			sent++
		}
	}
	if sent == 0 {
		log.Debug("保活未注入任何标签", logger.F("profile", profileID), logger.F("port", debugPort), logger.F("tabs", len(wsURLs)))
	}
}

// pageTargetWSURLs 从 CDP /json 响应解析出所有 page 类型标签的 WebSocket 调试地址。
func pageTargetWSURLs(jsonBody []byte) []string {
	var targets []cdpTarget
	if err := json.Unmarshal(jsonBody, &targets); err != nil {
		return nil
	}
	urls := make([]string, 0, len(targets))
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
			urls = append(urls, t.WebSocketDebuggerUrl)
		}
	}
	return urls
}

// cdpSendOne 在给定 WebSocket 地址上发送一次 CDP 命令并读一次响应(独立短连,用于逐标签注入)。
func cdpSendOne(wsURL string, method string, params map[string]any) error {
	conn, err := cdpDialWebSocket(wsURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(cdpWebSocketReadTimeout))
	if err := conn.WriteJSON(cdpMessage{Id: 1, Method: method, Params: params}); err != nil {
		return err
	}
	var resp cdpResponse
	return conn.ReadJSON(&resp)
}
