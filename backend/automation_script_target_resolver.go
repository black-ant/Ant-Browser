package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
)

const defaultAutomationCreateNameTemplate = "${templateName}-${timestamp}"

func (a *App) resolveAutomationEffectiveSelector(script automation.ScriptRecord, input automation.ScriptRunRequest, required bool) (map[string]any, string, error) {
	if mode := automationScriptRunTargetMode(script, input); mode != automationScriptTargetMode(script) {
		script.TargetConfig.Mode = mode
	}
	overrideSelectorText := strings.TrimSpace(input.SelectorText)
	if automationScriptTargetMode(script) == "manual" && !input.UseScriptSelector && overrideSelectorText != "" {
		selector, err := parseAutomationJSONObject(overrideSelectorText, required)
		return selector, "", err
	}

	if automationScriptTargetMode(script) != "manual" {
		targetInput := input.TargetInput
		if targetInput == nil && !input.UseScriptSelector && overrideSelectorText != "" {
			selector, err := parseAutomationJSONObject(overrideSelectorText, false)
			if err != nil {
				return nil, "", err
			}
			targetInput = selector
		}
		input.TargetInput = targetInput
		effectiveScript, err := applyAutomationRunTargetInput(script, input.TargetInput)
		if err != nil {
			return nil, "", err
		}
		return a.resolveAutomationScriptTarget(effectiveScript)
	}

	selectorText := resolveAutomationRunJSONText(input.SelectorText, script.SelectorText, input.UseScriptSelector)
	selector, err := parseAutomationJSONObject(selectorText, required)
	return selector, "", err
}

func automationScriptTargetMode(script automation.ScriptRecord) string {
	mode := strings.ToLower(strings.TrimSpace(script.TargetConfig.Mode))
	switch mode {
	case "existing", "create", "rotate":
		return mode
	default:
		return "manual"
	}
}

func automationScriptRunTargetMode(script automation.ScriptRecord, input automation.ScriptRunRequest) string {
	mode := strings.ToLower(strings.TrimSpace(input.TargetMode))
	switch mode {
	case "manual", "existing", "create", "rotate":
		return mode
	default:
		return automationScriptTargetMode(script)
	}
}

func applyAutomationRunTargetInput(script automation.ScriptRecord, value any) (automation.ScriptRecord, error) {
	if value == nil {
		return script, nil
	}
	payload, err := marshalAutomationRunTargetInput(value)
	if err != nil {
		return script, err
	}
	if len(payload) == 0 {
		return script, nil
	}

	switch automationScriptTargetMode(script) {
	case "existing":
		selector, err := decodeAutomationRunTargetSelector(payload)
		if err != nil {
			return script, fmt.Errorf("已有实例配置无效: %w", err)
		}
		script.TargetConfig.Selector = selector
	case "rotate":
		selector, err := decodeAutomationRunTargetSelector(payload)
		if err != nil {
			return script, fmt.Errorf("条件轮询配置无效: %w", err)
		}
		script.TargetConfig.Selector = selector
	case "create":
		var input struct {
			TemplateSelector   automation.ScriptTargetSelector `json:"templateSelector"`
			Selector           automation.ScriptTargetSelector `json:"selector"`
			CreateNameTemplate string                          `json:"createNameTemplate"`
			ProfileName        string                          `json:"profileName"`
		}
		if err := json.Unmarshal(payload, &input); err != nil {
			return script, fmt.Errorf("targetInput must be a JSON object")
		}
		selector := input.TemplateSelector
		if automationTargetSelectorEmpty(normalizeAutomationTargetSelector(selector)) {
			selector = input.Selector
		}
		script.TargetConfig.TemplateSelector = selector
		if name := strings.TrimSpace(input.CreateNameTemplate); name != "" {
			script.TargetConfig.CreateNameTemplate = name
		} else if name := strings.TrimSpace(input.ProfileName); name != "" {
			script.TargetConfig.CreateNameTemplate = name
		}
	}
	return script, nil
}

func marshalAutomationRunTargetInput(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("targetInput must be a JSON object")
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, fmt.Errorf("targetInput must be a JSON object")
	}
	if len(object) == 0 {
		return nil, nil
	}
	return data, nil
}

func decodeAutomationRunTargetSelector(payload []byte) (automation.ScriptTargetSelector, error) {
	var selector automation.ScriptTargetSelector
	if err := json.Unmarshal(payload, &selector); err != nil {
		return selector, fmt.Errorf("targetInput must be a JSON object")
	}
	return selector, nil
}

func (a *App) resolveAutomationScriptTarget(script automation.ScriptRecord) (map[string]any, string, error) {
	switch strings.ToLower(strings.TrimSpace(script.TargetConfig.Mode)) {
	case "existing":
		profile, err := a.resolveAutomationExactTargetProfile(script.TargetConfig.Selector, "使用已有实例")
		if err != nil {
			return nil, "", err
		}
		return automationProfileSelector(profile.ProfileId), automationProfileLabel(profile), nil
	case "rotate":
		profiles, err := a.resolveAutomationTargetProfiles(script.TargetConfig.Selector, "按条件轮询实例")
		if err != nil {
			return nil, "", err
		}
		profile := a.pickAutomationRoundRobinTarget(script.ID, script.TargetConfig.Selector, profiles)
		return automationProfileSelector(profile.ProfileId), fmt.Sprintf("轮询实例 %s", automationProfileLabel(profile)), nil
	case "create":
		templateProfile, err := a.resolveAutomationExactTargetProfile(script.TargetConfig.TemplateSelector, "按模板新建实例")
		if err != nil {
			return nil, "", err
		}
		createdName := buildAutomationCreatedProfileName(script.TargetConfig.CreateNameTemplate, script, templateProfile)
		createdProfile, err := a.browserMgr.Copy(templateProfile.ProfileId, createdName)
		if err != nil {
			return nil, "", fmt.Errorf("按模板新建实例失败: %w", err)
		}
		if createdProfile == nil {
			return nil, "", fmt.Errorf("按模板新建实例失败：未返回新实例")
		}
		return automationProfileSelector(createdProfile.ProfileId), fmt.Sprintf("新建实例 %s", automationProfileLabel(*createdProfile)), nil
	default:
		return map[string]any{}, "", nil
	}
}

func (a *App) resolveAutomationExactTargetProfile(selector automation.ScriptTargetSelector, actionLabel string) (browser.Profile, error) {
	if profile, ok := a.findAutomationTargetProfileByIDOrCode(selector); ok {
		return profile, nil
	}
	return a.resolveAutomationTargetProfile(selector, actionLabel)
}

func (a *App) resolveAutomationTargetProfile(selector automation.ScriptTargetSelector, actionLabel string) (browser.Profile, error) {
	profiles, err := a.resolveAutomationTargetProfiles(selector, actionLabel)
	if err != nil {
		return browser.Profile{}, err
	}
	if len(profiles) > 1 {
		return browser.Profile{}, fmt.Errorf("%s失败：%s", actionLabel, buildAutomationTargetAmbiguousError(profiles))
	}
	return profiles[0], nil
}

func (a *App) resolveAutomationTargetProfiles(selector automation.ScriptTargetSelector, actionLabel string) ([]browser.Profile, error) {
	if a.browserMgr == nil {
		return nil, fmt.Errorf("%s失败：实例管理器未初始化", actionLabel)
	}

	normalized := normalizeAutomationTargetSelector(selector)
	if automationTargetSelectorEmpty(normalized) {
		return nil, fmt.Errorf("%s失败：请至少填写一个实例条件", actionLabel)
	}

	snapshots := a.browserMgr.List()
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("%s失败：当前没有可用实例", actionLabel)
	}

	if normalized.Code != "" {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return strings.EqualFold(strings.TrimSpace(item.LaunchCode), normalized.Code)
		})
	}
	if normalized.ProfileID != "" {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return strings.TrimSpace(item.ProfileId) == normalized.ProfileID
		})
	}
	if normalized.ProfileName != "" {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return strings.EqualFold(strings.TrimSpace(item.ProfileName), normalized.ProfileName)
		})
	}
	if normalized.GroupID != "" {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return strings.TrimSpace(item.GroupId) == normalized.GroupID
		})
	}
	if len(normalized.Tags) > 0 {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return automationProfileHasAllTags(item, normalized.Tags)
		})
	}
	if len(normalized.Keywords) > 0 {
		snapshots = filterAutomationProfiles(snapshots, func(item browser.Profile) bool {
			return automationProfileMatchesAllKeywordQueries(item, normalized.Keywords)
		})
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("%s失败：没有匹配到实例", actionLabel)
	}

	sortAutomationProfilesForTarget(snapshots)
	return snapshots, nil
}

func normalizeAutomationTargetSelector(selector automation.ScriptTargetSelector) automation.ScriptTargetSelector {
	return automation.ScriptTargetSelector{
		Code:        strings.ToUpper(strings.TrimSpace(selector.Code)),
		ProfileID:   strings.TrimSpace(selector.ProfileID),
		ProfileName: strings.TrimSpace(selector.ProfileName),
		GroupID:     strings.TrimSpace(selector.GroupID),
		Keywords:    normalizeAutomationTargetTerms(selector.Keywords),
		Tags:        normalizeAutomationTargetTerms(selector.Tags),
	}
}

func (a *App) findAutomationTargetProfileByIDOrCode(selector automation.ScriptTargetSelector) (browser.Profile, bool) {
	if a.browserMgr == nil {
		return browser.Profile{}, false
	}

	normalizedProfileID := strings.TrimSpace(selector.ProfileID)
	normalizedCode := strings.ToUpper(strings.TrimSpace(selector.Code))
	if normalizedProfileID == "" && normalizedCode == "" {
		return browser.Profile{}, false
	}

	snapshots := a.browserMgr.List()
	if normalizedProfileID != "" {
		for _, item := range snapshots {
			if strings.TrimSpace(item.ProfileId) == normalizedProfileID {
				return item, true
			}
		}
	}

	if normalizedCode != "" {
		for _, item := range snapshots {
			if strings.EqualFold(strings.TrimSpace(item.LaunchCode), normalizedCode) {
				return item, true
			}
		}
	}

	return browser.Profile{}, false
}

func (a *App) enrichAutomationExactTargetSelector(selector automation.ScriptTargetSelector) automation.ScriptTargetSelector {
	normalized := normalizeAutomationTargetSelector(selector)
	profile, ok := a.findAutomationTargetProfileByIDOrCode(normalized)
	if !ok {
		return normalized
	}

	normalized.ProfileID = strings.TrimSpace(profile.ProfileId)
	if code := strings.ToUpper(strings.TrimSpace(profile.LaunchCode)); code != "" {
		normalized.Code = code
	}
	return normalized
}

func normalizeAutomationTargetTerms(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func automationTargetSelectorEmpty(selector automation.ScriptTargetSelector) bool {
	return selector.Code == "" &&
		selector.ProfileID == "" &&
		selector.ProfileName == "" &&
		selector.GroupID == "" &&
		len(selector.Keywords) == 0 &&
		len(selector.Tags) == 0
}

func automationProfileSelector(profileID string) map[string]any {
	return map[string]any{
		"profileId": strings.TrimSpace(profileID),
	}
}

func (a *App) pickAutomationRoundRobinTarget(scriptID string, selector automation.ScriptTargetSelector, profiles []browser.Profile) browser.Profile {
	rotationKey := buildAutomationTargetRotationKey(scriptID, selector)

	a.automationTargetMu.Lock()
	defer a.automationTargetMu.Unlock()

	lastProfileID := strings.TrimSpace(a.automationTargetCursor[rotationKey])
	nextIndex := 0
	if lastProfileID != "" {
		for idx, profile := range profiles {
			if profile.ProfileId == lastProfileID {
				nextIndex = (idx + 1) % len(profiles)
				break
			}
		}
	}

	selected := profiles[nextIndex]
	a.automationTargetCursor[rotationKey] = selected.ProfileId
	return selected
}

func buildAutomationTargetRotationKey(scriptID string, selector automation.ScriptTargetSelector) string {
	payload := normalizeAutomationTargetSelector(selector)
	data, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(scriptID)
	}
	return strings.TrimSpace(scriptID) + ":" + string(data)
}

func buildAutomationCreatedProfileName(template string, script automation.ScriptRecord, source browser.Profile) string {
	now := time.Now()
	pattern := strings.TrimSpace(template)
	if pattern == "" {
		pattern = defaultAutomationCreateNameTemplate
	}

	replacements := map[string]string{
		"${timestamp}":    now.Format("20060102-150405"),
		"${date}":         now.Format("20060102"),
		"${time}":         now.Format("150405"),
		"${templateName}": strings.TrimSpace(source.ProfileName),
		"${scriptName}":   strings.TrimSpace(script.Name),
	}
	for placeholder, value := range replacements {
		pattern = strings.ReplaceAll(pattern, placeholder, value)
	}
	pattern = strings.TrimSpace(pattern)
	if pattern != "" {
		return pattern
	}

	templateName := strings.TrimSpace(source.ProfileName)
	if templateName == "" {
		templateName = "自动化实例"
	}
	return fmt.Sprintf("%s-%s", templateName, now.Format("20060102-150405"))
}
