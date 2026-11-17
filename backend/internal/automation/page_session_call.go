package automation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 单个会话的 NDJSON 一问一答。
//
// 读写都在 mu 保护下完成：管道上同时发两条指令的话，两个响应无法可靠地
// 归属回各自的调用方。id 关联只用来跳过 ready/closed 这类无 id 的事件行，
// 不用来支持并发。

// call 发送一条指令并等待其响应。
//
// 返回 error 表示会话本身出了问题（管道断了、超时、进程死了），调用方应丢弃会话；
// 指令自身执行失败通过 PageStepResult.OK=false 表达，不算 error。
func (s *pageSession) call(command PageCommand, timeout time.Duration) (PageStepResult, error) {
	action := strings.TrimSpace(command.Action)
	if action == "" {
		return PageStepResult{}, fmt.Errorf("page action is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return PageStepResult{}, fmt.Errorf("页面会话已关闭")
	}

	s.nextID++
	id := s.nextID

	line, err := marshalCommand(id, PageCommand{Action: action, Args: command.Args})
	if err != nil {
		return PageStepResult{}, fmt.Errorf("序列化页面指令失败: %w", err)
	}
	if _, err := s.proc.Write(line); err != nil {
		s.markClosed()
		return PageStepResult{}, fmt.Errorf("页面会话已断开: %w", err)
	}

	// Node 侧执行指令本身也有超时，这里再宽限一点，避免 Go 先超时把会话判死。
	envelope, err := s.readEnvelope(id, timeout+10*time.Second)
	if err != nil {
		s.markClosed()
		return PageStepResult{}, err
	}

	s.lastUsed = time.Now()
	return PageStepResult{
		Action: action,
		OK:     envelope.OK,
		Result: envelope.Result,
		Error:  envelope.Error,
	}, nil
}

// awaitReady 等待 Node 侧完成 launch + CDP 握手后发出的 ready 行。
func (s *pageSession) awaitReady(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	envelope, err := s.readEnvelope(0, timeout)
	if err != nil {
		return err
	}
	if envelope.Type != "ready" {
		return fmt.Errorf("页面会话启动异常：%s", describeEnvelope(envelope))
	}

	s.session = envelope.Session
	s.lastUsed = time.Now()
	return nil
}

// readEnvelope 读取下一行响应。
//
// wantID > 0 时跳过所有不匹配的行（ready 事件、上一条指令的迟到响应），
// wantID == 0 时返回读到的第一行。closed 事件一律终止等待。
func (s *pageSession) readEnvelope(wantID int64, timeout time.Duration) (sessionEnvelope, error) {
	type readResult struct {
		envelope sessionEnvelope
		err      error
	}

	done := make(chan readResult, 1)
	go func() {
		for {
			line, err := s.readLine()
			if err != nil {
				done <- readResult{err: err}
				return
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			var envelope sessionEnvelope
			if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
				done <- readResult{err: fmt.Errorf("解析页面会话响应失败: %w", err)}
				return
			}
			if envelope.Type == "closed" {
				done <- readResult{err: fmt.Errorf("页面会话已结束：%s", strings.TrimSpace(envelope.Reason))}
				return
			}
			if wantID > 0 && envelope.ID != wantID {
				continue
			}
			done <- readResult{envelope: envelope}
			return
		}
	}()

	select {
	case result := <-done:
		return result.envelope, result.err
	case <-time.After(timeout):
		// 读协程会随管道关闭而退出，close() 由调用方负责。
		return sessionEnvelope{}, fmt.Errorf("页面会话响应超时（上限 %s）", formatTaskTimeout(timeout))
	}
}

// readLine 读一行，并对超长行报错而不是无限累积。
func (s *pageSession) readLine() (string, error) {
	var builder strings.Builder
	for {
		chunk, isPrefix, err := s.reader.ReadLine()
		if err != nil {
			return "", fmt.Errorf("页面会话已断开: %w", err)
		}
		builder.Write(chunk)
		if builder.Len() > pageSessionMaxLine {
			return "", fmt.Errorf("页面会话响应超出 %d 字节上限", pageSessionMaxLine)
		}
		if !isPrefix {
			return builder.String(), nil
		}
	}
}

func describeEnvelope(envelope sessionEnvelope) string {
	if message := strings.TrimSpace(envelope.Error); message != "" {
		return message
	}
	if reason := strings.TrimSpace(envelope.Reason); reason != "" {
		return reason
	}
	if envelope.Type != "" {
		return "unexpected message type " + envelope.Type
	}
	return "unknown response"
}

func (s *pageSession) markClosed() {
	if s.closed {
		return
	}
	s.closed = true
	if s.proc != nil {
		_ = s.proc.Kill()
	}
}

func (s *pageSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markClosed()
}

func (s *pageSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *pageSession) touchedAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUsed
}
