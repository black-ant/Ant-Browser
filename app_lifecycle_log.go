package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"ant-chrome/backend"
)

var lifecycleLogMu sync.Mutex

func lifecycleLog(appRoot string, event string, fields map[string]interface{}) {
	if appRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			appRoot = cwd
		}
	}

	record := map[string]interface{}{
		"time":  time.Now().Format(time.RFC3339Nano),
		"pid":   os.Getpid(),
		"event": event,
	}
	for key, value := range fields {
		record[key] = value
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}

	path := backend.ResolveRuntimePath(appRoot, filepath.Join("data", "logs", "app-lifecycle.log"))
	lifecycleLogMu.Lock()
	defer lifecycleLogMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = file.Write(append(payload, '\n'))
	_ = file.Sync()
	_ = file.Close()
}

func logLifecyclePanic(appRoot string, event string, recovered interface{}) {
	lifecycleLog(appRoot, event, map[string]interface{}{
		"panic": fmt.Sprint(recovered),
		"stack": string(debug.Stack()),
	})
}

type wailsLifecycleLogger struct {
	appRoot string
}

func newWailsLifecycleLogger(appRoot string) *wailsLifecycleLogger {
	return &wailsLifecycleLogger{appRoot: appRoot}
}

func (l *wailsLifecycleLogger) write(level string, message string) {
	lifecycleLog(l.appRoot, "wails.log", map[string]interface{}{
		"level":   level,
		"message": message,
	})
}

func (l *wailsLifecycleLogger) Print(message string)   { l.write("print", message) }
func (l *wailsLifecycleLogger) Trace(message string)   { l.write("trace", message) }
func (l *wailsLifecycleLogger) Debug(message string)   { l.write("debug", message) }
func (l *wailsLifecycleLogger) Info(message string)    { l.write("info", message) }
func (l *wailsLifecycleLogger) Warning(message string) { l.write("warning", message) }
func (l *wailsLifecycleLogger) Error(message string)   { l.write("error", message) }
func (l *wailsLifecycleLogger) Fatal(message string) {
	l.write("fatal", message)
	os.Exit(1)
}

func runTraySafely(appRoot string, callbacks backend.TrayCallbacks) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logLifecyclePanic(appRoot, "tray.panic", recovered)
		}
	}()
	backend.RunTray(callbacks)
	lifecycleLog(appRoot, "tray.run.returned", nil)
}
