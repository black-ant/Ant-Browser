package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
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
	if settings.SpeedIntervalMinutes <= 0 {
		settings.SpeedIntervalMinutes = a.config.ProxyCheck.SpeedIntervalMinutes
	}
	if settings.FailureRetryMinutes <= 0 {
		settings.FailureRetryMinutes = a.config.ProxyCheck.FailureRetryMinutes
	}
	if settings.SpeedConcurrency <= 0 {
		settings.SpeedConcurrency = a.config.ProxyCheck.SpeedConcurrency
	}
	// 测速设置弹窗不携带虚拟分组字段，缺省（0/nil）时沿用现有配置。
	// 否则一次保存就会把核验间隔重置为“跟随全局”，周期核验随之失去约束。
	if settings.VirtualGroupProbeIntervalMinutes <= 0 {
		settings.VirtualGroupProbeIntervalMinutes = a.config.ProxyCheck.VirtualGroupProbeIntervalMinutes
	}
	if settings.DegradeFailureThreshold <= 0 {
		settings.DegradeFailureThreshold = a.config.ProxyCheck.DegradeFailureThreshold
	}
	if settings.RestoreSuccessThreshold <= 0 {
		settings.RestoreSuccessThreshold = a.config.ProxyCheck.RestoreSuccessThreshold
	}
	if settings.RuntimeFailoverCooldownMinutes <= 0 {
		settings.RuntimeFailoverCooldownMinutes = a.config.ProxyCheck.RuntimeFailoverCooldownMinutes
	}
	if settings.RuntimeFailoverMaxAttempts <= 0 {
		settings.RuntimeFailoverMaxAttempts = a.config.ProxyCheck.RuntimeFailoverMaxAttempts
	}
	if settings.VirtualGroupHealthEnabled == nil {
		settings.VirtualGroupHealthEnabled = a.config.ProxyCheck.VirtualGroupHealthEnabled
	}
	if settings.RuntimeFailoverEnabled == nil {
		settings.RuntimeFailoverEnabled = a.config.ProxyCheck.RuntimeFailoverEnabled
	}
	a.config.ProxyCheck = proxy.NormalizeCheckSettings(settings)
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		return err
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
	a.ensureSpeedScheduler()
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
