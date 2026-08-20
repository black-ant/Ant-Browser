package gateway

import (
	"net"
	"strings"
)

func ResolveAction(config RoutingConfig, host string) string {
	config = NormalizeRoutingConfig(config)
	if config.Mode == ModeDirect {
		return ActionDirect
	}
	if config.Mode != ModeRule {
		return ActionProxy
	}

	normalizedHost := normalizeHost(host)
	for _, rule := range config.Rules {
		if !rule.Enabled {
			continue
		}
		if ruleMatches(rule, normalizedHost) {
			return rule.Action
		}
	}
	return ActionProxy
}

func ruleMatches(rule Rule, host string) bool {
	pattern := normalizeHost(rule.Pattern)
	switch rule.MatchType {
	case MatchDomain:
		return host != "" && host == pattern
	case MatchDomainSuffix:
		pattern = strings.TrimPrefix(pattern, ".")
		return host != "" && pattern != "" && (host == pattern || strings.HasSuffix(host, "."+pattern))
	case MatchDomainKeyword:
		return host != "" && pattern != "" && strings.Contains(host, pattern)
	case MatchIPCIDR:
		ip := net.ParseIP(host)
		_, network, err := net.ParseCIDR(strings.TrimSpace(rule.Pattern))
		return err == nil && ip != nil && network.Contains(ip)
	default:
		return false
	}
}

func normalizeHost(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}
