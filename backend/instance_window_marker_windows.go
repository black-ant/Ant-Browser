//go:build windows
// +build windows

package backend

import (
	"bytes"
	_ "embed"
	"image/png"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmSetIcon          = 0x0080
	iconSmall          = 0
	iconBig            = 1
	wmGetIcon          = 0x007F
	wmSetText          = 0x000C
	smtoAbortIfHung    = 0x0002
	sendMessageTimeout = 150
	iconSmall2         = 2
	diNormal           = 0x0003
	clsctxInprocServer = 0x0001
	coinitApartment    = 0x0002
	rpcEChangedMode    = 0x80010106
	vtEmpty            = 0
	vtLPWSTR           = 31

	gwOwner       = 4
	dibRGBColors  = 0
	biRGB         = 0
	rdwInvalidate = 0x0001
	rdwFrame      = 0x0400

	fwBold             = 700
	defaultCharset     = 1
	antialiasedQuality = 4
	transparentBk      = 1
	dtCenter           = 0x0001
	dtVCenter          = 0x0004
	dtSingleLine       = 0x0020

	profileWindowMarkerOverlayIconWidth          = 32
	profileWindowMarkerOverlayIconHeight         = 32
	profileWindowMarkerFallbackIconWidth         = 32
	profileWindowMarkerFallbackIconHeight        = 32
	profileWindowMarkerInstanceIconWidth         = 64
	profileWindowMarkerInstanceIconHeight        = 64
	profileWindowMarkerSmallIconWidth            = 32
	profileWindowMarkerSmallIconHeight           = 32
	profileWindowMarkerFrameColor         uint32 = 0xFF0F172A
	profileWindowMarkerGlyphColor         uint32 = 0xFF111827
	profileWindowMarkerGlyphLightColor    uint32 = 0xFF111827
	profileWindowMarkerBadgeBackground    uint32 = 0xFFFFFFFF
	profileWindowMarkerBadgeShadowColor   uint32 = 0xFF020617
	profileWindowMarkerFingerprintOuter   uint32 = 0xFFF8FAFC
	profileWindowMarkerFingerprintInner   uint32 = 0xFFEAF1F9
	profileWindowMarkerFingerprintLine    uint32 = 0xFF2563EB
	profileWindowMarkerFingerprintAccent  uint32 = 0xFF3B82F6
	profileWindowMarkerFingerprintDot     uint32 = 0xFF2563EB
	profileWindowMarkerFallbackFrameColor uint32 = 0xFF0F172A
	profileWindowMarkerFallbackLineColor  uint32 = 0xFFCBD5E1
	profileWindowMarkerFallbackDotColor   uint32 = 0xFFD1FAE5
)

var (
	user32WindowMarker  = windows.NewLazySystemDLL("user32.dll")
	gdi32WindowMarker   = windows.NewLazySystemDLL("gdi32.dll")
	shell32WindowMarker = windows.NewLazySystemDLL("shell32.dll")
	ole32WindowMarker   = windows.NewLazySystemDLL("ole32.dll")

	procEnumWindowsMarker              = user32WindowMarker.NewProc("EnumWindows")
	procGetWindowThreadProcessIDMarker = user32WindowMarker.NewProc("GetWindowThreadProcessId")
	procIsWindowVisibleMarker          = user32WindowMarker.NewProc("IsWindowVisible")
	procIsIconicMarker                 = user32WindowMarker.NewProc("IsIconic")
	procGetWindowMarker                = user32WindowMarker.NewProc("GetWindow")
	procGetWindowTextLengthMarker      = user32WindowMarker.NewProc("GetWindowTextLengthW")
	procGetWindowTextMarker            = user32WindowMarker.NewProc("GetWindowTextW")
	procSendMessageTimeoutMarker       = user32WindowMarker.NewProc("SendMessageTimeoutW")
	procGetClassLongPtrMarker          = user32WindowMarker.NewProc("GetClassLongPtrW")
	procDrawIconExMarker               = user32WindowMarker.NewProc("DrawIconEx")
	procExtractIconExMarker            = shell32WindowMarker.NewProc("ExtractIconExW")
	procSHGetPropertyStoreForWindow    = shell32WindowMarker.NewProc("SHGetPropertyStoreForWindow")

	procCreateDIBSectionMarker   = gdi32WindowMarker.NewProc("CreateDIBSection")
	procCreateBitmapMarker       = gdi32WindowMarker.NewProc("CreateBitmap")
	procCreateCompatibleDCMarker = gdi32WindowMarker.NewProc("CreateCompatibleDC")
	procSelectObjectMarker       = gdi32WindowMarker.NewProc("SelectObject")
	procDeleteDCMarker           = gdi32WindowMarker.NewProc("DeleteDC")
	procDeleteObjectMarker       = gdi32WindowMarker.NewProc("DeleteObject")
	procCreateFontWMarker        = gdi32WindowMarker.NewProc("CreateFontW")
	procSetTextColorMarker       = gdi32WindowMarker.NewProc("SetTextColor")
	procSetBkModeMarker          = gdi32WindowMarker.NewProc("SetBkMode")
	procDrawTextWMarker          = user32WindowMarker.NewProc("DrawTextW")
	procCreateIconIndirect       = user32WindowMarker.NewProc("CreateIconIndirect")
	procDestroyIconMarker        = user32WindowMarker.NewProc("DestroyIcon")
	procCoInitializeExMarker     = ole32WindowMarker.NewProc("CoInitializeEx")
	procCoUninitializeMarker     = ole32WindowMarker.NewProc("CoUninitialize")
	procCoCreateInstanceMarker   = ole32WindowMarker.NewProc("CoCreateInstance")
)

var procRedrawWindowMarker = user32WindowMarker.NewProc(`RedrawWindow`)

//go:embed instance_window_marker_logo.png
var profileWindowMarkerLogoPNG []byte

var (
	clsidTaskbarListWindowMarker    = windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11D0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3WindowMarker     = windows.GUID{Data1: 0xEA1AFB91, Data2: 0x9E28, Data3: 0x4B86, Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF}}
	iidPropertyStoreWindowMarker    = windows.GUID{Data1: 0x886D8EEB, Data2: 0x8CF2, Data3: 0x4446, Data4: [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
	appUserModelIDPropertyKey       = profileWindowMarkerPropertyKey{Fmtid: windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39, Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, Pid: 5}
	relaunchIconResourcePropertyKey = profileWindowMarkerPropertyKey{Fmtid: windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39, Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, Pid: 3}
)

type profileWindowMarkerPropertyKey struct {
	Fmtid windows.GUID
	Pid   uint32
}

type profileWindowMarkerPropVariant struct {
	VT         uint16
	Reserved1  uint16
	Reserved2  uint16
	Reserved3  uint16
	Pointer    uintptr
	Reserved64 uintptr
}

type profileWindowMarkerPropertyStore struct {
	VTable *profileWindowMarkerPropertyStoreVTable
}

type profileWindowMarkerPropertyStoreVTable struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	GetCount       uintptr
	GetAt          uintptr
	GetValue       uintptr
	SetValue       uintptr
	Commit         uintptr
}

type profileWindowMarkerTaskbarList3 struct {
	VTable *profileWindowMarkerTaskbarList3VTable
}

type profileWindowMarkerTaskbarList3VTable struct {
	QueryInterface        uintptr
	AddRef                uintptr
	Release               uintptr
	HrInit                uintptr
	AddTab                uintptr
	DeleteTab             uintptr
	ActivateTab           uintptr
	SetActiveAlt          uintptr
	MarkFullscreenWindow  uintptr
	SetProgressValue      uintptr
	SetProgressState      uintptr
	RegisterTab           uintptr
	UnregisterTab         uintptr
	SetTabOrder           uintptr
	SetTabActive          uintptr
	ThumbBarAddButtons    uintptr
	ThumbBarUpdateButtons uintptr
	ThumbBarSetImageList  uintptr
	SetOverlayIcon        uintptr
	SetThumbnailTooltip   uintptr
	SetThumbnailClip      uintptr
}

type profileWindowMarkerBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type profileWindowMarkerBitmapInfo struct {
	Header profileWindowMarkerBitmapInfoHeader
}

type profileWindowMarkerIconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type profileWindowMarkerTextRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type profileWindowMarkerWindow struct {
	hwnd      uintptr
	processID uint32
	visible   bool
	titled    bool
	owned     bool
	minimized bool
}

func profileWindowMarkerSupported() bool {
	return true
}

func runProfileWindowMarker(app *App, marker *profileWindowMarker) {
	const pollInterval = 500 * time.Millisecond
	const fallbackScanInterval = 2 * time.Second

	nextFallbackScan := time.Time{}
	defer func() {
		cleared := make(map[uintptr]struct{})
		for _, hwnd := range marker.rememberedWindows() {
			state, _ := marker.windowState(hwnd)
			if !isProfileWindowMarkerWindowProcess(hwnd, state.processID) {
				destroyProfileWindowMarkerDecoratedIcons(&state)
				destroyProfileWindowMarkerChromeIcons(&state)
				continue
			}
			clearProfileWindowMarkerTitle(hwnd)
			clearProfileWindowMarker(hwnd, state, 0)
			cleared[hwnd] = struct{}{}
		}
		for _, window := range findTopLevelWindowsForProfileMarker(profileWindowMarkerPID(marker)) {
			if _, exists := cleared[window.hwnd]; exists {
				continue
			}
			state, _ := marker.windowState(window.hwnd)
			if state.processID > 0 && !isProfileWindowMarkerWindowProcess(window.hwnd, state.processID) {
				destroyProfileWindowMarkerDecoratedIcons(&state)
				destroyProfileWindowMarkerChromeIcons(&state)
				continue
			}
			clearProfileWindowMarkerTitle(window.hwnd)
			clearProfileWindowMarker(window.hwnd, state, 0)
		}
		if app != nil {
			app.releaseProfileWindowMarker(marker.profileID, marker)
		}
	}()

	for {
		select {
		case <-marker.stop:
			return
		default:
		}

		target, ok := app.currentProfileWindowMarkerTarget(marker.profileID)
		if !ok {
			return
		}

		windowsForMarker := findTopLevelWindowsForProfileMarker(uint32(target.pid))
		if len(windowsForMarker) == 0 && time.Now().After(nextFallbackScan) {
			windowsForMarker = mergeProfileWindowMarkerWindows(
				windowsForMarker,
				findTopLevelWindowsForProfileMarkerByUserData(target),
			)
			nextFallbackScan = time.Now().Add(fallbackScanInterval)
		}
		for _, window := range windowsForMarker {
			applyProfileWindowMarker(marker, window, target)
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-marker.stop:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func profileWindowMarkerPID(marker *profileWindowMarker) uint32 {
	if marker == nil {
		return 0
	}
	pid := marker.currentPID()
	if pid <= 0 {
		return 0
	}
	return uint32(pid)
}

type profileWindowMarkerWindowScan struct {
	pid     uint32
	windows []profileWindowMarkerWindow
}

var profileWindowMarkerEnumCallback = windows.NewCallback(enumProfileWindowMarkerWindow)

func enumProfileWindowMarkerWindow(hwnd uintptr, scan *profileWindowMarkerWindowScan) uintptr {
	if scan == nil || scan.pid == 0 {
		return 1
	}

	var windowPID uint32
	procGetWindowThreadProcessIDMarker.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
	if windowPID != scan.pid {
		return 1
	}

	visible, _, _ := procIsWindowVisibleMarker.Call(hwnd)
	titleLength, _, _ := procGetWindowTextLengthMarker.Call(hwnd)
	owner, _, _ := procGetWindowMarker.Call(hwnd, gwOwner)
	minimized, _, _ := procIsIconicMarker.Call(hwnd)
	scan.windows = append(scan.windows, profileWindowMarkerWindow{
		hwnd:      hwnd,
		processID: windowPID,
		visible:   visible != 0,
		titled:    titleLength > 0,
		owned:     owner != 0,
		minimized: minimized != 0,
	})
	return 1
}

func findTopLevelWindowsForProfileMarker(pid uint32) []profileWindowMarkerWindow {
	if pid == 0 {
		return nil
	}

	scan := profileWindowMarkerWindowScan{pid: pid}
	procEnumWindowsMarker.Call(profileWindowMarkerEnumCallback, uintptr(unsafe.Pointer(&scan)))
	runtime.KeepAlive(&scan)

	return prioritizeProfileMarkerWindows(scan.windows)
}

func findTopLevelWindowsForProfileMarkerByUserData(target profileWindowMarkerTarget) []profileWindowMarkerWindow {
	if strings.TrimSpace(target.userDataDir) == "" {
		return nil
	}
	processes, err := findBrowserUserDataProcessesOS(target.userDataDir)
	if err != nil {
		return nil
	}
	matched, fallback := classifyProfileMarkerProcessCandidates(processes, target.debugPort)
	candidates := matched
	if len(candidates) == 0 {
		candidates = fallback
	}
	var windowsForProfile []profileWindowMarkerWindow
	for _, processPID := range candidates {
		windowsForProfile = append(windowsForProfile, findTopLevelWindowsForProfileMarker(processPID)...)
	}
	return prioritizeProfileMarkerWindows(windowsForProfile)
}

func classifyProfileMarkerProcessCandidates(processes []browserUserDataProcess, targetDebugPort int) (matched []uint32, fallback []uint32) {
	seen := make(map[uint32]struct{})
	for _, process := range processes {
		if process.PID <= 0 {
			continue
		}
		processPID := uint32(process.PID)
		if _, exists := seen[processPID]; exists {
			continue
		}
		seen[processPID] = struct{}{}
		if targetDebugPort > 0 && process.DebugPort > 0 && process.DebugPort != targetDebugPort {
			continue
		}
		if targetDebugPort > 0 && process.DebugPort == 0 {
			fallback = append(fallback, processPID)
			continue
		}
		matched = append(matched, processPID)
	}
	return matched, fallback
}

func prioritizeProfileMarkerWindows(candidates []profileWindowMarkerWindow) []profileWindowMarkerWindow {
	if len(candidates) < 2 {
		return candidates
	}
	result := make([]profileWindowMarkerWindow, 0, len(candidates))
	appendMatching := func(predicate func(profileWindowMarkerWindow) bool) {
		for _, candidate := range candidates {
			if predicate(candidate) {
				result = append(result, candidate)
			}
		}
	}
	appendMatching(func(candidate profileWindowMarkerWindow) bool {
		return candidate.visible && candidate.titled && !candidate.owned
	})
	appendMatching(func(candidate profileWindowMarkerWindow) bool {
		return candidate.titled && !candidate.owned
	})
	appendMatching(func(candidate profileWindowMarkerWindow) bool {
		return candidate.visible && !candidate.owned
	})
	appendMatching(func(candidate profileWindowMarkerWindow) bool {
		return candidate.minimized && !candidate.owned
	})
	appendMatching(func(candidate profileWindowMarkerWindow) bool {
		return true
	})

	seen := make(map[uintptr]struct{}, len(result))
	unique := result[:0]
	for _, candidate := range result {
		if _, exists := seen[candidate.hwnd]; exists {
			continue
		}
		seen[candidate.hwnd] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func mergeProfileWindowMarkerWindows(groups ...[]profileWindowMarkerWindow) []profileWindowMarkerWindow {
	if len(groups) == 0 {
		return nil
	}
	merged := make([]profileWindowMarkerWindow, 0)
	seen := make(map[uintptr]struct{})
	for _, group := range groups {
		for _, candidate := range group {
			if candidate.hwnd == 0 {
				continue
			}
			if _, exists := seen[candidate.hwnd]; exists {
				continue
			}
			seen[candidate.hwnd] = struct{}{}
			merged = append(merged, candidate)
		}
	}
	return prioritizeProfileMarkerWindows(merged)
}

func applyProfileWindowMarker(marker *profileWindowMarker, window profileWindowMarkerWindow, target profileWindowMarkerTarget) {
	if window.hwnd == 0 {
		return
	}
	markerCode := ""
	if marker != nil {
		markerCode = marker.code
		if previousState, exists := marker.windowState(window.hwnd); exists && previousState.processID > 0 && window.processID > 0 && previousState.processID != window.processID {
			destroyProfileWindowMarkerDecoratedIcons(&previousState)
			destroyProfileWindowMarkerChromeIcons(&previousState)
		}
		marker.rememberWindow(window.hwnd, window.processID)
	}

	currentTitle := getProfileWindowMarkerTitle(window.hwnd)
	desiredTitle := applyProfileWindowMarkerTitle(currentTitle, markerCode, target.profileName)
	if currentTitle != desiredTitle {
		setProfileWindowMarkerTitle(window.hwnd, desiredTitle)
	}
	state := profileWindowMarkerWindowState{processID: window.processID}
	if marker != nil {
		state, _ = marker.windowState(window.hwnd)
	}
	captureProfileWindowMarkerOriginalIcons(window.hwnd, &state, 0)
	iconChanged := applyProfileWindowMarkerChromeIcon(window.hwnd, target.chromeBinaryPath, &state)
	applyProfileWindowMarkerTaskbarOverlay(window.hwnd, markerCode, &state)
	if iconChanged {
		refreshProfileWindowMarkerIcon(window.hwnd)
	}
	if marker != nil {
		marker.setWindowState(window.hwnd, state)
	}
}

func applyProfileWindowMarkerChromeIcon(hwnd uintptr, chromeBinaryPath string, state *profileWindowMarkerWindowState) bool {
	if hwnd == 0 || state == nil {
		return false
	}
	chromeBinaryPath = strings.TrimSpace(chromeBinaryPath)
	if chromeBinaryPath == "" {
		return false
	}
	if state.chromeIconPath != "" && state.chromeIconPath != chromeBinaryPath {
		clearProfileWindowMarkerChromeIcon(hwnd, state)
		destroyProfileWindowMarkerChromeIcons(state)
	}
	if state.chromeBigIcon == 0 || state.chromeSmallIcon == 0 {
		bigIcon, smallIcon := extractProfileWindowMarkerExecutableIcons(chromeBinaryPath)
		if bigIcon == 0 && smallIcon == 0 {
			return false
		}
		if bigIcon == 0 {
			bigIcon = smallIcon
		}
		if smallIcon == 0 {
			smallIcon = bigIcon
		}
		state.chromeBigIcon = bigIcon
		state.chromeSmallIcon = smallIcon
		state.chromeIconPath = chromeBinaryPath
	}

	iconChanged := false
	if !isProfileWindowMarkerIcon(hwnd, iconBig, state.chromeBigIcon) && setProfileWindowMarkerIcon(hwnd, iconBig, state.chromeBigIcon) {
		iconChanged = true
	}
	if !isProfileWindowMarkerIcon(hwnd, iconSmall, state.chromeSmallIcon) && setProfileWindowMarkerIcon(hwnd, iconSmall, state.chromeSmallIcon) {
		iconChanged = true
	}
	state.chromeIconApplied = true
	return iconChanged
}

func applyProfileWindowMarkerAppUserModelID(hwnd uintptr, profileID, markerCode string, state *profileWindowMarkerWindowState) bool {
	if hwnd == 0 || state == nil {
		return false
	}
	appID := profileWindowMarkerAppUserModelID(profileID, markerCode)
	if state.appUserModelIDApplied && state.appUserModelID == appID {
		return false
	}
	if !setProfileWindowMarkerAppUserModelID(hwnd, appID) {
		return false
	}
	state.appUserModelID = appID
	state.appUserModelIDApplied = true
	return true
}

func applyProfileWindowMarkerRelaunchIconResource(hwnd uintptr, resourcePath string, state *profileWindowMarkerWindowState) bool {
	if hwnd == 0 || state == nil || strings.TrimSpace(resourcePath) == "" {
		return false
	}
	resourceValue := strings.TrimSpace(resourcePath) + ",0"
	if state.relaunchIconResourceApplied && state.relaunchIconResource == resourceValue {
		return false
	}
	if !setProfileWindowMarkerStringProperty(hwnd, relaunchIconResourcePropertyKey, resourceValue) {
		return false
	}
	state.relaunchIconResource = resourceValue
	state.relaunchIconResourceApplied = true
	return true
}

func applyProfileWindowMarkerTaskbarOverlay(hwnd uintptr, markerCode string, state *profileWindowMarkerWindowState) bool {
	if hwnd == 0 || state == nil {
		return false
	}
	if state.taskbarOverlayIcon == 0 {
		overlayIcon, _ := createProfileWindowMarkerOverlayIcon(markerCode)
		if overlayIcon == 0 {
			return false
		}
		state.taskbarOverlayIcon = overlayIcon
	}
	if !state.taskbarOverlayNeedsReapply() {
		return false
	}
	if !setProfileWindowMarkerTaskbarOverlay(hwnd, state.taskbarOverlayIcon, markerCode) {
		return false
	}
	state.taskbarOverlayIconApplied = true
	state.taskbarOverlayLastAppliedAt = time.Now()
	return true
}

func applyProfileWindowMarkerGeneratedIcon(hwnd, iconType uintptr, width, height int, markerCode string, state *profileWindowMarkerWindowState) (bool, bool) {
	if hwnd == 0 || state == nil {
		return false, false
	}
	markerIcon := &state.decoratedBigIcon
	markerApplied := &state.decoratedBigIconApplied
	if iconType == iconSmall {
		markerIcon = &state.decoratedSmallIcon
		markerApplied = &state.decoratedSmallIconApplied
	}
	if *markerIcon == 0 {
		createdIcon, _ := createProfileWindowMarkerInstanceIcon(width, height, markerCode)
		if createdIcon == 0 {
			return false, false
		}
		*markerIcon = createdIcon
	}
	if *markerApplied && isProfileWindowMarkerIcon(hwnd, iconType, *markerIcon) {
		return true, false
	}
	if iconType == iconBig && *markerApplied && state.decoratedSmallIconApplied &&
		state.decoratedSmallIcon != 0 &&
		isProfileWindowMarkerIcon(hwnd, iconSmall, state.decoratedSmallIcon) &&
		isProfileWindowMarkerIcon(hwnd, iconBig, state.decoratedSmallIcon) {
		return true, false
	}
	if !setProfileWindowMarkerIcon(hwnd, iconType, *markerIcon) {
		return false, false
	}
	*markerApplied = true
	if iconType == iconBig {
		state.fallbackBigIconApplied = false
	} else {
		state.fallbackSmallIconApplied = false
	}
	return true, true
}

func captureProfileWindowMarkerOriginalIcons(hwnd uintptr, state *profileWindowMarkerWindowState, fallbackIcon uintptr) {
	if hwnd == 0 || state == nil {
		return
	}
	if !state.originalBigIconCaptured && !(state.fallbackBigIconApplied && isProfileWindowMarkerIcon(hwnd, iconBig, fallbackIcon)) {
		if originalIcon, ok := getProfileWindowMarkerIcon(hwnd, iconBig); ok && originalIcon != 0 {
			state.originalBigIcon = originalIcon
			state.originalBigIconCaptured = true
		}
	}
	if !state.originalSmallIconCaptured && !(state.fallbackSmallIconApplied && isProfileWindowMarkerIcon(hwnd, iconSmall, fallbackIcon)) {
		if originalIcon, ok := getProfileWindowMarkerIcon(hwnd, iconSmall); ok && originalIcon != 0 {
			state.originalSmallIcon = originalIcon
			state.originalSmallIconCaptured = true
		}
	}
}

func applyProfileWindowMarkerDecoratedIcon(hwnd, iconType uintptr, width, height int, markerCode string, sourceIcon uintptr, state *profileWindowMarkerWindowState) (bool, bool) {
	if hwnd == 0 || state == nil {
		return false, false
	}
	originalIcon := state.originalBigIcon
	decoratedIcon := &state.decoratedBigIcon
	decoratedApplied := &state.decoratedBigIconApplied
	if iconType == iconSmall {
		originalIcon = state.originalSmallIcon
		decoratedIcon = &state.decoratedSmallIcon
		decoratedApplied = &state.decoratedSmallIconApplied
	}
	if *decoratedIcon == 0 {
		iconSource := sourceIcon
		if iconSource == 0 {
			iconSource = originalIcon
		}
		if iconSource == 0 {
			return false, false
		}
		createdIcon, _ := createProfileWindowMarkerDecoratedIcon(iconSource, width, height, markerCode)
		if createdIcon == 0 {
			return false, false
		}
		*decoratedIcon = createdIcon
	}
	if *decoratedApplied && isProfileWindowMarkerIcon(hwnd, iconType, *decoratedIcon) {
		return true, false
	}
	if iconType == iconBig && *decoratedApplied && state.decoratedSmallIconApplied &&
		state.decoratedSmallIcon != 0 &&
		isProfileWindowMarkerIcon(hwnd, iconSmall, state.decoratedSmallIcon) &&
		isProfileWindowMarkerIcon(hwnd, iconBig, state.decoratedSmallIcon) {
		return true, false
	}
	if !setProfileWindowMarkerIcon(hwnd, iconType, *decoratedIcon) {
		return false, false
	}
	*decoratedApplied = true
	if iconType == iconBig {
		state.fallbackBigIconApplied = false
	} else {
		state.fallbackSmallIconApplied = false
	}
	return true, true
}

func applyProfileWindowMarkerFallbackIcon(hwnd, iconType uintptr, state *profileWindowMarkerWindowState, markerIcon uintptr) (bool, bool) {
	if hwnd == 0 || state == nil || markerIcon == 0 {
		return false, false
	}
	applied := &state.fallbackBigIconApplied
	if iconType == iconSmall {
		applied = &state.fallbackSmallIconApplied
	}
	if *applied && isProfileWindowMarkerIcon(hwnd, iconType, markerIcon) {
		return true, false
	}
	if !setProfileWindowMarkerIcon(hwnd, iconType, markerIcon) {
		return false, false
	}
	*applied = true
	if iconType == iconBig {
		state.decoratedBigIconApplied = false
	} else {
		state.decoratedSmallIconApplied = false
	}
	return true, true
}

func setProfileWindowMarkerIcon(hwnd uintptr, iconType uintptr, icon uintptr) bool {
	if hwnd == 0 {
		return false
	}
	iconTypes := []uintptr{iconType}
	if iconType == iconSmall {
		iconTypes = append(iconTypes, iconSmall2)
	}
	set := false
	for _, requestedIconType := range iconTypes {
		if _, ok := sendProfileWindowMarkerMessageResult(hwnd, wmSetIcon, requestedIconType, icon); !ok {
			continue
		}
		if icon == 0 {
			set = true
			continue
		}
		currentIcon, currentOK := getProfileWindowMarkerIcon(hwnd, iconType)
		if currentOK && currentIcon == icon {
			set = true
		}
	}
	return set
}

func clearProfileWindowMarkerChromeIcon(hwnd uintptr, state *profileWindowMarkerWindowState) bool {
	if hwnd == 0 || state == nil || !state.chromeIconApplied {
		return false
	}
	iconChanged := false
	if state.chromeBigIcon != 0 && isProfileWindowMarkerIcon(hwnd, iconBig, state.chromeBigIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconBig, profileWindowMarkerRestoreIcon(state, iconBig)) {
			iconChanged = true
		}
	}
	if state.chromeSmallIcon != 0 && isProfileWindowMarkerIcon(hwnd, iconSmall, state.chromeSmallIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconSmall, profileWindowMarkerRestoreIcon(state, iconSmall)) {
			iconChanged = true
		}
	}
	return iconChanged
}

func profileWindowMarkerRestoreIcon(state *profileWindowMarkerWindowState, iconType uintptr) uintptr {
	if state == nil {
		return 0
	}
	if iconType == iconSmall {
		if state.originalSmallIconCaptured {
			return state.originalSmallIcon
		}
		return 0
	}
	if state.originalBigIconCaptured {
		return state.originalBigIcon
	}
	return 0
}

func clearProfileWindowMarker(hwnd uintptr, state profileWindowMarkerWindowState, markerIcon uintptr) {
	if hwnd == 0 {
		destroyProfileWindowMarkerDecoratedIcons(&state)
		destroyProfileWindowMarkerChromeIcons(&state)
		return
	}
	iconChanged := false
	if state.taskbarOverlayIconApplied {
		if clearProfileWindowMarkerTaskbarOverlay(hwnd) {
			iconChanged = true
		}
	}
	if state.appUserModelIDApplied {
		clearProfileWindowMarkerAppUserModelID(hwnd)
	}
	if state.relaunchIconResourceApplied {
		setProfileWindowMarkerStringProperty(hwnd, relaunchIconResourcePropertyKey, "")
	}
	if clearProfileWindowMarkerChromeIcon(hwnd, &state) {
		iconChanged = true
	}
	if state.decoratedBigIconApplied && isProfileWindowMarkerIcon(hwnd, iconBig, state.decoratedBigIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconBig, profileWindowMarkerRestoreIcon(&state, iconBig)) {
			iconChanged = true
		}
	}
	if state.decoratedSmallIconApplied && isProfileWindowMarkerIcon(hwnd, iconSmall, state.decoratedSmallIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconSmall, profileWindowMarkerRestoreIcon(&state, iconSmall)) {
			iconChanged = true
		}
	}
	if state.fallbackBigIconApplied && isProfileWindowMarkerIcon(hwnd, iconBig, markerIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconBig, profileWindowMarkerRestoreIcon(&state, iconBig)) {
			iconChanged = true
		}
	}
	if state.fallbackSmallIconApplied && isProfileWindowMarkerIcon(hwnd, iconSmall, markerIcon) {
		if setProfileWindowMarkerIcon(hwnd, iconSmall, profileWindowMarkerRestoreIcon(&state, iconSmall)) {
			iconChanged = true
		}
	}
	if iconChanged {
		refreshProfileWindowMarkerIcon(hwnd)
	}
	destroyProfileWindowMarkerDecoratedIcons(&state)
	destroyProfileWindowMarkerChromeIcons(&state)
}

func clearProfileWindowMarkerTitle(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	currentTitle := getProfileWindowMarkerTitle(hwnd)
	pageTitle := stripProfileWindowMarkerTitle(currentTitle)
	if pageTitle == currentTitle {
		return
	}
	if pageTitle == "" {
		pageTitle = "Ant Browser"
	}
	setProfileWindowMarkerTitle(hwnd, pageTitle)
}

func getProfileWindowMarkerTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthMarker.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length+1)
	procGetWindowTextMarker.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)
	return syscall.UTF16ToString(buffer)
}

func setProfileWindowMarkerTitle(hwnd uintptr, title string) {
	utf16Title, err := windows.UTF16FromString(title)
	if err != nil {
		return
	}
	sendProfileWindowMarkerMessage(hwnd, wmSetText, 0, uintptr(unsafe.Pointer(&utf16Title[0])))
}

func extractProfileWindowMarkerExecutableIcons(executablePath string) (uintptr, uintptr) {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath == "" {
		return 0, 0
	}
	pathPtr, err := windows.UTF16PtrFromString(executablePath)
	if err != nil {
		return 0, 0
	}
	var largeIcon uintptr
	var smallIcon uintptr
	count, _, _ := procExtractIconExMarker.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if count == 0 {
		if largeIcon != 0 {
			procDestroyIconMarker.Call(largeIcon)
		}
		if smallIcon != 0 && smallIcon != largeIcon {
			procDestroyIconMarker.Call(smallIcon)
		}
		return 0, 0
	}
	return largeIcon, smallIcon
}

func getProfileWindowMarkerIcon(hwnd uintptr, iconType uintptr) (uintptr, bool) {
	iconTypes := []uintptr{iconType}
	if iconType == iconSmall {
		iconTypes = append(iconTypes, iconSmall2)
	}
	for _, requestedIconType := range iconTypes {
		if icon, ok := sendProfileWindowMarkerMessageResult(hwnd, wmGetIcon, requestedIconType, 0); ok && icon != 0 {
			return icon, true
		}
	}
	classIconIndex := ^uintptr(13)
	if iconType == iconSmall {
		classIconIndex = ^uintptr(33)
	}
	if icon, _, _ := procGetClassLongPtrMarker.Call(hwnd, classIconIndex); icon != 0 {
		return icon, true
	}
	return 0, false
}

func isProfileWindowMarkerIcon(hwnd uintptr, iconType uintptr, expectedIcon uintptr) bool {
	if expectedIcon == 0 {
		return false
	}
	currentIcon, ok := getProfileWindowMarkerIcon(hwnd, iconType)
	return ok && currentIcon == expectedIcon
}

func refreshProfileWindowMarkerIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procRedrawWindowMarker.Call(hwnd, 0, 0, rdwInvalidate|rdwFrame)
}

func destroyProfileWindowMarkerDecoratedIcons(state *profileWindowMarkerWindowState) {
	if state == nil {
		return
	}
	if state.decoratedBigIcon != 0 {
		procDestroyIconMarker.Call(state.decoratedBigIcon)
		state.decoratedBigIcon = 0
	}
	if state.decoratedSmallIcon != 0 {
		procDestroyIconMarker.Call(state.decoratedSmallIcon)
		state.decoratedSmallIcon = 0
	}
	if state.taskbarOverlayIcon != 0 {
		procDestroyIconMarker.Call(state.taskbarOverlayIcon)
		state.taskbarOverlayIcon = 0
	}
	state.decoratedBigIconApplied = false
	state.decoratedSmallIconApplied = false
	state.taskbarOverlayIconApplied = false
	state.taskbarOverlayLastAppliedAt = time.Time{}
}

func destroyProfileWindowMarkerChromeIcons(state *profileWindowMarkerWindowState) {
	if state == nil {
		return
	}
	bigIcon := state.chromeBigIcon
	smallIcon := state.chromeSmallIcon
	if bigIcon != 0 {
		procDestroyIconMarker.Call(bigIcon)
	}
	if smallIcon != 0 && smallIcon != bigIcon {
		procDestroyIconMarker.Call(smallIcon)
	}
	state.chromeBigIcon = 0
	state.chromeSmallIcon = 0
	state.chromeIconPath = ""
	state.chromeIconApplied = false
}

func isProfileWindowMarkerWindowProcess(hwnd uintptr, processID uint32) bool {
	if hwnd == 0 || processID == 0 {
		return hwnd != 0
	}
	var currentProcessID uint32
	procGetWindowThreadProcessIDMarker.Call(hwnd, uintptr(unsafe.Pointer(&currentProcessID)))
	return currentProcessID == processID
}

func sendProfileWindowMarkerMessage(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) {
	_, _ = sendProfileWindowMarkerMessageResult(hwnd, message, wParam, lParam)
}

func sendProfileWindowMarkerMessageResult(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) (uintptr, bool) {
	var result uintptr
	status, _, _ := procSendMessageTimeoutMarker.Call(
		hwnd,
		uintptr(message),
		wParam,
		lParam,
		smtoAbortIfHung,
		sendMessageTimeout,
		uintptr(unsafe.Pointer(&result)),
	)
	return result, status != 0
}

func withProfileWindowMarkerCOM(work func() bool) bool {
	if work == nil {
		return false
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	hr, _, _ := procCoInitializeExMarker.Call(0, coinitApartment)
	if profileWindowMarkerHRESULTSucceeded(hr) {
		initialized = true
	} else if uint32(hr) != rpcEChangedMode {
		return false
	}
	if initialized {
		defer procCoUninitializeMarker.Call()
	}
	return work()
}

func profileWindowMarkerHRESULTSucceeded(hr uintptr) bool {
	return uint32(hr)&0x80000000 == 0
}

func setProfileWindowMarkerAppUserModelID(hwnd uintptr, appID string) bool {
	return setProfileWindowMarkerStringProperty(hwnd, appUserModelIDPropertyKey, appID)
}

func setProfileWindowMarkerStringProperty(hwnd uintptr, propertyKey profileWindowMarkerPropertyKey, value string) bool {
	if hwnd == 0 {
		return false
	}
	return withProfileWindowMarkerCOM(func() bool {
		var store *profileWindowMarkerPropertyStore
		hr, _, _ := procSHGetPropertyStoreForWindow.Call(
			hwnd,
			uintptr(unsafe.Pointer(&iidPropertyStoreWindowMarker)),
			uintptr(unsafe.Pointer(&store)),
		)
		if !profileWindowMarkerHRESULTSucceeded(hr) || store == nil || store.VTable == nil {
			return false
		}
		defer store.release()

		propValue := profileWindowMarkerPropVariant{VT: vtEmpty}
		var appIDChars []uint16
		if strings.TrimSpace(value) != "" {
			var err error
			appIDChars, err = windows.UTF16FromString(value)
			if err != nil || len(appIDChars) == 0 {
				return false
			}
			propValue.VT = vtLPWSTR
			propValue.Pointer = uintptr(unsafe.Pointer(&appIDChars[0]))
		}

		hr, _, _ = syscall.SyscallN(
			store.VTable.SetValue,
			uintptr(unsafe.Pointer(store)),
			uintptr(unsafe.Pointer(&propertyKey)),
			uintptr(unsafe.Pointer(&propValue)),
		)
		runtime.KeepAlive(appIDChars)
		if !profileWindowMarkerHRESULTSucceeded(hr) {
			return false
		}
		hr, _, _ = syscall.SyscallN(store.VTable.Commit, uintptr(unsafe.Pointer(store)))
		return profileWindowMarkerHRESULTSucceeded(hr)
	})
}

func clearProfileWindowMarkerAppUserModelID(hwnd uintptr) bool {
	return setProfileWindowMarkerAppUserModelID(hwnd, "")
}

func setProfileWindowMarkerTaskbarOverlay(hwnd uintptr, overlayIcon uintptr, markerCode string) bool {
	if hwnd == 0 {
		return false
	}
	return withProfileWindowMarkerCOM(func() bool {
		taskbar := createProfileWindowMarkerTaskbarList()
		if taskbar == nil || taskbar.VTable == nil {
			return false
		}
		defer taskbar.release()

		description := strings.TrimSpace(markerCode)
		if description == "" {
			description = "实例标识"
		} else {
			description = "实例 " + description
		}
		descriptionChars, err := windows.UTF16FromString(description)
		if err != nil || len(descriptionChars) == 0 {
			return false
		}
		hr, _, _ := syscall.SyscallN(
			taskbar.VTable.SetOverlayIcon,
			uintptr(unsafe.Pointer(taskbar)),
			hwnd,
			overlayIcon,
			uintptr(unsafe.Pointer(&descriptionChars[0])),
		)
		runtime.KeepAlive(descriptionChars)
		return profileWindowMarkerHRESULTSucceeded(hr)
	})
}

func clearProfileWindowMarkerTaskbarOverlay(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	return withProfileWindowMarkerCOM(func() bool {
		taskbar := createProfileWindowMarkerTaskbarList()
		if taskbar == nil || taskbar.VTable == nil {
			return false
		}
		defer taskbar.release()
		hr, _, _ := syscall.SyscallN(
			taskbar.VTable.SetOverlayIcon,
			uintptr(unsafe.Pointer(taskbar)),
			hwnd,
			0,
			0,
		)
		return profileWindowMarkerHRESULTSucceeded(hr)
	})
}

func createProfileWindowMarkerTaskbarList() *profileWindowMarkerTaskbarList3 {
	var taskbar *profileWindowMarkerTaskbarList3
	hr, _, _ := procCoCreateInstanceMarker.Call(
		uintptr(unsafe.Pointer(&clsidTaskbarListWindowMarker)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidTaskbarList3WindowMarker)),
		uintptr(unsafe.Pointer(&taskbar)),
	)
	if !profileWindowMarkerHRESULTSucceeded(hr) || taskbar == nil || taskbar.VTable == nil {
		return nil
	}
	hr, _, _ = syscall.SyscallN(taskbar.VTable.HrInit, uintptr(unsafe.Pointer(taskbar)))
	if !profileWindowMarkerHRESULTSucceeded(hr) {
		taskbar.release()
		return nil
	}
	return taskbar
}

func (store *profileWindowMarkerPropertyStore) release() {
	if store == nil || store.VTable == nil || store.VTable.Release == 0 {
		return
	}
	syscall.SyscallN(store.VTable.Release, uintptr(unsafe.Pointer(store)))
}

func (taskbar *profileWindowMarkerTaskbarList3) release() {
	if taskbar == nil || taskbar.VTable == nil || taskbar.VTable.Release == 0 {
		return
	}
	syscall.SyscallN(taskbar.VTable.Release, uintptr(unsafe.Pointer(taskbar)))
}

func createProfileWindowMarkerOverlayIcon(markerCode string) (uintptr, error) {
	return createProfileWindowMarkerIcon(
		profileWindowMarkerOverlayIconWidth,
		profileWindowMarkerOverlayIconHeight,
		func(pixels []uint32) { drawProfileWindowMarkerOverlayBadge(pixels, markerCode) },
	)
}

func createProfileWindowMarkerDecoratedIcon(originalIcon uintptr, width, height int, markerCode string) (uintptr, error) {
	return createProfileWindowMarkerIconFromSource(
		originalIcon,
		width,
		height,
		func(pixels []uint32) { drawProfileWindowMarkerDecoratedBadge(pixels, width, height, markerCode) },
	)
}

func createProfileWindowMarkerInstanceIcon(width, height int, markerCode string) (uintptr, error) {
	return createProfileWindowMarkerIcon(
		width,
		height,
		func(pixels []uint32) { drawProfileWindowMarkerInstanceIcon(pixels, width, height, markerCode) },
	)
}

func createProfileWindowMarkerFallbackIcon(markerCode string) (uintptr, error) {
	return createProfileWindowMarkerIcon(
		profileWindowMarkerFallbackIconWidth,
		profileWindowMarkerFallbackIconHeight,
		func(pixels []uint32) { drawProfileWindowMarkerFallbackIcon(pixels, markerCode) },
	)
}

func createProfileWindowMarkerIcon(width, height int, draw func([]uint32)) (uintptr, error) {
	return createProfileWindowMarkerIconFromSource(0, width, height, draw)
}

func createProfileWindowMarkerIconFromSource(sourceIcon uintptr, width, height int, draw func([]uint32)) (uintptr, error) {
	var bitmapInfo profileWindowMarkerBitmapInfo
	bitmapInfo.Header = profileWindowMarkerBitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(profileWindowMarkerBitmapInfoHeader{})),
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}

	var bits unsafe.Pointer
	colorBitmap, _, err := procCreateDIBSectionMarker.Call(
		0,
		uintptr(unsafe.Pointer(&bitmapInfo)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if colorBitmap == 0 || bits == nil {
		return 0, err
	}

	maskBitmap, _, maskErr := procCreateBitmapMarker.Call(uintptr(width), uintptr(height), 1, 1, 0)
	if maskBitmap == 0 {
		procDeleteObjectMarker.Call(colorBitmap)
		return 0, maskErr
	}

	pixels := unsafe.Slice((*uint32)(bits), width*height)
	for index := range pixels {
		pixels[index] = 0
	}
	if sourceIcon != 0 {
		memoryDC, _, dcErr := procCreateCompatibleDCMarker.Call(0)
		if memoryDC == 0 {
			procDeleteObjectMarker.Call(maskBitmap)
			procDeleteObjectMarker.Call(colorBitmap)
			return 0, dcErr
		}
		previousObject, _, selectErr := procSelectObjectMarker.Call(memoryDC, colorBitmap)
		if previousObject == 0 {
			procDeleteDCMarker.Call(memoryDC)
			procDeleteObjectMarker.Call(maskBitmap)
			procDeleteObjectMarker.Call(colorBitmap)
			if selectErr != nil {
				return 0, selectErr
			}
			return 0, syscall.EINVAL
		}
		drawn, _, drawErr := procDrawIconExMarker.Call(
			memoryDC,
			0,
			0,
			uintptr(width),
			uintptr(height),
			sourceIcon,
			0,
			0,
			0,
			diNormal,
		)
		procSelectObjectMarker.Call(memoryDC, previousObject)
		procDeleteDCMarker.Call(memoryDC)
		if drawn == 0 {
			procDeleteObjectMarker.Call(maskBitmap)
			procDeleteObjectMarker.Call(colorBitmap)
			if drawErr != nil {
				return 0, drawErr
			}
			return 0, selectErr
		}
	}

	draw(pixels)

	iconInfo := profileWindowMarkerIconInfo{
		FIcon:    1,
		HbmMask:  maskBitmap,
		HbmColor: colorBitmap,
	}
	icon, _, iconErr := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&iconInfo)))
	procDeleteObjectMarker.Call(maskBitmap)
	procDeleteObjectMarker.Call(colorBitmap)
	if icon == 0 {
		return 0, iconErr
	}
	return icon, nil
}

func fillProfileWindowMarkerCircle(pixels []uint32, centerX, centerY, radius int, color uint32) {
	fillProfileWindowMarkerCircleOnCanvas(pixels, profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight, centerX, centerY, radius, color)
}

type profileWindowMarkerBadgeGeometry struct {
	centerX      int
	centerY      int
	outerRadius  int
	innerRadius  int
	shadowRadius int
}

func profileWindowMarkerBadgeGeometryFor(width, height int) profileWindowMarkerBadgeGeometry {
	outerRadius := 6
	if width >= 24 {
		outerRadius = width/3 + 2
	}
	if width >= 48 {
		outerRadius = width/3 + 3
	}
	innerRadius := outerRadius - 1
	if innerRadius < 5 {
		innerRadius = 5
	}
	shadowRadius := outerRadius + 1
	centerX := width - shadowRadius - 2
	if centerX < shadowRadius {
		centerX = shadowRadius
	}
	centerY := shadowRadius + 1
	if centerY > height-shadowRadius {
		centerY = height - shadowRadius
	}
	return profileWindowMarkerBadgeGeometry{
		centerX:      centerX,
		centerY:      centerY,
		outerRadius:  outerRadius,
		innerRadius:  innerRadius,
		shadowRadius: shadowRadius,
	}
}

func fillProfileWindowMarkerCircleOnCanvas(pixels []uint32, width, height, centerX, centerY, radius int, color uint32) {
	radiusSquared := radius * radius
	for pixelY := centerY - radius; pixelY <= centerY+radius; pixelY++ {
		for pixelX := centerX - radius; pixelX <= centerX+radius; pixelX++ {
			deltaX := pixelX - centerX
			deltaY := pixelY - centerY
			if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
				setProfileWindowMarkerPixelOnCanvas(pixels, width, height, pixelX, pixelY, color)
			}
		}
	}
}

func fillProfileWindowMarkerCircleAntiAliasedOnCanvas(pixels []uint32, width, height, centerX, centerY, radius int, color uint32) {
	if width <= 0 || height <= 0 || len(pixels) < width*height || radius <= 0 {
		return
	}
	radiusSquared := float64(radius * radius)
	for pixelY := 0; pixelY < height; pixelY++ {
		for pixelX := 0; pixelX < width; pixelX++ {
			coveredSamples := 0
			for sampleY := 0; sampleY < 4; sampleY++ {
				for sampleX := 0; sampleX < 4; sampleX++ {
					pointX := float64(pixelX) + (float64(sampleX)+0.5)/4
					pointY := float64(pixelY) + (float64(sampleY)+0.5)/4
					deltaX := pointX - float64(centerX)
					deltaY := pointY - float64(centerY)
					if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
						coveredSamples++
					}
				}
			}
			if coveredSamples == 0 {
				continue
			}
			blendProfileWindowMarkerPixelOnCanvas(pixels, width, height, pixelX, pixelY, color, uint32(coveredSamples)*255/16)
		}
	}
}

func blendProfileWindowMarkerPixelOnCanvas(pixels []uint32, width, height, x, y int, color uint32, alpha uint32) {
	if x < 0 || x >= width || y < 0 || y >= height || alpha == 0 {
		return
	}
	if alpha >= 255 {
		pixels[y*width+x] = color
		return
	}
	destination := pixels[y*width+x]
	destinationAlpha := (destination >> 24) & 0xFF
	sourceRed := (color >> 16) & 0xFF
	sourceGreen := (color >> 8) & 0xFF
	sourceBlue := color & 0xFF
	destinationRed := (destination >> 16) & 0xFF
	destinationGreen := (destination >> 8) & 0xFF
	destinationBlue := destination & 0xFF
	outAlpha := alpha + destinationAlpha*(255-alpha)/255
	outRed := (sourceRed*alpha + destinationRed*destinationAlpha*(255-alpha)/255) / outAlpha
	outGreen := (sourceGreen*alpha + destinationGreen*destinationAlpha*(255-alpha)/255) / outAlpha
	outBlue := (sourceBlue*alpha + destinationBlue*destinationAlpha*(255-alpha)/255) / outAlpha
	pixels[y*width+x] = outAlpha<<24 | outRed<<16 | outGreen<<8 | outBlue
}

func setProfileWindowMarkerPixel(pixels []uint32, x, y int, color uint32) {
	setProfileWindowMarkerPixelOnCanvas(pixels, profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight, x, y, color)
}

func setProfileWindowMarkerPixelOnCanvas(pixels []uint32, width, height, x, y int, color uint32) {
	if x < 0 || x >= width || y < 0 || y >= height {
		return
	}
	pixels[y*width+x] = color
}

func fillProfileWindowMarkerRectOnCanvas(pixels []uint32, width, height, left, top, right, bottom int, color uint32) {
	for pixelY := top; pixelY <= bottom; pixelY++ {
		for pixelX := left; pixelX <= right; pixelX++ {
			setProfileWindowMarkerPixelOnCanvas(pixels, width, height, pixelX, pixelY, color)
		}
	}
}

func fillProfileWindowMarkerRoundedRectOnCanvas(pixels []uint32, width, height, left, top, right, bottom, radius int, color uint32) {
	if radius <= 0 {
		fillProfileWindowMarkerRectOnCanvas(pixels, width, height, left, top, right, bottom, color)
		return
	}
	radiusSquared := radius * radius
	for pixelY := top; pixelY <= bottom; pixelY++ {
		for pixelX := left; pixelX <= right; pixelX++ {
			cornerX := pixelX
			cornerY := pixelY
			if pixelX < left+radius {
				cornerX = left + radius
			} else if pixelX > right-radius {
				cornerX = right - radius
			}
			if pixelY < top+radius {
				cornerY = top + radius
			} else if pixelY > bottom-radius {
				cornerY = bottom - radius
			}
			deltaX := pixelX - cornerX
			deltaY := pixelY - cornerY
			if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
				setProfileWindowMarkerPixelOnCanvas(pixels, width, height, pixelX, pixelY, color)
			}
		}
	}
}

func drawProfileWindowMarkerLineOnCanvas(pixels []uint32, width, height, startX, startY, endX, endY, radius int, color uint32) {
	deltaX := absProfileWindowMarkerInt(endX - startX)
	deltaY := -absProfileWindowMarkerInt(endY - startY)
	stepX := -1
	if startX < endX {
		stepX = 1
	}
	stepY := -1
	if startY < endY {
		stepY = 1
	}
	errorValue := deltaX + deltaY
	for {
		fillProfileWindowMarkerCircleOnCanvas(pixels, width, height, startX, startY, radius, color)
		if startX == endX && startY == endY {
			break
		}
		doubleError := 2 * errorValue
		if doubleError >= deltaY {
			errorValue += deltaY
			startX += stepX
		}
		if doubleError <= deltaX {
			errorValue += deltaX
			startY += stepY
		}
	}
}

func drawProfileWindowMarkerScaledPolylineOnCanvas(pixels []uint32, width, height int, points [][2]int, radius int, color uint32) {
	if len(points) < 2 {
		return
	}
	scaleX := func(value int) int { return value * (width - 1) / (profileWindowMarkerFallbackIconWidth - 1) }
	scaleY := func(value int) int { return value * (height - 1) / (profileWindowMarkerFallbackIconHeight - 1) }
	for index := 1; index < len(points); index++ {
		start := points[index-1]
		end := points[index]
		drawProfileWindowMarkerLineOnCanvas(pixels, width, height, scaleX(start[0]), scaleY(start[1]), scaleX(end[0]), scaleY(end[1]), radius, color)
	}
}

func absProfileWindowMarkerInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minProfileWindowMarkerInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxProfileWindowMarkerInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func drawProfileWindowMarkerOverlayBadge(pixels []uint32, markerCode string) {
	drawProfileWindowMarkerDecoratedBadge(pixels, profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight, markerCode)
}

func drawProfileWindowMarkerDecoratedBadge(pixels []uint32, width, height int, markerCode string) {
	if width <= 0 || height <= 0 {
		return
	}
	geometry := profileWindowMarkerBadgeGeometryFor(width, height)
	badgeColor := profileWindowMarkerBadgeBackground
	glyphColor := profileWindowMarkerGlyphColor
	fillProfileWindowMarkerCircleAntiAliasedOnCanvas(pixels, width, height, geometry.centerX, geometry.centerY, geometry.shadowRadius, profileWindowMarkerBadgeShadowColor)
	fillProfileWindowMarkerCircleAntiAliasedOnCanvas(pixels, width, height, geometry.centerX, geometry.centerY, geometry.outerRadius, profileWindowMarkerFrameColor)
	fillProfileWindowMarkerCircleAntiAliasedOnCanvas(pixels, width, height, geometry.centerX, geometry.centerY, geometry.innerRadius, badgeColor)
	drawProfileWindowMarkerBadgeGlyphOnCanvas(pixels, width, height, markerCode, geometry.centerX, geometry.centerY, geometry.innerRadius, glyphColor)
}

func drawProfileWindowMarkerInstanceIcon(pixels []uint32, width, height int, markerCode string) {
	if drawProfileWindowMarkerEmbeddedLogoIcon(pixels, width, height) {
		drawProfileWindowMarkerDecoratedBadge(pixels, width, height, markerCode)
		return
	}
	drawProfileWindowMarkerFallbackFingerprintIcon(pixels, width, height, markerCode)
}

func drawProfileWindowMarkerEmbeddedLogoIcon(pixels []uint32, width, height int) bool {
	if width <= 0 || height <= 0 || len(pixels) < width*height || len(profileWindowMarkerLogoPNG) == 0 {
		return false
	}
	logo, err := png.Decode(bytes.NewReader(profileWindowMarkerLogoPNG))
	if err != nil {
		return false
	}
	bounds := logo.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return false
	}
	for pixelY := 0; pixelY < height; pixelY++ {
		sourceY := bounds.Min.Y + pixelY*bounds.Dy()/height
		for pixelX := 0; pixelX < width; pixelX++ {
			sourceX := bounds.Min.X + pixelX*bounds.Dx()/width
			r, g, b, a := logo.At(sourceX, sourceY).RGBA()
			pixels[pixelY*width+pixelX] = uint32(a>>8)<<24 | uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
		}
	}
	return true
}

func drawProfileWindowMarkerFallbackFingerprintIcon(pixels []uint32, width, height int, markerCode string) {
	if width <= 0 || height <= 0 {
		return
	}
	strokeRadius := 1
	fingerprintLines := [][][2]int{
		{{4, 18}, {5, 14}, {8, 10}, {12, 7}, {17, 7}, {22, 9}, {25, 13}, {26, 17}},
		{{5, 23}, {8, 21}, {10, 17}, {13, 13}, {17, 12}, {21, 14}, {23, 18}, {23, 22}},
		{{7, 27}, {11, 25}, {13, 21}, {15, 17}, {18, 16}, {20, 18}, {20, 22}, {18, 25}},
		{{12, 29}, {15, 27}, {16, 23}, {16, 20}, {18, 19}, {19, 21}, {18, 24}},
		{{3, 25}, {6, 25}, {9, 23}, {11, 20}},
		{{27, 14}, {28, 18}, {28, 22}},
	}
	for index, points := range fingerprintLines {
		lineColor := profileWindowMarkerFingerprintLine
		if index == 1 || index == 3 {
			lineColor = profileWindowMarkerFingerprintAccent
		}
		drawProfileWindowMarkerScaledPolylineOnCanvas(pixels, width, height, points, strokeRadius, lineColor)
	}
	dotX := width - width/4
	dotY := height / 5
	dotRadius := minProfileWindowMarkerInt(width, height) / 12
	if dotRadius < 1 {
		dotRadius = 1
	}
	fillProfileWindowMarkerCircleOnCanvas(pixels, width, height, dotX, dotY, dotRadius, profileWindowMarkerFingerprintDot)
	drawProfileWindowMarkerDecoratedBadge(pixels, width, height, markerCode)
}

func drawProfileWindowMarkerFallbackIcon(pixels []uint32, markerCode string) {
	drawProfileWindowMarkerInstanceIcon(pixels, profileWindowMarkerFallbackIconWidth, profileWindowMarkerFallbackIconHeight, markerCode)
}

func drawProfileWindowMarkerBadgeGlyphOnCanvas(pixels []uint32, width, height int, markerCode string, centerX, centerY, innerRadius int, color uint32) {
	normalizedCode := strings.ToUpper(strings.TrimSpace(markerCode))
	if normalizedCode == "" {
		return
	}
	if innerRadius >= 7 && renderProfileWindowMarkerTextGlyph(pixels, width, height, normalizedCode, centerX, centerY, innerRadius, color) {
		return
	}
	if innerRadius < 13 {
		glyphScale := 1
		if innerRadius >= 7 {
			glyphScale = 2
		}
		drawProfileWindowMarkerGlyphClippedOnCanvas(
			pixels,
			width,
			height,
			normalizedCode[:1],
			centerX-5*glyphScale/2,
			centerY-7*glyphScale/2,
			glyphScale,
			innerRadius,
			color,
		)
		return
	}
	glyphScale := 3
	drawProfileWindowMarkerGlyphOnCanvas(pixels, width, height, normalizedCode, centerX-5*glyphScale/2, centerY-7*glyphScale/2, glyphScale, color)
}

func drawProfileWindowMarkerGlyphClippedOnCanvas(
	pixels []uint32,
	width, height int,
	markerCode string,
	startX, startY, scale, radius int,
	color uint32,
) {
	if width <= 0 || height <= 0 || len(pixels) < width*height || radius <= 0 {
		return
	}

	glyphPixels := make([]uint32, width*height)
	drawProfileWindowMarkerGlyphOnCanvas(
		glyphPixels,
		width,
		height,
		markerCode,
		startX,
		startY,
		scale,
		color,
	)

	centerX := startX + 5*scale/2
	centerY := startY + 7*scale/2
	radiusSquared := radius * radius

	for pixelY := 0; pixelY < height; pixelY++ {
		for pixelX := 0; pixelX < width; pixelX++ {
			glyph := glyphPixels[pixelY*width+pixelX]
			if glyph == 0 {
				continue
			}
			deltaX := pixelX - centerX
			deltaY := pixelY - centerY
			if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
				pixels[pixelY*width+pixelX] = glyph
			}
		}
	}
}

func renderProfileWindowMarkerTextGlyph(pixels []uint32, width, height int, markerCode string, centerX, centerY, innerRadius int, color uint32) bool {
	if markerCode == "" || innerRadius < 7 || width <= 0 || height <= 0 || len(pixels) < width*height {
		return false
	}
	fontSize := innerRadius
	if len(markerCode) < 2 {
		fontSize = innerRadius * 3 / 2
	}
	if fontSize < 7 {
		fontSize = 7
	}
	textWidth := len(markerCode)*fontSize*3/2 + fontSize
	textHeight := fontSize * 2

	var bitmapInfo profileWindowMarkerBitmapInfo
	bitmapInfo.Header = profileWindowMarkerBitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(profileWindowMarkerBitmapInfoHeader{})),
		Width:       int32(textWidth),
		Height:      -int32(textHeight),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	var bits unsafe.Pointer
	colorBitmap, _, _ := procCreateDIBSectionMarker.Call(
		0,
		uintptr(unsafe.Pointer(&bitmapInfo)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if colorBitmap == 0 || bits == nil {
		return false
	}
	memoryDC, _, _ := procCreateCompatibleDCMarker.Call(0)
	if memoryDC == 0 {
		procDeleteObjectMarker.Call(colorBitmap)
		return false
	}
	previousObject, _, _ := procSelectObjectMarker.Call(memoryDC, colorBitmap)
	if previousObject == 0 {
		procDeleteDCMarker.Call(memoryDC)
		procDeleteObjectMarker.Call(colorBitmap)
		return false
	}
	textPixels := unsafe.Slice((*uint32)(bits), textWidth*textHeight)
	for index := range textPixels {
		textPixels[index] = 0
	}
	faceName, faceErr := windows.UTF16PtrFromString("Segoe UI")
	if faceErr != nil {
		procSelectObjectMarker.Call(memoryDC, previousObject)
		procDeleteDCMarker.Call(memoryDC)
		procDeleteObjectMarker.Call(colorBitmap)
		return false
	}
	font, _, _ := procCreateFontWMarker.Call(
		uintptr(int32(-fontSize)),
		0,
		0,
		0,
		fwBold,
		0,
		0,
		0,
		defaultCharset,
		0,
		0,
		antialiasedQuality,
		0,
		uintptr(unsafe.Pointer(faceName)),
	)
	if font == 0 {
		procSelectObjectMarker.Call(memoryDC, previousObject)
		procDeleteDCMarker.Call(memoryDC)
		procDeleteObjectMarker.Call(colorBitmap)
		return false
	}
	procSelectObjectMarker.Call(memoryDC, font)
	procSetTextColorMarker.Call(memoryDC, 0x00FFFFFF)
	procSetBkModeMarker.Call(memoryDC, transparentBk)
	text, textErr := windows.UTF16FromString(markerCode)
	if textErr != nil {
		procSelectObjectMarker.Call(memoryDC, previousObject)
		procDeleteObjectMarker.Call(font)
		procDeleteDCMarker.Call(memoryDC)
		procDeleteObjectMarker.Call(colorBitmap)
		return false
	}
	textRect := profileWindowMarkerTextRect{
		Left:   0,
		Top:    0,
		Right:  int32(textWidth),
		Bottom: int32(textHeight),
	}
	procDrawTextWMarker.Call(
		memoryDC,
		uintptr(unsafe.Pointer(&text[0])),
		^uintptr(0),
		uintptr(unsafe.Pointer(&textRect)),
		dtCenter|dtVCenter|dtSingleLine,
	)

	radiusSquared := innerRadius * innerRadius
	glyphRed := (color >> 16) & 0xFF
	glyphGreen := (color >> 8) & 0xFF
	glyphBlue := color & 0xFF
	for textY := 0; textY < textHeight; textY++ {
		for textX := 0; textX < textWidth; textX++ {
			luminance := int(textPixels[textY*textWidth+textX] & 0xFF)
			if luminance <= 0 {
				continue
			}
			deltaX := textX - textWidth/2
			deltaY := textY - textHeight/2
			if deltaX*deltaX+deltaY*deltaY > radiusSquared {
				continue
			}
			pixelX := centerX + deltaX
			pixelY := centerY + deltaY
			if pixelX < 0 || pixelX >= width || pixelY < 0 || pixelY >= height {
				continue
			}
			inverse := 255 - luminance
			destination := pixels[pixelY*width+pixelX]
			blendedRed := (glyphRed*uint32(luminance) + ((destination>>16)&0xFF)*uint32(inverse)) / 255
			blendedGreen := (glyphGreen*uint32(luminance) + ((destination>>8)&0xFF)*uint32(inverse)) / 255
			blendedBlue := (glyphBlue*uint32(luminance) + (destination&0xFF)*uint32(inverse)) / 255
			pixels[pixelY*width+pixelX] = 0xFF000000 | blendedRed<<16 | blendedGreen<<8 | blendedBlue
		}
	}

	procSelectObjectMarker.Call(memoryDC, previousObject)
	procDeleteObjectMarker.Call(font)
	procDeleteDCMarker.Call(memoryDC)
	procDeleteObjectMarker.Call(colorBitmap)
	return true
}

func profileWindowMarkerColor(_ string) uint32 {
	return profileWindowMarkerBadgeBackground
}

func profileWindowMarkerGlyphColorFor(_ uint32) uint32 {
	return profileWindowMarkerGlyphColor
}

func drawProfileWindowMarkerGlyph(pixels []uint32, markerCode string, startX, startY, scale int, color uint32) {
	drawProfileWindowMarkerGlyphOnCanvas(pixels, profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight, markerCode, startX, startY, scale, color)
}

func drawProfileWindowMarkerGlyphOnCanvas(pixels []uint32, width, height int, markerCode string, startX, startY, scale int, color uint32) {
	markerCode = strings.ToUpper(strings.TrimSpace(markerCode))
	if markerCode == "" {
		return
	}
	const (
		glyphWidth  = 5
		glyphHeight = 7
	)
	patterns := map[byte][glyphHeight]string{
		'0': {"11111", "10001", "10011", "10101", "11001", "10001", "11111"},
		'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
		'2': {"11110", "00001", "00001", "01110", "10000", "10000", "11111"},
		'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
		'4': {"10010", "10010", "10010", "11111", "00010", "00010", "00010"},
		'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
		'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
		'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
		'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
		'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
		'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
		'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
		'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
		'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
		'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
		'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
		'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01111"},
		'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
		'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
		'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
		'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
		'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
		'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
		'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
		'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
		'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
		'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
		'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
		'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
		'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
		'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
		'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
		'W': {"10001", "10001", "10001", "10101", "10101", "11011", "10001"},
		'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
		'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
		'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
	}
	visibleCode := markerCode[:1]
	for index := 0; index < len(visibleCode); index++ {
		pattern, ok := patterns[visibleCode[index]]
		if !ok {
			continue
		}
		glyphX := startX + index*glyphWidth*scale
		for row, line := range pattern {
			for column, value := range line {
				if value != '1' {
					continue
				}
				for y := 0; y < scale; y++ {
					for x := 0; x < scale; x++ {
						pixelX := glyphX + column*scale + x
						pixelY := startY + row*scale + y
						if pixelX >= 0 && pixelX < width && pixelY >= 0 && pixelY < height {
							pixels[pixelY*width+pixelX] = color
						}
					}
				}
			}
		}
	}
}
