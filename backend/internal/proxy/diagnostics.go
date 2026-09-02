package proxy

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// BuildDiagnosticOptions 控制代理诊断构建行为。
type BuildDiagnosticOptions struct {
	XrayMgr       *XrayManager
	SingBoxMgr    *SingBoxManager
	ClashMgr      *ClashManager
	ConnectorType string
}

// ProxyBuildDiagnostic 是不启动桥接进程的代理构建诊断结果。
type ProxyBuildDiagnostic struct {
	ProxyId         string                 `json:"proxyId"`
	ProxyName       string                 `json:"proxyName"`
	Found           bool                   `json:"found"`
	Ok              bool                   `json:"ok"`
	Engine          string                 `json:"engine"`
	NodeKey         string                 `json:"nodeKey"`
	RawConfigMasked string                 `json:"rawConfigMasked"`
	DnsServers      string                 `json:"dnsServers"`
	DnsSummary      DnsDiagnosticSummary   `json:"dnsSummary"`
	StandardProxy   string                 `json:"standardProxy"`
	Outbounds       []interface{}          `json:"outbounds"`
	Routes          []interface{}          `json:"routes"`
	Inbound         map[string]interface{} `json:"inbound"`
	Outbound        map[string]interface{} `json:"outbound"`
	Runtime         ProxyRuntimeDiagnostic `json:"runtime"`
	Errors          []string               `json:"errors"`
}

// ProxyRuntimeDiagnostic 描述桥接运行时文件位置和最近日志。
type ProxyRuntimeDiagnostic struct {
	WorkDir     string            `json:"workDir"`
	ConfigPath  string            `json:"configPath"`
	StderrPath  string            `json:"stderrPath"`
	LogPath     string            `json:"logPath"`
	ErrorPath   string            `json:"errorPath"`
	RecentLogs  map[string]string `json:"recentLogs"`
	BridgeAlive bool              `json:"bridgeAlive"`
	BridgePort  int               `json:"bridgePort"`
	LastError   string            `json:"lastError"`
}

// BuildProxyDiagnostic 复用真实解析路径生成诊断信息，但不会启动或写入桥接进程。
func BuildProxyDiagnostic(proxyConfig string, proxies []config.BrowserProxy, proxyId string, options BuildDiagnosticOptions) ProxyBuildDiagnostic {
	proxyId = strings.TrimSpace(proxyId)
	item, found := findProxyForDiagnostic(proxies, proxyId)
	src := strings.TrimSpace(proxyConfig)
	if proxyId != "" && found {
		src = strings.TrimSpace(item.ProxyConfig)
	}

	result := ProxyBuildDiagnostic{
		ProxyId:         proxyId,
		Found:           found || proxyId == "",
		RawConfigMasked: maskProxyConfig(src),
		DnsServers:      item.DnsServers,
		DnsSummary:      buildDnsDiagnosticSummary(item.DnsServers),
	}
	if found {
		result.ProxyName = item.ProxyName
	}
	if proxyId != "" && !found && src == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("代理池节点已不存在: %s", proxyId))
		return result
	}
	if src == "" {
		result.Engine = "empty"
		result.Ok = true
		return result
	}
	if strings.EqualFold(src, "direct://") {
		result.Engine = "direct"
		result.Ok = true
		return result
	}

	src = normalizeNodeScheme(src)
	result.RawConfigMasked = maskProxyConfig(src)
	connectorType := config.NormalizeBrowserConnectorType(options.ConnectorType)
	resolution, err := ResolveProxyKernelForConnector(src, proxies, proxyId, connectorType)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	switch resolution.Kernel {
	case ProxyKernelMihomo:
		buildMihomoDiagnostic(src, options.ClashMgr, &result)
		return result
	case ProxyKernelSingBox:
		buildSingBoxDiagnostic(src, options.SingBoxMgr, &result)
		return result
	case ProxyKernelXray:
		buildXrayDiagnostic(src, proxies, proxyId, options.XrayMgr, &result)
		return result
	case ProxyKernelNative:
		result.Engine = "native"
		result.Ok = true
		result.StandardProxy = maskProxyConfig(src)
		return result
	}
	result.Errors = append(result.Errors, fmt.Sprintf("无法为协议 %s 选择代理内核", resolution.Protocol))
	return result
}

func buildMihomoDiagnostic(src string, manager *ClashManager, result *ProxyBuildDiagnostic) {
	result.Engine = "mihomo"
	node, err := buildMihomoNode(src)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	result.Ok = true
	result.NodeKey = computeNodeKey(src + "\x00mihomo")
	result.Outbound = sanitizeDiagnosticMap(node)
	if manager != nil {
		workDir := manager.resolveMihomoWorkdir(result.NodeKey)
		result.Runtime = buildRuntimeDiagnostic(workDir, "mihomo-config.yaml", "mihomo-stderr.log", "", "")
		fillMihomoBridgeState(manager, result.NodeKey, &result.Runtime)
	}
}

func buildSingBoxDiagnostic(src string, manager *SingBoxManager, result *ProxyBuildDiagnostic) {
	result.Engine = "sing-box"
	outbound, err := BuildSingBoxOutbound(src)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	result.Ok = true
	result.NodeKey = computeNodeKey(src)
	result.Outbound = sanitizeDiagnosticMap(outbound)
	if manager != nil {
		workDir := manager.resolveWorkdir(result.NodeKey)
		result.Runtime = buildRuntimeDiagnostic(workDir, "singbox-config.json", "singbox-stderr.log", "singbox.log", "")
		fillSingBoxBridgeState(manager, result.NodeKey, &result.Runtime)
	}
}

func buildXrayDiagnostic(src string, proxies []config.BrowserProxy, proxyId string, manager *XrayManager, result *ProxyBuildDiagnostic) {
	result.Engine = "xray"
	var preferredKeySource string
	if IsChainSocks5Proxy(src) {
		chainCfg, err := ParseChainSocks5Config(src)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return
		}
		result.Outbounds = []interface{}{
			sanitizeDiagnosticMap(chainSocks5Outbound(chainCfg.First, "first-hop", "")),
			sanitizeDiagnosticMap(chainSocks5Outbound(chainCfg.Second, "second-hop", "first-hop")),
		}
		result.Routes = []interface{}{map[string]interface{}{"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": "second-hop"}}
	} else if outbound, shouldBridge, err := buildDirectProxyBridgeOutbound(src); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	} else if shouldBridge {
		result.Outbounds = []interface{}{sanitizeDiagnosticMap(outbound)}
		result.Routes = []interface{}{map[string]interface{}{"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": "proxy-out"}}
	} else {
		standardProxy, outbound, err := ParseProxyNode(src)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return
		}
		if standardProxy != "" {
			result.Engine = "none"
			result.StandardProxy = maskProxyConfig(standardProxy)
			result.Ok = true
			return
		}
		if outbound == nil {
			result.Errors = append(result.Errors, "节点解析失败")
			return
		}
		result.Outbounds = []interface{}{sanitizeDiagnosticMap(outbound)}
		result.Routes = []interface{}{map[string]interface{}{"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": "proxy-out"}}
	}

	result.Ok = true
	result.Inbound = buildXrayDiagnosticInbound()
	preferredKeySource = src + "\x00" + dnsServersForDiagnostic(proxies, proxyId)
	result.NodeKey = computeNodeKey(preferredKeySource)
	if manager != nil {
		workDir := manager.resolveWorkdir(result.NodeKey)
		result.Runtime = buildRuntimeDiagnostic(workDir, "xray-config.json", "xray-stderr.log", "", "xray-error.log")
		fillXrayBridgeState(manager, result.NodeKey, &result.Runtime)
	}
}

func buildXrayDiagnosticInbound() map[string]interface{} {
	return map[string]interface{}{
		"tag":      "socks-in",
		"listen":   "127.0.0.1",
		"protocol": "socks",
		"settings": map[string]interface{}{
			"udp": true,
		},
		"sniffing": xrayBrowserSniffingConfig(),
	}
}

func buildRuntimeDiagnostic(workDir string, configName string, stderrName string, logName string, errorName string) ProxyRuntimeDiagnostic {
	runtime := ProxyRuntimeDiagnostic{WorkDir: workDir, RecentLogs: map[string]string{}}
	if workDir == "" {
		return runtime
	}
	runtime.ConfigPath = filepath.Join(workDir, configName)
	runtime.StderrPath = filepath.Join(workDir, stderrName)
	if logName != "" {
		runtime.LogPath = filepath.Join(workDir, logName)
	}
	if errorName != "" {
		runtime.ErrorPath = filepath.Join(workDir, errorName)
	}
	if tail := readLogTail(runtime.StderrPath, 2000); tail != "" {
		runtime.RecentLogs["stderr"] = tail
	}
	if tail := readLogTail(runtime.LogPath, 2000); tail != "" {
		runtime.RecentLogs["log"] = tail
	}
	if tail := readLogTail(runtime.ErrorPath, 2000); tail != "" {
		runtime.RecentLogs["error"] = tail
	}
	return runtime
}

func fillXrayBridgeState(manager *XrayManager, key string, runtime *ProxyRuntimeDiagnostic) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	bridge := manager.Bridges[key]
	if bridge == nil {
		return
	}
	runtime.BridgeAlive = bridge.Running && !bridge.Stopping
	runtime.BridgePort = bridge.Port
	runtime.LastError = bridge.LastError
}

func fillSingBoxBridgeState(manager *SingBoxManager, key string, runtime *ProxyRuntimeDiagnostic) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	bridge := manager.Bridges[key]
	if bridge == nil {
		return
	}
	runtime.BridgeAlive = bridge.Running && !bridge.Stopping
	runtime.BridgePort = bridge.Port
	runtime.LastError = bridge.LastError
}

func fillMihomoBridgeState(manager *ClashManager, key string, runtime *ProxyRuntimeDiagnostic) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	bridge := manager.NodeBridges[key]
	if bridge == nil {
		return
	}
	runtime.BridgeAlive = bridge.Running
	runtime.BridgePort = bridge.Port
	if bridge.ExitErr != nil {
		runtime.LastError = bridge.ExitErr.Error()
	}
}

func findProxyForDiagnostic(proxies []config.BrowserProxy, proxyId string) (config.BrowserProxy, bool) {
	if strings.TrimSpace(proxyId) == "" {
		return config.BrowserProxy{}, false
	}
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			return item, true
		}
	}
	return config.BrowserProxy{}, false
}

func dnsServersForDiagnostic(proxies []config.BrowserProxy, proxyId string) string {
	item, ok := findProxyForDiagnostic(proxies, proxyId)
	if !ok {
		return ""
	}
	return item.DnsServers
}

func sanitizeDiagnosticMap(src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(src))
	for key, value := range src {
		if isSensitiveDiagnosticKey(key) {
			out[key] = "***"
			continue
		}
		out[key] = sanitizeDiagnosticValue(value)
	}
	return out
}

func sanitizeDiagnosticValue(value interface{}) interface{} {
	switch item := value.(type) {
	case map[string]interface{}:
		return sanitizeDiagnosticMap(item)
	case []interface{}:
		out := make([]interface{}, len(item))
		for i, value := range item {
			out[i] = sanitizeDiagnosticValue(value)
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, len(item))
		for i, value := range item {
			out[i] = sanitizeDiagnosticMap(value)
		}
		return out
	default:
		return value
	}
}

func isSensitiveDiagnosticKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "password") || strings.Contains(key, "passwd") || strings.Contains(key, "private") || strings.Contains(key, "token") || strings.Contains(key, "auth")
}

func maskProxyConfig(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	parsed, err := url.Parse(src)
	if err != nil || parsed.Scheme == "" {
		return src
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			username = "***"
		}
		parsed.User = url.UserPassword(username, "***")
	}
	query := parsed.Query()
	for key := range query {
		if isSensitiveDiagnosticKey(key) {
			query.Set(key, "***")
		}
	}
	parsed.RawQuery = query.Encode()
	return strings.ReplaceAll(parsed.String(), "%2A%2A%2A", "***")
}
