package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const profileWindowMarkerSeparator = " " + string(rune(0x00b7)) + " "
const profileWindowMarkerLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const profileWindowMarkerOverlayReapplyInterval = 5 * time.Second

type profileWindowMarkerWindowState struct {
	originalBigIcon             uintptr
	originalSmallIcon           uintptr
	originalBigIconCaptured     bool
	originalSmallIconCaptured   bool
	fallbackBigIconApplied      bool
	fallbackSmallIconApplied    bool
	decoratedBigIcon            uintptr
	decoratedSmallIcon          uintptr
	decoratedBigIconApplied     bool
	decoratedSmallIconApplied   bool
	taskbarOverlayIcon          uintptr
	taskbarOverlayIconApplied   bool
	taskbarOverlayLastAppliedAt time.Time
	chromeBigIcon               uintptr
	chromeSmallIcon             uintptr
	chromeIconPath              string
	chromeIconApplied           bool
	appUserModelID              string
	appUserModelIDApplied       bool
	relaunchIconResource        string
	relaunchIconResourceApplied bool
	processID                   uint32
}

func (state *profileWindowMarkerWindowState) taskbarOverlayNeedsReapply() bool {
	if state == nil || !state.taskbarOverlayIconApplied {
		return true
	}
	return time.Since(state.taskbarOverlayLastAppliedAt) >= profileWindowMarkerOverlayReapplyInterval
}

type profileWindowMarker struct {
	profileID  string
	code       string
	stop       chan struct{}
	stopOnce   sync.Once
	pidMu      sync.Mutex
	pid        int
	windowMu   sync.Mutex
	windowRefs map[uintptr]profileWindowMarkerWindowState
}

type profileWindowMarkerTarget struct {
	profileID        string
	profileName      string
	pid              int
	debugPort        int
	userDataDir      string
	chromeBinaryPath string
	iconResourcePath string
}

func nextProfileWindowMarkerCode(used map[string]struct{}) string {
	for index := 0; ; index++ {
		var code string
		switch {
		case index < 10:
			code = strconv.Itoa(index + 1)
		case index < 10+len(profileWindowMarkerLetters):
			code = string(profileWindowMarkerLetters[index-10])
		default:
			code = strconv.Itoa(index - len(profileWindowMarkerLetters) + 1)
		}
		if _, exists := used[code]; !exists {
			return code
		}
	}
}

func (a *App) allocateProfileWindowMarkerCodeLocked(profileID string) string {
	used := make(map[string]struct{}, len(a.profileWindowMarkers))
	for currentProfileID, marker := range a.profileWindowMarkers {
		if currentProfileID == profileID || marker == nil {
			continue
		}
		if code := strings.TrimSpace(marker.code); code != "" {
			used[code] = struct{}{}
		}
	}
	return nextProfileWindowMarkerCode(used)
}

func sanitizeProfileWindowMarkerName(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		if r == rune(0x00b7) {
			return '-'
		}
		return r
	}, name)
	return strings.Join(strings.Fields(cleaned), " ")
}

func truncateProfileWindowMarkerName(name string, maxRunes int) string {
	name = strings.TrimSpace(name)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= maxRunes {
		return name
	}
	if maxRunes == 1 {
		return string(runes[:1])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func profileWindowMarkerPrefix(markerCode, profileName string) string {
	name := sanitizeProfileWindowMarkerName(profileName)
	if name == "" {
		name = "实例"
	}
	name = truncateProfileWindowMarkerName(name, 24)
	code := strings.TrimSpace(markerCode)
	if code == "" {
		code = "?"
	}
	return fmt.Sprintf("[%s] %s%s", code, name, profileWindowMarkerSeparator)
}

func profileWindowMarkerAppUserModelID(profileID, markerCode string) string {
	suffix := sanitizeProfileWindowMarkerAppUserModelIDSuffix(profileID)
	if suffix == "" {
		suffix = sanitizeProfileWindowMarkerAppUserModelIDSuffix(markerCode)
	}
	if suffix == "" {
		suffix = "unknown"
	}
	appID := "AntBrowser.Instance." + suffix
	if len(appID) > 128 {
		appID = appID[:128]
		appID = strings.TrimRight(appID, ".-_")
	}
	return appID
}

func sanitizeProfileWindowMarkerAppUserModelIDSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastSeparator := false
	for _, char := range value {
		valid := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
		if valid {
			builder.WriteRune(char)
			lastSeparator = false
			continue
		}
		if char == '-' || char == '_' || char == '.' {
			if builder.Len() > 0 && !lastSeparator {
				builder.WriteRune(char)
				lastSeparator = true
			}
			continue
		}
		if builder.Len() > 0 && !lastSeparator {
			builder.WriteByte('.')
			lastSeparator = true
		}
	}
	return strings.Trim(builder.String(), ".-_")
}

func applyProfileWindowMarkerTitle(currentTitle, markerCode, profileName string) string {
	pageTitle := stripProfileWindowMarkerTitle(currentTitle)
	if pageTitle == "" {
		pageTitle = "Ant Browser"
	}
	return profileWindowMarkerPrefix(markerCode, profileName) + pageTitle
}

func stripProfileWindowMarkerTitle(currentTitle string) string {
	pageTitle := strings.TrimSpace(currentTitle)
	for {
		if !strings.HasPrefix(pageTitle, "[") {
			return pageTitle
		}
		closeIndex := strings.Index(pageTitle, "] ")
		if closeIndex <= 1 || !isProfileWindowMarkerCode(pageTitle[1:closeIndex]) {
			return pageTitle
		}
		remainder := strings.TrimSpace(pageTitle[closeIndex+2:])
		separatorIndex := strings.Index(remainder, profileWindowMarkerSeparator)
		if separatorIndex < 0 {
			return pageTitle
		}
		pageTitle = strings.TrimSpace(remainder[separatorIndex+len(profileWindowMarkerSeparator):])
	}
}

func isProfileWindowMarkerCode(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func (a *App) currentProfileWindowMarkerTarget(profileID string) (profileWindowMarkerTarget, bool) {
	if a == nil || a.browserMgr == nil {
		return profileWindowMarkerTarget{}, false
	}

	a.browserMgr.Mutex.Lock()

	profile, exists := a.browserMgr.Profiles[profileID]
	if !exists || profile == nil || !profile.Running {
		a.browserMgr.Mutex.Unlock()
		return profileWindowMarkerTarget{}, false
	}
	profileSnapshot := *profile
	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	a.browserMgr.Mutex.Unlock()

	chromeBinaryPath, _ := a.browserMgr.ResolveChromeBinary(&profileSnapshot)

	return profileWindowMarkerTarget{
		profileID:        profileSnapshot.ProfileId,
		profileName:      profileSnapshot.ProfileName,
		pid:              profileSnapshot.Pid,
		debugPort:        profileSnapshot.DebugPort,
		userDataDir:      userDataDir,
		chromeBinaryPath: chromeBinaryPath,
		iconResourcePath: profileWindowMarkerIconResourcePath(a.appRoot),
	}, true
}

func profileWindowMarkerIconResourcePath(appRoot string) string {
	candidates := []string{
		filepath.Join(appRoot, "AntBrowser.ico"),
		filepath.Join(appRoot, "build", "windows", "icon.ico"),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (a *App) startProfileWindowMarkerLocked(profileID string, profile *BrowserProfile) {
	if a == nil || profile == nil || !profile.Running || !profileWindowMarkerSupported() {
		return
	}

	a.profileWindowMarkersMu.Lock()
	if a.profileWindowMarkers == nil {
		a.profileWindowMarkers = make(map[string]*profileWindowMarker)
	}
	if marker, exists := a.profileWindowMarkers[profileID]; exists {
		marker.setPID(profile.Pid)
		profile.WindowMarkerCode = marker.code
		a.profileWindowMarkersMu.Unlock()
		return
	}

	marker := &profileWindowMarker{
		profileID:  profileID,
		code:       a.allocateProfileWindowMarkerCodeLocked(profileID),
		stop:       make(chan struct{}),
		pid:        profile.Pid,
		windowRefs: make(map[uintptr]profileWindowMarkerWindowState),
	}
	a.profileWindowMarkers[profileID] = marker
	profile.WindowMarkerCode = marker.code
	a.profileWindowMarkersMu.Unlock()

	go runProfileWindowMarker(a, marker)
}

func (marker *profileWindowMarker) setPID(pid int) {
	if marker == nil {
		return
	}
	marker.pidMu.Lock()
	marker.pid = pid
	marker.pidMu.Unlock()
}

func (marker *profileWindowMarker) currentPID() int {
	if marker == nil {
		return 0
	}
	marker.pidMu.Lock()
	defer marker.pidMu.Unlock()
	return marker.pid
}

func (marker *profileWindowMarker) rememberWindow(hwnd uintptr, processID uint32) {
	if marker == nil || hwnd == 0 {
		return
	}
	marker.windowMu.Lock()
	if marker.windowRefs == nil {
		marker.windowRefs = make(map[uintptr]profileWindowMarkerWindowState)
	}
	state, exists := marker.windowRefs[hwnd]
	if !exists || (processID > 0 && state.processID > 0 && state.processID != processID) {
		state = profileWindowMarkerWindowState{}
	}
	if state.processID == 0 {
		state.processID = processID
	}
	marker.windowRefs[hwnd] = state
	marker.windowMu.Unlock()
}

func (marker *profileWindowMarker) windowState(hwnd uintptr) (profileWindowMarkerWindowState, bool) {
	if marker == nil || hwnd == 0 {
		return profileWindowMarkerWindowState{}, false
	}
	marker.windowMu.Lock()
	defer marker.windowMu.Unlock()
	state, exists := marker.windowRefs[hwnd]
	return state, exists
}

func (marker *profileWindowMarker) setWindowState(hwnd uintptr, state profileWindowMarkerWindowState) {
	if marker == nil || hwnd == 0 {
		return
	}
	marker.windowMu.Lock()
	if marker.windowRefs == nil {
		marker.windowRefs = make(map[uintptr]profileWindowMarkerWindowState)
	}
	marker.windowRefs[hwnd] = state
	marker.windowMu.Unlock()
}

func (marker *profileWindowMarker) rememberedWindows() []uintptr {
	if marker == nil {
		return nil
	}
	marker.windowMu.Lock()
	defer marker.windowMu.Unlock()
	windows := make([]uintptr, 0, len(marker.windowRefs))
	for hwnd := range marker.windowRefs {
		windows = append(windows, hwnd)
	}
	return windows
}

func (a *App) releaseProfileWindowMarker(profileID string, marker *profileWindowMarker) {
	if a == nil || marker == nil {
		return
	}
	a.profileWindowMarkersMu.Lock()
	if current, exists := a.profileWindowMarkers[profileID]; exists && current == marker {
		delete(a.profileWindowMarkers, profileID)
	}
	a.profileWindowMarkersMu.Unlock()
}

func (a *App) stopProfileWindowMarker(profileID string) {
	if a == nil {
		return
	}

	a.profileWindowMarkersMu.Lock()
	marker := a.profileWindowMarkers[profileID]
	delete(a.profileWindowMarkers, profileID)
	a.profileWindowMarkersMu.Unlock()
	if marker != nil {
		marker.stopOnce.Do(func() { close(marker.stop) })
	}
}
