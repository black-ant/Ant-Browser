package backend

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"time"
)

const (
	defaultBrowserStartReadyTimeout = 3 * time.Second
	defaultBrowserStartStableWindow = 1200 * time.Millisecond
	defaultBrowserStartMaxAttempts  = 5
	defaultBrowserStartMaxConcurrent = 3
	minBrowserStartMaxConcurrent     = 1
	maxBrowserStartMaxConcurrent     = 8
)

func browserStartReadyTimeoutMillis(cfg *config.Config) int {
	fallback := int(defaultBrowserStartReadyTimeout / time.Millisecond)
	if cfg == nil {
		return fallback
	}
	if cfg.Browser.StartReadyTimeoutMs > 0 {
		return cfg.Browser.StartReadyTimeoutMs
	}
	return fallback
}

func browserStartStableWindowMillis(cfg *config.Config) int {
	fallback := int(defaultBrowserStartStableWindow / time.Millisecond)
	if cfg == nil {
		return fallback
	}
	if cfg.Browser.StartStableWindowMs > 0 {
		return cfg.Browser.StartStableWindowMs
	}
	return fallback
}

func (a *App) browserStartTimingSettings() (time.Duration, time.Duration) {
	return time.Duration(browserStartReadyTimeoutMillis(a.config)) * time.Millisecond,
		time.Duration(browserStartStableWindowMillis(a.config)) * time.Millisecond
}

func browserStartAttemptCount() int {
	return defaultBrowserStartMaxAttempts
}

// browserStartMaxConcurrent 返回批量启动并发上限，读取配置并钳制到 [1, 8]，默认 3。
func browserStartMaxConcurrent(cfg *config.Config) int {
	value := defaultBrowserStartMaxConcurrent
	if cfg != nil && cfg.Browser.StartMaxConcurrent > 0 {
		value = cfg.Browser.StartMaxConcurrent
	}
	if value < minBrowserStartMaxConcurrent {
		value = minBrowserStartMaxConcurrent
	}
	if value > maxBrowserStartMaxConcurrent {
		value = maxBrowserStartMaxConcurrent
	}
	return value
}

func shouldRetryBrowserReadyFailure(err error) bool {
	if err == nil {
		return false
	}

	var exitErr *browserStartupExitError
	return !errors.As(err, &exitErr)
}
