package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
	"reflect"
)

type ProxyCheckSettings = config.ProxyCheckConfig
type ProxyCheckTarget = config.ProxyCheckTarget

func (a *App) GetProxyCheckSettings() ProxyCheckSettings {
	if a.config == nil {
		return config.DefaultConfig().ProxyCheck
	}
	settings := a.config.ProxyCheck
	settings.Targets = append([]config.ProxyCheckTarget{}, settings.Targets...)
	return settings
}

func (a *App) SaveProxyCheckSettings(settings ProxyCheckSettings) error {
	if a.config == nil {
		return nil
	}
	previousSpeedConfig := proxy.BuildSpeedTestConfig(a.config.ProxyCheck)
	if settings.FailureRetryMinutes <= 0 {
		settings.FailureRetryMinutes = a.config.ProxyCheck.FailureRetryMinutes
	}
	if settings.SpeedConcurrency <= 0 {
		settings.SpeedConcurrency = a.config.ProxyCheck.SpeedConcurrency
	}
	a.config.ProxyCheck = proxy.NormalizeCheckSettings(settings)
	nextSpeedConfig := proxy.BuildSpeedTestConfig(a.config.ProxyCheck)
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		return err
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
	a.ensureSpeedScheduler()
	if !reflect.DeepEqual(*previousSpeedConfig, *nextSpeedConfig) {
		a.emitRuntimeEvent("proxy:speed:context:changed")
	}
	return nil
}

func (a *App) proxySpeedTestConfig() *proxy.SpeedTestConfig {
	if a == nil || a.config == nil {
		cfg := proxy.DefaultSpeedTestConfig
		return &cfg
	}
	return proxy.BuildSpeedTestConfig(a.config.ProxyCheck)
}

func (a *App) proxyIPHealthConfig() *proxy.IPHealthConfig {
	if a == nil || a.config == nil {
		return &proxy.IPHealthConfig{Source: "ip_health"}
	}
	return proxy.BuildIPHealthConfig(a.config.ProxyCheck)
}
