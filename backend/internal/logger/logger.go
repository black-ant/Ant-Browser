package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String 返回日志级别的字符串表示
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 解析日志级别字符串
func ParseLevel(levelStr string) Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// Field 结构化日志字段
type Field struct {
	Key   string
	Value interface{}
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level           string
	FileEnabled     bool
	FilePath        string
	Format          string // "text" or "json"
	BufferSize      int    // 缓冲区大小（KB）
	AsyncQueueSize  int    // 异步队列大小
	FlushIntervalMs int    // 刷新间隔（毫秒）

	// 分片配置
	Rotation RotationConfig
}

// RotationConfig 日志分片配置
type RotationConfig struct {
	Enabled      bool
	MaxSizeMB    int    // 单文件最大大小（MB）
	MaxAge       int    // 保留天数
	MaxBackups   int    // 保留文件数
	TimeInterval string // 时间间隔: "daily", "hourly"
}

// Logger 日志记录器
type Logger struct {
	level     Level
	component string
	ctx       context.Context

	// 写入器
	writers       []Writer
	consoleWriter Writer
	fileWriter    *FileWriter

	// 分片管理器
	rotationManager *RotationManager

	// 并发安全
	mu sync.RWMutex

	// 文件写入失败标志
	fileWriteFailed bool
}

// 全局日志窗口
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// DefaultLoggerConfig 返回默认日志配置
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:           "info",
		FileEnabled:     false,
		FilePath:        "data/logs/app.log",
		Format:          "text",
		BufferSize:      4, // 4KB
		AsyncQueueSize:  1000,
		FlushIntervalMs: 1000, // 1秒
		Rotation: RotationConfig{
			Enabled:      false,
			MaxSizeMB:    100,
			MaxAge:       7,
			MaxBackups:   5,
			TimeInterval: "daily",
		},
	}
}

// Init 初始化全局日志（简单版本，仅控制台输出）
func Init(ctx context.Context, levelStr string) {
	InitWithConfig(ctx, LoggerConfig{
		Level:       levelStr,
		FileEnabled: false,
		Format:      "text",
	})
}

// InitWithConfig 使用配置初始化全局日志
func InitWithConfig(ctx context.Context, config LoggerConfig) {
	globalMu.Lock()
	defer globalMu.Unlock()

	// 解析日志级别，无效级别使用默认 INFO
	level := ParseLevel(config.Level)
	if config.Level != "" && level == INFO && strings.ToLower(config.Level) != "info" {
		// 无效级别，记录警告（使用 fmt 因为 logger 还未初始化）
		fmt.Printf("[WARN] Invalid log level '%s', using default 'INFO'\n", config.Level)
	}

	// 创建格式化器
	var formatter Formatter
	switch strings.ToLower(config.Format) {
	case "json":
		formatter = NewJSONFormatter()
	default:
		formatter = NewTextFormatter()
	}

	// 创建控制台写入器
	consoleWriter := NewConsoleWriter(formatter)

	logger := &Logger{
		level:         level,
		ctx:           ctx,
		writers:       []Writer{consoleWriter, globalMemoryWriter},
		consoleWriter: consoleWriter,
	}

	// 如果启用文件日志，创建文件写入器
	if config.FileEnabled && config.FilePath != "" {
		fileWriter, rotationManager, err := createFileWriterWithRotation(config, formatter)
		if err != nil {
			// 文件写入器创建失败，回退到仅控制台输出
			fmt.Printf("[WARN] Failed to create file writer: %v, falling back to console only\n", err)
			logger.fileWriteFailed = true
		} else {
			logger.fileWriter = fileWriter
			logger.rotationManager = rotationManager
			logger.writers = append(logger.writers, fileWriter)
		}
	}

	globalLogger = logger
}

// createFileWriterWithRotation 创建带分片功能的文件写入器
func createFileWriterWithRotation(config LoggerConfig, formatter Formatter) (*FileWriter, *RotationManager, error) {
	// 确保目录存在
	dir := filepath.Dir(config.FilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// 计算缓冲区大小（KB -> 字节）
	bufferSize := config.BufferSize * 1024
	if bufferSize <= 0 {
		bufferSize = 4 * 1024 // 默认 4KB
	}

	// 计算刷新间隔
	flushInterval := time.Duration(config.FlushIntervalMs) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = time.Second
	}

	// 异步队列大小
	asyncQueueSize := config.AsyncQueueSize
	if asyncQueueSize <= 0 {
		asyncQueueSize = 1000
	}

	fileConfig := FileWriterConfig{
		FilePath:       config.FilePath,
		BufferSize:     bufferSize,
		FlushInterval:  flushInterval,
		AsyncQueueSize: asyncQueueSize,
	}

	// 使用异步文件写入器
	fileWriter, err := NewAsyncFileWriter(fileConfig, formatter)
	if err != nil {
		return nil, nil, err
	}

	// 创建分片管理器（如果启用）
	var rotationManager *RotationManager
	if config.Rotation.Enabled {
		rotationPolicy := createRotationPolicy(config.Rotation)
		rotationManager = NewRotationManager(RotationManagerConfig{
			BasePath:   config.FilePath,
			MaxBackups: config.Rotation.MaxBackups,
			MaxAge:     config.Rotation.MaxAge,
			Policy:     rotationPolicy,
		})
	}

	return fileWriter, rotationManager, nil
}

// createRotationPolicy 根据配置创建分片策略
func createRotationPolicy(config RotationConfig) RotationPolicy {
	var policies []RotationPolicy

	// 时间分片策略
	if config.TimeInterval != "" {
		var interval TimeInterval
		switch strings.ToLower(config.TimeInterval) {
		case "hourly":
			interval = Hourly
		default:
			interval = Daily
		}
		policies = append(policies, NewTimeRotationPolicy(interval))
	}

	// 大小分片策略
	if config.MaxSizeMB > 0 {
		policies = append(policies, NewSizeRotationPolicyMB(config.MaxSizeMB))
	}

	// 如果有多个策略，使用组合策略
	if len(policies) > 1 {
		return NewCompositeRotationPolicy(policies...)
	} else if len(policies) == 1 {
		return policies[0]
	}

	// 默认按天分片
	return NewTimeRotationPolicy(Daily)
}

// Close 关闭全局日志
func Close() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger == nil {
		return nil
	}

	var lastErr error
	for _, writer := range globalLogger.writers {
		if err := writer.Close(); err != nil {
			lastErr = err
		}
	}

	globalLogger = nil
	return lastErr
}

// New 创建新的日志记录器
func New(component string) *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalLogger == nil {
		// 如果全局日志未初始化，创建一个默认的
		consoleWriter := NewConsoleWriter(NewTextFormatter())
		return &Logger{
			level:         INFO,
			component:     component,
			writers:       []Writer{consoleWriter},
			consoleWriter: consoleWriter,
		}
	}

	return &Logger{
		level:           globalLogger.level,
		component:       component,
		ctx:             globalLogger.ctx,
		writers:         globalLogger.writers,
		consoleWriter:   globalLogger.consoleWriter,
		fileWriter:      globalLogger.fileWriter,
		rotationManager: globalLogger.rotationManager,
		fileWriteFailed: globalLogger.fileWriteFailed,
	}
}

// SetLevel 动态设置日志级别（并发安全）
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetLevelString 通过字符串动态设置日志级别
func (l *Logger) SetLevelString(levelStr string) {
	l.SetLevel(ParseLevel(levelStr))
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetGlobalLevel 设置全局日志级别
func SetGlobalLevel(level Level) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger != nil {
		globalLogger.mu.Lock()
		globalLogger.level = level
		globalLogger.mu.Unlock()
	}
}

// SetGlobalLevelString 通过字符串设置全局日志级别
func SetGlobalLevelString(levelStr string) {
	SetGlobalLevel(ParseLevel(levelStr))
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, fields ...Field) {
	l.mu.RLock()
	level := l.level
	l.mu.RUnlock()

	if level <= DEBUG {
		l.log(DEBUG, msg, fields...)
	}
}

// Info 记录信息日志
func (l *Logger) Info(msg string, fields ...Field) {
	l.mu.RLock()
	level := l.level
	l.mu.RUnlock()

	if level <= INFO {
		l.log(INFO, msg, fields...)
	}
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, fields ...Field) {
	l.mu.RLock()
	level := l.level
	l.mu.RUnlock()

	if level <= WARN {
		l.log(WARN, msg, fields...)
	}
}

// Error 记录错误日志
func (l *Logger) Error(msg string, fields ...Field) {
	l.mu.RLock()
	level := l.level
	l.mu.RUnlock()

	if level <= ERROR {
		l.log(ERROR, msg, fields...)
	}
}

// log 内部日志记录方法
func (l *Logger) log(level Level, msg string, fields ...Field) {
	// 创建日志条目
	entry := NewLogEntry(level, l.component, msg)

	// 添加字段
	if len(fields) > 0 {
		fieldMap := make(map[string]interface{}, len(fields))
		for _, field := range fields {
			fieldMap[field.Key] = field.Value
		}
		entry.WithFields(fieldMap)
	}

	// 写入所有写入器
	l.writeEntry(entry)
}

// writeEntry 写入日志条目到所有写入器
func (l *Logger) writeEntry(entry *LogEntry) {
	l.mu.RLock()
	writers := l.writers
	fileWriter := l.fileWriter
	consoleWriter := l.consoleWriter
	fileWriteFailed := l.fileWriteFailed
	l.mu.RUnlock()

	// 如果文件写入已失败，只写入控制台
	if fileWriteFailed {
		if consoleWriter != nil {
			_ = consoleWriter.Write(entry)
		}
		return
	}

	// 写入所有写入器
	for _, writer := range writers {
		if err := writer.Write(entry); err != nil {
			// 如果是文件写入器失败，标记并回退到控制台
			if writer == fileWriter {
				l.handleFileWriteError(entry, err)
			}
		}
	}
}

// handleFileWriteError 处理文件写入错误
func (l *Logger) handleFileWriteError(entry *LogEntry, err error) {
	l.mu.Lock()
	if !l.fileWriteFailed {
		l.fileWriteFailed = true
		// 记录错误到控制台
		fmt.Printf("[ERROR] File write failed: %v, falling back to console only\n", err)
	}
	l.mu.Unlock()
}

// LogEntry 直接写入日志条目（用于拦截器等高级用法）
func (l *Logger) LogEntry(entry *LogEntry) {
	l.mu.RLock()
	level := l.level
	l.mu.RUnlock()

	// 检查日志级别
	if entry.Level < level {
		return
	}

	l.writeEntry(entry)
}

// WithComponent 创建带有组件名的新日志记录器
func (l *Logger) WithComponent(component string) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		level:           l.level,
		component:       component,
		ctx:             l.ctx,
		writers:         l.writers,
		consoleWriter:   l.consoleWriter,
		fileWriter:      l.fileWriter,
		rotationManager: l.rotationManager,
		fileWriteFailed: l.fileWriteFailed,
	}
}

// Flush 刷新所有写入器的缓冲区
func (l *Logger) Flush() error {
	l.mu.RLock()
	fileWriter := l.fileWriter
	l.mu.RUnlock()

	if fileWriter != nil {
		return fileWriter.Flush()
	}
	return nil
}

// GetRotationManager 获取分片管理器
func (l *Logger) GetRotationManager() *RotationManager {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rotationManager
}

// F 创建字段的便捷函数
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Fs 创建多个字段的便捷函数
func Fs(keyValues ...interface{}) []Field {
	fields := make([]Field, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues)-1; i += 2 {
		if key, ok := keyValues[i].(string); ok {
			fields = append(fields, Field{Key: key, Value: keyValues[i+1]})
		}
	}
	return fields
}

// IsFileEnabled 检查文件日志是否启用
func (l *Logger) IsFileEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.fileWriter != nil && !l.fileWriteFailed
}

// GetWriters 获取所有写入器（用于测试）
func (l *Logger) GetWriters() []Writer {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.writers
}

// ShouldLog 检查指定级别是否应该被记录
func (l *Logger) ShouldLog(level Level) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}
