package backend

import (
	"strings"
	"testing"
)

func TestGatingEnabled(t *testing.T) {
	if gatingEnabled(profileStartupActions{}) {
		t.Fatalf("no gating flags should report disabled")
	}
	if !gatingEnabled(profileStartupActions{KeepNetworkOn: true}) {
		t.Fatalf("keepNetworkOn should enable gating")
	}
	if !gatingEnabled(profileStartupActions{StopOnIpChange: true}) {
		t.Fatalf("stopOnIpChange should enable gating")
	}
	if !gatingEnabled(profileStartupActions{StopOnIpRegionChange: true}) {
		t.Fatalf("stopOnIpRegionChange should enable gating")
	}
}

func TestEvaluateProfileStartGateNetworkDown(t *testing.T) {
	// 网络门控关闭时，探测失败应放行且不更新基线。
	open := evaluateProfileStartGate(profileStartGateInput{ProbeOK: false, ProbeError: "timeout"})
	if open.Blocked {
		t.Fatalf("probe failure without keepNetworkOn should not block")
	}
	if open.StateChanged {
		t.Fatalf("probe failure should not change baseline")
	}

	// 网络门控开启时，探测失败应拦截。
	blocked := evaluateProfileStartGate(profileStartGateInput{KeepNetworkOn: true, ProbeOK: false, ProbeError: "timeout"})
	if !blocked.Blocked {
		t.Fatalf("probe failure with keepNetworkOn should block")
	}
	if !strings.Contains(blocked.Reason, "网络不通") {
		t.Fatalf("unexpected reason: %s", blocked.Reason)
	}
}

func TestEvaluateProfileStartGateFirstLaunchRecordsBaseline(t *testing.T) {
	d := evaluateProfileStartGate(profileStartGateInput{
		StopOnIpChange:       true,
		StopOnIpRegionChange: true,
		ProbeOK:              true,
		CurrentIP:            "1.2.3.4",
		CurrentCtry:          "US",
	})
	if d.Blocked {
		t.Fatalf("first launch with no baseline should not block")
	}
	if !d.StateChanged {
		t.Fatalf("first launch should record baseline")
	}
	if d.NextState.LastIP != "1.2.3.4" || d.NextState.LastCountry != "US" {
		t.Fatalf("baseline not recorded correctly: %+v", d.NextState)
	}
}

func TestEvaluateProfileStartGateIPChangeBlocks(t *testing.T) {
	in := profileStartGateInput{
		StopOnIpChange: true,
		Baseline:       profileRuntimeState{LastIP: "1.1.1.1", LastCountry: "US"},
		ProbeOK:        true,
		CurrentIP:      "2.2.2.2",
		CurrentCtry:    "US",
	}
	d := evaluateProfileStartGate(in)
	if !d.Blocked {
		t.Fatalf("changed IP with stopOnIpChange should block")
	}
	if !strings.Contains(d.Reason, "出口 IP 发生变化") {
		t.Fatalf("unexpected reason: %s", d.Reason)
	}

	// 同一 IP 不应拦截。
	in.CurrentIP = "1.1.1.1"
	if evaluateProfileStartGate(in).Blocked {
		t.Fatalf("same IP should not block")
	}
}

func TestEvaluateProfileStartGateCountryChangeBlocks(t *testing.T) {
	in := profileStartGateInput{
		StopOnIpRegionChange: true,
		Baseline:             profileRuntimeState{LastIP: "1.1.1.1", LastCountry: "US"},
		ProbeOK:              true,
		CurrentIP:            "1.1.1.1",
		CurrentCtry:          "JP",
	}
	d := evaluateProfileStartGate(in)
	if !d.Blocked {
		t.Fatalf("changed country with stopOnIpRegionChange should block")
	}
	if !strings.Contains(d.Reason, "国家/地区发生变化") {
		t.Fatalf("unexpected reason: %s", d.Reason)
	}

	// 国家大小写归一化：us == US 不拦截。
	in.CurrentCtry = "us"
	if evaluateProfileStartGate(in).Blocked {
		t.Fatalf("same country (case-insensitive) should not block")
	}
}

func TestEvaluateProfileStartGateIPChangeWithoutGateRefreshesBaseline(t *testing.T) {
	// 仅开启国家门控时，IP 变化（国家不变）不拦截，但应刷新 IP 基线。
	d := evaluateProfileStartGate(profileStartGateInput{
		StopOnIpRegionChange: true,
		Baseline:             profileRuntimeState{LastIP: "1.1.1.1", LastCountry: "US"},
		ProbeOK:              true,
		CurrentIP:            "9.9.9.9",
		CurrentCtry:          "US",
	})
	if d.Blocked {
		t.Fatalf("IP change without stopOnIpChange should not block")
	}
	if !d.StateChanged || d.NextState.LastIP != "9.9.9.9" {
		t.Fatalf("IP baseline should refresh: %+v", d)
	}
}

func TestSetProfileConfigRuntimeState(t *testing.T) {
	// 空配置：构造最小合法对象。
	out, changed, err := setProfileConfigRuntimeState("", profileRuntimeState{LastIP: "1.2.3.4", LastCountry: "us"})
	if err != nil || !changed {
		t.Fatalf("empty config should change, err=%v changed=%v", err, changed)
	}
	if !strings.Contains(out, "\"lastIp\":\"1.2.3.4\"") || !strings.Contains(out, "\"lastCountry\":\"US\"") {
		t.Fatalf("runtimeState not written/normalized: %s", out)
	}

	// 保留已有键。
	out2, changed2, err := setProfileConfigRuntimeState(`{"version":1,"formState":{"a":1}}`, profileRuntimeState{LastIP: "5.5.5.5"})
	if err != nil || !changed2 {
		t.Fatalf("should change, err=%v changed=%v", err, changed2)
	}
	if !strings.Contains(out2, "\"version\":1") || !strings.Contains(out2, "formState") {
		t.Fatalf("existing keys lost: %s", out2)
	}

	// 相同值不再写回。
	_, changed3, err := setProfileConfigRuntimeState(out, profileRuntimeState{LastIP: "1.2.3.4", LastCountry: "US"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed3 {
		t.Fatalf("identical runtimeState should not report change")
	}

	// 非法 JSON：报错，不覆盖。
	if _, _, err := setProfileConfigRuntimeState("{not json", profileRuntimeState{LastIP: "x"}); err == nil {
		t.Fatalf("invalid JSON should return error")
	}
}

func TestEvaluateProfileStartGateReminderOnIPChange(t *testing.T) {
	// 开启提醒、未开启停止门控，IP 相对基线变化 → 放行但带提醒。
	decision := evaluateProfileStartGate(profileStartGateInput{
		IPChangeReminder: true,
		Baseline:         profileRuntimeState{LastIP: "1.1.1.1", LastCountry: "US"},
		ProbeOK:          true,
		CurrentIP:        "2.2.2.2",
		CurrentCtry:      "US",
	})
	if decision.Blocked {
		t.Fatalf("提醒不应拦截启动")
	}
	if decision.ReminderReason == "" {
		t.Fatalf("IP 变化应生成提醒原因: %+v", decision)
	}
	if !decision.StateChanged || decision.NextState.LastIP != "2.2.2.2" {
		t.Fatalf("应刷新基线: %+v", decision)
	}
}

func TestEvaluateProfileStartGateReminderFirstLaunchNoReminder(t *testing.T) {
	// 首次启动（无基线）即使开启提醒也不应提醒。
	decision := evaluateProfileStartGate(profileStartGateInput{
		IPChangeReminder: true,
		Baseline:         profileRuntimeState{},
		ProbeOK:          true,
		CurrentIP:        "2.2.2.2",
		CurrentCtry:      "US",
	})
	if decision.ReminderReason != "" {
		t.Fatalf("首次记录基线不应提醒: %+v", decision)
	}
	if !decision.StateChanged {
		t.Fatalf("首次应记录基线: %+v", decision)
	}
}

func TestEvaluateProfileStartGateReminderNoChangeNoReminder(t *testing.T) {
	decision := evaluateProfileStartGate(profileStartGateInput{
		IPChangeReminder: true,
		Baseline:         profileRuntimeState{LastIP: "1.1.1.1", LastCountry: "US"},
		ProbeOK:          true,
		CurrentIP:        "1.1.1.1",
		CurrentCtry:      "US",
	})
	if decision.ReminderReason != "" {
		t.Fatalf("IP 未变化不应提醒: %+v", decision)
	}
}
