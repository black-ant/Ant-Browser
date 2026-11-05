package logger

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// beforeCall 方法调用前的处理
func (m *MethodInterceptor) beforeCall(methodName string, params []interface{}) *CallContext {
	ctx := &CallContext{
		RequestID:  GenerateRequestID(),
		MethodName: methodName,
		StartTime:  time.Now(),
		Parameters: params,
	}

	// 记录方法入口日志
	entry := NewLogEntry(DEBUG, "interceptor", fmt.Sprintf("Method call started: %s", methodName))
	entry.WithRequestID(ctx.RequestID)
	entry.WithMethod(methodName)

	// 添加参数信息
	if m.config.LogParameters && len(params) > 0 {
		maskedParams := m.maskSensitiveParams(params)
		entry.WithFields(map[string]interface{}{
			"parameters": maskedParams,
		})
	}

	// 添加调用位置
	if file, line := m.getCaller(); file != "" {
		entry.WithCaller(file, line)
	}

	m.safeLog(entry)

	return ctx
}

// afterCallRecover 方法调用后的处理（带 panic 恢复）
func (m *MethodInterceptor) afterCallRecover(ctx *CallContext, result interface{}, err error) {
	// 捕获 panic，确保日志错误不影响业务
	if r := recover(); r != nil {
		m.handlePanic(ctx, r)
		// 重新抛出 panic，让业务代码处理
		panic(r)
	}

	m.afterCall(ctx, result, err)
}

// afterCall 方法调用后的处理
func (m *MethodInterceptor) afterCall(ctx *CallContext, result interface{}, err error) {
	duration := time.Since(ctx.StartTime).Milliseconds()

	var entry *LogEntry
	if err != nil {
		// 错误情况
		entry = NewLogEntry(ERROR, "interceptor", fmt.Sprintf("Method call failed: %s", ctx.MethodName))
		entry.WithError(err.Error())

		// 获取堆栈信息
		stack := m.getStackTrace()
		if stack != "" {
			if entry.Fields == nil {
				entry.Fields = make(map[string]interface{})
			}
			entry.Fields["stack_trace"] = stack
		}
	} else {
		// 成功情况
		entry = NewLogEntry(DEBUG, "interceptor", fmt.Sprintf("Method call completed: %s", ctx.MethodName))

		// 记录返回结果
		if m.config.LogResults && result != nil {
			maskedResult := m.maskSensitiveValue("result", result)
			if entry.Fields == nil {
				entry.Fields = make(map[string]interface{})
			}
			entry.Fields["result"] = maskedResult
		}
	}

	entry.WithRequestID(ctx.RequestID)
	entry.WithMethod(ctx.MethodName)
	entry.WithDuration(duration)

	m.safeLog(entry)
}

// handlePanic 处理 panic
func (m *MethodInterceptor) handlePanic(ctx *CallContext, panicValue interface{}) {
	duration := time.Since(ctx.StartTime).Milliseconds()

	entry := NewLogEntry(ERROR, "interceptor", fmt.Sprintf("Method call panicked: %s", ctx.MethodName))
	entry.WithRequestID(ctx.RequestID)
	entry.WithMethod(ctx.MethodName)
	entry.WithDuration(duration)
	entry.WithError(fmt.Sprintf("panic: %v", panicValue))

	// 获取堆栈信息
	stack := m.getStackTrace()
	if stack != "" {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		entry.Fields["stack_trace"] = stack
	}

	m.safeLog(entry)
}

// safeLog 安全地记录日志（捕获所有错误）
func (m *MethodInterceptor) safeLog(entry *LogEntry) {
	defer func() {
		if r := recover(); r != nil {
			// 日志系统出错，静默处理，不影响业务
			fmt.Printf("[INTERCEPTOR ERROR] Failed to log: %v\n", r)
		}
	}()

	if m.logger != nil {
		m.logger.LogEntry(entry)
	}
}

// getCaller 获取调用位置
func (m *MethodInterceptor) getCaller() (string, int) {
	// 跳过拦截器内部的调用栈
	for i := 3; i < 10; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// 跳过拦截器自身的文件
		if !strings.Contains(file, "interceptor.go") && !strings.Contains(file, "interceptor_") {
			// 只保留文件名
			parts := strings.Split(file, "/")
			if len(parts) > 0 {
				return parts[len(parts)-1], line
			}
			return file, line
		}
	}
	return "", 0
}

// getStackTrace 获取堆栈信息
func (m *MethodInterceptor) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
