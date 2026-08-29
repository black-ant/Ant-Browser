package mcpserver

import (
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

// 能力接口按域拆分，方便测试只实现用得到的那部分。
// 这些签名与 launchcode 的服务门面（service_facade.go / service_proxy.go）
// 保持一致，*launchcode.LaunchServer 天然满足，无需适配层。

// InstanceProvider 提供实例与运行时会话能力。
type InstanceProvider interface {
	ListProfiles() ([]browser.Profile, error)
	FindProfiles(selector launchcode.LaunchSelector) ([]browser.Profile, error)
	FindProfile(selector launchcode.LaunchSelector) (*browser.Profile, error)
	CreateProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error)
	UpdateProfile(profileID string, input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error)
	DeleteProfile(profileID string) error
	StartProfile(selector launchcode.LaunchSelector, params launchcode.LaunchRequestParams) (*browser.Profile, string, error)
	StopProfile(profileID string) (*browser.Profile, error)
	StatusProfile(profileID string) (*browser.Profile, error)
	OpenRuntimeSession(selector launchcode.LaunchSelector, params launchcode.LaunchRequestParams, timeout time.Duration) (*launchcode.RuntimeSession, error)
	ActiveRuntimeSession() (*launchcode.RuntimeSession, error)
}

// AutomationProvider 提供自动化脚本能力。
type AutomationProvider interface {
	ListScripts() ([]automation.ScriptRecord, error)
	GetScript(scriptID string) (*automation.ScriptRecord, error)
	RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error)
	ListScriptRuns(limit int) ([]automation.ScriptRunRecord, error)
}

// ProxyProvider 提供代理池与内核能力。
type ProxyProvider interface {
	ListProxies() ([]config.BrowserProxy, error)
	TestProxySpeed(proxyID string) (*launchcode.ProxySpeedResult, error)
	CheckProxyHealth(proxyID string) (*launchcode.ProxyHealthResult, error)
	ListCores() ([]config.BrowserCore, error)
}

// PageProvider 提供 Playwright / CDP 页面操作能力。
type PageProvider interface {
	RunPageSteps(req launchcode.PageRequest) (*launchcode.PageResult, error)
	ClosePageSession(selector launchcode.LaunchSelector) (string, error)
}

// 编译期确认 LaunchServer 满足全部能力接口。
var _ Provider = (*launchcode.LaunchServer)(nil)
