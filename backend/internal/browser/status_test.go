package browser

import "testing"

func TestStatusTransitionsAndDescriptions(t *testing.T) {
	// CanStart：仅 stopped / crashed 可启动
	for _, s := range []ProfileStatus{StatusStopped, StatusCrashed} {
		if !CanStart(s) {
			t.Errorf("CanStart(%q) = false, want true", s)
		}
	}
	for _, s := range []ProfileStatus{StatusStarting, StatusRunning, StatusDebugPending, StatusStopping} {
		if CanStart(s) {
			t.Errorf("CanStart(%q) = true, want false", s)
		}
	}

	// CanStop：running / debug_pending / starting 可停止
	for _, s := range []ProfileStatus{StatusRunning, StatusDebugPending, StatusStarting} {
		if !CanStop(s) {
			t.Errorf("CanStop(%q) = false, want true", s)
		}
	}
	for _, s := range []ProfileStatus{StatusStopped, StatusCrashed, StatusStopping} {
		if CanStop(s) {
			t.Errorf("CanStop(%q) = true, want false", s)
		}
	}

	// IsTransitionalState：starting / debug_pending / stopping 为过渡态
	for _, s := range []ProfileStatus{StatusStarting, StatusDebugPending, StatusStopping} {
		if !IsTransitionalState(s) {
			t.Errorf("IsTransitionalState(%q) = false, want true", s)
		}
	}
	for _, s := range []ProfileStatus{StatusStopped, StatusRunning, StatusCrashed} {
		if IsTransitionalState(s) {
			t.Errorf("IsTransitionalState(%q) = true, want false", s)
		}
	}

	// GetStatusDescription：每个已知状态都有非空中文描述，未知状态有兜底
	known := []ProfileStatus{StatusStopped, StatusStarting, StatusDebugPending, StatusRunning, StatusStopping, StatusCrashed}
	for _, s := range known {
		if GetStatusDescription(s) == "" {
			t.Errorf("GetStatusDescription(%q) is empty", s)
		}
	}
	if GetStatusDescription(ProfileStatus("nonsense")) != "未知状态" {
		t.Errorf("unknown status description = %q, want 未知状态", GetStatusDescription(ProfileStatus("nonsense")))
	}
}
