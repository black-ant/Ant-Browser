package backend

import "strings"

const (
	profileProxyBridgeEngineXray    = "xray"
	profileProxyBridgeEngineSingBox = "sing-box"
	profileProxyBridgeEngineMihomo  = "mihomo"
)

type profileProxyBridgeRef struct {
	Engine string
	Key    string
}

func (ref profileProxyBridgeRef) valid() bool {
	return strings.TrimSpace(ref.Engine) != "" && strings.TrimSpace(ref.Key) != ""
}

func newProfileProxyBridgeRef(engine string, key string) profileProxyBridgeRef {
	return profileProxyBridgeRef{
		Engine: strings.TrimSpace(engine),
		Key:    strings.TrimSpace(key),
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
