package backend

import "strings"

func (a *App) ensureProfileLaunchCode(profile *BrowserProfile) string {
	if profile == nil {
		return ""
	}

	profileID := strings.TrimSpace(profile.ProfileId)
	if profileID == "" {
		return strings.TrimSpace(profile.LaunchCode)
	}
	if a == nil || a.launchCodeSvc == nil {
		return strings.TrimSpace(profile.LaunchCode)
	}

	code, err := a.launchCodeSvc.EnsureCode(profileID)
	if err != nil {
		return strings.TrimSpace(profile.LaunchCode)
	}

	code = strings.TrimSpace(code)
	if code != "" {
		profile.LaunchCode = code
	}
	return code
}

func (a *App) setManagedProfileLaunchCode(profileID string, launchCode string) {
	if a == nil || a.browserMgr == nil {
		return
	}

	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, ok := a.browserMgr.Profiles[profileID]
	if !ok || profile == nil {
		return
	}
	profile.LaunchCode = strings.TrimSpace(launchCode)
}
