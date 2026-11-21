package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/config"

	"gopkg.in/yaml.v3"
)

const (
	singBoxPublicDNSTag     = "public-dns"
	singBoxBackupDNSTag     = "backup-dns"
	singBoxProxyResolverTag = "proxy-server-dns"
)

// clashBridgeDNS 是 browser_proxies.dns_servers 的结构化视图。
// 订阅导入时该字段保存 Clash 的 "dns:" YAML 段（含 proxy-server-nameserver），
// 同时兼容历史逗号分隔的 DNS 列表。
type clashBridgeDNS struct {
	Enable                *bool
	IPv6                  *bool
	UseHosts              *bool
	DefaultNameserver     []string
	Nameserver            []string
	Fallback              []string
	ProxyServerNameserver []string
	FallbackFilter        map[string]interface{}
}

func (c clashBridgeDNS) hasServers() bool {
	return len(c.Nameserver) > 0 || len(c.ProxyServerNameserver) > 0 ||
		len(c.DefaultNameserver) > 0 || len(c.Fallback) > 0
}

// BridgeDNSServers 返回可注入桥接配置的 DNS。
// 优先级：全局自定义 DNS（browser.bridge_custom_dns）> 节点自带订阅 DNS > 空（回退系统/公共 DNS）。
func BridgeDNSServers(cfg *config.Config, proxyConfig string, proxies []config.BrowserProxy, proxyId string) string {
	if custom := config.BridgeCustomDNSValue(cfg); custom != "" {
		return custom
	}
	return proxyDNSServersFor(proxyConfig, proxies, proxyId)
}

// proxyDNSServersFor 返回节点绑定的订阅 DNS。
// 先按 proxyId 精确匹配；proxyId 为空时按节点配置文本反查，避免桥接丢失订阅 DNS。
func proxyDNSServersFor(proxyConfig string, proxies []config.BrowserProxy, proxyId string) string {
	proxyId = strings.TrimSpace(proxyId)
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyId) {
				return strings.TrimSpace(item.DnsServers)
			}
		}
	}
	src := strings.TrimSpace(proxyConfig)
	if src == "" {
		return ""
	}
	for _, item := range proxies {
		if strings.TrimSpace(item.ProxyConfig) == src {
			return strings.TrimSpace(item.DnsServers)
		}
	}
	return ""
}

func parseClashBridgeDNS(raw string) clashBridgeDNS {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return clashBridgeDNS{}
	}

	var wrapper struct {
		DNS map[string]interface{} `yaml:"dns"`
	}
	if err := yaml.Unmarshal([]byte(raw), &wrapper); err == nil && len(wrapper.DNS) > 0 {
		cfg := clashBridgeDNS{
			Enable:                boolYAMLValue(wrapper.DNS["enable"]),
			IPv6:                  boolYAMLValue(wrapper.DNS["ipv6"]),
			UseHosts:              boolYAMLValue(wrapper.DNS["use-hosts"]),
			DefaultNameserver:     stringListYAMLValue(wrapper.DNS["default-nameserver"]),
			Nameserver:            stringListYAMLValue(wrapper.DNS["nameserver"]),
			Fallback:              stringListYAMLValue(wrapper.DNS["fallback"]),
			ProxyServerNameserver: stringListYAMLValue(wrapper.DNS["proxy-server-nameserver"]),
		}
		if filter, ok := wrapper.DNS["fallback-filter"].(map[string]interface{}); ok {
			cfg.FallbackFilter = filter
		}
		// dns.enable: false 明确表示不启用，mihomo / sing-box 统一不使用这段配置
		if cfg.Enable != nil && !*cfg.Enable {
			return clashBridgeDNS{}
		}
		if cfg.hasServers() {
			return cfg
		}
	}

	list := splitDNSList(raw)
	if len(list) == 0 {
		return clashBridgeDNS{}
	}
	return clashBridgeDNS{Nameserver: list}
}

func boolYAMLValue(value interface{}) *bool {
	switch typed := value.(type) {
	case bool:
		return &typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func stringListYAMLValue(value interface{}) []string {
	switch typed := value.(type) {
	case string:
		return splitDNSList(typed)
	case []string:
		return trimDNSList(typed)
	case []interface{}:
		list := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				list = append(list, text)
			}
		}
		return trimDNSList(list)
	}
	return nil
}

func splitDNSList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(char rune) bool {
		return char == ',' || char == '\n' || char == '\r'
	})
	return trimDNSList(parts)
}

func trimDNSList(items []string) []string {
	list := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			list = append(list, item)
		}
	}
	return list
}

// buildMihomoDNSConfig 把订阅 DNS 转成 mihomo 的 dns 段。
// 桥接进程只服务单个节点，因此只透传节点域名解析相关字段：
// listen / fake-ip 等字段会带来端口占用与解析行为偏差，不下发。
func buildMihomoDNSConfig(raw string) map[string]interface{} {
	cfg := parseClashBridgeDNS(raw)
	// 过滤无法下发的地址：填错 DNS 时宁可不用，也不能让桥接配置被内核拒绝
	cfg.DefaultNameserver = usableDNSList(cfg.DefaultNameserver)
	cfg.Nameserver = usableDNSList(cfg.Nameserver)
	cfg.Fallback = usableDNSList(cfg.Fallback)
	cfg.ProxyServerNameserver = usableDNSList(cfg.ProxyServerNameserver)
	if !cfg.hasServers() {
		return nil
	}
	dns := map[string]interface{}{
		"enable": true,
		"ipv6":   false,
	}
	if cfg.Enable != nil {
		dns["enable"] = *cfg.Enable
	}
	if cfg.IPv6 != nil {
		dns["ipv6"] = *cfg.IPv6
	}
	if cfg.UseHosts != nil {
		dns["use-hosts"] = *cfg.UseHosts
	}
	if len(cfg.DefaultNameserver) > 0 {
		dns["default-nameserver"] = cfg.DefaultNameserver
	}
	if len(cfg.Nameserver) > 0 {
		dns["nameserver"] = cfg.Nameserver
	}
	if len(cfg.Fallback) > 0 {
		dns["fallback"] = cfg.Fallback
	}
	if len(cfg.ProxyServerNameserver) > 0 {
		dns["proxy-server-nameserver"] = cfg.ProxyServerNameserver
	}
	if len(cfg.FallbackFilter) > 0 {
		dns["fallback-filter"] = cfg.FallbackFilter
	}
	return dns
}

// buildSingBoxDNSConfig 把订阅 DNS 转成 sing-box 的 dns 段。
// 返回的 resolverTag 用于给节点 outbound 指定 domain_resolver，
// 等价于 Clash 的 proxy-server-nameserver：只用订阅 DNS 解析节点域名。
func buildSingBoxDNSConfig(raw string) (map[string]interface{}, string) {
	cfg := parseClashBridgeDNS(raw)
	if !cfg.hasServers() {
		return defaultSingBoxDNSConfig(), ""
	}

	servers := make([]interface{}, 0, 8)
	// 公共 DNS 始终保留：作为 final 兜底，也作为 DoH/DoT 域名的引导解析
	servers = append(servers,
		map[string]interface{}{"type": "udp", "tag": singBoxPublicDNSTag, "server": "223.5.5.5"},
		map[string]interface{}{"type": "udp", "tag": singBoxBackupDNSTag, "server": "119.29.29.29"},
	)

	bootstrapTag := singBoxPublicDNSTag
	if tags := appendSingBoxDNSServers(&servers, cfg.DefaultNameserver, "bootstrap-dns", ""); len(tags) > 0 {
		bootstrapTag = tags[0]
	}
	primaryTags := appendSingBoxDNSServers(&servers, cfg.Nameserver, "subscription-dns", bootstrapTag)
	appendSingBoxDNSServers(&servers, cfg.Fallback, "subscription-fallback", bootstrapTag)
	proxyTags := appendSingBoxDNSServers(&servers, cfg.ProxyServerNameserver, singBoxProxyResolverTag, bootstrapTag)

	resolverTag := ""
	if len(proxyTags) > 0 {
		resolverTag = proxyTags[0]
	} else if len(primaryTags) > 0 {
		resolverTag = primaryTags[0]
	}

	finalTag := singBoxPublicDNSTag
	if len(primaryTags) > 0 {
		finalTag = primaryTags[0]
	}
	strategy := "ipv4_only"
	if cfg.IPv6 != nil && *cfg.IPv6 {
		strategy = "prefer_ipv4"
	}

	return map[string]interface{}{
		"servers":  servers,
		"final":    finalTag,
		"strategy": strategy,
	}, resolverTag
}

func appendSingBoxDNSServers(target *[]interface{}, list []string, prefix string, bootstrapTag string) []string {
	tags := make([]string, 0, len(list))
	for index, raw := range list {
		tag := prefix
		if index > 0 {
			tag = fmt.Sprintf("%s-%d", prefix, index)
		}
		server := convertSingBoxDNSServer(raw, tag, bootstrapTag)
		if server == nil {
			continue
		}
		tags = append(tags, tag)
		*target = append(*target, server)
	}
	return tags
}

func convertSingBoxDNSServer(raw string, tag string, bootstrapTag string) map[string]interface{} {
	value := strings.TrimSpace(raw)
	if value == "" || tag == "" {
		return nil
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "https://"):
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" {
			return nil
		}
		server := map[string]interface{}{
			"type":   "https",
			"tag":    tag,
			"server": parsed.Hostname(),
		}
		if port := parsed.Port(); port != "" && port != "443" {
			if number, convErr := strconv.Atoi(port); convErr == nil {
				server["server_port"] = number
			}
		}
		path := parsed.EscapedPath()
		if strings.TrimSpace(path) == "" || path == "/" {
			path = "/dns-query"
		}
		server["path"] = path
		// DoH 域名本身需要先解析，交给引导 DNS，避免自引用
		if bootstrapTag != "" && !isIPHost(parsed.Hostname()) {
			server["domain_resolver"] = bootstrapTag
		}
		return server
	case strings.HasPrefix(lower, "tls://"):
		host, port := splitDNSHostPort(value[len("tls://"):])
		if host == "" || (!isIPHost(host) && !strings.Contains(host, ".")) {
			return nil
		}
		server := map[string]interface{}{
			"type":   "tls",
			"tag":    tag,
			"server": host,
		}
		if port != "" && port != "853" {
			if number, convErr := strconv.Atoi(port); convErr == nil {
				server["server_port"] = number
			}
		}
		if bootstrapTag != "" && !isIPHost(host) {
			server["domain_resolver"] = bootstrapTag
		}
		return server
	}

	// Clash 允许 "#h3=true" 之类后缀，sing-box 不认，直接丢弃
	if index := strings.IndexByte(value, '#'); index >= 0 {
		value = value[:index]
	}
	if strings.HasPrefix(strings.ToLower(value), "udp://") {
		value = value[len("udp://"):]
	}
	host, port := splitDNSHostPort(value)
	if host == "" || strings.Contains(host, "://") {
		return nil
	}
	if !isIPHost(host) && !strings.Contains(host, ".") {
		return nil
	}
	server := map[string]interface{}{
		"type":   "udp",
		"tag":    tag,
		"server": host,
	}
	if port != "" && port != "53" {
		if number, convErr := strconv.Atoi(port); convErr == nil {
			server["server_port"] = number
		}
	}
	return server
}

// usableDNSList 过滤掉桥接内核无法识别的 DNS 地址，避免错误配置导致桥接启动失败。
func usableDNSList(list []string) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		if convertSingBoxDNSServer(item, "check", "") != nil {
			out = append(out, item)
		}
	}
	return out
}

// HasUsableDNS 判断输入的 DNS 是否至少含一个可下发到桥接内核的地址，用于保存前校验。
func HasUsableDNS(raw string) bool {
	cfg := parseClashBridgeDNS(raw)
	return len(usableDNSList(cfg.DefaultNameserver)) > 0 ||
		len(usableDNSList(cfg.Nameserver)) > 0 ||
		len(usableDNSList(cfg.Fallback)) > 0 ||
		len(usableDNSList(cfg.ProxyServerNameserver)) > 0
}

func splitDNSHostPort(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if strings.HasPrefix(value, "[") {
		if host, port, err := net.SplitHostPort(value); err == nil {
			return host, port
		}
		return strings.Trim(value, "[]"), ""
	}
	if strings.Count(value, ":") == 1 {
		if host, port, err := net.SplitHostPort(value); err == nil {
			return host, port
		}
	}
	return value, ""
}

func isIPHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	return net.ParseIP(host) != nil
}
