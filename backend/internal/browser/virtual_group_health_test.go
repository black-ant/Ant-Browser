package browser

import (
	"testing"
	"time"
)

func testMemberPolicy() MemberHealthPolicy {
	return MemberHealthPolicy{
		DegradeFailureThreshold: 2,
		RestoreSuccessThreshold: 2,
		FailureRetry:            5 * time.Minute,
	}
}

func probe(ok bool, at time.Time) MemberProbeOutcome {
	outcome := MemberProbeOutcome{Ok: ok, At: at, Engine: "xray", ConnectorType: "xray"}
	if !ok {
		outcome.Error = "probe failed"
	}
	return outcome
}

func applyProbes(state ProxyVirtualGroupMemberState, policy MemberHealthPolicy, oks ...bool) ProxyVirtualGroupMemberState {
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, ok := range oks {
		state = NextMemberState(state, probe(ok, at), policy)
		at = at.Add(time.Minute)
	}
	return state
}

func TestNextMemberStateTransitions(t *testing.T) {
	policy := testMemberPolicy()
	initial := ProxyVirtualGroupMemberState{GroupId: "g1", ProxyId: "p1", State: MemberStateUnknown}

	tests := []struct {
		name  string
		from  MemberHealthState
		probes []bool
		want  MemberHealthState
	}{
		{name: "healthy one failure becomes suspect", from: MemberStateHealthy, probes: []bool{false}, want: MemberStateSuspect},
		{name: "healthy two failures becomes degraded", from: MemberStateHealthy, probes: []bool{false, false}, want: MemberStateDegraded},
		{name: "suspect success returns healthy", from: MemberStateSuspect, probes: []bool{true}, want: MemberStateHealthy},
		{name: "degraded single success enters recovering", from: MemberStateDegraded, probes: []bool{true}, want: MemberStateRecovering},
		{name: "recovering two successes restores healthy", from: MemberStateRecovering, probes: []bool{true, true}, want: MemberStateHealthy},
		{name: "recovering failure drops to degraded", from: MemberStateRecovering, probes: []bool{false}, want: MemberStateDegraded},
		{name: "degraded stays degraded on failure", from: MemberStateDegraded, probes: []bool{false}, want: MemberStateDegraded},
		{name: "unknown first failure becomes suspect", from: MemberStateUnknown, probes: []bool{false}, want: MemberStateSuspect},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := initial
			state.State = tc.from
			got := applyProbes(state, policy, tc.probes...)
			if got.State != tc.want {
				t.Fatalf("state = %s, want %s", got.State, tc.want)
			}
		})
	}
}

func TestNextMemberStateCountersReset(t *testing.T) {
	policy := testMemberPolicy()
	state := applyProbes(ProxyVirtualGroupMemberState{State: MemberStateUnknown}, policy, false, false)
	if state.ConsecutiveFailures != 2 || state.ConsecutiveSuccesses != 0 {
		t.Fatalf("after two failures = failures %d successes %d, want 2/0", state.ConsecutiveFailures, state.ConsecutiveSuccesses)
	}
	state = NextMemberState(state, probe(true, time.Now()), policy)
	if state.ConsecutiveFailures != 0 || state.ConsecutiveSuccesses != 1 {
		t.Fatalf("after one success = failures %d successes %d, want 0/1", state.ConsecutiveFailures, state.ConsecutiveSuccesses)
	}
}

// 反复失败/成功交替时不应来回降级，避免 flapping。
func TestNextMemberStateIgnoresFlapping(t *testing.T) {
	policy := testMemberPolicy()
	state := applyProbes(ProxyVirtualGroupMemberState{State: MemberStateHealthy}, policy, false, true, false, true, false, true)
	if state.State != MemberStateHealthy {
		t.Fatalf("flapping ended at %s, want healthy", state.State)
	}
}

func TestNextProbeDelayBackoff(t *testing.T) {
	policy := testMemberPolicy()
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 2, want: time.Minute},
		{failures: 3, want: 2 * time.Minute},
		{failures: 4, want: 4 * time.Minute},
		{failures: 5, want: 5 * time.Minute},
		{failures: 50, want: 5 * time.Minute},
	}
	for _, tc := range tests {
		if got := NextProbeDelay(MemberStateDegraded, tc.failures, policy); got != tc.want {
			t.Fatalf("failures %d delay = %s, want %s", tc.failures, got, tc.want)
		}
	}
	if got := NextProbeDelay(MemberStateHealthy, 0, policy); got != 0 {
		t.Fatalf("healthy delay = %s, want 0", got)
	}
}

func TestNextMemberStateSetsNextProbeAtForDegraded(t *testing.T) {
	policy := testMemberPolicy()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	state := NextMemberState(ProxyVirtualGroupMemberState{State: MemberStateSuspect, ConsecutiveFailures: 1}, probe(false, at), policy)
	if state.State != MemberStateDegraded {
		t.Fatalf("state = %s, want degraded", state.State)
	}
	next, err := time.Parse(time.RFC3339, state.NextProbeAt)
	if err != nil {
		t.Fatalf("NextProbeAt parse failed: %v", err)
	}
	if want := at.Add(time.Minute); !next.Equal(want) {
		t.Fatalf("NextProbeAt = %s, want %s", next, want)
	}

	recovered := NextMemberState(state, probe(true, at.Add(2*time.Minute)), policy)
	if recovered.NextProbeAt == "" {
		t.Fatal("recovering member should schedule a confirmation probe")
	}
}

func TestIsMemberStateUsable(t *testing.T) {
	usable := []MemberHealthState{MemberStateHealthy, MemberStateSuspect}
	unusable := []MemberHealthState{MemberStateDegraded, MemberStateRecovering, MemberStateUnknown}
	for _, state := range usable {
		if !IsMemberStateUsable(state) {
			t.Fatalf("state %s should be usable", state)
		}
	}
	for _, state := range unusable {
		if IsMemberStateUsable(state) {
			t.Fatalf("state %s should not be usable", state)
		}
	}
}
