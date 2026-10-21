package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

const proxySpeedContractVersion = 3

type proxySpeedFingerprintInput struct {
	ContractVersion  int                         `json:"contractVersion"`
	ProxyId          string                      `json:"proxyId"`
	ConnectorType    string                      `json:"connectorType"`
	Engine           string                      `json:"engine"`
	TargetID         string                      `json:"targetId"`
	TimeoutMs        int64                       `json:"timeoutMs"`
	PrepareTimeoutMs int64                       `json:"prepareTimeoutMs"`
	URLs             []string                    `json:"urls"`
	ExpectedStatus   []int                       `json:"expectedStatus"`
	Proxies          []proxySpeedFingerprintNode `json:"proxies"`
}

type proxySpeedFingerprintNode struct {
	ProxyId         string `json:"proxyId"`
	ProxyConfig     string `json:"proxyConfig"`
	PreferredKernel string `json:"preferredKernel"`
	DnsServers      string `json:"dnsServers"`
	GroupName       string `json:"groupName"`
}

func proxySpeedTestFingerprint(proxyID string, proxies []BrowserProxy, connectorType string, speedConfig *proxy.SpeedTestConfig) string {
	if speedConfig == nil {
		defaultConfig := proxy.DefaultSpeedTestConfig
		speedConfig = &defaultConfig
	}
	urls := make([]string, 0, len(speedConfig.URLs))
	seenURLs := make(map[string]struct{}, len(speedConfig.URLs))
	for _, targetURL := range speedConfig.URLs {
		targetURL = strings.TrimSpace(targetURL)
		if targetURL == "" {
			continue
		}
		if _, exists := seenURLs[targetURL]; exists {
			continue
		}
		seenURLs[targetURL] = struct{}{}
		urls = append(urls, targetURL)
	}
	if len(urls) == 0 {
		urls = []string{proxy.DefaultSpeedTestURL}
	}

	nodes := make([]proxySpeedFingerprintNode, 0, len(proxies))
	for _, item := range proxies {
		nodes = append(nodes, proxySpeedFingerprintNode{
			ProxyId:         strings.TrimSpace(item.ProxyId),
			ProxyConfig:     strings.TrimSpace(item.ProxyConfig),
			PreferredKernel: strings.TrimSpace(item.PreferredKernel),
			DnsServers:      strings.TrimSpace(item.DnsServers),
			GroupName:       strings.TrimSpace(item.GroupName),
		})
	}
	sort.Slice(nodes, func(left int, right int) bool {
		return strings.ToLower(nodes[left].ProxyId) < strings.ToLower(nodes[right].ProxyId)
	})

	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	input := proxySpeedFingerprintInput{
		ContractVersion:  proxySpeedContractVersion,
		ProxyId:          strings.ToLower(strings.TrimSpace(proxyID)),
		ConnectorType:    connectorType,
		Engine:           expectedProxySpeedEngine(proxyID, proxies, connectorType),
		TargetID:         strings.TrimSpace(speedConfig.TargetID),
		TimeoutMs:        speedConfig.Timeout.Milliseconds(),
		PrepareTimeoutMs: speedConfig.TCPTimeout.Milliseconds(),
		URLs:             urls,
		ExpectedStatus:   append([]int{}, speedConfig.ExpectedStatus...),
		Proxies:          nodes,
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
