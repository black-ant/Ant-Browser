package backend

import (
	"ant-chrome/backend/internal/launchcode"
)

// 代理池与内核的 LaunchServer 适配层。
//
// 这些方法有意与 Wails 绑定方法（BrowserProxyList 等）分开命名：Wails 会把
// *App 上所有导出方法都绑定给前端，如果直接复用接口名会让前端多出一批
// 语义重复的绑定。这里保持薄适配，不承载业务逻辑。

// ListProxyNodes 实现 launchcode.ProxyLister 接口。
func (a *App) ListProxyNodes() []BrowserProxy {
	if a.browserMgr == nil || a.config == nil {
		return []BrowserProxy{}
	}
	return a.BrowserProxyList()
}

// TestProxyNodeSpeed 实现 launchcode.ProxySpeedTester 接口。
func (a *App) TestProxyNodeSpeed(proxyID string) launchcode.ProxySpeedResult {
	result := a.BrowserProxyTestSpeed(proxyID)
	return launchcode.ProxySpeedResult{
		ProxyID:   result.ProxyId,
		Ok:        result.Ok,
		LatencyMs: result.LatencyMs,
		Engine:    result.Engine,
		Error:     result.Error,
	}
}

// CheckProxyNodeHealth 实现 launchcode.ProxyHealthChecker 接口。
// RawData 有意不透传：它是各 IP 库的原始响应，字段不稳定且体积大，
// 对外部工具没有稳定契约价值。
func (a *App) CheckProxyNodeHealth(proxyID string) launchcode.ProxyHealthResult {
	result := a.BrowserProxyCheckIPHealth(proxyID)
	return launchcode.ProxyHealthResult{
		ProxyID:        result.ProxyId,
		Ok:             result.Ok,
		Source:         result.Source,
		Error:          result.Error,
		IP:             result.IP,
		FraudScore:     result.FraudScore,
		IsResidential:  result.IsResidential,
		Country:        result.Country,
		Region:         result.Region,
		City:           result.City,
		AsOrganization: result.AsOrganization,
		UpdatedAt:      result.UpdatedAt,
	}
}

// ListBrowserCores 实现 launchcode.CoreLister 接口。
func (a *App) ListBrowserCores() []BrowserCore {
	if a.browserMgr == nil {
		return []BrowserCore{}
	}
	return a.BrowserCoreList()
}

var _ launchcode.ProxyLister = (*App)(nil)
var _ launchcode.ProxySpeedTester = (*App)(nil)
var _ launchcode.ProxyHealthChecker = (*App)(nil)
var _ launchcode.CoreLister = (*App)(nil)
