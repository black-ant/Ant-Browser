package config

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

var defaultBrowserStartURLs = []string{}

const (
	// BrowserConnectorXray 是历史 default_connector_type 的默认值。
	// 新代理运行入口不再依赖全局连接栈，而是按单个代理自动解析 xray/sing-box/mihomo。
	BrowserConnectorXray = "xray"
	// BrowserConnectorMihomo 仅保留用于兼容旧配置、旧 API 和历史数据。
	BrowserConnectorMihomo = "mihomo"
)

const (
	BrowserConnectorXrayStack   = BrowserConnectorXray
	BrowserConnectorMihomoStack = BrowserConnectorMihomo
)

// NormalizeBrowserConnectorType 只用于兼容历史 default_connector_type 输入。
// 新代理执行入口应使用 proxy.ResolveProxyKernel 按单个代理选择内核。
func NormalizeBrowserConnectorType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BrowserConnectorMihomo, "clash", "clash-meta":
		return BrowserConnectorMihomo
	case BrowserConnectorXray, "sing-box", "singbox", "sing_box", "":
		return BrowserConnectorXray
	default:
		return BrowserConnectorXray
	}
}

func DefaultBrowserStartURLs() []string {
	return append([]string{}, defaultBrowserStartURLs...)
}

// normalizeConfig 对历史配置进行字段补齐，不覆盖用户已配置值。
func normalizeConfig(config *Config) {
	defaultConfig := DefaultConfig()

	if strings.TrimSpace(config.Database.Type) == "" {
		config.Database.Type = defaultConfig.Database.Type
	}
	if strings.TrimSpace(config.Database.SQLite.Path) == "" {
		config.Database.SQLite.Path = defaultConfig.Database.SQLite.Path
	}

	if strings.TrimSpace(config.App.Name) == "" {
		config.App.Name = defaultConfig.App.Name
	}
	if config.App.Window.Width <= 0 {
		config.App.Window.Width = defaultConfig.App.Window.Width
	}
	if config.App.Window.Height <= 0 {
		config.App.Window.Height = defaultConfig.App.Window.Height
	}
	if config.App.Window.MinWidth <= 0 {
		config.App.Window.MinWidth = defaultConfig.App.Window.MinWidth
	}
	if config.App.Window.MinHeight <= 0 {
		config.App.Window.MinHeight = defaultConfig.App.Window.MinHeight
	}
	if config.Runtime.MaxMemoryMB <= 0 {
		config.Runtime.MaxMemoryMB = defaultConfig.Runtime.MaxMemoryMB
	}
	if config.Runtime.GCPercent <= 0 {
		config.Runtime.GCPercent = defaultConfig.Runtime.GCPercent
	}

	if strings.TrimSpace(config.Logging.Level) == "" {
		config.Logging.Level = defaultConfig.Logging.Level
	}
	if isLegacyDefaultLogPath(config.Logging.FilePath) || strings.TrimSpace(config.Logging.FilePath) == "" {
		config.Logging.FilePath = defaultConfig.Logging.FilePath
	}
	if strings.TrimSpace(config.Logging.Format) == "" {
		config.Logging.Format = defaultConfig.Logging.Format
	}
	if config.Logging.BufferSize <= 0 {
		config.Logging.BufferSize = defaultConfig.Logging.BufferSize
	}
	if config.Logging.AsyncQueueSize <= 0 {
		config.Logging.AsyncQueueSize = defaultConfig.Logging.AsyncQueueSize
	}
	if config.Logging.FlushIntervalMs <= 0 {
		config.Logging.FlushIntervalMs = defaultConfig.Logging.FlushIntervalMs
	}
	if config.Logging.Rotation.MaxSizeMB <= 0 {
		config.Logging.Rotation.MaxSizeMB = defaultConfig.Logging.Rotation.MaxSizeMB
	}
	if config.Logging.Rotation.MaxAge <= 0 {
		config.Logging.Rotation.MaxAge = defaultConfig.Logging.Rotation.MaxAge
	}
	if config.Logging.Rotation.MaxBackups <= 0 {
		config.Logging.Rotation.MaxBackups = defaultConfig.Logging.Rotation.MaxBackups
	}
	if strings.TrimSpace(config.Logging.Rotation.TimeInterval) == "" {
		config.Logging.Rotation.TimeInterval = defaultConfig.Logging.Rotation.TimeInterval
	}

	interceptorAllZero := !config.Logging.Interceptor.Enabled &&
		!config.Logging.Interceptor.LogParameters &&
		!config.Logging.Interceptor.LogResults &&
		config.Logging.Interceptor.SensitiveFields == nil
	if interceptorAllZero {
		config.Logging.Interceptor = cloneInterceptorConfig(defaultConfig.Logging.Interceptor)
	} else if config.Logging.Interceptor.SensitiveFields == nil {
		config.Logging.Interceptor.SensitiveFields = append([]string{}, defaultConfig.Logging.Interceptor.SensitiveFields...)
	}

	if strings.TrimSpace(config.Browser.UserDataRoot) == "" {
		config.Browser.UserDataRoot = defaultConfig.Browser.UserDataRoot
	}
	if len(config.Browser.DefaultFingerprintArgs) == 0 {
		config.Browser.DefaultFingerprintArgs = append([]string{}, defaultConfig.Browser.DefaultFingerprintArgs...)
	} else if isLegacyMinimalDefaultFingerprintArgs(config.Browser.DefaultFingerprintArgs) {
		config.Browser.DefaultFingerprintArgs = appendEffectiveRuntimeFingerprintArgs(config.Browser.DefaultFingerprintArgs)
	}
	if len(config.Browser.DefaultLaunchArgs) == 0 {
		config.Browser.DefaultLaunchArgs = append([]string{}, defaultConfig.Browser.DefaultLaunchArgs...)
	}
	if config.Browser.DefaultStartURLs == nil {
		config.Browser.DefaultStartURLs = append([]string{}, defaultConfig.Browser.DefaultStartURLs...)
	} else if isLegacyVerificationStartURLs(config.Browser.DefaultStartURLs) {
		config.Browser.DefaultStartURLs = []string{}
	}
	if config.Browser.LightStartEnabled == nil {
		config.Browser.LightStartEnabled = defaultConfig.Browser.LightStartEnabled
	}
	if config.Browser.StartReadyTimeoutMs <= 0 {
		config.Browser.StartReadyTimeoutMs = defaultConfig.Browser.StartReadyTimeoutMs
	}
	if config.Browser.StartStableWindowMs <= 0 {
		config.Browser.StartStableWindowMs = defaultConfig.Browser.StartStableWindowMs
	}
	config.Browser.DefaultConnectorType = NormalizeBrowserConnectorType(config.Browser.DefaultConnectorType)
	if config.Browser.DefaultBookmarks == nil {
		config.Browser.DefaultBookmarks = []BrowserBookmark{}
	}
	if config.Browser.Cores == nil {
		config.Browser.Cores = []BrowserCore{}
	}
	if config.Browser.Proxies == nil {
		config.Browser.Proxies = []BrowserProxy{}
	}
	if config.Browser.Profiles == nil {
		config.Browser.Profiles = []BrowserProfileConfig{}
	}
	if config.ProxyCheck.BridgeStartTimeoutMs <= 0 {
		config.ProxyCheck.BridgeStartTimeoutMs = defaultConfig.ProxyCheck.BridgeStartTimeoutMs
	}
	if strings.TrimSpace(config.ProxyCheck.SpeedTargetID) == "" {
		config.ProxyCheck.SpeedTargetID = defaultConfig.ProxyCheck.SpeedTargetID
	}
	if strings.TrimSpace(config.ProxyCheck.IPHealthTargetID) == "" {
		config.ProxyCheck.IPHealthTargetID = defaultConfig.ProxyCheck.IPHealthTargetID
	}
	if len(config.ProxyCheck.Targets) == 0 {
		config.ProxyCheck.Targets = append([]ProxyCheckTarget{}, defaultConfig.ProxyCheck.Targets...)
	}

	if config.LaunchServer.Port <= 0 {
		config.LaunchServer.Port = defaultConfig.LaunchServer.Port
	}
	config.LaunchServer.Auth.APIKey = strings.TrimSpace(config.LaunchServer.Auth.APIKey)
	if strings.TrimSpace(config.LaunchServer.Auth.Header) == "" {
		config.LaunchServer.Auth.Header = defaultConfig.LaunchServer.Auth.Header
	}

	// 整段缺失时套用默认值，让老配置升级后也能拿到完整的 mcp 段；
	// 否则零值会把 MCP 静默关掉，用户很难察觉。
	if !config.MCP.Enabled && !config.MCP.Stateless && strings.TrimSpace(config.MCP.Path) == "" {
		config.MCP = defaultConfig.MCP
	} else {
		config.MCP.Path = normalizeMCPPath(config.MCP.Path)
	}

	automationUnset := !config.Automation.Enabled &&
		!config.Automation.HeadlessDefault &&
		!config.Automation.KeepRuntimeOnDisable &&
		strings.TrimSpace(config.Automation.InstallPolicy) == "" &&
		strings.TrimSpace(config.Automation.RuntimeVersion) == "" &&
		strings.TrimSpace(config.Automation.ArtifactsDir) == "" &&
		strings.TrimSpace(config.Automation.NodeSource) == "" &&
		strings.TrimSpace(config.Automation.SystemNodePath) == "" &&
		strings.TrimSpace(config.Automation.NodeVersion) == "" &&
		strings.TrimSpace(config.Automation.PlaywrightCoreVersion) == "" &&
		config.Automation.PageSessionIdleMs == 0
	if automationUnset {
		config.Automation = defaultConfig.Automation
	} else {
		if strings.TrimSpace(config.Automation.InstallPolicy) == "" {
			config.Automation.InstallPolicy = defaultConfig.Automation.InstallPolicy
		}
		if config.Automation.PageSessionIdleMs <= 0 {
			config.Automation.PageSessionIdleMs = defaultConfig.Automation.PageSessionIdleMs
		}
		if strings.TrimSpace(config.Automation.NodeVersion) == "" {
			config.Automation.NodeVersion = defaultConfig.Automation.NodeVersion
		}
		if strings.TrimSpace(config.Automation.PlaywrightCoreVersion) == "" {
			config.Automation.PlaywrightCoreVersion = defaultConfig.Automation.PlaywrightCoreVersion
		}
		if strings.TrimSpace(config.Automation.ArtifactsDir) == "" {
			config.Automation.ArtifactsDir = defaultConfig.Automation.ArtifactsDir
		} else {
			config.Automation.ArtifactsDir = strings.TrimSpace(config.Automation.ArtifactsDir)
		}
		config.Automation.NodeSource = normalizeAutomationNodeSource(config.Automation.NodeSource)
		config.Automation.SystemNodePath = strings.TrimSpace(config.Automation.SystemNodePath)
		if strings.TrimSpace(config.Automation.RuntimeVersion) == "" {
			config.Automation.RuntimeVersion = DefaultAutomationRuntimeVersion(
				config.Automation.NodeVersion,
				config.Automation.PlaywrightCoreVersion,
			)
		}
	}
}

func cloneInterceptorConfig(src InterceptorConfig) InterceptorConfig {
	dst := src
	dst.SensitiveFields = append([]string{}, src.SensitiveFields...)
	return dst
}

func isLegacyDefaultLogPath(path string) bool {
	return strings.EqualFold(filepath.ToSlash(strings.TrimSpace(path)), "logs/app.log")
}

func isLegacyVerificationStartURLs(urls []string) bool {
	legacy := []string{"https://ippure.com/", "https://iplark.com/", "https://ping0.cc/"}
	if len(urls) != len(legacy) {
		return false
	}
	for i, url := range urls {
		if !strings.EqualFold(strings.TrimSpace(url), legacy[i]) {
			return false
		}
	}
	return true
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type: "sqlite",
			SQLite: SQLiteConfig{
				Path: "data/app.db",
			},
		},
		App: AppConfig{
			Name: "Ant Browser",
			Window: WindowConfig{
				Width:     1750,
				Height:    1000,
				MinWidth:  1200,
				MinHeight: 700,
			},
		},
		Runtime: RuntimeConfig{
			MaxMemoryMB: 0,
			GCPercent:   100,
		},
		Browser: BrowserConfig{
			UserDataRoot:           "data",
			DefaultFingerprintArgs: defaultFingerprintArgsForOS(goruntime.GOOS),
			DefaultLaunchArgs:      []string{"--disable-sync", "--no-first-run"},
			DefaultStartURLs:       DefaultBrowserStartURLs(),
			LightStartEnabled:      boolPtr(true),
			RestoreLastSession:     false,
			StartReadyTimeoutMs:    3000,
			StartStableWindowMs:    1200,
			DefaultConnectorType:   BrowserConnectorXray,
		},
		ProxyCheck: ProxyCheckConfig{
			BridgeStartTimeoutMs: 15000,
			SpeedTargetID:        "",
			IPHealthTargetID:     "",
			Targets:              []ProxyCheckTarget{},
		},
		Logging: LoggingConfig{
			Level:           "info",
			FileEnabled:     false,
			FilePath:        "data/logs/app.log",
			Format:          "text",
			BufferSize:      4,
			AsyncQueueSize:  1000,
			FlushIntervalMs: 1000,
			Rotation: RotationConfig{
				Enabled:      false,
				MaxSizeMB:    100,
				MaxAge:       7,
				MaxBackups:   5,
				TimeInterval: "daily",
			},
			Interceptor: InterceptorConfig{
				Enabled:         true,
				LogParameters:   true,
				LogResults:      true,
				SensitiveFields: []string{"password", "token", "secret"},
			},
		},
		LaunchServer: LaunchServerConfig{
			Port: DefaultLaunchServerPort,
			Auth: LaunchServerAuthConfig{
				Enabled: false,
				APIKey:  "",
				Header:  DefaultLaunchServerAPIKeyHeader,
			},
		},
		MCP: MCPConfig{
			Enabled:   true,
			Path:      DefaultMCPPath,
			Stateless: false,
		},
		Automation: AutomationConfig{
			Enabled:               false,
			InstallPolicy:         DefaultAutomationInstallPolicy,
			RuntimeVersion:        DefaultAutomationRuntimeVersion(DefaultAutomationNodeVersion, DefaultAutomationPWVersion),
			HeadlessDefault:       false,
			KeepRuntimeOnDisable:  true,
			AllowTypeScriptBuild:  false,
			ArtifactsDir:          "data/automation/artifacts",
			NodeSource:            DefaultAutomationNodeSource,
			SystemNodePath:        "",
			NodeVersion:           DefaultAutomationNodeVersion,
			PlaywrightCoreVersion: DefaultAutomationPWVersion,
			PageSessionIdleMs:     DefaultAutomationPageSessionIdleMs,
		},
	}
}

func defaultFingerprintArgsForOS(goos string) []string {
	platform := "windows"
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		platform = "mac"
	case "linux":
		platform = "linux"
	}
	return []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=" + platform,
		"--disable-non-proxied-udp",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
	}
}

func isLegacyMinimalDefaultFingerprintArgs(args []string) bool {
	if len(args) != 2 {
		return false
	}
	hasBrand := false
	hasPlatform := false
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(trimmed, "--fingerprint-brand=") {
			hasBrand = true
		}
		if strings.HasPrefix(trimmed, "--fingerprint-platform=") {
			hasPlatform = true
		}
	}
	return hasBrand && hasPlatform
}

func appendEffectiveRuntimeFingerprintArgs(args []string) []string {
	defaultRuntimeArgs := []string{
		"--disable-non-proxied-udp",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
	}
	out := append([]string{}, args...)
	for _, defaultArg := range defaultRuntimeArgs {
		if !containsFingerprintArg(out, defaultArg) {
			out = append(out, defaultArg)
		}
	}
	return out
}

func containsFingerprintArg(args []string, expected string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == expected {
			return true
		}
	}
	return false
}
func DefaultAutomationRuntimeVersion(nodeVersion, playwrightVersion string) string {
	return fmt.Sprintf("node-%s-playwright-core-%s", strings.TrimSpace(nodeVersion), strings.TrimSpace(playwrightVersion))
}

func normalizeAutomationNodeSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AutomationNodeSourceSystem:
		return AutomationNodeSourceSystem
	case AutomationNodeSourceBundled:
		return AutomationNodeSourceBundled
	default:
		return AutomationNodeSourceAuto
	}
}

// normalizeMCPPath 保证挂载路径以 / 开头且不以 / 结尾。
//
// 结尾斜杠必须去掉：net/http 的 ServeMux 里 "/mcp/" 是子树模式，会吞掉
// 所有以此为前缀的路径；而 MCP 端点应当是精确匹配的单一路径。
func normalizeMCPPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultMCPPath
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return DefaultMCPPath
	}
	return trimmed
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}
