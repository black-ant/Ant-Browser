package proxy

import "strings"

const defaultXrayMuxConcurrency = 8

func applyXrayBrowserOutboundTuning(node map[string]interface{}, outbound map[string]interface{}) {
	if node == nil || outbound == nil {
		return
	}
	protocol := strings.ToLower(strings.TrimSpace(getMapString(node, "type")))
	network := strings.ToLower(strings.TrimSpace(getMapString(node, "network")))
	if !shouldEnableXrayMuxForBrowser(node, protocol, network) {
		return
	}
	outbound["mux"] = map[string]interface{}{
		"enabled":     true,
		"concurrency": defaultXrayMuxConcurrency,
	}
}

func shouldEnableXrayMuxForBrowser(node map[string]interface{}, protocol string, network string) bool {
	if protocol != "vmess" && protocol != "vless" {
		return false
	}
	if network != "ws" {
		return false
	}
	return hasExplicitMuxEnabled(node)
}

func hasExplicitMuxEnabled(node map[string]interface{}) bool {
	for _, key := range []string{"mux", "xray-mux", "browser-mux"} {
		value, ok := node[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v
		case string:
			text := strings.ToLower(strings.TrimSpace(v))
			return text == "true" || text == "on" || text == "1" || text == "enabled" || text == "yes"
		case map[string]interface{}:
			if enabled, ok := v["enabled"]; ok {
				return truthyValue(enabled)
			}
		case map[interface{}]interface{}:
			settings := toStringMap(v)
			if enabled, ok := settings["enabled"]; ok {
				return truthyValue(enabled)
			}
		}
	}
	return false
}

func truthyValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		return text == "true" || text == "on" || text == "1" || text == "enabled" || text == "yes"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return value != nil
	}
}

func xrayBrowserSniffingConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic"},
	}
}
