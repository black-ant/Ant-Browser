package backend

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setRuntimeContext(ctx context.Context) {
	a.runtimeMu.Lock()
	a.ctx = ctx
	a.runtimeStopped = false
	a.runtimeMu.Unlock()
}

func (a *App) stopRuntimeEvents() {
	a.runtimeMu.Lock()
	a.runtimeStopped = true
	a.runtimeMu.Unlock()
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
