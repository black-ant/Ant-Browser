package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const TaskEventName = "automation:task:state"

const (
	taskTypeScript = "script"
)

func (m *Manager) RunScriptTask(ctx context.Context, req ScriptTaskRequest) (ScriptTaskResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeoutLimit := req.Timeout
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if deadline, ok := ctx.Deadline(); ok {
		timeoutLimit = time.Until(deadline)
	}

	state := m.CurrentState()
	if !state.Ready {
		return ScriptTaskResult{}, fmt.Errorf("自动化运行时尚未就绪")
	}

	req.TaskKey = strings.TrimSpace(req.TaskKey)
	if req.TaskKey == "" {
		return ScriptTaskResult{}, fmt.Errorf("taskKey is required")
	}
	req.ScriptPath = strings.TrimSpace(req.ScriptPath)
	if req.ScriptPath == "" {
		return ScriptTaskResult{}, fmt.Errorf("scriptPath is required")
	}
	req.LaunchBaseURL = strings.TrimSpace(req.LaunchBaseURL)
	if req.LaunchBaseURL == "" {
		return ScriptTaskResult{}, fmt.Errorf("launchBaseUrl is required")
	}

	payload := taskRunnerPayload{
		TaskType:         taskTypeScript,
		RuntimeDir:       state.RuntimeDir,
		ScriptPath:       req.ScriptPath,
		Selector:         req.Selector,
		Params:           req.Params,
		LaunchBaseURL:    req.LaunchBaseURL,
		LaunchAuthHeader: strings.TrimSpace(req.LaunchAuthHeader),
		LaunchAuthValue:  strings.TrimSpace(req.LaunchAuthValue),
		ArtifactDir:      strings.TrimSpace(req.ArtifactDir),
	}

	taskID, runnerResp, rawOutput, durationMs, err := m.executeTask(
		ctx,
		req.TaskKey,
		payload,
		"自动化 script task 已启动",
		"自动化 script task 已完成",
		timeoutLimit,
	)
	if err != nil {
		return ScriptTaskResult{}, err
	}

	result := ScriptTaskResult{
		TaskID:            taskID,
		TaskKey:           req.TaskKey,
		OK:                runnerResp.OK,
		Summary:           strings.TrimSpace(runnerResp.Summary),
		Error:             strings.TrimSpace(runnerResp.Error),
		ResultText:        rawOutput,
		LogText:           formatTaskRunnerLogs(runnerResp.Logs),
		DurationMs:        durationMs,
		StartedAt:         runnerResp.StartedAt,
		FinishedAt:        runnerResp.FinishedAt,
		RuntimeVersion:    state.RuntimeVersion,
		NodeVersion:       state.NodeVersion,
		PlaywrightVersion: state.PlaywrightVersion,
	}
	if result.Summary == "" {
		if result.OK {
			result.Summary = "脚本执行完成"
		} else {
			result.Summary = "脚本执行失败"
		}
	}
	return result, nil
}

func formatTaskRunnerLogs(logs []taskRunnerLogEntry) string {
	if len(logs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(logs))
	for _, entry := range logs {
		valueText := formatTaskRunnerLogValues(entry.Values)
		if valueText == "" {
			continue
		}
		timeText := strings.TrimSpace(entry.Time)
		if timeText == "" {
			lines = append(lines, valueText)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s", timeText, valueText))
	}
	return strings.Join(lines, "\n")
}

func formatTaskRunnerLogValues(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, formatTaskRunnerLogValue(value))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func formatTaskRunnerLogValue(value any) string {
	if value == nil {
		return "null"
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	if reflect.TypeOf(value).Kind() == reflect.Map || reflect.TypeOf(value).Kind() == reflect.Slice {
		if data, err := json.Marshal(value); err == nil {
			return string(data)
		}
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (m *Manager) executeTask(ctx context.Context, taskKey string, payload taskRunnerPayload, startMessage string, completeMessage string, timeoutLimit time.Duration) (string, taskRunnerResponse, string, int64, error) {
	taskID, err := m.registerTask(taskKey)
	if err != nil {
		return "", taskRunnerResponse{}, "", 0, err
	}
	defer m.unregisterTask(taskID)

	payloadPath, err := m.writeTaskPayload(payload)
	if err != nil {
		return "", taskRunnerResponse{}, "", 0, err
	}
	defer os.Remove(payloadPath)

	state := m.CurrentState()
	cmd := exec.CommandContext(ctx, state.NodePath, state.RunnerPath, payloadPath)
	cmd.Dir = state.RuntimeDir
	prepareTaskCommand(cmd)
	cmd.Cancel = func() error {
		return stopTaskProcess(cmd)
	}
	cmd.WaitDelay = 5 * time.Second

	startedAt := time.Now()
	m.attachTaskCommand(taskID, cmd)
	m.emitTaskEvent(TaskEvent{
		TaskID:    taskID,
		ProfileID: taskKey,
		Phase:     "started",
		Message:   startMessage,
		StartedAt: startedAt.Format(time.RFC3339),
	})

	output, runErr := cmd.CombinedOutput()
	durationMs := time.Since(startedAt).Milliseconds()
	if runErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = stopTaskProcess(cmd)
			message := taskContextErrorMessage(ctxErr, timeoutLimit)
			m.emitTaskEvent(TaskEvent{
				TaskID:     taskID,
				ProfileID:  taskKey,
				Phase:      "failed",
				Message:    message,
				StartedAt:  startedAt.Format(time.RFC3339),
				FinishedAt: time.Now().Format(time.RFC3339),
				DurationMs: durationMs,
			})
			return "", taskRunnerResponse{}, "", durationMs, fmt.Errorf("%s", message)
		}

		message := strings.TrimSpace(string(output))
		if message == "" {
			message = runErr.Error()
		}
		m.emitTaskEvent(TaskEvent{
			TaskID:     taskID,
			ProfileID:  taskKey,
			Phase:      "failed",
			Message:    message,
			StartedAt:  startedAt.Format(time.RFC3339),
			FinishedAt: time.Now().Format(time.RFC3339),
			DurationMs: durationMs,
		})
		return "", taskRunnerResponse{}, "", durationMs, fmt.Errorf("自动化任务执行失败: %s", message)
	}

	var runnerResp taskRunnerResponse
	if err := json.Unmarshal(output, &runnerResp); err != nil {
		return "", taskRunnerResponse{}, "", durationMs, fmt.Errorf("解析自动化任务结果失败: %w", err)
	}

	m.emitTaskEvent(TaskEvent{
		TaskID:     taskID,
		ProfileID:  taskKey,
		Phase:      "completed",
		Message:    completeMessage,
		StartedAt:  runnerResp.StartedAt,
		FinishedAt: runnerResp.FinishedAt,
		DurationMs: durationMs,
	})

	return taskID, runnerResp, string(output), durationMs, nil
}

func taskContextErrorMessage(err error, timeoutLimit time.Duration) string {
	if err == context.DeadlineExceeded {
		if timeoutText := formatTaskTimeout(timeoutLimit); timeoutText != "" {
			return fmt.Sprintf("自动化任务超时，已终止（上限 %s）", timeoutText)
		}
		return "自动化任务超时，已终止"
	}
	if err == context.Canceled {
		return "自动化任务已取消"
	}
	return err.Error()
}

func formatTaskTimeout(timeout time.Duration) string {
	if timeout <= 0 {
		return ""
	}
	if timeout >= time.Minute && timeout%time.Minute == 0 {
		return fmt.Sprintf("%d 分钟", int64(timeout/time.Minute))
	}
	if timeout >= time.Second && timeout%time.Second == 0 {
		return fmt.Sprintf("%d 秒", int64(timeout/time.Second))
	}
	return fmt.Sprintf("%d 毫秒", timeout.Milliseconds())
}

func (m *Manager) writeTaskPayload(payload taskRunnerPayload) (string, error) {
	tempDir := filepath.Join(m.runtimeRoot(), "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("创建自动化任务临时目录失败: %w", err)
	}
	file, err := os.CreateTemp(tempDir, "task-*.json")
	if err != nil {
		return "", fmt.Errorf("创建自动化任务临时文件失败: %w", err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		return "", fmt.Errorf("写入自动化任务 payload 失败: %w", err)
	}
	return file.Name(), nil
}
