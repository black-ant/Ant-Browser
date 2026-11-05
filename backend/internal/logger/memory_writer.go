package logger

import (
	"strings"
	"sync"
)

const defaultMemoryBufferSize = 2000

// MemoryLogEntry 内存日志条目（供前端消费）
type MemoryLogEntry struct {
	Time      string                 `json:"time"`
	Level     string                 `json:"level"`
	Component string                 `json:"component"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// MemoryWriter 内存环形缓冲写入器，线程安全
type MemoryWriter struct {
	mu      sync.RWMutex
	entries []MemoryLogEntry
	maxSize int
}

var globalMemoryWriter *MemoryWriter

func init() {
	globalMemoryWriter = &MemoryWriter{
		entries: make([]MemoryLogEntry, 0, defaultMemoryBufferSize),
		maxSize: defaultMemoryBufferSize,
	}
}

// GetMemoryWriter 获取全局内存写入器
func GetMemoryWriter() *MemoryWriter {
	return globalMemoryWriter
}

func (w *MemoryWriter) Write(entry *LogEntry) error {
	if entry == nil {
		return nil
	}
	if shouldSkipMemoryLog(entry) {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	item := MemoryLogEntry{
		Time:      entry.Timestamp.Format("2006-01-02 15:04:05"),
		Level:     entry.Level.String(),
		Component: entry.Component,
		Message:   entry.Message,
		Fields:    entry.Fields,
	}
	if len(w.entries) >= w.maxSize {
		w.entries = w.entries[1:]
	}
	w.entries = append(w.entries, item)
	return nil
}

func shouldSkipMemoryLog(entry *LogEntry) bool {
	if entry.Level != INFO || entry.Component != "Xray" {
		return false
	}
	message := strings.TrimSpace(entry.Message)
	switch message {
	case "xray 内核进程已启动", "回收空闲桥接进程", "复用 xray 桥接进程", "复用已就绪 xray 桥接进程":
		return true
	default:
		return false
	}
}

func (w *MemoryWriter) Close() error { return nil }

// GetEntries 返回所有缓冲日志（最新在后）
func (w *MemoryWriter) GetEntries() []MemoryLogEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]MemoryLogEntry, len(w.entries))
	copy(result, w.entries)
	return result
}

// Clear 清空缓冲
func (w *MemoryWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = w.entries[:0]
}
