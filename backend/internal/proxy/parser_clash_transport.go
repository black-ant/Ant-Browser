package proxy

import (
	"net/url"
	"strings"
)

func applyClashTLSClientOptions(node map[string]interface{}, tlsSettings map[string]interface{}) {
	if fingerprint := getMapString(node, "client-fingerprint"); fingerprint != "" {
		tlsSettings["fingerprint"] = fingerprint
	}
	if getMapBool(node, "skip-cert-verify") {
		tlsSettings[xrayTLSInsecurePinKey] = true
	}
	if alpnRaw, ok := node["alpn"]; ok {
		if alpnList := toStringSlice(alpnRaw); len(alpnList) > 0 {
			tlsSettings["alpn"] = alpnList
		}
	}
}

func buildClashWSSettings(node map[string]interface{}) map[string]interface{} {
	ws := map[string]interface{}{}
	if wsOpts, ok := node["ws-opts"]; ok {
		if wsMap := toStringMap(wsOpts); wsMap != nil {
			if path := getMapString(wsMap, "path"); path != "" {
				ws["path"] = path
			}
			if headers := buildClashWSHeaders(wsMap); len(headers) > 0 {
				ws["headers"] = headers
			}
		}
	}
	if _, ok := ws["path"]; !ok {
		if path := getMapString(node, "ws-path"); path != "" {
			ws["path"] = path
		}
	}
	if _, ok := ws["headers"]; !ok {
		if host := firstNonEmptyMapString(node, "ws-host", "host"); host != "" {
			ws["headers"] = map[string]interface{}{"Host": host}
		}
	}
	return ws
}

func buildClashWSHeaders(wsMap map[string]interface{}) map[string]interface{} {
	headers := toStringMap(wsMap["headers"])
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(headers))
	for key := range headers {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		out[name] = getMapString(headers, key)
	}
	return out
}

func buildClashGRPCSettings(node map[string]interface{}) map[string]interface{} {
	serviceName := ""
	if grpcOpts, ok := node["grpc-opts"]; ok {
		if grpcMap := toStringMap(grpcOpts); grpcMap != nil {
			serviceName = firstNonEmptyMapString(grpcMap, "grpc-service-name", "service-name", "serviceName")
		}
	}
	if serviceName == "" {
		serviceName = firstNonEmptyMapString(node, "grpc-service-name", "service-name", "serviceName")
	}
	if serviceName == "" {
		return nil
	}
	return map[string]interface{}{"serviceName": serviceName}
}

func firstNonEmptyMapString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := getMapString(m, key); value != "" {
			return value
		}
	}
	return ""
}

func queryBool(query url.Values, keys ...string) bool {
	for _, key := range keys {
		value := strings.ToLower(strings.TrimSpace(query.Get(key)))
		if value == "1" || value == "true" || value == "yes" {
			return true
		}
	}
	return false
}
