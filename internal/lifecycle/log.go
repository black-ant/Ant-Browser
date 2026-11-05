package lifecycle

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

func Log(appRoot string, event string, fields map[string]interface{}) {
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

func LogPanic(appRoot string, event string, recovered interface{}) {
	Log(appRoot, event, map[string]interface{}{
		"panic": fmt.Sprint(recovered),
		"stack": string(debug.Stack()),
	})
}

type WailsLogger struct {
	appRoot string
}

func NewWailsLogger(appRoot string) *WailsLogger {
	return &WailsLogger{appRoot: appRoot}
}

func (l *WailsLogger) write(level string, message string) {
	Log(l.appRoot, "wails.log", map[string]interface{}{
		"level":   level,
		"message": message,
	})
}

func (l *WailsLogger) Print(message string)   { l.write("print", message) }
func (l *WailsLogger) Trace(message string)   { l.write("trace", message) }
func (l *WailsLogger) Debug(message string)   { l.write("debug", message) }
func (l *WailsLogger) Info(message string)    { l.write("info", message) }
func (l *WailsLogger) Warning(message string) { l.write("warning", message) }
func (l *WailsLogger) Error(message string)   { l.write("error", message) }
func (l *WailsLogger) Fatal(message string) {
	l.write("fatal", message)
	os.Exit(1)
}

func RunTraySafely(appRoot string, callbacks backend.TrayCallbacks) {
	defer func() {
		if recovered := recover(); recovered != nil {
			LogPanic(appRoot, "tray.panic", recovered)
		}
	}()
	backend.RunTray(callbacks)
	Log(appRoot, "tray.run.returned", nil)
}
