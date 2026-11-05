package proxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type directProxyBridgeSpec struct {
	Scheme   string
	Server   string
	Port     int
	Username string
	Password string
}

func RequiresLocalProxyBridgeForBrowser(src string) bool {
	spec, err := parseDirectProxyBridgeSpec(src)
	return err == nil && spec != nil
}

func buildDirectProxyBridgeOutbound(src string) (map[string]interface{}, bool, error) {
	spec, err := parseDirectProxyBridgeSpec(src)
	if err != nil {
		return nil, false, err
	}
	if spec == nil {
		return nil, false, nil
	}

	if spec.Scheme == "socks5" || spec.Scheme == "http" || spec.Scheme == "https" {
		return chainSocks5Outbound(chainSocks5Hop{
			Protocol: spec.Scheme,
			Server:   spec.Server,
			Port:     spec.Port,
			Username: spec.Username,
			Password: spec.Password,
			TLS:      spec.Scheme == "https",
		}, "proxy-out", ""), true, nil
	}

	return nil, false, nil
}

func parseDirectProxyBridgeSpec(src string) (*directProxyBridgeSpec, error) {
	raw := strings.TrimSpace(src)
	if raw == "" {
		return nil, nil
	}
	lowerRaw := strings.ToLower(raw)
	if !strings.HasPrefix(lowerRaw, "http://") && !strings.HasPrefix(lowerRaw, "https://") && !strings.HasPrefix(lowerRaw, "socks5://") {
		return nil, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	switch scheme {
	case "socks5", "http", "https":
		if parsed.User == nil {
			return nil, nil
		}
		username := strings.TrimSpace(parsed.User.Username())
		if username == "" {
			return nil, nil
		}
		server := strings.TrimSpace(parsed.Hostname())
		if server == "" {
			return nil, fmt.Errorf("代理地址缺少主机名")
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("代理端口无效")
		}
		password, _ := parsed.User.Password()
		return &directProxyBridgeSpec{
			Scheme:   scheme,
			Server:   server,
			Port:     port,
			Username: username,
			Password: password,
		}, nil
	default:
		return nil, nil
	}
}
