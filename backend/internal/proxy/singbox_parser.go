package proxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// IsSingBoxProtocol 判断是否为 sing-box 支持的协议（hysteria2/tuic）
func IsSingBoxProtocol(proxyConfig string) bool {
	l := strings.ToLower(strings.TrimSpace(proxyConfig))
	if strings.HasPrefix(l, "hysteria2://") || strings.HasPrefix(l, "hy2://") || strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "anytls://") || strings.HasPrefix(l, "tuic://") {
		return true
	}
	// Clash YAML 格式
	if strings.Contains(l, "type: hysteria2") || strings.Contains(l, "type:hysteria2") ||
		strings.Contains(l, "type: hysteria") || strings.Contains(l, "type:hysteria") ||
		strings.Contains(l, "type: tuic") || strings.Contains(l, "type:tuic") ||
		strings.Contains(l, "type: anytls") || strings.Contains(l, "type:anytls") {
		return true
	}
	return false
}

// BuildSingBoxOutbound 解析节点配置，返回 sing-box outbound map
func BuildSingBoxOutbound(node string) (map[string]interface{}, error) {
	src := normalizeNodeScheme(node)
	l := strings.ToLower(src)

	if strings.HasPrefix(l, "hysteria2://") {
		return parseHysteria2URI(src)
	}
	if strings.HasPrefix(l, "hysteria://") {
		return parseHysteriaURI(src)
	}
	if strings.HasPrefix(l, "anytls://") {
		return parseAnyTLSURI(src)
	}
	if strings.HasPrefix(l, "tuic://") {
		return parseTUICURI(src)
	}

	// Clash YAML 格式
	if strings.Contains(l, "type:") || strings.Contains(l, "proxies:") {
		return parseClashSingBoxNode(src)
	}

	return nil, fmt.Errorf("不支持的 sing-box 节点格式")
}

// parseHysteria2URI 解析 hysteria2:// URI
// 格式: hysteria2://password@host:port?sni=xxx&insecure=1
func parseHysteria2URI(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("hysteria2 URI 解析失败: %v", err)
	}

	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	password := u.User.Username()
	if password == "" {
		// 有些格式把密码放在 userinfo 里不带 @
		password = strings.TrimPrefix(u.Host, "@")
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	insecure := q.Get("insecure") == "1" || strings.ToLower(q.Get("insecure")) == "true"
	obfsPassword := q.Get("obfs-password")
	serverPorts := parseSingBoxServerPorts(q.Get("mport"))

	if host == "" || port == 0 {
		return nil, fmt.Errorf("hysteria2 节点信息不完整: host=%s port=%d", host, port)
	}

	out := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls": map[string]interface{}{
			"enabled":  true,
			"insecure": insecure,
		},
	}

	if sni != "" {
		out["tls"].(map[string]interface{})["server_name"] = sni
	}
	if serverPorts != "" {
		out["server_ports"] = serverPorts
		delete(out, "server_port")
	}
	applySingBoxTLSClientOptionsFromQuery(q, out["tls"].(map[string]interface{}))

	if obfsPassword != "" {
		out["obfs"] = map[string]interface{}{
			"type":     "salamander",
			"password": obfsPassword,
		}
	}

	return out, nil
}

func parseHysteriaURI(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("hysteria URI 解析失败: %v", err)
	}

	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	q := u.Query()
	auth := firstNonEmptyQueryValue(q, "auth", "auth_str", "auth-str", "password")
	if auth == "" && u.User != nil {
		auth = u.User.Username()
		if password, ok := u.User.Password(); ok && password != "" {
			auth = password
		}
	}
	sni := firstNonEmptyQueryValue(q, "sni", "peer", "servername")
	insecure := queryBool(q, "insecure", "allowInsecure", "skip-cert-verify")
	serverPorts := parseSingBoxServerPorts(firstNonEmptyQueryValue(q, "mport", "ports", "server_ports", "server-ports"))

	if host == "" || port == 0 {
		return nil, fmt.Errorf("hysteria 节点信息不完整: host=%s port=%d", host, port)
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": insecure,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromQuery(q, tls)

	out := map[string]interface{}{
		"type":        "hysteria",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"tls":         tls,
	}
	if serverPorts != "" {
		out["server_ports"] = serverPorts
		delete(out, "server_port")
	}
	if auth != "" {
		out["auth_str"] = auth
	}
	if obfs := strings.TrimSpace(q.Get("obfs")); obfs != "" {
		out["obfs"] = obfs
	}
	if up := firstNonEmptyQueryValue(q, "upmbps", "up_mbps", "up"); up != "" {
		out["up_mbps"] = parseBandwidthMbps(up)
	}
	if down := firstNonEmptyQueryValue(q, "downmbps", "down_mbps", "down"); down != "" {
		out["down_mbps"] = parseBandwidthMbps(down)
	}

	return out, nil
}

func parseAnyTLSURI(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("anytls URI 解析失败: %v", err)
	}
	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	password := u.User.Username()
	q := u.Query()
	sni := firstNonEmptyQueryValue(q, "sni", "peer", "servername")
	insecure := queryBool(q, "insecure", "allowInsecure", "skip-cert-verify")

	if host == "" || port == 0 || password == "" {
		return nil, fmt.Errorf("anytls URI 节点信息不完整: host=%s port=%d password_empty=%v", host, port, password == "")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": insecure,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromQuery(q, tls)

	out := map[string]interface{}{
		"type":        "anytls",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         tls,
	}
	if interval := queryDurationSecondsString(q, "idle-session-check-interval"); interval != "" {
		out["idle_session_check_interval"] = interval
	}
	if timeout := queryDurationSecondsString(q, "idle-session-timeout"); timeout != "" {
		out["idle_session_timeout"] = timeout
	}
	if minIdleSession, err := strconv.Atoi(strings.TrimSpace(q.Get("min-idle-session"))); err == nil && minIdleSession > 0 {
		out["min_idle_session"] = minIdleSession
	}
	return out, nil
}

func queryDurationSecondsString(q url.Values, key string) string {
	value := strings.TrimSpace(q.Get(key))
	if value == "" {
		return ""
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value + "s"
	}
	return value
}

// parseClashSingBoxNode 解析 Clash YAML 格式的 sing-box 节点
func parseClashSingBoxNode(src string) (map[string]interface{}, error) {
	// 复用已有的 YAML 解析基础设施
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %v", err)
	}

	nodeMap := pickClashNode(payload)
	if nodeMap == nil {
		return nil, fmt.Errorf("节点解析失败")
	}

	nodeType := strings.ToLower(getMapString(nodeMap, "type"))
	switch nodeType {
	case "hysteria2":
		return buildSingBoxHysteria2FromClash(nodeMap)
	case "hysteria":
		return buildSingBoxHysteriaFromClash(nodeMap)
	case "tuic":
		return buildSingBoxTUICFromClash(nodeMap)
	case "anytls":
		return buildSingBoxAnyTLSFromClash(nodeMap)
	default:
		return nil, fmt.Errorf("不支持的 sing-box 节点类型: %s", nodeType)
	}
}

func buildSingBoxAnyTLSFromClash(node map[string]interface{}) (map[string]interface{}, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	password := getMapString(node, "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	skipVerify := getMapBool(node, "skip-cert-verify")

	if host == "" || port == 0 || password == "" {
		return nil, fmt.Errorf("anytls node info incomplete")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": skipVerify,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromClash(node, tls)

	out := map[string]interface{}{
		"type":        "anytls",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         tls,
	}
	if interval := clashDurationSecondsString(node, "idle-session-check-interval"); interval != "" {
		out["idle_session_check_interval"] = interval
	}
	if timeout := clashDurationSecondsString(node, "idle-session-timeout"); timeout != "" {
		out["idle_session_timeout"] = timeout
	}
	if minIdleSession := getMapInt(node, "min-idle-session"); minIdleSession != 0 {
		out["min_idle_session"] = minIdleSession
	}

	return out, nil
}

func buildSingBoxHysteriaFromClash(node map[string]interface{}) (map[string]interface{}, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	auth := firstNonEmptyMapString(node, "auth-str", "auth_str", "auth", "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	skipVerify := getMapBool(node, "skip-cert-verify")

	if host == "" || port == 0 {
		return nil, fmt.Errorf("hysteria 节点信息不完整")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": skipVerify,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromClash(node, tls)

	out := map[string]interface{}{
		"type":        "hysteria",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"tls":         tls,
	}
	if serverPorts := clashHysteria2ServerPorts(node); serverPorts != "" {
		out["server_ports"] = serverPorts
	}
	if auth != "" {
		out["auth_str"] = auth
	}
	if up := getMapString(node, "up"); up != "" {
		out["up_mbps"] = parseBandwidthMbps(up)
	}
	if down := getMapString(node, "down"); down != "" {
		out["down_mbps"] = parseBandwidthMbps(down)
	}
	if obfs := getMapString(node, "obfs"); obfs != "" {
		out["obfs"] = obfs
	}

	return out, nil
}

func buildSingBoxHysteria2FromClash(node map[string]interface{}) (map[string]interface{}, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	password := getMapString(node, "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	skipVerify := getMapBool(node, "skip-cert-verify")

	if host == "" || port == 0 {
		return nil, fmt.Errorf("hysteria2 节点信息不完整")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": skipVerify,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromClash(node, tls)

	out := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         "proxy-out",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         tls,
	}
	if serverPorts := clashHysteria2ServerPorts(node); serverPorts != "" {
		out["server_ports"] = serverPorts
	}

	// 带宽限制（可选）
	if up := getMapString(node, "up"); up != "" {
		out["up_mbps"] = parseBandwidthMbps(up)
	}
	if down := getMapString(node, "down"); down != "" {
		out["down_mbps"] = parseBandwidthMbps(down)
	}

	// obfs
	if obfsPassword := getMapString(node, "obfs-password"); obfsPassword != "" {
		out["obfs"] = map[string]interface{}{
			"type":     "salamander",
			"password": obfsPassword,
		}
	}
	if congestion := firstNonEmptyMapString(node, "congestion-control", "congestion_controller", "congestion-controller"); congestion != "" {
		out["congestion_control"] = congestion
	}

	return out, nil
}

func clashHysteria2ServerPorts(node map[string]interface{}) string {
	return parseSingBoxServerPorts(firstNonEmptyMapString(node, "ports", "mport"))
}

func parseSingBoxServerPorts(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	ranges := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		item = strings.ReplaceAll(item, "-", ":")
		ranges = append(ranges, item)
	}
	return strings.Join(ranges, ",")
}

func parseTUICURI(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("tuic URI 解析失败: %v", err)
	}
	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	uuid := u.User.Username()
	password, _ := u.User.Password()
	q := u.Query()
	if password == "" {
		password = q.Get("password")
	}
	sni := firstNonEmptyQueryValue(q, "sni", "peer", "servername")
	insecure := queryBool(q, "insecure", "allowInsecure", "skip-cert-verify")

	if host == "" || port == 0 || uuid == "" || password == "" {
		return nil, fmt.Errorf("tuic URI 节点信息不完整: host=%s port=%d uuid_empty=%v password_empty=%v", host, port, uuid == "", password == "")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": insecure,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	applySingBoxTLSClientOptionsFromQuery(q, tls)

	congestionControl := firstNonEmptyQueryValue(q, "congestion_control", "congestion-control", "congestion_controller", "congestion-controller")
	if congestionControl == "" {
		congestionControl = "bbr"
	}

	return map[string]interface{}{
		"type":               "tuic",
		"tag":                "proxy-out",
		"server":             host,
		"server_port":        port,
		"uuid":               uuid,
		"password":           password,
		"congestion_control": congestionControl,
		"tls":                tls,
	}, nil
}

func buildSingBoxTUICFromClash(node map[string]interface{}) (map[string]interface{}, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	uuid := getMapString(node, "uuid")
	password := getMapString(node, "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	skipVerify := getMapBool(node, "skip-cert-verify")

	if host == "" || port == 0 {
		return nil, fmt.Errorf("tuic 节点信息不完整")
	}

	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": skipVerify,
	}
	if sni != "" {
		tls["server_name"] = sni
	}

	applySingBoxTLSClientOptionsFromClash(node, tls)
	congestionControl := firstNonEmptyMapString(node, "congestion-control", "congestion_controller", "congestion-controller")
	if congestionControl == "" {
		congestionControl = "bbr"
	}

	return map[string]interface{}{
		"type":               "tuic",
		"tag":                "proxy-out",
		"server":             host,
		"server_port":        port,
		"uuid":               uuid,
		"password":           password,
		"congestion_control": congestionControl,
		"tls":                tls,
	}, nil
}

func applySingBoxTLSClientOptionsFromClash(node map[string]interface{}, tls map[string]interface{}) {
	if alpnRaw, ok := node["alpn"]; ok {
		if alpnList := toStringSlice(alpnRaw); len(alpnList) > 0 {
			tls["alpn"] = alpnList
		}
	}
	if fingerprint := getMapString(node, "client-fingerprint"); fingerprint != "" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fingerprint,
		}
	}
}

func applySingBoxTLSClientOptionsFromQuery(query url.Values, tls map[string]interface{}) {
	if alpn := strings.TrimSpace(query.Get("alpn")); alpn != "" {
		tls["alpn"] = splitCommaList(alpn)
	}
	if fingerprint := firstNonEmptyQueryValue(query, "client-fingerprint", "fingerprint"); fingerprint != "" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fingerprint,
		}
	}
}

func firstNonEmptyQueryValue(query url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// parseBandwidthMbps 解析带宽字符串，返回 Mbps 整数
// 支持: "100 Mbps", "100", "100M"
func parseBandwidthMbps(s string) int {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "BPS")
	s = strings.TrimSuffix(s, "B")
	s = strings.TrimSuffix(s, "M")
	n, _ := strconv.Atoi(s)
	return n
}

func clashDurationSecondsString(node map[string]interface{}, key string) string {
	v, ok := node[key]
	if !ok {
		return ""
	}
	switch value := v.(type) {
	case int:
		if value <= 0 {
			return ""
		}
		return fmt.Sprintf("%ds", value)
	case int64:
		if value <= 0 {
			return ""
		}
		return fmt.Sprintf("%ds", value)
	case float64:
		if value <= 0 {
			return ""
		}
		return fmt.Sprintf("%ds", int(value))
	case string:
		s := strings.TrimSpace(value)
		if s == "" || s == "0" {
			return ""
		}
		if _, err := strconv.Atoi(s); err == nil {
			return s + "s"
		}
		return s
	}
	return ""
}

// toStringSlice 将 interface{} 转为 []string
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if s, ok := v.(string); ok && s != "" {
		return []string{s}
	}
	return nil
}
