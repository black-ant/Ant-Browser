package backend

import (
	"fmt"
	"net/url"
	"strings"
)

// sanitizeProxyForLog 脱敏代理配置，保留排查价值（协议/host/port），隐藏凭据。
// 输入示例：
//   - "socks5://user:pass@192.168.1.1:1080"     → "socks5://***:***@192.168.1.1:1080"
//   - "http://admin:secret@proxy.com:8080"      → "http://***:***@proxy.com:8080"
//   - "vmess://base64..."                        → "vmess://***"
//   - "hysteria2://token@server:port?..."       → "hysteria2://***@server:port"
//   - "socks5://192.168.1.1:1080"                → "socks5://192.168.1.1:1080" (无凭据保持原样)
//   - "direct://"                                 → "direct://"
func sanitizeProxyForLog(proxyConfig string) string {
	proxyConfig = strings.TrimSpace(proxyConfig)
	if proxyConfig == "" || proxyConfig == "direct://" {
		return proxyConfig
	}

	// vmess/vless/trojan/ss/hysteria2/tuic 等协议通常无法可靠解析，直接隐藏全部内容
	lowerProto := strings.ToLower(proxyConfig)
	if strings.HasPrefix(lowerProto, "vmess://") ||
		strings.HasPrefix(lowerProto, "vless://") ||
		strings.HasPrefix(lowerProto, "trojan://") ||
		strings.HasPrefix(lowerProto, "ss://") {
		prefix := strings.SplitN(proxyConfig, "://", 2)[0]
		return prefix + "://***"
	}

	// hysteria2 / tuic 协议：保留协议+host，隐藏凭据
	if strings.HasPrefix(lowerProto, "hysteria2://") || strings.HasPrefix(lowerProto, "tuic://") {
		parts := strings.SplitN(proxyConfig, "://", 2)
		if len(parts) != 2 {
			return parts[0] + "://***"
		}
		protocol := parts[0]
		rest := parts[1]

		// hysteria2://token@server:port?params → hysteria2://***@server:port
		if atIdx := strings.Index(rest, "@"); atIdx > 0 {
			hostPart := rest[atIdx+1:]
			// 去掉 query 参数
			if qIdx := strings.Index(hostPart, "?"); qIdx > 0 {
				hostPart = hostPart[:qIdx]
			}
			return protocol + "://***@" + hostPart
		}
		return protocol + "://***"
	}

	// http/https/socks5 等标准 URL 可以用 net/url 解析
	parsed, err := url.Parse(proxyConfig)
	if err != nil || parsed.Host == "" {
		// 解析失败，直接隐藏整个内容
		if strings.Contains(proxyConfig, "://") {
			proto := strings.SplitN(proxyConfig, "://", 2)[0]
			return proto + "://***"
		}
		return "***"
	}

	// 如果没有用户名密码，保持原样（但仍需检查 query 参数）
	if parsed.User == nil || parsed.User.Username() == "" {
		// 即使没有用户名密码，query 中也可能包含敏感信息
		if parsed.RawQuery != "" {
			return parsed.Scheme + "://" + parsed.Host + parsed.Path + "?***"
		}
		return proxyConfig
	}

	// 有凭据：隐藏用户名和密码，保留 host/port，移除 query（可能含敏感信息）
	sanitized := parsed.Scheme + "://***:***@" + parsed.Host
	if parsed.Path != "" && parsed.Path != "/" {
		sanitized += parsed.Path
	}
	// 移除 RawQuery，因为可能包含 token/password/secret/obfs-password 等敏感参数
	if parsed.RawQuery != "" {
		sanitized += "?***"
	}
	return sanitized
}

// sanitizeLaunchArgs 脱敏启动参数列表，隐藏 --proxy-server 中的凭据。
func sanitizeLaunchArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	sanitized := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "--proxy-server=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				sanitized[i] = "--proxy-server=" + sanitizeProxyForLog(parts[1])
			} else {
				sanitized[i] = arg
			}
		} else {
			sanitized[i] = arg
		}
	}
	return sanitized
}

// sanitizeProxyConfigField 为日志字段脱敏，适配 logger.F() 调用。
// 用法：logger.F("proxy", sanitizeProxyConfigField(proxyConfig))
func sanitizeProxyConfigField(proxyConfig interface{}) string {
	if proxyConfig == nil {
		return ""
	}
	if s, ok := proxyConfig.(string); ok {
		return sanitizeProxyForLog(s)
	}
	return fmt.Sprintf("%v", proxyConfig)
}
