package backend

import "ant-chrome/backend/internal/config"

// browserFingerprintPlanOptions 控制指纹启动参数矩阵中可由配置调整的策略。
type browserFingerprintPlanOptions struct {
	// crossPlatformFontPassthrough 为 true 时，目标平台与宿主不同会追加 --disable-spoofing=font，
	// 使用宿主真实字体以避免缺字；为 false 时保留内核的字体伪装，字体列表与目标平台一致。
	crossPlatformFontPassthrough bool
}

func defaultBrowserFingerprintPlanOptions() browserFingerprintPlanOptions {
	return browserFingerprintPlanOptions{crossPlatformFontPassthrough: true}
}

func browserCrossPlatformFontPassthrough(cfg *config.Config) bool {
	if cfg == nil || cfg.Browser.CrossPlatformFontPassthrough == nil {
		return true
	}
	return *cfg.Browser.CrossPlatformFontPassthrough
}

func (a *App) browserFingerprintPlanOptions() browserFingerprintPlanOptions {
	options := defaultBrowserFingerprintPlanOptions()
	if a != nil {
		options.crossPlatformFontPassthrough = browserCrossPlatformFontPassthrough(a.config)
	}
	return options
}
