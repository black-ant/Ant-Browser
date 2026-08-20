package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/gateway"
)

const proxyGatewayWorkerArg = "--ant-proxy-gateway-worker"

type ProxyRoutingRule = gateway.Rule
type ProxyRoutingConfig = gateway.RoutingConfig
type ProxyGatewayStatus = gateway.Status

type BrowserProxySwitchResult struct {
	Profile     *BrowserProfile `json:"profile"`
	Gateway     gateway.Status  `json:"gateway"`
	AppliedLive bool            `json:"appliedLive"`
}

type BrowserProxySwitchInput struct {
	ProxyID     string `json:"proxyId"`
	ProxyConfig string `json:"proxyConfig"`
	Force       bool   `json:"force"`
}

type proxyGatewayWorkerLaunchConfig struct {
	AppRoot     string `json:"appRoot"`
	ControlPort int    `json:"controlPort"`
	Token       string `json:"token"`
}

type proxyGatewayProfileRequest struct {
	ProfileID        string                `json:"profileId"`
	ProxyID          string                `json:"proxyId"`
	ProxyConfig      string                `json:"proxyConfig"`
	Proxies          []config.BrowserProxy `json:"proxies"`
	ConnectorType    string                `json:"connectorType"`
	Routing          gateway.RoutingConfig `json:"routing"`
	Force            bool                  `json:"force"`
	BrowserPID       int                   `json:"browserPid"`
	BrowserDebugPort int                   `json:"browserDebugPort"`
}

type proxyGatewayProfileResponse struct {
	Status gateway.Status `json:"status"`
}

type proxyGatewayStatusResponse struct {
	OK       bool             `json:"ok"`
	PID      int              `json:"pid"`
	Profiles []gateway.Status `json:"profiles"`
}
