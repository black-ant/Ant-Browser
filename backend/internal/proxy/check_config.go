package proxy

import (
	"ant-chrome/backend/internal/config"
	"strings"
	"time"
)

const defaultBridgeStartTimeoutMs = 15000
const defaultSpeedTargetTimeoutMs = 3000
const defaultIPHealthTargetTimeoutMs = 10000

func NormalizeCheckSettings(settings config.ProxyCheckConfig) config.ProxyCheckConfig {
	settings.BridgeStartTimeoutMs = normalizePositiveInt(settings.BridgeStartTimeoutMs, defaultBridgeStartTimeoutMs)
	settings.FailureRetryMinutes = normalizePositiveInt(settings.FailureRetryMinutes, config.DefaultProxyFailureRetryMinutes)
	if settings.FailureRetryMinutes > 24*60 {
		settings.FailureRetryMinutes = 24 * 60
	}
	settings.SpeedConcurrency = normalizePositiveInt(settings.SpeedConcurrency, config.DefaultProxySpeedConcurrency)
	if settings.SpeedConcurrency > config.MaxProxySpeedConcurrency {
		settings.SpeedConcurrency = config.MaxProxySpeedConcurrency
	}
	settings.SpeedTargetID = strings.TrimSpace(settings.SpeedTargetID)
	settings.IPHealthTargetID = strings.TrimSpace(settings.IPHealthTargetID)
	settings.Targets = NormalizeCheckTargets(settings.Targets)
	if len(settings.Targets) == 0 {
		settings.Targets = config.DefaultConfig().ProxyCheck.Targets
	}
	if settings.SpeedTargetID == "" {
		settings.SpeedTargetID = FirstCheckTargetID(settings.Targets, "speed", "")
	}
	if settings.IPHealthTargetID == "" {
		settings.IPHealthTargetID = FirstCheckTargetID(settings.Targets, "ip_health", "")
	}
	return settings
}

func BuildSpeedTestConfig(settings config.ProxyCheckConfig) *SpeedTestConfig {
	cfg := DefaultSpeedTestConfig
	if settings.BridgeStartTimeoutMs > 0 {
		cfg.TCPTimeout = time.Duration(settings.BridgeStartTimeoutMs) * time.Millisecond
	}
	target := FindCheckTarget(settings.Targets, settings.SpeedTargetID, "speed")
	if strings.TrimSpace(target.URL) != "" {
		cfg.URLs = []string{strings.TrimSpace(target.URL)}
	}
	if target.TimeoutMs > 0 {
		cfg.Timeout = time.Duration(target.TimeoutMs) * time.Millisecond
	}
	if len(target.ExpectedStatus) > 0 {
		cfg.ExpectedStatus = append([]int{}, target.ExpectedStatus...)
	}
	return &cfg
}

func BuildIPHealthConfig(settings config.ProxyCheckConfig) *IPHealthConfig {
	cfg := &IPHealthConfig{Source: "ip_health"}
	target := FindCheckTarget(settings.Targets, settings.IPHealthTargetID, "ip_health")
	if strings.TrimSpace(target.URL) != "" {
		cfg.URL = strings.TrimSpace(target.URL)
	}
	if strings.TrimSpace(target.ID) != "" {
		cfg.Source = strings.TrimSpace(target.ID)
	}
	if strings.TrimSpace(target.Parser) != "" {
		cfg.Parser = strings.TrimSpace(target.Parser)
	}
	if target.TimeoutMs > 0 {
		cfg.Timeout = time.Duration(target.TimeoutMs) * time.Millisecond
	}
	return cfg
}

func FindCheckTarget(targets []config.ProxyCheckTarget, id string, targetType string) config.ProxyCheckTarget {
	normalizedID := strings.TrimSpace(id)
	normalizedType := strings.TrimSpace(targetType)
	for _, target := range targets {
		if normalizedID != "" && strings.EqualFold(strings.TrimSpace(target.ID), normalizedID) {
			return target
		}
	}
	for _, target := range targets {
		if normalizedType != "" && strings.EqualFold(strings.TrimSpace(target.Type), normalizedType) {
			return target
		}
	}
	return config.ProxyCheckTarget{}
}

func NormalizeCheckTargets(targets []config.ProxyCheckTarget) []config.ProxyCheckTarget {
	result := make([]config.ProxyCheckTarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		target.ID = strings.TrimSpace(target.ID)
		target.Name = strings.TrimSpace(target.Name)
		target.Type = strings.TrimSpace(target.Type)
		target.URL = strings.TrimSpace(target.URL)
		target.Parser = strings.TrimSpace(target.Parser)
		if target.ID == "" || target.URL == "" {
			continue
		}
		key := strings.ToLower(target.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if target.Name == "" {
			target.Name = target.ID
		}
		if target.Type == "" {
			target.Type = "speed"
		}
		if target.TimeoutMs <= 0 {
			if strings.EqualFold(target.Type, "ip_health") {
				target.TimeoutMs = defaultIPHealthTargetTimeoutMs
			} else {
				target.TimeoutMs = defaultSpeedTargetTimeoutMs
			}
		}
		result = append(result, target)
	}
	return result
}

func FirstCheckTargetID(targets []config.ProxyCheckTarget, targetType string, fallback string) string {
	for _, target := range targets {
		if strings.EqualFold(strings.TrimSpace(target.Type), targetType) {
			return strings.TrimSpace(target.ID)
		}
	}
	return fallback
}

func normalizePositiveInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
