package proxy

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type DnsDiagnosticSummary struct {
	HasConfig       bool     `json:"hasConfig"`
	SourceFormat    string   `json:"sourceFormat"`
	EnhancedMode    string   `json:"enhancedMode"`
	NameserverCount int      `json:"nameserverCount"`
	FallbackCount   int      `json:"fallbackCount"`
	XrayServerCount int      `json:"xrayServerCount"`
	Unsupported     []string `json:"unsupported"`
}

// parseDnsConfig 解析 DNS 配置，支持两种格式：
// 1. Clash dns: YAML 块（含 nameserver/fallback 等字段）
// 2. 逗号分隔的 IP 列表（兼容旧格式）
// 返回 xray dns 配置 map，若无有效配置则返回 nil
//
// 注意：xray dns.servers 只支持纯 IP 或 DoH（https://）地址，
// 不支持 Clash 的 tls:// 格式（DoT），会被自动过滤。
func parseDnsConfig(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	type clashDns struct {
		Enable     bool     `yaml:"enable"`
		Nameserver []string `yaml:"nameserver"`
		Fallback   []string `yaml:"fallback"`
	}
	type clashDnsWrapper struct {
		Dns clashDns `yaml:"dns"`
	}

	var wrapper clashDnsWrapper
	if err := yaml.Unmarshal([]byte(raw), &wrapper); err == nil && len(wrapper.Dns.Nameserver) > 0 {
		servers := make([]interface{}, 0)
		for _, s := range wrapper.Dns.Nameserver {
			if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
				servers = append(servers, s)
			}
		}
		for _, s := range wrapper.Dns.Fallback {
			if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
				servers = append(servers, s)
			}
		}
		if len(servers) > 0 {
			return map[string]interface{}{"servers": servers}
		}
	}

	var result []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}

	servers := make([]interface{}, len(result))
	for i, s := range result {
		servers[i] = s
	}
	return map[string]interface{}{"servers": servers}
}

// isXrayDnsAddr 判断 DNS 地址是否为 xray 支持的格式。
// xray 支持：纯 IP（如 8.8.8.8）、IP:port（如 8.8.8.8:53）、
// DoH（https://...）、localhost。
// 不支持：Clash 的 tls:// 格式（DoT）。
func isXrayDnsAddr(s string) bool {
	l := strings.ToLower(s)
	if strings.HasPrefix(l, "tls://") {
		return false
	}
	return true
}

func buildDnsDiagnosticSummary(raw string) DnsDiagnosticSummary {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DnsDiagnosticSummary{}
	}
	summary := DnsDiagnosticSummary{HasConfig: true, SourceFormat: "list"}
	type clashDns struct {
		Enable       bool     `yaml:"enable"`
		EnhancedMode string   `yaml:"enhanced-mode"`
		Nameserver   []string `yaml:"nameserver"`
		Fallback     []string `yaml:"fallback"`
	}
	type clashDnsWrapper struct {
		Dns clashDns `yaml:"dns"`
	}
	var wrapper clashDnsWrapper
	if err := yaml.Unmarshal([]byte(raw), &wrapper); err == nil && (len(wrapper.Dns.Nameserver) > 0 || len(wrapper.Dns.Fallback) > 0 || wrapper.Dns.EnhancedMode != "") {
		summary.SourceFormat = "clash"
		summary.EnhancedMode = strings.TrimSpace(wrapper.Dns.EnhancedMode)
		summary.NameserverCount = len(wrapper.Dns.Nameserver)
		summary.FallbackCount = len(wrapper.Dns.Fallback)
		for _, server := range append(append([]string{}, wrapper.Dns.Nameserver...), wrapper.Dns.Fallback...) {
			server = strings.TrimSpace(server)
			if server == "" {
				continue
			}
			if isXrayDnsAddr(server) {
				summary.XrayServerCount++
			} else {
				summary.Unsupported = append(summary.Unsupported, server)
			}
		}
		return summary
	}
	for _, server := range strings.Split(raw, ",") {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}
		summary.NameserverCount++
		if isXrayDnsAddr(server) {
			summary.XrayServerCount++
		} else {
			summary.Unsupported = append(summary.Unsupported, server)
		}
	}
	return summary
}
