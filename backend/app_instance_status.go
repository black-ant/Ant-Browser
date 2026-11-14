package backend

import (
	"fmt"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/logger"
)

func (a *App) BrowserInstanceStatus(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	a.ensureProfileLaunchCode(profile)
	if !profile.Running {
		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		if detection, ok := detectBrowserRuntimeByUserDataDir(userDataDir); ok && detection.DebugReady {
			a.markProfileRunningLocked(profileId, profile, nil, detection.PID, detection.DebugPort, true, "")
			logger.New("Browser").Warn("状态查询发现同一用户数据目录浏览器已运行，已同步实例状态",
				logger.F("profile_id", profileId),
				logger.F("user_data_dir", userDataDir),
				logger.F("pid", detection.PID),
				logger.F("debug_port", detection.DebugPort),
			)
		}
	}
	return profile, nil
}

func (a *App) BrowserInstanceOpenUrl(profileId string, targetUrl string) (bool, error) {
	profileId = strings.TrimSpace(profileId)
	normalizedTargetURL := strings.TrimSpace(targetUrl)
	if normalizedTargetURL == "" {
		return false, fmt.Errorf("打开地址失败：目标地址不能为空")
	}

	log := logger.New("Browser")

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		a.browserMgr.Mutex.Unlock()
		return false, fmt.Errorf("打开地址失败：未找到实例配置（ID=%s）。请刷新列表后重试。", profileId)
	}
	a.ensureProfileLaunchCode(profile)
	trackedCmd := a.browserMgr.BrowserProcesses[profileId]
	if !profile.Running {
		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		if detection, ok := detectBrowserRuntimeByUserDataDir(userDataDir); ok && detection.DebugReady {
			a.markProfileRunningLocked(profileId, profile, nil, detection.PID, detection.DebugPort, true, "")
			log.Warn("打开地址前发现同一用户数据目录浏览器已运行，已同步实例状态",
				logger.F("profile_id", profileId),
				logger.F("user_data_dir", userDataDir),
				logger.F("pid", detection.PID),
				logger.F("debug_port", detection.DebugPort),
			)
		} else {
			a.browserMgr.Mutex.Unlock()
			return false, fmt.Errorf("打开地址失败：实例当前未运行，请先启动实例后再试。")
		}
	}
	if !isBrowserProfileLive(profile, trackedCmd) {
		staleDebugPort := profile.DebugPort
		stalePID := profile.Pid
		a.markProfileStoppedLocked(profileId, profile)
		profile.LastError = "打开地址失败：检测到实例运行状态已失效，请先重新启动实例。"
		a.browserMgr.Mutex.Unlock()

		log.Warn("检测到实例运行状态已失效，取消复用打开地址",
			logger.F("profile_id", profileId),
			logger.F("debug_port", staleDebugPort),
			logger.F("pid", stalePID),
		)
		a.emitRuntimeEvent("browser:instance:stopped", profileId)
		return false, fmt.Errorf("%s", profile.LastError)
	}

	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	fingerprintExpectedArgs := a.fingerprintCheckExpectedArgsFromProfile(snapshot)
	normalizedTargetURL = a.resolveFingerprintCheckStartURLForExpectedArgsAndProfile(snapshot.ProfileId, fingerprintExpectedArgs, snapshot, normalizedTargetURL)

	if snapshot.DebugReady && snapshot.DebugPort > 0 {
		if err := createBrowserStartTarget(snapshot.DebugPort, normalizedTargetURL); err == nil {
			log.Info("复用运行中实例通过 CDP 打开地址",
				logger.F("profile_id", profileId),
				logger.F("debug_port", snapshot.DebugPort),
				logger.F("target_url", normalizedTargetURL),
			)
			return true, nil
		} else {
			log.Warn("运行中实例通过 CDP 打开地址失败，回退到浏览器进程唤起",
				logger.F("profile_id", profileId),
				logger.F("debug_port", snapshot.DebugPort),
				logger.F("target_url", normalizedTargetURL),
				logger.F("error", err.Error()),
			)
		}
	}

	if err := a.openBrowserWindowForRunningProfile(snapshot, nil, []string{normalizedTargetURL}); err != nil {
		openErr := fmt.Errorf("打开地址失败：实例运行中，但复用现有会话打开页面失败：%w", err)
		log.Error("运行中实例打开地址失败",
			logger.F("profile_id", profileId),
			logger.F("debug_ready", snapshot.DebugReady),
			logger.F("debug_port", snapshot.DebugPort),
			logger.F("target_url", normalizedTargetURL),
			logger.F("error", err.Error()),
			logger.F("reason", openErr.Error()),
		)
		return false, openErr
	}

	log.Info("复用运行中实例通过浏览器进程打开地址",
		logger.F("profile_id", profileId),
		logger.F("debug_ready", snapshot.DebugReady),
		logger.F("debug_port", snapshot.DebugPort),
		logger.F("target_url", normalizedTargetURL),
	)
	return true, nil
}

type browserDebugTabTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (a *App) BrowserInstanceGetTabs(profileId string) []BrowserTab {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return []BrowserTab{}
	}

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists || !profile.Running || !profile.DebugReady || profile.DebugPort <= 0 {
		a.browserMgr.Mutex.Unlock()
		return []BrowserTab{}
	}
	debugPort := profile.DebugPort
	a.browserMgr.Mutex.Unlock()

	var targets []browserDebugTabTarget
	client := &http.Client{Timeout: cdpHTTPTimeout}
	if err := fetchBrowserDebugJSON(client, debugPort, "/json/list", &targets); err != nil {
		logger.New("Browser").Warn("读取实例标签页失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()),
		)
		return []BrowserTab{}
	}

	tabs := make([]BrowserTab, 0, len(targets))
	for index, target := range targets {
		if !strings.EqualFold(strings.TrimSpace(target.Type), "page") {
			continue
		}
		tabID := strings.TrimSpace(target.ID)
		if tabID == "" {
			tabID = fmt.Sprintf("page-%d", index)
		}
		url := strings.TrimSpace(target.URL)
		title := strings.TrimSpace(target.Title)
		if title == "" {
			title = url
		}
		if title == "" {
			title = "未命名标签页"
		}
		tabs = append(tabs, BrowserTab{
			TabId:  tabID,
			Title:  title,
			Url:    url,
			Active: false,
		})
	}
	return tabs
}
