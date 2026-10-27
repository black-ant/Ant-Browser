package backend

import (
	"sort"
	"strings"

	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
)

const (
	profileProxyBridgeEngineXray    = "xray"
	profileProxyBridgeEngineSingBox = "sing-box"
	profileProxyBridgeEngineMihomo  = "mihomo"
	profileProxyBridgeEngineMieru   = "mieru"
)

type profileProxyBridgeRef struct {
	Engine  string
	Key     string
	ProxyID string
}

func (ref profileProxyBridgeRef) valid() bool {
	return strings.TrimSpace(ref.Engine) != "" && strings.TrimSpace(ref.Key) != ""
}

func newProfileProxyBridgeRef(engine string, key string, proxyID string) profileProxyBridgeRef {
	return profileProxyBridgeRef{
		Engine:  strings.TrimSpace(engine),
		Key:     strings.TrimSpace(key),
		ProxyID: strings.TrimSpace(proxyID),
	}
}

func (a *App) bindProfileProxyBridge(profileId string, ref profileProxyBridgeRef) {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" || !ref.valid() {
		return
	}

	a.bridgeMu.Lock()
	if a.profileBridgeRefs == nil {
		a.profileBridgeRefs = make(map[string]profileProxyBridgeRef)
	}
	a.profileBridgeRefs[profileId] = ref
	a.bridgeMu.Unlock()
}

func (a *App) releaseProfileProxyBridge(profileId string) {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return
	}

	a.bridgeMu.Lock()
	ref := a.profileBridgeRefs[profileId]
	delete(a.profileBridgeRefs, profileId)
	a.bridgeMu.Unlock()

	a.releaseProxyBridgeRef(ref)
}

func (a *App) releaseProxyBridgeRef(ref profileProxyBridgeRef) {
	if !ref.valid() {
		return
	}
	switch ref.Engine {
	case profileProxyBridgeEngineXray:
		if a.xrayMgr != nil {
			a.xrayMgr.ReleaseBridge(ref.Key)
		}
	case profileProxyBridgeEngineSingBox:
		if a.singboxMgr != nil {
			a.singboxMgr.ReleaseBridge(ref.Key)
		}
	case profileProxyBridgeEngineMihomo:
		if a.clashMgr != nil {
			a.clashMgr.ReleaseNodeBridge(ref.Key)
		}
	case profileProxyBridgeEngineMieru:
		if a.mieruMgr != nil {
			a.mieruMgr.ReleaseBridge(ref.Key)
		}
	}
}

func (a *App) clearProfileProxyBridges() {
	a.bridgeMu.Lock()
	refs := make([]profileProxyBridgeRef, 0, len(a.profileBridgeRefs))
	for _, ref := range a.profileBridgeRefs {
		refs = append(refs, ref)
	}
	a.profileBridgeRefs = make(map[string]profileProxyBridgeRef)
	a.bridgeMu.Unlock()

	for _, ref := range refs {
		a.releaseProxyBridgeRef(ref)
	}
}

func (a *App) handleProxyBridgeDied(engine string, key string, err error) {
	engine = strings.TrimSpace(engine)
	key = strings.TrimSpace(key)
	if a == nil || engine == "" || key == "" {
		return
	}
	profileIDs := make([]string, 0)
	proxyIDs := make(map[string]string)
	a.bridgeMu.Lock()
	for profileID, ref := range a.profileBridgeRefs {
		if !strings.EqualFold(strings.TrimSpace(ref.Engine), engine) || strings.TrimSpace(ref.Key) != key {
			continue
		}
		profileIDs = append(profileIDs, profileID)
		if proxyID := strings.TrimSpace(ref.ProxyID); proxyID != "" {
			proxyIDs[strings.ToLower(proxyID)] = proxyID
		}
	}
	a.bridgeMu.Unlock()
	if a.browserMgr != nil && a.config != nil {
		for _, item := range a.getLatestProxies() {
			if proxy.ComputeBridgeKey(item.ProxyConfig, item.DnsServers, engine) != key {
				continue
			}
			proxyID := strings.TrimSpace(item.ProxyId)
			if proxyID != "" {
				proxyIDs[strings.ToLower(proxyID)] = proxyID
			}
		}
	}
	proxyIDList := make([]string, 0, len(proxyIDs))
	for _, proxyID := range proxyIDs {
		proxyIDList = append(proxyIDList, proxyID)
		a.recordProxyBridgeFailure(proxyID)
	}
	sort.Strings(profileIDs)
	sort.Strings(proxyIDList)
	a.emitRuntimeEvent("proxy:bridge:died", map[string]interface{}{
		"engine":     engine,
		"key":        bridgeEventKey(key),
		"error":      errorMessage(err),
		"profileIds": profileIDs,
		"proxyIds":   proxyIDList,
	})
}

func (a *App) recordProxyBridgeFailure(proxyID string) {
	proxyID = strings.TrimSpace(proxyID)
	if a == nil || proxyID == "" {
		return
	}
	if a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	if invalidator, ok := a.browserMgr.ProxyDAO.(interface{ InvalidateSpeedResult(string) error }); ok {
		if err := invalidator.InvalidateSpeedResult(proxyID); err != nil {
			logger.New("ProxyBridge").Warn("桥接故障后清除测速结果失败", logger.F("proxy_id", proxyID), logger.F("error", err))
		}
		return
	}
	if err := a.browserMgr.ProxyDAO.UpdateSpeedResult(proxyID, false, -1, ""); err != nil {
		logger.New("ProxyBridge").Warn("桥接故障后清除测速结果失败", logger.F("proxy_id", proxyID), logger.F("error", err))
	}
}

func (a *App) reportProxyBridgeFailure(profileID string, profileName string, proxyID string, engine string, cause error) {
	a.recordProxyBridgeFailure(proxyID)
	a.emitRuntimeEvent("proxy:bridge:failed", map[string]interface{}{
		"profileId":   profileID,
		"profileName": profileName,
		"proxyId":     proxyID,
		"engine":      engine,
		"error":       errorMessage(cause),
	})
}
