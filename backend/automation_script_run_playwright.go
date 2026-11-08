package backend

import (
	"context"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
)

func automationSelectorProfileID(selector map[string]any) string {
	if selector == nil {
		return ""
	}

	profileID, _ := selector["profileId"].(string)
	return strings.TrimSpace(profileID)
}

func automationSelectorCode(selector map[string]any) string {
	if selector == nil {
		return ""
	}

	code, _ := selector["code"].(string)
	return strings.ToUpper(strings.TrimSpace(code))
}

func cloneAutomationSelector(selector map[string]any) map[string]any {
	if selector == nil {
		return nil
	}

	cloned := make(map[string]any, len(selector))
	for key, value := range selector {
		cloned[key] = value
	}
	return cloned
}

// ensurePlaywrightTargetReady 预启动脚本目标实例。
// startedByRun 表示实例是本次任务拉起来的（任务开始前未运行），任务结束时必须由后端负责停掉。
func (a *App) ensurePlaywrightTargetReady(selector map[string]any) (map[string]any, string, bool, error) {
	normalized := cloneAutomationSelector(selector)
	profileID := automationSelectorProfileID(normalized)
	code := automationSelectorCode(normalized)
	if profileID == "" && code == "" {
		return normalized, "", false, nil
	}

	startedByRun := !a.automationTargetRunning(profileID, code)

	var (
		profile *browser.Profile
		err     error
	)
	switch {
	case profileID != "":
		profile, err = a.BrowserInstanceStart(profileID)
	case code != "":
		profile, err = a.BrowserInstanceStartByCode(code)
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("预启动脚本目标实例失败: %w", err)
	}
	if profile == nil {
		return nil, "", false, fmt.Errorf("预启动脚本目标实例失败：未返回实例")
	}

	resolvedProfileID := strings.TrimSpace(profile.ProfileId)
	if resolvedProfileID != "" && normalized != nil {
		normalized["profileId"] = resolvedProfileID
	}

	resolvedCode := strings.ToUpper(strings.TrimSpace(profile.LaunchCode))
	if resolvedCode == "" {
		resolvedCode = code
	}
	if resolvedCode != "" && normalized != nil {
		normalized["code"] = resolvedCode
	}

	return normalized, resolvedProfileID, startedByRun, nil
}

func automationScriptTaskKey(scriptID string, selector map[string]any) string {
	if profileID := automationSelectorProfileID(selector); profileID != "" {
		return profileID
	}
	return "script:" + strings.TrimSpace(scriptID)
}

func (a *App) runPlaywrightScript(ctx context.Context, script automation.ScriptRecord, input automation.ScriptRunRequest) (resultText string, logText string, summary string, errText string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.automationMgr == nil {
		return "", "", "脚本执行失败", "automation runtime manager is not initialized"
	}
	if a.config == nil || !a.config.Automation.Enabled {
		return "", "", "脚本执行失败", "自动化支持尚未启用"
	}
	if err := ctx.Err(); err != nil {
		return "", "", "脚本执行失败", automationRunContextErrorMessage(err)
	}
	if err := a.automationMgr.EnsureInstalled(ctx); err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	if err := ctx.Err(); err != nil {
		return "", "", "脚本执行失败", automationRunContextErrorMessage(err)
	}

	state := a.automationMgr.CurrentState()
	if !state.Ready {
		return "", "", "脚本执行失败", "自动化运行时尚未就绪"
	}

	paramsText := resolveAutomationRunJSONText(input.ParamsText, script.ParamsText, input.UseScriptParams)

	// 先校验 params 再预启动实例：参数不合法时不应该留下一个已经拉起的浏览器。
	params, err := parseAutomationJSONObject(paramsText, false)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}

	selector, targetSummary, err := a.resolveAutomationEffectiveSelector(script, input, false)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	selector, taskProfileID, startedByRun, err := a.ensurePlaywrightTargetReady(selector)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	// 收尾保证：任务自己拉起的实例必须在任务结束时停止并校验；脚本可用 keepProfileAfterRun 显式豁免。
	if startedByRun && strings.TrimSpace(taskProfileID) != "" && !automationRunKeepProfile(params) {
		defer func() {
			if ok, detail := a.stopAutomationTargetProfile(taskProfileID); !ok {
				errText = appendAutomationCleanupError(errText, detail)
				summary = appendAutomationRunSummary(summary, "实例清理失败")
			}
		}()
	}
	if err := ctx.Err(); err != nil {
		return "", "", "脚本执行失败", automationRunContextErrorMessage(err)
	}

	baseURL, authHeader, authValue, err := a.automationDemoEndpoint()
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}

	scriptPath, artifactDir, cleanup, err := a.preparePlaywrightScriptWorkspace(state.RuntimeDir, script)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	defer cleanup()
	if err := ctx.Err(); err != nil {
		return "", "", "脚本执行失败", automationRunContextErrorMessage(err)
	}

	taskResult, err := a.automationMgr.RunScriptTask(ctx, automation.ScriptTaskRequest{
		TaskKey:          automationScriptTaskKey(script.ID, selector),
		ScriptPath:       scriptPath,
		Selector:         selector,
		Params:           params,
		LaunchBaseURL:    baseURL,
		LaunchAuthHeader: authHeader,
		LaunchAuthValue:  authValue,
		ArtifactDir:      artifactDir,
		Timeout:          automationScriptRunTimeout(input),
	})
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	if taskResult.TaskKey == "" && taskProfileID != "" {
		taskResult.TaskKey = taskProfileID
	}
	if !taskResult.OK {
		errorText := strings.TrimSpace(taskResult.Error)
		if errorText == "" {
			errorText = "playwright script returned ok=false"
		}
		return taskResult.ResultText, taskResult.LogText, appendAutomationRunSummary(taskResult.Summary, targetSummary), errorText
	}
	return taskResult.ResultText, taskResult.LogText, appendAutomationRunSummary(taskResult.Summary, targetSummary), ""
}
