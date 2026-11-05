package backend

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
)

const (
	automationScriptRunDefaultTimeout = 5 * time.Minute
	automationScriptRunMinTimeout     = 1 * time.Second
	automationScriptRunMaxTimeout     = 30 * time.Minute
)

func (a *App) automationScriptRunStore() *automation.ScriptRunStore {
	return automation.NewScriptRunStore(a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "automation", "runs"))))
}

func (a *App) AutomationScriptRunList(limit int) ([]automation.ScriptRunRecord, error) {
	return a.automationScriptRunStore().List(limit)
}

func (a *App) AutomationScriptRun(scriptID string) (*automation.ScriptRunRecord, error) {
	return a.AutomationScriptRunWithOptions(automation.ScriptRunRequest{
		ScriptID:          scriptID,
		UseScriptSelector: true,
		UseScriptParams:   true,
	})
}

func (a *App) AutomationScriptRunWithOptions(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	startedAt := time.Now()
	run := automation.ScriptRunRecord{
		ScriptID:  strings.TrimSpace(input.ScriptID),
		Status:    "failed",
		StartedAt: startedAt.Format(time.RFC3339),
	}

	store := a.automationScriptStore()
	if err := a.ensureAutomationScriptDefaults(store); err != nil {
		run.Summary = "脚本读取失败"
		run.Error = err.Error()
		return a.finalizeAutomationScriptRun(run, startedAt)
	}

	script, err := store.Get(run.ScriptID)
	if err != nil {
		run.Summary = "脚本读取失败"
		run.Error = err.Error()
		return a.finalizeAutomationScriptRun(run, startedAt)
	}

	run.ScriptName = script.Name
	run.ScriptType = script.Type

	runCtx := a.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel := context.WithTimeout(runCtx, automationScriptRunTimeout(input))
	defer cancel()

	switch script.Type {
	case "launch-api":
		resultText, summary, errText := a.runLaunchAPIScript(runCtx, script, input)
		run.ResultText = resultText
		run.Summary = summary
		run.Error = errText
		if errText == "" {
			run.Status = "success"
		}
	case "playwright-cdp":
		resultText, logText, summary, errText := a.runPlaywrightScript(runCtx, script, input)
		run.ResultText = resultText
		run.LogText = logText
		run.Summary = summary
		run.Error = errText
		if errText == "" {
			run.Status = "success"
		}
	default:
		run.Summary = "当前脚本类型暂不支持直接执行"
		run.Error = fmt.Sprintf("script type %q is not supported yet", script.Type)
	}

	return a.finalizeAutomationScriptRun(run, startedAt)
}

func automationScriptRunTimeout(input automation.ScriptRunRequest) time.Duration {
	if input.TimeoutMs <= 0 {
		return automationScriptRunDefaultTimeout
	}
	timeout := time.Duration(input.TimeoutMs) * time.Millisecond
	if timeout < automationScriptRunMinTimeout {
		return automationScriptRunMinTimeout
	}
	if timeout > automationScriptRunMaxTimeout {
		return automationScriptRunMaxTimeout
	}
	return timeout
}

func automationRunContextErrorMessage(err error) string {
	if err == context.DeadlineExceeded {
		return "自动化任务超时，已终止"
	}
	if err == context.Canceled {
		return "自动化任务已取消"
	}
	return err.Error()
}

func (a *App) finalizeAutomationScriptRun(run automation.ScriptRunRecord, startedAt time.Time) (*automation.ScriptRunRecord, error) {
	run.FinishedAt = time.Now().Format(time.RFC3339)
	run.DurationMs = time.Since(startedAt).Milliseconds()
	saved, err := a.automationScriptRunStore().Save(run)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}
