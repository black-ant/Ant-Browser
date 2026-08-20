package backend

import (
	"ant-chrome/backend/internal/gateway"
	"ant-chrome/backend/internal/launchcode"
	"fmt"
)

// launchProxyGatewayController adapts the App/Wails proxy methods to the
// narrow public LaunchServer API. Keeping this adapter in backend avoids a
// dependency from launchcode back into the application package.
type launchProxyGatewayController struct {
	app *App
}

func (c launchProxyGatewayController) SwitchProxy(profileID, proxyID, proxyConfig string, force bool) (launchcode.ProxyGatewaySwitchResult, error) {
	result, err := c.app.BrowserProfileSwitchProxy(profileID, BrowserProxySwitchInput{ProxyID: proxyID, ProxyConfig: proxyConfig, Force: force})
	if err != nil {
		return launchcode.ProxyGatewaySwitchResult{}, err
	}
	return launchcode.ProxyGatewaySwitchResult{
		Gateway:     result.Gateway,
		AppliedLive: result.AppliedLive,
	}, nil
}

func (c launchProxyGatewayController) GetRouting(profileID string) (gateway.RoutingConfig, error) {
	if c.app.profileForGatewayUpdate(profileID) == nil {
		return gateway.RoutingConfig{}, fmt.Errorf("profile not found")
	}
	return c.app.BrowserProxyRoutingGet(profileID), nil
}

func (c launchProxyGatewayController) SaveRouting(profileID string, routing gateway.RoutingConfig, force bool) (gateway.Status, error) {
	result, err := c.app.BrowserProxyRoutingSave(profileID, routing, force)
	if err != nil {
		return gateway.Status{}, err
	}
	if result == nil {
		return gateway.Status{ProfileID: profileID, Mode: routing.Mode}, nil
	}
	return *result, nil
}

func (c launchProxyGatewayController) Status(profileID string) (gateway.Status, error) {
	if c.app.profileForGatewayUpdate(profileID) == nil {
		return gateway.Status{}, fmt.Errorf("profile not found")
	}
	result, err := c.app.BrowserProxyGatewayStatus(profileID)
	if err != nil {
		return gateway.Status{}, err
	}
	if result == nil {
		return gateway.Status{ProfileID: profileID}, nil
	}
	return *result, nil
}

func (a *App) newLaunchProxyGatewayController() launchcode.ProxyGatewayController {
	if a == nil {
		return nil
	}
	return launchProxyGatewayController{app: a}
}
