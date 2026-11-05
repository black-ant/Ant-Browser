package browser

import "time"

// MemberProbeOutcome 一次成员探测的结果。
type MemberProbeOutcome struct {
	Ok            bool
	LatencyMs     int64
	Error         string
	Engine        string
	ConnectorType string
	At            time.Time
}

// MemberHealthPolicy 降级/升级阈值与退避参数。
type MemberHealthPolicy struct {
	DegradeFailureThreshold int
	RestoreSuccessThreshold int
	FailureRetry            time.Duration
}

const (
	defaultDegradeFailureThreshold = 2
	defaultRestoreSuccessThreshold = 2
	defaultMemberFailureRetry      = 5 * time.Minute
	memberHealthCounterCeiling     = 100
	memberHealthBackoffFloor       = time.Minute
)

func (p MemberHealthPolicy) normalized() MemberHealthPolicy {
	if p.DegradeFailureThreshold <= 0 {
		p.DegradeFailureThreshold = defaultDegradeFailureThreshold
	}
	if p.RestoreSuccessThreshold <= 0 {
		p.RestoreSuccessThreshold = defaultRestoreSuccessThreshold
	}
	if p.FailureRetry <= 0 {
		p.FailureRetry = defaultMemberFailureRetry
	}
	return p
}

// NextMemberState 纯函数状态迁移，不读写数据库。
//
// 迁移规则：
//
//	healthy   --fail--> suspect    --fail--> degraded
//	suspect   --ok----> healthy
//	degraded  --ok----> recovering --ok----> healthy
//	recovering--fail--> degraded
func NextMemberState(prev ProxyVirtualGroupMemberState, outcome MemberProbeOutcome, policy MemberHealthPolicy) ProxyVirtualGroupMemberState {
	policy = policy.normalized()
	at := outcome.At
	if at.IsZero() {
		at = time.Now()
	}
	next := prev
	next.State = NormalizeMemberHealthState(string(prev.State))
	next.LastProbeAt = at.Format(time.RFC3339)
	next.UpdatedAt = next.LastProbeAt
	if engine := outcome.Engine; engine != "" {
		next.LastEngine = engine
	}
	if connectorType := outcome.ConnectorType; connectorType != "" {
		next.LastConnectorType = connectorType
	}

	if outcome.Ok {
		next.ConsecutiveSuccesses = prev.ConsecutiveSuccesses + 1
		next.ConsecutiveFailures = 0
		next.LastSuccessAt = next.LastProbeAt
		next.LastError = ""
		switch next.State {
		case MemberStateDegraded:
			next.State = MemberStateRecovering
		case MemberStateRecovering:
			if next.ConsecutiveSuccesses >= policy.RestoreSuccessThreshold {
				next.State = MemberStateHealthy
			}
		default:
			next.State = MemberStateHealthy
		}
	} else {
		next.ConsecutiveFailures = prev.ConsecutiveFailures + 1
		next.ConsecutiveSuccesses = 0
		next.LastFailureAt = next.LastProbeAt
		next.LastError = outcome.Error
		switch next.State {
		case MemberStateDegraded:
			next.State = MemberStateDegraded
		case MemberStateRecovering:
			next.State = MemberStateDegraded
		default:
			if next.ConsecutiveFailures >= policy.DegradeFailureThreshold {
				next.State = MemberStateDegraded
			} else {
				next.State = MemberStateSuspect
			}
		}
	}

	if next.ConsecutiveFailures > memberHealthCounterCeiling {
		next.ConsecutiveFailures = memberHealthCounterCeiling
	}
	if next.ConsecutiveSuccesses > memberHealthCounterCeiling {
		next.ConsecutiveSuccesses = memberHealthCounterCeiling
	}

	if delay := NextProbeDelay(next.State, next.ConsecutiveFailures, policy); delay > 0 {
		next.NextProbeAt = at.Add(delay).Format(time.RFC3339)
	} else {
		next.NextProbeAt = ""
	}
	return next
}

// NextProbeDelay 降级成员的探测退避：1m → 2m → 4m → 封顶 failureRetry。
// 返回 0 表示按常规调度（不需要单独排期）。
func NextProbeDelay(state MemberHealthState, failures int, policy MemberHealthPolicy) time.Duration {
	policy = policy.normalized()
	switch state {
	case MemberStateDegraded:
		excess := failures - policy.DegradeFailureThreshold
		if excess < 0 {
			excess = 0
		}
		delay := memberHealthBackoffFloor
		for i := 0; i < excess; i++ {
			delay *= 2
			if delay >= policy.FailureRetry {
				return policy.FailureRetry
			}
		}
		if delay > policy.FailureRetry {
			delay = policy.FailureRetry
		}
		return delay
	case MemberStateRecovering:
		return policy.FailureRetry
	default:
		return 0
	}
}
