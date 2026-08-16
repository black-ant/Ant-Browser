package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var startBrowserWindowProcess = func(chromeBinaryPath string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(chromeBinaryPath, args...)
	cmd.Dir = filepath.Dir(chromeBinaryPath)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func tryCloseBrowserViaCDP(debugPort int, timeout time.Duration) bool {
	if debugPort <= 0 || !canConnectDebugPort(debugPort, 250*time.Millisecond) {
		return false
	}

	_ = cdpBrowserCall(debugPort, "Browser.close", nil)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !canConnectDebugPort(debugPort, 250*time.Millisecond) {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

func normalizeNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func ensureNewWindowLaunchArg(args []string) []string {
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), "--new-window") {
			return args
		}
	}
	return append(args, "--new-window")
}

func browserDefaultStartURLs(cfg *config.Config) []string {
	if cfg != nil && cfg.Browser.DefaultStartURLs != nil {
		return normalizeNonEmptyStrings(cfg.Browser.DefaultStartURLs)
	}
	return config.DefaultBrowserStartURLs()
}

func (a *App) browserDefaultStartURLs() []string {
	return mergeStartURLs(browserDefaultStartURLs(a.config), bookmarkStartURLs(a.BookmarkList()))
}

func bookmarkStartURLs(bookmarks []BrowserBookmark) []string {
	if len(bookmarks) == 0 {
		return nil
	}
	urls := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		if bookmark.OpenOnStart {
			urls = append(urls, bookmark.URL)
		}
	}
	return normalizeNonEmptyStrings(urls)
}

func mergeStartURLs(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, group := range groups {
		for _, item := range normalizeNonEmptyStrings(group) {
			key := strings.ToLower(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func browserRestoreLastSession(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Browser.RestoreLastSession
}

func profileRestoreLastSession(profile *BrowserProfile, cfg *config.Config) bool {
	if profile == nil {
		return browserRestoreLastSession(cfg)
	}
	switch browser.NormalizeRestoreLastSessionMode(profile.RestoreLastSession) {
	case browser.RestoreLastSessionEnabled:
		return true
	case browser.RestoreLastSessionDisabled:
		return false
	default:
		return browserRestoreLastSession(cfg)
	}
}

func appendLaunchTargets(args []string, startURLs []string, defaultStartURLs []string, skipDefaultStartURLs bool, restoreLastSession bool) []string {
	launchTargets, _, _ := buildBrowserLaunchTargets(startURLs, defaultStartURLs, skipDefaultStartURLs, restoreLastSession, false)
	return browser.BuildLaunchArgs(args, launchTargets)
}

func (a *App) markProfileStoppedLocked(profileId string, profile *BrowserProfile) {
	if profile == nil {
		return
	}
	profile.Running = false
	profile.DebugReady = false
	profile.Pid = 0
	profile.DebugPort = 0
	profile.RuntimeWarning = ""
	profile.LastStopAt = time.Now().Format(time.RFC3339)
	delete(a.browserMgr.BrowserProcesses, profileId)
	a.clearDeferredStartTargets(profileId)
	a.releaseProfileProxyBridge(profileId)
	if a.launchServer != nil {
		a.launchServer.ClearActiveProfile(profileId)
	}
}

func (a *App) openBrowserWindowForRunningProfile(profile *BrowserProfile, extraLaunchArgs []string, startURLs []string) error {
	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(profile)
	if err != nil {
		return err
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return fmt.Errorf("无法创建用户数据目录 %s：%w", userDataDir, err)
	}
	ensureLaunchPreferences(userDataDir)

	args := []string{
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
	}
	sanitizedExtraLaunchArgs, managedExtraArgs := sanitizeManagedLaunchArgs(extraLaunchArgs)
	logManagedLaunchArgOverrides(logger.New("Browser"), profile.ProfileId, "running-window.extraLaunchArgs", managedExtraArgs)
	args = append(args, sanitizedExtraLaunchArgs...)
	if len(startURLs) > 0 {
		args = append(args, startURLs...)
	} else {
		args = append(args, "about:blank")
	}

	cmd, err := startBrowserWindowProcess(chromeBinaryPath, args)
	if err != nil {
		return fmt.Errorf("%s", describeChromeProcessStartError(chromeBinaryPath, err))
	}

	if cmd != nil {
		go func() {
			_ = cmd.Wait()
		}()
	}
	return nil
}

func (a *App) openBrowserTabForRunningProfile(profile *BrowserProfile, extraLaunchArgs []string, startURLs []string) error {
	explicitTargets := normalizeNonEmptyStrings(startURLs)
	targets := explicitTargets
	if len(targets) == 0 {
		targets = []string{"about:blank"}
	}
	if profile != nil && profile.DebugReady && profile.DebugPort > 0 {
		for _, target := range targets {
			if err := createBrowserStartTarget(profile.DebugPort, target); err != nil {
				if len(explicitTargets) == 0 && len(normalizeNonEmptyStrings(extraLaunchArgs)) == 0 {
					return nil
				}
				return err
			}
		}
		return nil
	}
	err := a.openBrowserWindowForRunningProfile(profile, extraLaunchArgs, targets)
	if err != nil && len(explicitTargets) == 0 && len(normalizeNonEmptyStrings(extraLaunchArgs)) == 0 {
		return nil
	}
	return err
}
