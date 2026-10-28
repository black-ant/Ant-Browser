package browser

import "strings"

// 虚拟分组成员健康状态。
//
// 状态机只描述"这个成员在当前连接栈下最近是否可用"，与代理池的 last_test_ok 分开存储：
// 同一个代理可以同时属于多个虚拟分组，健康判定需要按 (group, proxy) 维度独立演进。
type MemberHealthState string

const (
	// MemberStateUnknown 尚未观测：不参与健康统计，也不会被硬性排除。
	MemberStateUnknown MemberHealthState = "unknown"
	// MemberStateHealthy 健康：正常参与轮询。
	MemberStateHealthy MemberHealthState = "healthy"
	// MemberStateSuspect 可疑：连续失败但未达降级阈值，仍可用但排在最后。
	MemberStateSuspect MemberHealthState = "suspect"
	// MemberStateDegraded 已降级：移出轮询池，按退避间隔继续探测。
	MemberStateDegraded MemberHealthState = "degraded"
	// MemberStateRecovering 恢复观察：已成功但未达升级阈值。
	MemberStateRecovering MemberHealthState = "recovering"
)

// ProxyVirtualGroupMemberState 单个虚拟分组成员的健康状态。
type ProxyVirtualGroupMemberState struct {
	GroupId              string
	ProxyId              string
	State                MemberHealthState
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastProbeAt          string
	LastSuccessAt        string
	LastFailureAt        string
	NextProbeAt          string
	LastError            string
	LastEngine           string
	LastConnectorType    string
	UpdatedAt            string
}

// ProxyVirtualGroupMemberStateDAO 虚拟分组成员健康状态持久化接口。
type ProxyVirtualGroupMemberStateDAO interface {
	List() ([]ProxyVirtualGroupMemberState, error)
	ListByGroup(groupId string) ([]ProxyVirtualGroupMemberState, error)
	Upsert(state ProxyVirtualGroupMemberState) error
	DeleteByGroup(groupId string) error
	// ResetByGroup 把整个分组重置为未观测，用于手动清除降级。
	ResetByGroup(groupId string) error
}

// NormalizeMemberHealthState 归一化状态字符串，未知值一律降级为 unknown。
func NormalizeMemberHealthState(value string) MemberHealthState {
	switch MemberHealthState(strings.ToLower(strings.TrimSpace(value))) {
	case MemberStateHealthy:
		return MemberStateHealthy
	case MemberStateSuspect:
		return MemberStateSuspect
	case MemberStateDegraded:
		return MemberStateDegraded
	case MemberStateRecovering:
		return MemberStateRecovering
	default:
		return MemberStateUnknown
	}
}

// IsMemberStateUsable 该状态是否仍可作为轮询候选。
// suspect 可用但应排在健康节点之后；degraded / recovering 不参与轮询。
func IsMemberStateUsable(state MemberHealthState) bool {
	switch state {
	case MemberStateHealthy, MemberStateSuspect:
		return true
	default:
		return false
	}
}

// MemberStateSortRank 轮询排序用的等级（越小越优先）。
func MemberStateSortRank(state MemberHealthState) int {
	switch state {
	case MemberStateHealthy:
		return 0
	case MemberStateSuspect:
		return 1
	case MemberStateUnknown:
		return 2
	case MemberStateRecovering:
		return 3
	case MemberStateDegraded:
		return 4
	default:
		return 5
	}
}
