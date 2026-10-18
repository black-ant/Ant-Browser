package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

const proxySpeedContractVersion = 1

type proxySpeedFingerprintInput struct {
	ContractVersion  int                        `json:"contractVersion"`
	ProxyId          string                     `json:"proxyId"`
	ConnectorType    string                     `json:"connectorType"`
	Engine           string                     `json:"engine"`
	TimeoutMs        int64                      `json:"timeoutMs"`
	PrepareTimeoutMs int64                      `json:"prepareTimeoutMs"`
	URLs             []string                   `json:"urls"`
	ExpectedStatus   []int                      `json:"expectedStatus"`
	Proxy            proxySpeedFingerprintProxy `json:"proxy"`
}

type proxySpeedFingerprintProxy struct {
	ProxyConfig     string `json:"proxyConfig"`
	PreferredKernel string `json:"preferredKernel"`
	DnsServers      string `json:"dnsServers"`
}

func proxySpeedTestFingerprint(proxyID string, proxies []BrowserProxy, connectorType string, speedConfig *proxy.SpeedTestConfig) string {
	if speedConfig == nil {
		defaultConfig := proxy.DefaultSpeedTestConfig
		speedConfig = &defaultConfig
	}
	var target BrowserProxy
	for _, item := range proxies {
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), strings.TrimSpace(proxyID)) {
			target = item
			break
		}
	}
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	input := proxySpeedFingerprintInput{
		ContractVersion:  proxySpeedContractVersion,
		ProxyId:          strings.ToLower(strings.TrimSpace(proxyID)),
		ConnectorType:    connectorType,
		Engine:           expectedProxySpeedEngine(proxyID, proxies, connectorType),
		TimeoutMs:        speedConfig.Timeout.Milliseconds(),
		PrepareTimeoutMs: speedConfig.TCPTimeout.Milliseconds(),
		URLs:             append([]string{}, speedConfig.URLs...),
		ExpectedStatus:   append([]int{}, speedConfig.ExpectedStatus...),
		Proxy: proxySpeedFingerprintProxy{
			ProxyConfig:     strings.TrimSpace(target.ProxyConfig),
			PreferredKernel: strings.TrimSpace(target.PreferredKernel),
			DnsServers:      strings.TrimSpace(target.DnsServers),
		},
	}
	for index := range input.URLs {
		input.URLs[index] = strings.TrimSpace(input.URLs[index])
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func expectedProxySpeedEngine(proxyID string, proxies []BrowserProxy, connectorType string) string {
	proxyConfig := ""
	for _, item := range proxies {
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), strings.TrimSpace(proxyID)) {
			proxyConfig = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if strings.EqualFold(proxyConfig, "direct://") {
		return "direct"
	}
	resolution, err := proxy.ResolveProxyKernelForConnector(proxyConfig, proxies, proxyID, connectorType)
	if err != nil {
		if resolution.Kernel != "" {
			return resolution.Kernel
		}
		return config.NormalizeBrowserConnectorType(connectorType)
	}
	if resolution.Kernel == proxy.ProxyKernelNative {
		return "native"
	}
	return resolution.Kernel
}
