package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type clashSubscriptionDNS struct {
	Enable                *bool    `yaml:"enable"`
	IPv6                  *bool    `yaml:"ipv6"`
	PreferH3              *bool    `yaml:"prefer-h3"`
	EnhancedMode          string   `yaml:"enhanced-mode"`
	DefaultNameserver     []string `yaml:"default-nameserver"`
	Nameserver            []string `yaml:"nameserver"`
	ProxyServerNameserver []string `yaml:"proxy-server-nameserver"`
	DirectNameserver      []string `yaml:"direct-nameserver"`
	Fallback              []string `yaml:"fallback"`
}

type clashSubscriptionDNSWrapper struct {
	DNS *clashSubscriptionDNS `yaml:"dns"`
}

func parseSubscriptionDNS(raw string) *clashSubscriptionDNS {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var wrapper clashSubscriptionDNSWrapper
	if err := yaml.Unmarshal([]byte(raw), &wrapper); err == nil && wrapper.DNS != nil && hasAnyClashDNSField(wrapper.DNS) {
		if wrapper.DNS.Enable != nil && !*wrapper.DNS.Enable {
			return nil
		}
		return wrapper.DNS
	}

	var direct clashSubscriptionDNS
	if err := yaml.Unmarshal([]byte(raw), &direct); err == nil && hasAnyClashDNSField(&direct) {
		if direct.Enable != nil && !*direct.Enable {
			return nil
		}
		return &direct
	}

	var yamlList []string
	if err := yaml.Unmarshal([]byte(raw), &yamlList); err == nil && len(yamlList) > 0 {
		cleaned := make([]string, 0, len(yamlList))
		for _, entry := range yamlList {
			if trimmed := strings.TrimSpace(entry); trimmed != "" && !strings.ContainsAny(trimmed, " \t") {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			return &clashSubscriptionDNS{
				Nameserver:            cleaned,
				ProxyServerNameserver: cleaned,
			}
		}
	}

	// Fallback: comma or newline separated server list.
	items := splitFlatDNSList(raw)
	if len(items) == 0 {
		return nil
	}
	return &clashSubscriptionDNS{
		Nameserver:            items,
		ProxyServerNameserver: items,
	}
}

func hasAnyClashDNSField(cfg *clashSubscriptionDNS) bool {
	if cfg == nil {
		return false
	}
	return cfg.Enable != nil ||
		cfg.IPv6 != nil ||
		cfg.PreferH3 != nil ||
		strings.TrimSpace(cfg.EnhancedMode) != "" ||
		len(cfg.DefaultNameserver) > 0 ||
		len(cfg.Nameserver) > 0 ||
		len(cfg.ProxyServerNameserver) > 0 ||
		len(cfg.DirectNameserver) > 0 ||
		len(cfg.Fallback) > 0
}

func splitFlatDNSList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" || strings.Contains(trimmed, " ") || strings.HasSuffix(trimmed, ":") {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// buildMihomoDNSConfig constructs a safe, self-contained Mihomo `dns` section
// for a single-node bridge without binding host DNS ports (`listen`) or
// triggering external GeoIP/GeoSite downloads (`nameserver-policy`, `fallback-filter`).
func buildMihomoDNSConfig(raw string) map[string]interface{} {
	parsed := parseSubscriptionDNS(raw)
	if parsed == nil {
		return nil
	}

	defaultNS := filterMihomoIPBootstrapServers(parsed.DefaultNameserver)
	proxyNS := sanitizeMihomoDNSList(parsed.ProxyServerNameserver, false)
	directNS := sanitizeMihomoDNSList(parsed.DirectNameserver, false)
	nameserver := sanitizeMihomoDNSList(parsed.Nameserver, true)
	fallback := sanitizeMihomoDNSList(parsed.Fallback, true)

	if len(defaultNS) == 0 && len(proxyNS) == 0 && len(directNS) == 0 && len(nameserver) == 0 && len(fallback) == 0 {
		return nil
	}

	if len(nameserver) == 0 {
		switch {
		case len(proxyNS) > 0:
			nameserver = append([]string{}, proxyNS...)
		case len(defaultNS) > 0:
			nameserver = append([]string{}, defaultNS...)
		default:
			nameserver = []string{"223.5.5.5", "119.29.29.29"}
		}
	}

	if len(defaultNS) == 0 && (listHasDomainBasedDNS(proxyNS) || listHasDomainBasedDNS(directNS) || listHasDomainBasedDNS(nameserver) || listHasDomainBasedDNS(fallback)) {
		defaultNS = []string{"223.5.5.5", "119.29.29.29"}
	}

	ipv6 := false
	if parsed.IPv6 != nil {
		ipv6 = *parsed.IPv6
	}

	dnsMap := map[string]interface{}{
		"enable":     true,
		"ipv6":       ipv6,
		"nameserver": nameserver,
	}
	if parsed.PreferH3 != nil && *parsed.PreferH3 {
		dnsMap["prefer-h3"] = true
	}
	if len(defaultNS) > 0 {
		dnsMap["default-nameserver"] = defaultNS
	}
	if len(proxyNS) > 0 {
		dnsMap["proxy-server-nameserver"] = proxyNS
	}
	if len(directNS) > 0 {
		dnsMap["direct-nameserver"] = directNS
	}
	if len(fallback) > 0 {
		dnsMap["fallback"] = fallback
	}
	return dnsMap
}

func sanitizeMihomoDNSList(items []string, allowProxyOut bool) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		cleaned := sanitizeMihomoDNSEntry(item, allowProxyOut)
		if cleaned == "" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func sanitizeMihomoDNSEntry(raw string, allowProxyOut bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	hashIdx := strings.IndexByte(raw, '#')
	if hashIdx < 0 {
		return raw
	}
	base := strings.TrimSpace(raw[:hashIdx])
	frag := strings.TrimSpace(raw[hashIdx+1:])
	if base == "" {
		return ""
	}
	if frag == "" {
		return base
	}

	parts := strings.Split(frag, "&")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kept = append(kept, part)
			continue
		}
		if strings.EqualFold(part, "DIRECT") {
			kept = append(kept, "DIRECT")
			continue
		}
		if allowProxyOut {
			kept = append(kept, "proxy-out")
		}
	}
	if len(kept) == 0 {
		return base
	}
	return base + "#" + strings.Join(kept, "&")
}

func filterMihomoIPBootstrapServers(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		cleaned := sanitizeMihomoDNSEntry(item, false)
		if cleaned == "" {
			continue
		}
		host, _, scheme, ok := parseRawDNSHostPortScheme(cleaned)
		if !ok || net.ParseIP(host) == nil {
			continue
		}
		if scheme != "" && scheme != "udp" && scheme != "tcp" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func listHasDomainBasedDNS(items []string) bool {
	for _, item := range items {
		base := item
		if idx := strings.IndexByte(base, '#'); idx >= 0 {
			base = base[:idx]
		}
		base = strings.TrimSpace(base)
		if base == "" || strings.EqualFold(base, "system") || strings.HasPrefix(strings.ToLower(base), "dhcp://") || strings.HasPrefix(strings.ToLower(base), "rcode://") {
			continue
		}
		host, _, _, ok := parseRawDNSHostPortScheme(base)
		if ok && host != "" && net.ParseIP(host) == nil {
			return true
		}
	}
	return false
}

// buildSingBoxDNSConfig converts saved subscription DNS settings into sing-box 1.12+
// typed DNS servers and returns (dnsConfig, defaultDomainResolverTag).
func buildSingBoxDNSConfig(raw string) (map[string]interface{}, string) {
	parsed := parseSubscriptionDNS(raw)
	if parsed == nil {
		return defaultSingBoxDNSConfig(), "public-dns"
	}

	bootstrapServer, bootstrapPort := pickSingBoxIPv4Bootstrap(parsed.DefaultNameserver)
	publicEntry := map[string]interface{}{
		"type":   "udp",
		"tag":    "public-dns",
		"server": bootstrapServer,
	}
	if bootstrapPort > 0 && bootstrapPort != 53 {
		publicEntry["server_port"] = bootstrapPort
	}

	backupServer := "119.29.29.29"
	if bootstrapServer == "119.29.29.29" && (bootstrapPort == 0 || bootstrapPort == 53) {
		backupServer = "223.5.5.5"
	}
	backupEntry := map[string]interface{}{
		"type":   "udp",
		"tag":    "backup-dns",
		"server": backupServer,
	}
	servers := []interface{}{
		publicEntry,
		backupEntry,
	}

	preferH3 := parsed.PreferH3 != nil && *parsed.PreferH3
	addedFingerprints := map[string]string{
		singBoxServerFingerprint(backupEntry): "backup-dns",
		singBoxServerFingerprint(publicEntry): "public-dns",
	}

	appendGroup := func(entries []string, prefix string) []string {
		tags := make([]string, 0, len(entries))
		seq := 0
		for _, rawItem := range entries {
			seq++
			tag := prefix
			if seq > 1 {
				tag = fmt.Sprintf("%s-%d", prefix, seq)
			}
			srv := buildSingBoxDNSServerEntry(rawItem, tag, "public-dns", preferH3)
			if srv == nil {
				seq--
				continue
			}
			fp := singBoxServerFingerprint(srv)
			if existingTag, exists := addedFingerprints[fp]; exists {
				tags = append(tags, existingTag)
				seq--
				continue
			}
			addedFingerprints[fp] = tag
			servers = append(servers, srv)
			tags = append(tags, tag)
		}
		return tags
	}

	proxyTags := appendGroup(parsed.ProxyServerNameserver, "proxy-server-dns")
	nameserverTags := appendGroup(parsed.Nameserver, "custom-dns")

	defaultResolverTag := "public-dns"
	switch {
	case len(proxyTags) > 0:
		defaultResolverTag = proxyTags[0]
	case len(nameserverTags) > 0:
		defaultResolverTag = nameserverTags[0]
	}

	finalTag := "public-dns"
	switch {
	case len(nameserverTags) > 0:
		finalTag = nameserverTags[0]
	case len(proxyTags) > 0:
		finalTag = proxyTags[0]
	}

	strategy := "ipv4_only"
	if parsed.IPv6 != nil && *parsed.IPv6 {
		strategy = "prefer_ipv4"
	}

	return map[string]interface{}{
		"servers":  servers,
		"final":    finalTag,
		"strategy": strategy,
	}, defaultResolverTag
}

func pickSingBoxIPv4Bootstrap(items []string) (string, int) {
	for _, item := range items {
		base := item
		if idx := strings.IndexByte(base, '#'); idx >= 0 {
			base = base[:idx]
		}
		host, port, scheme, ok := parseRawDNSHostPortScheme(strings.TrimSpace(base))
		if !ok || (scheme != "" && scheme != "udp") {
			continue
		}
		ip := net.ParseIP(host)
		if ip != nil && ip.To4() != nil {
			return ip.String(), port
		}
	}
	return "223.5.5.5", 0
}

func buildSingBoxDNSServerEntry(raw string, tag string, bootstrapTag string, globalPreferH3 bool) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	base := raw
	frag := ""
	if idx := strings.IndexByte(raw, '#'); idx >= 0 {
		base = strings.TrimSpace(raw[:idx])
		frag = strings.TrimSpace(raw[idx+1:])
	}
	if base == "" {
		return nil
	}
	if strings.EqualFold(base, "system") {
		return map[string]interface{}{
			"type": "local",
			"tag":  tag,
		}
	}

	h3Param := globalPreferH3
	if frag != "" {
		for _, part := range strings.Split(frag, "&") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), "h3") && strings.EqualFold(strings.TrimSpace(kv[1]), "true") {
				h3Param = true
			}
		}
	}

	if !strings.Contains(base, "://") {
		host, port, _, ok := parseRawDNSHostPortScheme(base)
		if !ok || host == "" {
			return nil
		}
		entry := map[string]interface{}{
			"type":   "udp",
			"tag":    tag,
			"server": host,
		}
		if port > 0 && port != 53 {
			entry["server_port"] = port
		}
		if net.ParseIP(host) == nil && bootstrapTag != "" {
			entry["domain_resolver"] = bootstrapTag
		}
		return entry
	}

	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return nil
	}
	port := 0
	if portStr := strings.TrimSpace(u.Port()); portStr != "" {
		parsedPort, convErr := strconv.Atoi(portStr)
		if convErr != nil || parsedPort <= 0 || parsedPort > 65535 {
			return nil
		}
		port = parsedPort
	}

	var serverType string
	defaultPort := 0
	supportsPath := false
	switch scheme {
	case "udp":
		serverType = "udp"
		defaultPort = 53
	case "tcp":
		serverType = "tcp"
		defaultPort = 53
	case "tls":
		serverType = "tls"
		defaultPort = 853
	case "quic":
		serverType = "quic"
		defaultPort = 853
	case "https":
		serverType = "https"
		if h3Param {
			serverType = "h3"
		}
		defaultPort = 443
		supportsPath = true
	case "h3":
		serverType = "h3"
		defaultPort = 443
		supportsPath = true
	default:
		return nil
	}

	entry := map[string]interface{}{
		"type":   serverType,
		"tag":    tag,
		"server": host,
	}
	if port > 0 && port != defaultPort {
		entry["server_port"] = port
	}
	if supportsPath && u.Path != "" && u.Path != "/" {
		entry["path"] = u.Path
	}
	if net.ParseIP(host) == nil && bootstrapTag != "" {
		entry["domain_resolver"] = bootstrapTag
	}
	return entry
}

func singBoxServerFingerprint(entry map[string]interface{}) string {
	t, _ := entry["type"].(string)
	s, _ := entry["server"].(string)
	p, _ := entry["server_port"].(int)
	path, _ := entry["path"].(string)
	return fmt.Sprintf("%s|%s|%d|%s", t, s, p, path)
}

func parseRawDNSHostPortScheme(raw string) (host string, port int, scheme string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, "", false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || strings.TrimSpace(u.Hostname()) == "" {
			return "", 0, "", false
		}
		p := 0
		if portText := strings.TrimSpace(u.Port()); portText != "" {
			parsedPort, convErr := strconv.Atoi(portText)
			if convErr != nil || parsedPort <= 0 || parsedPort > 65535 {
				return "", 0, "", false
			}
			p = parsedPort
		}
		return strings.TrimSpace(u.Hostname()), p, strings.ToLower(strings.TrimSpace(u.Scheme)), true
	}

	// Raw IPv6 without brackets, e.g. 2400:3200::1
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String(), 0, "", true
	}

	if h, pStr, err := net.SplitHostPort(raw); err == nil && strings.TrimSpace(h) != "" {
		parsedPort, convErr := strconv.Atoi(strings.TrimSpace(pStr))
		if convErr != nil || parsedPort <= 0 || parsedPort > 65535 {
			return "", 0, "", false
		}
		return strings.TrimSpace(h), parsedPort, "", true
	}

	if strings.Contains(raw, "/") || strings.ContainsAny(raw, " \t") {
		return "", 0, "", false
	}
	return raw, 0, "", true
}
