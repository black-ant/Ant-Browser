//go:build windows
// +build windows

package backend

import (
	"os"
	"strings"
	"sync"
)

var mainApplicationWindowIconState struct {
	sync.Mutex
	resourcePath string
	bigIcon      uintptr
	smallIcon    uintptr
}

func ApplyMainApplicationWindowIcon(appRoot, expectedTitle string) {
	resourcePath := profileWindowMarkerIconResourcePath(appRoot)
	if strings.TrimSpace(resourcePath) == "" {
		return
	}

	windowsForApp := findTopLevelWindowsForProfileMarker(uint32(os.Getpid()))
	window, ok := selectMainApplicationWindow(windowsForApp, expectedTitle)
	if !ok {
		return
	}

	mainApplicationWindowIconState.Lock()
	defer mainApplicationWindowIconState.Unlock()

	if mainApplicationWindowIconState.resourcePath != resourcePath ||
		mainApplicationWindowIconState.bigIcon == 0 ||
		mainApplicationWindowIconState.smallIcon == 0 {
		bigIcon, smallIcon := extractProfileWindowMarkerExecutableIcons(resourcePath)
		if bigIcon == 0 && smallIcon == 0 {
			return
		}
		if bigIcon == 0 {
			bigIcon = smallIcon
		}
		if smallIcon == 0 {
			smallIcon = bigIcon
		}
		mainApplicationWindowIconState.resourcePath = resourcePath
		mainApplicationWindowIconState.bigIcon = bigIcon
		mainApplicationWindowIconState.smallIcon = smallIcon
	}

	setProfileWindowMarkerIcon(window.hwnd, iconBig, mainApplicationWindowIconState.bigIcon)
	setProfileWindowMarkerIcon(window.hwnd, iconSmall, mainApplicationWindowIconState.smallIcon)
	clearProfileWindowMarkerTaskbarOverlay(window.hwnd)
	applyProfileWindowMarkerRelaunchIconResource(
		window.hwnd,
		resourcePath,
		&profileWindowMarkerWindowState{},
	)
	refreshProfileWindowMarkerIcon(window.hwnd)
}

func selectMainApplicationWindow(candidates []profileWindowMarkerWindow, expectedTitle string) (profileWindowMarkerWindow, bool) {
	expectedTitle = strings.TrimSpace(expectedTitle)
	for _, candidate := range candidates {
		if !candidate.visible || candidate.owned || !candidate.titled {
			continue
		}
		if expectedTitle != "" && strings.TrimSpace(getProfileWindowMarkerTitle(candidate.hwnd)) == expectedTitle {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.visible && !candidate.owned && candidate.titled {
			return candidate, true
		}
	}
	return profileWindowMarkerWindow{}, false
}
