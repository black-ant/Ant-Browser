package browser

import "time"

// ProfileStatus 窗口状态
type ProfileStatus string

const (
	StatusStopped      ProfileStatus = "stopped"       // 已停止
	StatusStarting     ProfileStatus = "starting"      // 启动中（准备阶段）
	StatusDebugPending ProfileStatus = "debug_pending" // 进程已启动，等待调试端口
	StatusRunning      ProfileStatus = "running"       // 运行中
	StatusStopping     ProfileStatus = "stopping"      // 停止中
	StatusCrashed      ProfileStatus = "crashed"       // 崩溃
)

// ProfileStatusInfo 状态信息
type ProfileStatusInfo struct {
	Status          ProfileStatus `json:"status"`
	Message         string        `json:"message"`
	LastStateChange time.Time     `json:"lastStateChange"`
}

// GetStatusDescription 获取状态描述
func GetStatusDescription(status ProfileStatus) string {
	switch status {
	case StatusStopped:
		return "已停止"
	case StatusStarting:
		return "启动中"
	case StatusDebugPending:
		return "等待调试端口"
	case StatusRunning:
		return "运行中"
	case StatusStopping:
		return "停止中"
	case StatusCrashed:
		return "已崩溃"
	default:
		return "未知状态"
	}
}

// IsTransitionalState 是否为过渡状态（非稳定状态）
func IsTransitionalState(status ProfileStatus) bool {
	return status == StatusStarting ||
		status == StatusDebugPending ||
		status == StatusStopping
}

// CanStart 是否可以启动
func CanStart(status ProfileStatus) bool {
	return status == StatusStopped || status == StatusCrashed
}

// CanStop 是否可以停止
func CanStop(status ProfileStatus) bool {
	return status == StatusRunning ||
		status == StatusDebugPending ||
		status == StatusStarting
}
