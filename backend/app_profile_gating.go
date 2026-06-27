package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// profileStartGateInput 是一次启动门控判定所需的全部输入。
type profileStartGateInput struct {
	KeepNetworkOn        bool
	StopOnIpChange       bool
	StopOnIpRegionChange bool
	IPChangeReminder     bool
	Baseline             profileRuntimeState
	// Probe 出口 IP 探测结果
	ProbeOK     bool
	ProbeError  string
	CurrentIP   string
	CurrentCtry string
}

// profileStartGateDecision 是门控判定输出。Blocked=true 表示应中止启动。
type profileStartGateDecision struct {
	Blocked      bool
	Reason       string
	NextState    profileRuntimeState
	StateChanged bool
	// ReminderReason 非空表示：相对已有基线检测到出口 IP / 国家变化，但未被门控拦截，
	// 应向前端发提醒（仅在开启 IPChangeReminder 时填充）。首次记录基线不算变化。
	ReminderReason string
}

// gatingEnabled 报告该窗口是否需要在启动前探测出口 IP
// （任一停止门控或 IP 变化提醒开启时都需要探测）。
func gatingEnabled(actions profileStartupActions) bool {
	return actions.KeepNetworkOn || actions.StopOnIpChange || actions.StopOnIpRegionChange || actions.IPChangeReminder
}

// evaluateProfileStartGate 是纯函数门控判定，便于单测。它不发起任何网络请求。
//
// 规则：
//   - 探测失败：仅当开启「网络不通停止」时拦截；否则放行（探测失败不更新基线）。
//   - 探测成功：与基线比对。
//   - IP 变化（开启 stopOnIpChange 且存在 IP 基线且不同）→ 拦截。
//   - 国家变化（开启 stopOnIpRegionChange 且存在国家基线且不同）→ 拦截。
//   - 首次启动（无基线）放行，并记录当前 IP / 国家为基线。
//   - 放行时若当前值与基线不同（但未开启对应拦截），同样刷新基线。
func evaluateProfileStartGate(in profileStartGateInput) profileStartGateDecision {
	if !in.ProbeOK {
		if in.KeepNetworkOn {
			reason := "网络不通，已停止打开窗口"
			if strings.TrimSpace(in.ProbeError) != "" {
				reason = fmt.Sprintf("网络不通，已停止打开窗口：%s", in.ProbeError)
			}
			return profileStartGateDecision{Blocked: true, Reason: reason}
		}
		// 未开启网络门控时，探测失败不影响启动，也不更新基线。
		return profileStartGateDecision{Blocked: false}
	}

	currentIP := strings.TrimSpace(in.CurrentIP)
	currentCtry := normalizeGateCountry(in.CurrentCtry)
	baseIP := strings.TrimSpace(in.Baseline.LastIP)
	baseCtry := normalizeGateCountry(in.Baseline.LastCountry)

	if in.StopOnIpChange && baseIP != "" && currentIP != "" && !strings.EqualFold(currentIP, baseIP) {
		return profileStartGateDecision{
			Blocked: true,
			Reason:  fmt.Sprintf("检测到出口 IP 发生变化（%s → %s），已停止打开窗口", baseIP, currentIP),
		}
	}

	if in.StopOnIpRegionChange && baseCtry != "" && currentCtry != "" && baseCtry != currentCtry {
		return profileStartGateDecision{
			Blocked: true,
			Reason:  fmt.Sprintf("检测到出口 IP 所属国家/地区发生变化（%s → %s），已停止打开窗口", baseCtry, currentCtry),
		}
	}

	// 放行：刷新基线（只覆盖探测到的非空值，保留另一字段的历史值）。
	next := in.Baseline
	changed := false
	ipChanged := currentIP != "" && baseIP != "" && !strings.EqualFold(currentIP, baseIP)
	ctryChanged := currentCtry != "" && baseCtry != "" && currentCtry != baseCtry
	if currentIP != "" && currentIP != baseIP {
		next.LastIP = currentIP
		changed = true
	}
	if currentCtry != "" && !strings.EqualFold(currentCtry, baseCtry) {
		next.LastCountry = currentCtry
		changed = true
	}

	// IP 变化提醒：仅在存在旧基线且确实变化、且未被任何停止门控拦截时填充。
	// 首次记录基线（baseIP/baseCtry 为空）不算变化，不提醒。
	reminder := ""
	if in.IPChangeReminder {
		if ipChanged {
			reminder = fmt.Sprintf("出口 IP 已变化：%s → %s", baseIP, currentIP)
		} else if ctryChanged {
			reminder = fmt.Sprintf("出口 IP 所属国家/地区已变化：%s → %s", baseCtry, currentCtry)
		}
	}
	return profileStartGateDecision{Blocked: false, NextState: next, StateChanged: changed, ReminderReason: reminder}
}

func normalizeGateCountry(country string) string {
	c := strings.ToUpper(strings.TrimSpace(country))
	return c
}

// applyProfileStartGate 在启动前执行门控：探测出口 IP（经实际生效的代理链路），
// 判定是否放行，并在放行时持久化新的基线。返回非 nil error 表示应中止启动。
//
// effectiveProxyConfig 为本次启动实际使用的代理配置（可能是 direct:// 或桥接前的原始配置）。
// 探测沿用 IPPure 健康查询：对 direct:// 走本机直连，对代理走对应链路。
func (a *App) applyProfileStartGate(profileId string, actions profileStartupActions, proxyId string, effectiveProxyConfig string) error {
	if !gatingEnabled(actions) {
		return nil
	}

	log := logger.New("Browser")
	probeOK, currentIP, currentCtry, probeErr := a.probeProfileExitIP(proxyId, effectiveProxyConfig)

	decision := evaluateProfileStartGate(profileStartGateInput{
		KeepNetworkOn:        actions.KeepNetworkOn,
		StopOnIpChange:       actions.StopOnIpChange,
		StopOnIpRegionChange: actions.StopOnIpRegionChange,
		Baseline:             actions.Baseline,
		IPChangeReminder:     actions.IPChangeReminder,
		ProbeOK:              probeOK,
		ProbeError:           probeErr,
		CurrentIP:            currentIP,
		CurrentCtry:          currentCtry,
	})

	if decision.Blocked {
		log.Warn("启动门控拦截",
			logger.F("profile_id", profileId),
			logger.F("reason", decision.Reason),
			logger.F("current_ip", currentIP),
			logger.F("current_country", currentCtry),
			logger.F("baseline_ip", actions.Baseline.LastIP),
			logger.F("baseline_country", actions.Baseline.LastCountry),
		)
		return fmt.Errorf("%s", decision.Reason)
	}

	if decision.StateChanged {
		if err := a.persistProfileRuntimeState(profileId, decision.NextState); err != nil {
			// 基线持久化失败不应阻断启动，仅记录。
			log.Warn("启动门控基线持久化失败",
				logger.F("profile_id", profileId),
				logger.F("error", err.Error()))
		}
	}

	if decision.ReminderReason != "" {
		log.Info("出口 IP 变化提醒",
			logger.F("profile_id", profileId),
			logger.F("reason", decision.ReminderReason))
		a.emitProfileIPChangeReminder(profileId, decision.ReminderReason)
	}
	return nil
}

// emitProfileIPChangeReminder 向前端发送出口 IP 变化提醒事件（不阻断启动）。
func (a *App) emitProfileIPChangeReminder(profileId string, reason string) {
	if a == nil || a.ctx == nil {
		return
	}
	profileName := ""
	a.browserMgr.Mutex.Lock()
	if p, ok := a.browserMgr.Profiles[profileId]; ok && p != nil {
		profileName = p.ProfileName
	}
	a.browserMgr.Mutex.Unlock()
	runtime.EventsEmit(a.ctx, "browser:instance:ipChanged", map[string]interface{}{
		"profileId":   profileId,
		"profileName": profileName,
		"reason":      reason,
	})
}

// probeProfileExitIP 经实际代理链路查询出口 IP 健康信息，返回 (ok, ip, country, errMsg)。
func (a *App) probeProfileExitIP(proxyId string, effectiveProxyConfig string) (bool, string, string, string) {
	proxies := a.getLatestProxies()
	var data map[string]interface{}
	var err error

	if id := strings.TrimSpace(proxyId); id != "" {
		data, err = proxy.FetchIPPureInfo(id, proxies, a.xrayMgr, a.singboxMgr)
	} else {
		cfg := strings.TrimSpace(effectiveProxyConfig)
		if cfg == "" {
			cfg = "direct://"
		}
		data, err = proxy.FetchIPPureInfoByConfig(cfg, proxies, a.xrayMgr, a.singboxMgr)
	}

	result := buildProxyIPHealthResult(proxyId, data, err)
	if !result.Ok {
		return false, "", "", result.Error
	}
	return true, result.IP, result.Country, ""
}

// persistProfileRuntimeState 把门控基线写回 profileConfig 的 runtimeState 键并持久化。
func (a *App) persistProfileRuntimeState(profileId string, state profileRuntimeState) error {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists || profile == nil {
		return fmt.Errorf("profile not found")
	}
	nextConfig, changed, err := setProfileConfigRuntimeState(profile.ProfileConfig, state)
	if err != nil || !changed {
		return err
	}
	profile.ProfileConfig = nextConfig
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	if a.browserMgr.ProfileDAO != nil {
		return a.browserMgr.ProfileDAO.Upsert(profile)
	}
	return a.browserMgr.SaveProfiles()
}

// setProfileConfigRuntimeState 在 profileConfig JSON 顶层写入/更新 runtimeState 键。
// 对空配置或非法 JSON，构造一个最小的合法配置对象。changed=false 表示无需写回。
func setProfileConfigRuntimeState(raw string, state profileRuntimeState) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	cfg := map[string]any{}
	if trimmed != "" && trimmed != "{}" {
		if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
			// 非法 JSON：不冒险覆盖用户配置，直接报错由上层静默处理。
			return raw, false, err
		}
	}

	nextState := map[string]any{
		"lastIp":      strings.TrimSpace(state.LastIP),
		"lastCountry": normalizeGateCountry(state.LastCountry),
	}
	if existing, ok := cfg["runtimeState"].(map[string]any); ok {
		if fmt.Sprint(existing["lastIp"]) == nextState["lastIp"] &&
			fmt.Sprint(existing["lastCountry"]) == nextState["lastCountry"] {
			return raw, false, nil
		}
	}
	cfg["runtimeState"] = nextState

	out, err := json.Marshal(cfg)
	if err != nil {
		return raw, false, err
	}
	return string(out), true, nil
}
