package backend

import (
	goruntime "runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Dashboard 真实监控与事件
// ============================================================================

// MetricSample 一次资源采样（全部真实数据）
type MetricSample struct {
	Timestamp        int64   `json:"timestamp"`        // unix 秒
	CPUPercent       float64 `json:"cpuPercent"`       // 系统 CPU 使用率
	MemUsedMB        int     `json:"memUsedMB"`        // 系统已用内存 MB
	MemTotalMB       int     `json:"memTotalMB"`       // 系统总内存 MB
	AppMemMB         int     `json:"appMemMB"`         // 应用堆内存 MB
	RunningInstances int     `json:"runningInstances"` // 运行中窗口数
}

// ActivityEntry 活动日志条目（真实事件）
type ActivityEntry struct {
	Timestamp   int64  `json:"timestamp"`
	Type        string `json:"type"`  // start/stop/crash/import/speedtest/config
	Level       string `json:"level"` // info/warn/error
	Message     string `json:"message"`
	ProfileName string `json:"profileName,omitempty"`
}

// DashboardMetrics 实时指标 + 历史
type DashboardMetrics struct {
	Live    MetricSample   `json:"live"`
	History []MetricSample `json:"history"`
}

const (
	metricsHistorySize = 120 // 约 10 分钟（5s/采样）
	activityLogSize    = 120
	metricsInterval    = 5 * time.Second
)

type dashboardMonitor struct {
	mu       sync.Mutex
	samples  []MetricSample
	activity []ActivityEntry
	stopCh   chan struct{}
	running  bool
}

// startDashboardMonitor 启动后台资源采样（真实 CPU/内存/运行窗口）
func (a *App) startDashboardMonitor() {
	if a.dashboard == nil {
		a.dashboard = &dashboardMonitor{}
	}
	a.dashboard.mu.Lock()
	if a.dashboard.running {
		a.dashboard.mu.Unlock()
		return
	}
	a.dashboard.running = true
	a.dashboard.stopCh = make(chan struct{})
	a.dashboard.mu.Unlock()

	go func() {
		// 预热 cpu.Percent（首次调用返回 0，之后返回区间增量）
		_, _ = cpu.Percent(0, false)
		a.collectSample()
		ticker := time.NewTicker(metricsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.dashboard.stopCh:
				return
			case <-ticker.C:
				a.collectSample()
			}
		}
	}()
}

func (a *App) stopDashboardMonitor() {
	if a.dashboard == nil {
		return
	}
	a.dashboard.mu.Lock()
	defer a.dashboard.mu.Unlock()
	if !a.dashboard.running {
		return
	}
	a.dashboard.running = false
	close(a.dashboard.stopCh)
}

func (a *App) collectSample() {
	sample := MetricSample{Timestamp: time.Now().Unix()}

	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		sample.CPUPercent = float64(int(pcts[0]*10)) / 10
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		sample.MemUsedMB = int(vm.Used / 1024 / 1024)
		sample.MemTotalMB = int(vm.Total / 1024 / 1024)
	}
	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)
	sample.AppMemMB = int(ms.Alloc / 1024 / 1024)

	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		running := 0
		for _, p := range a.browserMgr.Profiles {
			if p != nil && p.Running {
				running++
			}
		}
		a.browserMgr.Mutex.Unlock()
		sample.RunningInstances = running
	}

	a.dashboard.mu.Lock()
	a.dashboard.samples = append(a.dashboard.samples, sample)
	if len(a.dashboard.samples) > metricsHistorySize {
		a.dashboard.samples = a.dashboard.samples[len(a.dashboard.samples)-metricsHistorySize:]
	}
	a.dashboard.mu.Unlock()
}

// GetDashboardMetrics 返回实时指标与历史（供图表使用，均为真实采样）
func (a *App) GetDashboardMetrics() DashboardMetrics {
	if a.dashboard == nil {
		return DashboardMetrics{History: []MetricSample{}}
	}
	a.dashboard.mu.Lock()
	defer a.dashboard.mu.Unlock()
	history := make([]MetricSample, len(a.dashboard.samples))
	copy(history, a.dashboard.samples)
	live := MetricSample{}
	if len(history) > 0 {
		live = history[len(history)-1]
	}
	return DashboardMetrics{Live: live, History: history}
}

// recordActivity 记录一条真实活动事件并实时推送给前端
func (a *App) recordActivity(typ, level, message, profileName string) {
	if a.dashboard == nil {
		a.dashboard = &dashboardMonitor{}
	}
	entry := ActivityEntry{
		Timestamp:   time.Now().Unix(),
		Type:        typ,
		Level:       level,
		Message:     message,
		ProfileName: profileName,
	}
	a.dashboard.mu.Lock()
	a.dashboard.activity = append(a.dashboard.activity, entry)
	if len(a.dashboard.activity) > activityLogSize {
		a.dashboard.activity = a.dashboard.activity[len(a.dashboard.activity)-activityLogSize:]
	}
	a.dashboard.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "dashboard:activity", entry)
	}
}

// GetActivityLog 返回活动日志（最新在前）
func (a *App) GetActivityLog() []ActivityEntry {
	if a.dashboard == nil {
		return []ActivityEntry{}
	}
	a.dashboard.mu.Lock()
	defer a.dashboard.mu.Unlock()
	n := len(a.dashboard.activity)
	out := make([]ActivityEntry, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, a.dashboard.activity[i])
	}
	return out
}

// GetRecentErrors 返回最近的错误级活动（最新在前），便于排障
func (a *App) GetRecentErrors() []ActivityEntry {
	if a.dashboard == nil {
		return []ActivityEntry{}
	}
	a.dashboard.mu.Lock()
	defer a.dashboard.mu.Unlock()
	out := make([]ActivityEntry, 0)
	for i := len(a.dashboard.activity) - 1; i >= 0; i-- {
		if a.dashboard.activity[i].Level == "error" {
			out = append(out, a.dashboard.activity[i])
		}
	}
	return out
}
