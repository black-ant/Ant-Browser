package backend

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setRuntimeContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.runtimeMu.Lock()
	a.ctx = ctx
	a.runtimeStopped = false
	a.backgroundTasksBlocked = false
	a.backgroundTaskCtx, a.backgroundTaskCancel = context.WithCancel(ctx)
	a.runtimeMu.Unlock()
}

func (a *App) stopRuntimeEvents() {
	a.runtimeMu.Lock()
	a.runtimeStopped = true
	a.backgroundTasksBlocked = true
	cancel := a.backgroundTaskCancel
	a.backgroundTaskCtx = nil
	a.backgroundTaskCancel = nil
	a.runtimeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) startBackgroundTask(task func(context.Context)) error {
	if a == nil {
		return fmt.Errorf("应用未初始化")
	}
	if task == nil {
		return fmt.Errorf("后台任务不能为空")
	}

	a.runtimeMu.Lock()
	if a.ctx == nil {
		a.runtimeMu.Unlock()
		return fmt.Errorf("应用上下文未初始化")
	}
	if a.runtimeStopped {
		a.runtimeMu.Unlock()
		return fmt.Errorf("应用正在关闭")
	}
	if a.backgroundTasksBlocked {
		a.runtimeMu.Unlock()
		return fmt.Errorf("应用正在维护")
	}
	if a.backgroundTaskCtx == nil || a.backgroundTaskCancel == nil {
		a.backgroundTaskCtx, a.backgroundTaskCancel = context.WithCancel(a.ctx)
	}
	taskCtx := a.backgroundTaskCtx
	a.backgroundTasks.Add(1)
	a.runtimeMu.Unlock()

	go func() {
		defer a.backgroundTasks.Done()
		task(taskCtx)
	}()
	return nil
}

func (a *App) cancelBackgroundTasksAndWait() {
	if a == nil {
		return
	}
	a.runtimeMu.Lock()
	a.backgroundTasksBlocked = true
	cancel := a.backgroundTaskCancel
	a.backgroundTaskCtx = nil
	a.backgroundTaskCancel = nil
	a.runtimeMu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.waitBackgroundTasks()
}

func (a *App) resumeBackgroundTasks() {
	if a == nil {
		return
	}
	a.runtimeMu.Lock()
	if !a.runtimeStopped {
		a.backgroundTasksBlocked = false
	}
	a.runtimeMu.Unlock()
}

// backgroundTaskWaitTimeout 是关闭/维护流程等待后台任务的上限。
// 后台任务（典型是代理测速扫描：每个代理都要拉起一个 sing-box/xray 内核进程）
// 在 ctx 取消后仍可能把整轮队列跑完，无上限等待会让「退出应用」被拖到数分钟。
// 超时只记录告警并继续关闭流程，不阻塞退出。
const backgroundTaskWaitTimeout = 8 * time.Second

// waitGroupWithTimeout 等待 wg 归零，最多等 timeout；返回 false 表示超时。
func waitGroupWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (a *App) waitBackgroundTasks() {
	if a == nil {
		return
	}
	if !waitGroupWithTimeout(&a.backgroundTasks, backgroundTaskWaitTimeout) {
		logger.New("App").Warn("等待后台任务超时，继续关闭流程",
			logger.F("timeout", backgroundTaskWaitTimeout.String()))
	}
}

func (a *App) emitRuntimeEvent(eventName string, optionalData ...interface{}) {
	a.emitRuntimeEventWithContext(nil, eventName, optionalData...)
}

func (a *App) emitRuntimeEventWithContext(ctx context.Context, eventName string, optionalData ...interface{}) {
	if a == nil {
		return
	}
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	if a.runtimeStopped {
		return
	}
	if ctx == nil {
		ctx = a.ctx
	}
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, eventName, optionalData...)
}

func (a *App) runtimeQuit() {
	if a == nil {
		return
	}
	a.runtimeMu.RLock()
	ctx := a.ctx
	a.runtimeMu.RUnlock()
	if ctx != nil {
		runtime.Quit(ctx)
	}
}
