package gateway

import (
	"context"
	"fmt"
	"net"
	"strings"
)

const (
	ModeProxy  = "proxy"
	ModeRule   = "rule"
	ModeDirect = "direct"

	ActionProxy  = "proxy"
	ActionDirect = "direct"
	ActionBlock  = "block"

	MatchDomain        = "domain"
	MatchDomainSuffix  = "domain_suffix"
	MatchDomainKeyword = "domain_keyword"
	MatchIPCIDR        = "ip_cidr"
)

type Rule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	MatchType string `json:"matchType"`
	Pattern   string `json:"pattern"`
	Action    string `json:"action"`
}

type RoutingConfig struct {
	Mode  string `json:"mode"`
	Rules []Rule `json:"rules"`
}

type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type RouteSpec struct {
	ID      string
	Dial    DialContextFunc
	Release func()
}

type Status struct {
	ProfileID           string `json:"profileId"`
	ProxyURL            string `json:"proxyUrl"`
	CurrentRouteID      string `json:"currentRouteId"`
	Mode                string `json:"mode"`
	ActiveConnections   int    `json:"activeConnections"`
	DrainingConnections int    `json:"drainingConnections"`
	BrowserPID          int    `json:"browserPid"`
	BrowserDebugPort    int    `json:"browserDebugPort"`
}

func ValidateRoutingConfig(input RoutingConfig) error {
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode != ModeProxy && mode != ModeRule && mode != ModeDirect {
		return fmt.Errorf("routing.mode must be proxy, rule or direct")
	}
	for index, item := range input.Rules {
		matchType := strings.ToLower(strings.TrimSpace(item.MatchType))
		pattern := strings.TrimSpace(item.Pattern)
		action := strings.ToLower(strings.TrimSpace(item.Action))
		if pattern == "" {
			return fmt.Errorf("routing.rules[%d].pattern is required", index)
		}
		if !validMatchType(matchType) {
			return fmt.Errorf("routing.rules[%d].matchType is invalid", index)
		}
		if !validAction(action) {
			return fmt.Errorf("routing.rules[%d].action is invalid", index)
		}
		if matchType == MatchIPCIDR {
			if _, _, err := net.ParseCIDR(pattern); err != nil {
				return fmt.Errorf("routing.rules[%d].pattern must be a valid CIDR", index)
			}
		}
	}
	return nil
}

func NormalizeRoutingConfig(input RoutingConfig) RoutingConfig {
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	switch mode {
	case ModeDirect, ModeRule, ModeProxy:
	default:
		mode = ModeProxy
	}
	rules := make([]Rule, 0, len(input.Rules))
	for _, item := range input.Rules {
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		item.MatchType = strings.ToLower(strings.TrimSpace(item.MatchType))
		item.Pattern = strings.TrimSpace(item.Pattern)
		item.Action = strings.ToLower(strings.TrimSpace(item.Action))
		if item.Pattern == "" || !validMatchType(item.MatchType) || !validAction(item.Action) {
			continue
		}
		rules = append(rules, item)
	}
	return RoutingConfig{Mode: mode, Rules: rules}
}

func validMatchType(value string) bool {
	switch value {
	case MatchDomain, MatchDomainSuffix, MatchDomainKeyword, MatchIPCIDR:
		return true
	default:
		return false
	}
}

func validAction(value string) bool {
	switch value {
	case ActionProxy, ActionDirect, ActionBlock:
		return true
	default:
		return false
	}
}
