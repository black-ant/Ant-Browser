package proxy

import (
	"ant-chrome/backend/internal/apppath"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const xrayTLSInsecurePinKey = "_antInsecureSkipVerify"

var xrayTLSPinCache sync.Map

func (m *XrayManager) buildRuntimeConfig(key string, outbound map[string]interface{}, port int, dnsServers string) (string, error) {
	return m.buildRuntimeConfigWithRoute(
		key,
		[]interface{}{outbound},
		[]interface{}{
			map[string]interface{}{
				"type":        "field",
				"inboundTag":  []string{"socks-in"},
				"outboundTag": "proxy-out",
			},
		},
		port,
		dnsServers,
	)
}

func (m *XrayManager) buildRuntimeConfigWithRoute(key string, outbounds []interface{}, rules []interface{}, port int, dnsServers string) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	outbounds = sanitizeXrayOutbounds(outbounds)
	outbounds = pinXrayInsecureTLSOutbounds(outbounds)
	cfgPath := filepath.Join(baseDir, "xray-config.json")
	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
			"error":    filepath.Join(baseDir, "xray-error.log"),
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks-in",
				"port":     port,
				"listen":   "127.0.0.1",
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
				"sniffing": xrayBrowserSniffingConfig(),
			},
		},
		"outbounds": append(outbounds,
			map[string]interface{}{
				"protocol": "direct",
				"tag":      "direct",
			},
			map[string]interface{}{
				"protocol": "blackhole",
				"tag":      "block",
			},
		),
		"routing": map[string]interface{}{
			"rules": rules,
		},
	}
	if dnsCfg := parseDnsConfig(dnsServers); dnsCfg != nil {
		cfg["dns"] = dnsCfg
	} else {
		cfg["dns"] = defaultXrayDNSConfig()
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func defaultXrayDNSConfig() map[string]interface{} {
	return map[string]interface{}{
		"servers": []interface{}{"223.5.5.5", "119.29.29.29"},
	}
}

func sanitizeXrayOutbounds(outbounds []interface{}) []interface{} {
	for _, outbound := range outbounds {
		removeXrayDeprecatedFields(outbound)
	}
	return outbounds
}

func pinXrayInsecureTLSOutbounds(outbounds []interface{}) []interface{} {
	for _, outbound := range outbounds {
		pinXrayInsecureTLSOutbound(outbound)
	}
	return outbounds
}

func pinXrayInsecureTLSOutbound(outbound interface{}) {
	item, ok := outbound.(map[string]interface{})
	if !ok {
		return
	}
	stream, _ := item["streamSettings"].(map[string]interface{})
	tlsSettings, _ := stream["tlsSettings"].(map[string]interface{})
	if tlsSettings == nil || !truthy(tlsSettings[xrayTLSInsecurePinKey]) {
		return
	}
	delete(tlsSettings, xrayTLSInsecurePinKey)
	serverName := getStringAny(tlsSettings["serverName"])
	host, port := firstXrayOutboundEndpoint(item)
	if host == "" || port == "" {
		return
	}
	if serverName == "" {
		serverName = host
	}
	if fingerprints := fetchTLSPeerCertPins(host, port, serverName); len(fingerprints) > 0 {
		tlsSettings["pinnedPeerCertSha256"] = strings.Join(fingerprints, ",")
	}
}

func firstXrayOutboundEndpoint(outbound map[string]interface{}) (string, string) {
	protocol := strings.ToLower(getStringAny(outbound["protocol"]))
	settings, _ := outbound["settings"].(map[string]interface{})
	if settings == nil {
		return "", ""
	}
	switch protocol {
	case "trojan":
		servers, _ := settings["servers"].([]interface{})
		if len(servers) == 0 {
			return "", ""
		}
		server, _ := servers[0].(map[string]interface{})
		if server == nil {
			return "", ""
		}
		return getStringAny(server["address"]), getPortString(server["port"])
	case "vless", "vmess":
		vnext, _ := settings["vnext"].([]interface{})
		if len(vnext) == 0 {
			return "", ""
		}
		server, _ := vnext[0].(map[string]interface{})
		if server == nil {
			return "", ""
		}
		return getStringAny(server["address"]), getPortString(server["port"])
	}
	return "", ""
}

func fetchTLSPeerCertPins(host string, port string, serverName string) []string {
	cacheKey := strings.ToLower(strings.TrimSpace(host)) + ":" + strings.TrimSpace(port) + "|" + strings.ToLower(strings.TrimSpace(serverName))
	if cached, ok := xrayTLSPinCache.Load(cacheKey); ok {
		if pins, ok := cached.([]string); ok && len(pins) > 0 {
			return append([]string(nil), pins...)
		}
	}
	fingerprint, err := fetchTLSPeerCertPin(host, port, serverName)
	if err != nil || fingerprint == "" {
		return nil
	}
	fingerprints := []string{fingerprint}
	if len(fingerprints) > 0 {
		xrayTLSPinCache.Store(cacheKey, append([]string(nil), fingerprints...))
	}
	return fingerprints
}

func fetchTLSPeerCertPin(host string, port string, serverName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: time.Second},
		Config:    &tls.Config{ServerName: serverName, InsecureSkipVerify: true},
	}
	conn, err := dialer.DialContext(ctx, "tcp", host+":"+port)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return "", nil
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", nil
	}
	sum := sha256.Sum256(state.PeerCertificates[0].Raw)
	return hex.EncodeToString(sum[:]), nil
}

func getPortString(value interface{}) string {
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.Itoa(int(v))
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func getStringAny(value interface{}) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}

func truthy(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	default:
		return false
	}
}

func removeXrayDeprecatedFields(value interface{}) {
	switch item := value.(type) {
	case map[string]interface{}:
		delete(item, "allowInsecure")
		for _, nested := range item {
			removeXrayDeprecatedFields(nested)
		}
	case []interface{}:
		for _, nested := range item {
			removeXrayDeprecatedFields(nested)
		}
	}
}

func (m *XrayManager) resolveWorkdir(key string) string {
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	if !filepath.IsAbs(root) {
		root = apppath.Resolve(m.AppRoot, root)
	}
	return filepath.Join(root, "_xray", key)
}
