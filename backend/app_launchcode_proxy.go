package backend

import (
	"ant-chrome/backend/internal/launchcode"
)

// 代理池与内核能力向 LaunchServer 的适配。
//
// 这里刻意用一个独立的小类型而不是把方法挂到 *App：Wails 会把 *App 上所有
// 导出方法都绑定给前端，挂上去会让前端凭空多出一批与现有 BrowserProxyList
// 等语义重复的绑定。适配器只做字段搬运，不承载业务逻辑。

type appProxyProvider struct {
	app *App
}

func newAppProxyProvider(app *App) launchcode.ProxyProvider {
	return appProxyProvider{app: app}
}

func (p appProxyProvider) ListProxyNodes() []BrowserProxy {
	if p.app == nil || p.app.browserMgr == nil || p.app.config == nil {
		return []BrowserProxy{}
	}
	return p.app.BrowserProxyList()
}

func (p appProxyProvider) TestProxyNodeSpeed(proxyID string) launchcode.ProxySpeedResult {
	if p.app == nil {
		return launchcode.ProxySpeedResult{ProxyID: proxyID}
	}
	result := p.app.BrowserProxyTestSpeed(proxyID)
	return launchcode.ProxySpeedResult{
		ProxyID:   result.ProxyId,
		Ok:        result.Ok,
		LatencyMs: result.LatencyMs,
		Engine:    result.Engine,
		Error:     result.Error,
	}
}

// CheckProxyNodeHealth 不透传 RawData：那是各 IP 库的原始响应，
// 字段不稳定且体积大，对外没有稳定契约价值。
func (p appProxyProvider) CheckProxyNodeHealth(proxyID string) launchcode.ProxyHealthResult {
	if p.app == nil {
		return launchcode.ProxyHealthResult{ProxyID: proxyID}
	}
	result := p.app.BrowserProxyCheckIPHealth(proxyID)
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

func (p appProxyProvider) ListBrowserCores() []BrowserCore {
	if p.app == nil || p.app.browserMgr == nil {
		return []BrowserCore{}
	}
	return p.app.BrowserCoreList()
}

var _ launchcode.ProxyProvider = appProxyProvider{}
