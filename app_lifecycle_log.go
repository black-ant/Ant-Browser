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

func runTraySafely(appRoot string, callbacks backend.TrayCallbacks) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logLifecyclePanic(appRoot, "tray.panic", recovered)
		}
	}()
	backend.RunTray(callbacks)
	lifecycleLog(appRoot, "tray.run.returned", nil)
}
