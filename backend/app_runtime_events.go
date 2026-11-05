package backend

import (
	"context"
	"fmt"

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
	a.backgroundTasks.Wait()
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

func (a *App) waitBackgroundTasks() {
	if a == nil {
		return
	}
	a.backgroundTasks.Wait()
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
