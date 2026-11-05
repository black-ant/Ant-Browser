package browser

import "ant-chrome/backend/internal/config"

type DashboardStats struct {
	TotalInstances   int
	RunningInstances int
	ProxyCount       int
	CoreCount        int
}

func BuildDashboardStats(profiles []Profile, cfg *config.Config) DashboardStats {
	stats := DashboardStats{
		TotalInstances: len(profiles),
	}
	for _, profile := range profiles {
		if profile.Running {
			stats.RunningInstances++
		}
	}
	if cfg != nil {
		stats.ProxyCount = len(cfg.Browser.Proxies)
		stats.CoreCount = len(cfg.Browser.Cores)
	}
	return stats
}

func RunningProfiles(profiles []Profile) []Profile {
	result := make([]Profile, 0)
	for _, profile := range profiles {
		if profile.Running {
			result = append(result, profile)
		}
	}
	return result
}
