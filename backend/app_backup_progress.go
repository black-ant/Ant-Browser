package backend

import (
	"strings"
	"time"
)

type backupProgressMeta struct {
	ComponentID   string
	ComponentName string
	EntryIndex    int
	EntryTotal    int
}

type backupProgressEvent struct {
	Phase         string `json:"phase"`
	Progress      int    `json:"progress"`
	Message       string `json:"message"`
	ComponentID   string `json:"componentId,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	EntryIndex    int    `json:"entryIndex,omitempty"`
	EntryTotal    int    `json:"entryTotal,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

func (a *App) backupEmitExportProgress(phase string, progress int, message string) {
	a.backupEmitExportProgressMeta(phase, progress, message, nil)
}

func (a *App) backupEmitExportProgressMeta(phase string, progress int, message string, meta *backupProgressMeta) {
	a.backupEmitProgress("backup:export:progress", phase, progress, message, meta)
}

func (a *App) backupEmitImportProgress(phase string, progress int, message string) {
	a.backupEmitImportProgressMeta(phase, progress, message, nil)
}

func (a *App) backupEmitImportProgressMeta(phase string, progress int, message string, meta *backupProgressMeta) {
	a.backupEmitProgress("backup:import:progress", phase, progress, message, meta)
}

func (a *App) backupEmitProgress(eventName, phase string, progress int, message string, meta *backupProgressMeta) {
	if a == nil {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	evt := backupProgressEvent{
		Phase:     strings.TrimSpace(phase),
		Progress:  progress,
		Message:   strings.TrimSpace(message),
		Timestamp: time.Now().Format("15:04:05"),
	}
	if meta != nil {
		evt.ComponentID = strings.TrimSpace(meta.ComponentID)
		evt.ComponentName = strings.TrimSpace(meta.ComponentName)
		evt.EntryIndex = meta.EntryIndex
		evt.EntryTotal = meta.EntryTotal
	}

	a.emitRuntimeEvent(eventName, backupProgressEvent{
		Phase:         evt.Phase,
		Progress:      evt.Progress,
		Message:       evt.Message,
		ComponentID:   evt.ComponentID,
		ComponentName: evt.ComponentName,
		EntryIndex:    evt.EntryIndex,
		EntryTotal:    evt.EntryTotal,
		Timestamp:     evt.Timestamp,
	})
}
