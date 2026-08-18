//go:build windows
// +build windows

package backend

import (
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

	gwOwner       = 4
	iconWidth     = 32
	iconHeight    = 32
	dibRGBColors  = 0
	biRGB         = 0
	rdwInvalidate = 0x0001
	rdwFrame      = 0x0400

	profileWindowMarkerFrameColor   uint32 = 0xFF0F172A
	profileWindowMarkerSurfaceColor uint32 = 0xFFF8FAFC
	profileWindowMarkerHeaderColor  uint32 = 0xFF0F766E
	profileWindowMarkerLineColor    uint32 = 0xFFCBD5E1
)

var (
	user32WindowMarker = windows.NewLazySystemDLL("user32.dll")
	gdi32WindowMarker  = windows.NewLazySystemDLL("gdi32.dll")

	procEnumWindowsMarker              = user32WindowMarker.NewProc("EnumWindows")
	procGetWindowThreadProcessIDMarker = user32WindowMarker.NewProc("GetWindowThreadProcessId")
	procIsWindowVisibleMarker          = user32WindowMarker.NewProc("IsWindowVisible")
	procIsIconicMarker                 = user32WindowMarker.NewProc("IsIconic")
	procGetWindowMarker                = user32WindowMarker.NewProc("GetWindow")
	procGetWindowTextLengthMarker      = user32WindowMarker.NewProc("GetWindowTextLengthW")
	procGetWindowTextMarker            = user32WindowMarker.NewProc("GetWindowTextW")
	procSendMessageTimeoutMarker       = user32WindowMarker.NewProc("SendMessageTimeoutW")

	procCreateDIBSectionMarker = gdi32WindowMarker.NewProc("CreateDIBSection")
	procCreateBitmapMarker     = gdi32WindowMarker.NewProc("CreateBitmap")
	procDeleteObjectMarker     = gdi32WindowMarker.NewProc("DeleteObject")
	procCreateIconIndirect     = user32WindowMarker.NewProc("CreateIconIndirect")
	procDestroyIconMarker      = user32WindowMarker.NewProc("DestroyIcon")
)

var procRedrawWindowMarker = user32WindowMarker.NewProc(`RedrawWindow`)

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

	var icon uintptr
	nextFallbackScan := time.Time{}
	defer func() {
		cleared := make(map[uintptr]struct{})
		for _, hwnd := range marker.rememberedWindows() {
			state, _ := marker.windowState(hwnd)
			if !isProfileWindowMarkerWindowProcess(hwnd, state.processID) {
				continue
			}
			clearProfileWindowMarkerTitle(hwnd)
			clearProfileWindowMarker(hwnd, state, icon)
			cleared[hwnd] = struct{}{}
		}
		for _, window := range findTopLevelWindowsForProfileMarker(profileWindowMarkerPID(marker)) {
			if _, exists := cleared[window.hwnd]; exists {
				continue
			}
			state, _ := marker.windowState(window.hwnd)
			if state.processID > 0 && !isProfileWindowMarkerWindowProcess(window.hwnd, state.processID) {
				continue
			}
			clearProfileWindowMarkerTitle(window.hwnd)
			clearProfileWindowMarker(window.hwnd, state, icon)
		}
		if icon != 0 {
			procDestroyIconMarker.Call(icon)
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
		if icon == 0 {
			icon, _ = createProfileWindowMarkerIcon(marker.code)
		}

		windowsForMarker := findTopLevelWindowsForProfileMarker(uint32(target.pid))
		if len(windowsForMarker) == 0 && time.Now().After(nextFallbackScan) {
			windowsForMarker = findTopLevelWindowsForProfileMarkerByUserData(target)
			nextFallbackScan = time.Now().Add(fallbackScanInterval)
		}
		for _, window := range windowsForMarker {
			applyProfileWindowMarker(marker, window, target, icon)
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

func findTopLevelWindowsForProfileMarker(pid uint32) []profileWindowMarkerWindow {
	if pid == 0 {
		return nil
	}

	var windowsForPID []profileWindowMarkerWindow
	callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessIDMarker.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID != pid {
			return 1
		}

		visible, _, _ := procIsWindowVisibleMarker.Call(hwnd)
		titleLength, _, _ := procGetWindowTextLengthMarker.Call(hwnd)
		owner, _, _ := procGetWindowMarker.Call(hwnd, gwOwner)
		minimized, _, _ := procIsIconicMarker.Call(hwnd)
		windowsForPID = append(windowsForPID, profileWindowMarkerWindow{
			hwnd:      hwnd,
			processID: windowPID,
			visible:   visible != 0,
			titled:    titleLength > 0,
			owned:     owner != 0,
			minimized: minimized != 0,
		})
		return 1
	})
	procEnumWindowsMarker.Call(callback, 0)

	return prioritizeProfileMarkerWindows(windowsForPID)
}

func findTopLevelWindowsForProfileMarkerByUserData(target profileWindowMarkerTarget) []profileWindowMarkerWindow {
	if strings.TrimSpace(target.userDataDir) == "" {
		return nil
	}
	processes, err := findBrowserUserDataProcessesOS(target.userDataDir)
	if err != nil {
		return nil
	}
	seen := make(map[uint32]struct{})
	var windowsForProfile []profileWindowMarkerWindow
	for _, process := range processes {
		if process.PID <= 0 {
			continue
		}
		processPID := uint32(process.PID)
		if _, exists := seen[processPID]; exists {
			continue
		}
		if target.debugPort > 0 && process.DebugPort > 0 && process.DebugPort != target.debugPort {
			continue
		}
		seen[processPID] = struct{}{}
		windowsForProfile = append(windowsForProfile, findTopLevelWindowsForProfileMarker(processPID)...)
	}
	return prioritizeProfileMarkerWindows(windowsForProfile)
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

func applyProfileWindowMarker(marker *profileWindowMarker, window profileWindowMarkerWindow, target profileWindowMarkerTarget, icon uintptr) {
	if window.hwnd == 0 {
		return
	}
	markerCode := ""
	if marker != nil {
		markerCode = marker.code
		marker.rememberWindow(window.hwnd, window.processID)
	}

	currentTitle := getProfileWindowMarkerTitle(window.hwnd)
	desiredTitle := applyProfileWindowMarkerTitle(currentTitle, markerCode, target.profileName)
	if currentTitle != desiredTitle {
		setProfileWindowMarkerTitle(window.hwnd, desiredTitle)
	}
	if icon != 0 {
		state := profileWindowMarkerWindowState{processID: window.processID}
		if marker != nil {
			state, _ = marker.windowState(window.hwnd)
		}
		if !state.originalBigIconCaptured {
			if originalIcon, ok := getProfileWindowMarkerIcon(window.hwnd, iconBig); ok {
				state.originalBigIcon = originalIcon
				state.originalBigIconCaptured = true
			}
		}
		if !state.originalSmallIconCaptured {
			if originalIcon, ok := getProfileWindowMarkerIcon(window.hwnd, iconSmall); ok {
				state.originalSmallIcon = originalIcon
				state.originalSmallIconCaptured = true
			}
		}
		iconChanged := false
		if isProfileWindowMarkerIcon(window.hwnd, iconBig, icon) {
			state.markerBigIconApplied = true
		} else if _, ok := sendProfileWindowMarkerMessageResult(window.hwnd, wmSetIcon, iconBig, icon); ok {
			state.markerBigIconApplied = true
			iconChanged = true
		}
		if isProfileWindowMarkerIcon(window.hwnd, iconSmall, icon) {
			state.markerSmallIconApplied = true
		} else if _, ok := sendProfileWindowMarkerMessageResult(window.hwnd, wmSetIcon, iconSmall, icon); ok {
			state.markerSmallIconApplied = true
			iconChanged = true
		}
		if iconChanged {
			refreshProfileWindowMarkerIcon(window.hwnd)
		}
		if marker != nil {
			marker.setWindowState(window.hwnd, state)
		}
	}
}

func clearProfileWindowMarker(hwnd uintptr, state profileWindowMarkerWindowState, markerIcon uintptr) {
	if hwnd == 0 {
		return
	}
	iconChanged := false
	if state.markerBigIconApplied && isProfileWindowMarkerIcon(hwnd, iconBig, markerIcon) {
		originalIcon := uintptr(0)
		if state.originalBigIconCaptured {
			originalIcon = state.originalBigIcon
		}
		if _, ok := sendProfileWindowMarkerMessageResult(hwnd, wmSetIcon, iconBig, originalIcon); ok {
			iconChanged = true
		}
	}
	if state.markerSmallIconApplied && isProfileWindowMarkerIcon(hwnd, iconSmall, markerIcon) {
		originalIcon := uintptr(0)
		if state.originalSmallIconCaptured {
			originalIcon = state.originalSmallIcon
		}
		if _, ok := sendProfileWindowMarkerMessageResult(hwnd, wmSetIcon, iconSmall, originalIcon); ok {
			iconChanged = true
		}
	}
	if iconChanged {
		refreshProfileWindowMarkerIcon(hwnd)
	}
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

func getProfileWindowMarkerIcon(hwnd uintptr, iconType uintptr) (uintptr, bool) {
	return sendProfileWindowMarkerMessageResult(hwnd, wmGetIcon, iconType, 0)
}

func isProfileWindowMarkerIcon(hwnd uintptr, iconType uintptr, expectedIcon uintptr) bool {
	if expectedIcon == 0 {
		return false
	}
	currentIcon, ok := getProfileWindowMarkerIcon(hwnd, iconType)
	return ok && currentIcon == expectedIcon
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

func refreshProfileWindowMarkerIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procRedrawWindowMarker.Call(hwnd, 0, 0, rdwInvalidate|rdwFrame)
}

func createProfileWindowMarkerIcon(markerCode string) (uintptr, error) {
	var bitmapInfo profileWindowMarkerBitmapInfo
	bitmapInfo.Header = profileWindowMarkerBitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(profileWindowMarkerBitmapInfoHeader{})),
		Width:       iconWidth,
		Height:      -iconHeight,
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

	maskBitmap, _, maskErr := procCreateBitmapMarker.Call(iconWidth, iconHeight, 1, 1, 0)
	if maskBitmap == 0 {
		procDeleteObjectMarker.Call(colorBitmap)
		return 0, maskErr
	}

	pixels := unsafe.Slice((*uint32)(bits), iconWidth*iconHeight)
	for index := range pixels {
		pixels[index] = 0
	}

	fillProfileWindowMarkerRoundedRect(pixels, 2, 4, 28, 30, 7, profileWindowMarkerFrameColor)
	fillProfileWindowMarkerRoundedRect(pixels, 4, 6, 26, 28, 5, profileWindowMarkerSurfaceColor)
	fillProfileWindowMarkerRoundedRect(pixels, 4, 6, 26, 12, 4, profileWindowMarkerHeaderColor)
	fillProfileWindowMarkerRect(pixels, 4, 9, 26, 12, profileWindowMarkerHeaderColor)
	drawProfileWindowMarkerWindowGlyph(pixels, markerCode)
	drawProfileWindowMarkerBadge(pixels, markerCode)

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

func profileWindowMarkerColor(markerCode string) uint32 {
	palette := []uint32{
		0xFFDC2626,
		0xFF2563EB,
		0xFF059669,
		0xFFD97706,
		0xFF7C3AED,
		0xFFDB2777,
		0xFF0891B2,
		0xFF4F46E5,
		0xFF65A30D,
		0xFFEA580C,
		0xFF0F766E,
		0xFF9333EA,
	}
	markerCode = strings.ToUpper(strings.TrimSpace(markerCode))
	if markerCode == "" {
		return palette[0]
	}
	index := int(markerCode[0])
	if markerCode[0] >= 'A' && markerCode[0] <= 'Z' {
		index = int(markerCode[0] - 'A')
	} else if markerCode[0] >= '0' && markerCode[0] <= '9' {
		index = int(markerCode[0]-'0') + len(profileWindowMarkerLetters)
	}
	index %= len(palette)
	return palette[index]
}

func fillProfileWindowMarkerRoundedRect(pixels []uint32, left, top, right, bottom, radius int, color uint32) {
	for pixelY := top; pixelY <= bottom; pixelY++ {
		for pixelX := left; pixelX <= right; pixelX++ {
			if profileWindowMarkerInsideRoundedRect(pixelX, pixelY, left, top, right, bottom, radius) {
				setProfileWindowMarkerPixel(pixels, pixelX, pixelY, color)
			}
		}
	}
}

func fillProfileWindowMarkerRect(pixels []uint32, left, top, right, bottom int, color uint32) {
	for pixelY := top; pixelY <= bottom; pixelY++ {
		for pixelX := left; pixelX <= right; pixelX++ {
			setProfileWindowMarkerPixel(pixels, pixelX, pixelY, color)
		}
	}
}

func fillProfileWindowMarkerCircle(pixels []uint32, centerX, centerY, radius int, color uint32) {
	radiusSquared := radius * radius
	for pixelY := centerY - radius; pixelY <= centerY+radius; pixelY++ {
		for pixelX := centerX - radius; pixelX <= centerX+radius; pixelX++ {
			deltaX := pixelX - centerX
			deltaY := pixelY - centerY
			if deltaX*deltaX+deltaY*deltaY <= radiusSquared {
				setProfileWindowMarkerPixel(pixels, pixelX, pixelY, color)
			}
		}
	}
}

func profileWindowMarkerInsideRoundedRect(x, y, left, top, right, bottom, radius int) bool {
	if x < left || x > right || y < top || y > bottom {
		return false
	}
	if radius <= 0 {
		return true
	}
	if x < left+radius && y < top+radius {
		dx := x - (left + radius)
		dy := y - (top + radius)
		return dx*dx+dy*dy <= radius*radius
	}
	if x > right-radius && y < top+radius {
		dx := x - (right - radius)
		dy := y - (top + radius)
		return dx*dx+dy*dy <= radius*radius
	}
	if x < left+radius && y > bottom-radius {
		dx := x - (left + radius)
		dy := y - (bottom - radius)
		return dx*dx+dy*dy <= radius*radius
	}
	if x > right-radius && y > bottom-radius {
		dx := x - (right - radius)
		dy := y - (bottom - radius)
		return dx*dx+dy*dy <= radius*radius
	}
	return true
}

func setProfileWindowMarkerPixel(pixels []uint32, x, y int, color uint32) {
	if x < 0 || x >= iconWidth || y < 0 || y >= iconHeight {
		return
	}
	pixels[y*iconWidth+x] = color
}

func drawProfileWindowMarkerWindowGlyph(pixels []uint32, markerCode string) {
	for pixelX := 8; pixelX <= 22; pixelX++ {
		setProfileWindowMarkerPixel(pixels, pixelX, 16, profileWindowMarkerFrameColor)
		setProfileWindowMarkerPixel(pixels, pixelX, 24, profileWindowMarkerFrameColor)
		setProfileWindowMarkerPixel(pixels, pixelX, 20, profileWindowMarkerLineColor)
	}
	for pixelY := 16; pixelY <= 24; pixelY++ {
		setProfileWindowMarkerPixel(pixels, 8, pixelY, profileWindowMarkerFrameColor)
		setProfileWindowMarkerPixel(pixels, 22, pixelY, profileWindowMarkerFrameColor)
	}
	fillProfileWindowMarkerRoundedRect(pixels, 11, 21, 18, 23, 1, profileWindowMarkerColor(markerCode))
	setProfileWindowMarkerPixel(pixels, 8, 9, 0xFFD1FAE5)
	setProfileWindowMarkerPixel(pixels, 11, 9, 0xFFD1FAE5)
	setProfileWindowMarkerPixel(pixels, 14, 9, 0xFFD1FAE5)
}

func drawProfileWindowMarkerBadge(pixels []uint32, markerCode string) {
	fillProfileWindowMarkerCircle(pixels, 24, 6, 6, profileWindowMarkerFrameColor)
	fillProfileWindowMarkerCircle(pixels, 24, 6, 5, profileWindowMarkerSurfaceColor)
	fillProfileWindowMarkerCircle(pixels, 24, 6, 4, profileWindowMarkerColor(markerCode))
	drawProfileWindowMarkerGlyph(pixels, markerCode, 22, 2, 1, 0xFFFFFFFF)
}

func drawProfileWindowMarkerGlyph(pixels []uint32, markerCode string, startX, startY, scale int, color uint32) {
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
						if pixelX >= 0 && pixelX < iconWidth && pixelY >= 0 && pixelY < iconHeight {
							pixels[pixelY*iconWidth+pixelX] = color
						}
					}
				}
			}
		}
	}
}
