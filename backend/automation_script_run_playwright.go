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

func (a *App) ensurePlaywrightTargetReady(selector map[string]any) (map[string]any, string, error) {
	normalized := cloneAutomationSelector(selector)
	profileID := automationSelectorProfileID(normalized)
	code := automationSelectorCode(normalized)
	if profileID == "" && code == "" {
		return normalized, "", nil
	}

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
		return nil, "", fmt.Errorf("预启动脚本目标实例失败: %w", err)
	}
	if profile == nil {
		return nil, "", fmt.Errorf("预启动脚本目标实例失败：未返回实例")
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

	return normalized, resolvedProfileID, nil
}

func automationScriptTaskKey(scriptID string, selector map[string]any) string {
	if profileID := automationSelectorProfileID(selector); profileID != "" {
		return profileID
	}
	return "script:" + strings.TrimSpace(scriptID)
}

func (a *App) runPlaywrightScript(ctx context.Context, script automation.ScriptRecord, input automation.ScriptRunRequest) (string, string, string, string) {
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

	selector, targetSummary, err := a.resolveAutomationEffectiveSelector(script, input, false)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	selector, taskProfileID, err := a.ensurePlaywrightTargetReady(selector)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
	}
	if err := ctx.Err(); err != nil {
		return "", "", "脚本执行失败", automationRunContextErrorMessage(err)
	}
	params, err := parseAutomationJSONObject(paramsText, false)
	if err != nil {
		return "", "", "脚本执行失败", err.Error()
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
