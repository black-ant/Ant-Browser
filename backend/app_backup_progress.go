package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"fmt"
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
	Phase            string  `json:"phase"`
	Progress         int     `json:"progress"`
	Message          string  `json:"message"`
	BytesTransferred int64   `json:"bytesTransferred,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	BytesPerSecond   float64 `json:"bytesPerSecond,omitempty"`
	ComponentID      string  `json:"componentId,omitempty"`
	ComponentName    string  `json:"componentName,omitempty"`
	EntryIndex       int     `json:"entryIndex,omitempty"`
	EntryTotal       int     `json:"entryTotal,omitempty"`
	Timestamp        string  `json:"timestamp,omitempty"`
}

type backupTransferProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	BytesPerSecond   float64
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
	a.backupEmitProgressWithTransfer(eventName, phase, progress, message, meta, nil)
}

func (a *App) backupEmitExportProgressTransfer(phase string, progress int, message string, transfer channels.UploadProgress) {
	a.backupEmitProgressWithTransfer("backup:export:progress", phase, progress, message, nil, &backupTransferProgress{
		BytesTransferred: transfer.BytesTransferred,
		TotalBytes:       transfer.TotalBytes,
		BytesPerSecond:   transfer.BytesPerSecond,
	})
}

func (a *App) backupEmitExportUploadProgress(artifactName string, startProgress, endProgress int, transfer channels.UploadProgress) {
	transferred := transfer.BytesTransferred
	if transferred < 0 {
		transferred = 0
	}
	total := transfer.TotalBytes
	if total < 0 {
		total = 0
	}
	if total > 0 && transferred > total {
		transferred = total
	}
	progress := startProgress
	if total > 0 {
		ratio := float64(transferred) / float64(total)
		if ratio > 1 {
			ratio = 1
		}
		progress += int(ratio * float64(endProgress-startProgress))
	}
	message := fmt.Sprintf("正在上传%s到 OpenList：%s / %s，速度 %s", artifactName, formatBackupFileSize(transferred), formatBackupFileSize(total), formatBackupTransferRate(transfer.BytesPerSecond))
	a.backupEmitExportProgressTransfer("uploading", progress, message, channels.UploadProgress{
		BytesTransferred: transferred,
		TotalBytes:       total,
		BytesPerSecond:   transfer.BytesPerSecond,
	})
}

func (a *App) backupEmitProgressWithTransfer(eventName, phase string, progress int, message string, meta *backupProgressMeta, transfer *backupTransferProgress) {
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
	if transfer != nil {
		if transfer.BytesTransferred > 0 {
			evt.BytesTransferred = transfer.BytesTransferred
		}
		if transfer.TotalBytes > 0 {
			evt.TotalBytes = transfer.TotalBytes
		}
		if transfer.BytesPerSecond > 0 {
			evt.BytesPerSecond = transfer.BytesPerSecond
		}
	}
	if meta != nil {
		evt.ComponentID = strings.TrimSpace(meta.ComponentID)
		evt.ComponentName = strings.TrimSpace(meta.ComponentName)
		evt.EntryIndex = meta.EntryIndex
		evt.EntryTotal = meta.EntryTotal
	}

	a.emitRuntimeEvent(eventName, evt)
}
