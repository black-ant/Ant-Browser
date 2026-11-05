package backend

import (
	"strings"

	"ant-chrome/backend/internal/config"
)

func (a *App) GetAutomationState() map[string]interface{} {
	return a.automationStatePayload()
}

func (a *App) automationStatePayload() map[string]interface{} {
	settings := map[string]interface{}{
		"enabled":              false,
		"installPolicy":        config.DefaultAutomationInstallPolicy,
		"runtimeVersion":       config.DefaultAutomationRuntimeVersion(config.DefaultAutomationNodeVersion, config.DefaultAutomationPWVersion),
		"headlessDefault":      false,
		"keepRuntimeOnDisable": true,
		"allowTypeScriptBuild": false,
		"artifactsDir":         "data/automation/artifacts",
		"nodeSource":           config.DefaultAutomationNodeSource,
		"systemNodePath":       "",
		"nodeVersion":          config.DefaultAutomationNodeVersion,
		"playwrightVersion":    config.DefaultAutomationPWVersion,
	}
	status := map[string]interface{}{
		"installed":          false,
		"ready":              false,
		"installing":         false,
		"lastError":          "",
		"runtimeDir":         "",
		"nodePath":           "",
		"nodeSource":         config.DefaultAutomationNodeSource,
		"nodeResolution":     "",
		"systemNodeDetected": false,
		"systemNodePath":     "",
		"systemNodeError":    "",
		"nodeVersion":        config.DefaultAutomationNodeVersion,
		"playwrightVersion":  config.DefaultAutomationPWVersion,
	}

	if a.config != nil {
		settings["enabled"] = a.config.Automation.Enabled
		settings["installPolicy"] = a.config.Automation.InstallPolicy
		settings["runtimeVersion"] = a.config.Automation.RuntimeVersion
		settings["headlessDefault"] = a.config.Automation.HeadlessDefault
		settings["keepRuntimeOnDisable"] = a.config.Automation.KeepRuntimeOnDisable
		settings["allowTypeScriptBuild"] = a.config.Automation.AllowTypeScriptBuild
		settings["artifactsDir"] = a.config.Automation.ArtifactsDir
		settings["nodeSource"] = a.config.Automation.NodeSource
		settings["systemNodePath"] = a.config.Automation.SystemNodePath
		settings["nodeVersion"] = a.config.Automation.NodeVersion
		settings["playwrightVersion"] = a.config.Automation.PlaywrightCoreVersion
	}

	if a.automationMgr != nil {
		state := a.automationMgr.CurrentState()
		status = map[string]interface{}{
			"installed":          state.Installed,
			"ready":              state.Ready,
			"installing":         state.Installing,
			"lastError":          state.LastError,
			"runtimeDir":         state.RuntimeDir,
			"nodePath":           state.NodePath,
			"runnerPath":         state.RunnerPath,
			"nodeSource":         state.NodeSource,
			"nodeResolution":     state.NodeResolution,
			"systemNodeDetected": state.SystemNodeDetected,
			"systemNodePath":     state.SystemNodePath,
			"systemNodeError":    state.SystemNodeError,
			"nodeVersion":        state.NodeVersion,
			"playwrightVersion":  state.PlaywrightVersion,
		}
	}

	return map[string]interface{}{
		"settings": settings,
		"status":   status,
	}
}

func applyAutomationConfigDefaults(auto *config.AutomationConfig) {
	if auto == nil {
		return
	}
	if strings.TrimSpace(auto.InstallPolicy) == "" {
		auto.InstallPolicy = config.DefaultAutomationInstallPolicy
	}
	auto.NodeSource = normalizeAutomationNodeSourceInput(auto.NodeSource)
	auto.SystemNodePath = strings.TrimSpace(auto.SystemNodePath)
	if strings.TrimSpace(auto.NodeVersion) == "" {
		auto.NodeVersion = config.DefaultAutomationNodeVersion
	}
	if strings.TrimSpace(auto.PlaywrightCoreVersion) == "" {
		auto.PlaywrightCoreVersion = config.DefaultAutomationPWVersion
	}
	if strings.TrimSpace(auto.RuntimeVersion) == "" {
		auto.RuntimeVersion = config.DefaultAutomationRuntimeVersion(
			auto.NodeVersion,
			auto.PlaywrightCoreVersion,
		)
	}
	if !auto.KeepRuntimeOnDisable {
		auto.KeepRuntimeOnDisable = true
	}
	if strings.TrimSpace(auto.ArtifactsDir) == "" {
		auto.ArtifactsDir = "data/automation/artifacts"
	} else {
		auto.ArtifactsDir = strings.TrimSpace(auto.ArtifactsDir)
	}
}

func normalizeAutomationNodeSourceInput(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case config.AutomationNodeSourceSystem:
		return config.AutomationNodeSourceSystem
	case config.AutomationNodeSourceBundled:
		return config.AutomationNodeSourceBundled
	default:
		return config.AutomationNodeSourceAuto
	}
}
