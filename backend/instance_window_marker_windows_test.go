//go:build windows
// +build windows

package backend

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestProfileWindowMarkerTaskbarList3GUID(t *testing.T) {
	expected := windows.GUID{
		Data1: 0xEA1AFB91,
		Data2: 0x9E28,
		Data3: 0x4B86,
		Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF},
	}
	if iidTaskbarList3WindowMarker != expected {
		t.Fatalf("ITaskbarList3 IID = %#v, want %#v", iidTaskbarList3WindowMarker, expected)
	}
}

func TestProfileWindowMarkerTaskbarOverlayRejectsInvalidWindow(t *testing.T) {
	state := profileWindowMarkerWindowState{processID: 1}
	if applyProfileWindowMarkerTaskbarOverlay(0, "A", &state) {
		t.Fatal("zero window handle must be rejected")
	}
	if state.taskbarOverlayIcon != 0 || state.taskbarOverlayIconApplied {
		t.Fatalf("invalid-window overlay state changed: %#v", state)
	}
}

func TestProfileWindowMarkerRestoreIconPrefersCapturedOriginal(t *testing.T) {
	state := &profileWindowMarkerWindowState{
		originalBigIcon:           101,
		originalBigIconCaptured:   true,
		originalSmallIcon:         202,
		originalSmallIconCaptured: true,
	}
	if restored := profileWindowMarkerRestoreIcon(state, iconBig); restored != 101 {
		t.Fatalf("big icon restore = %d, want 101", restored)
	}
	if restored := profileWindowMarkerRestoreIcon(state, iconSmall); restored != 202 {
		t.Fatalf("small icon restore = %d, want 202", restored)
	}

	state.originalBigIconCaptured = false
	state.originalSmallIconCaptured = false
	if restored := profileWindowMarkerRestoreIcon(state, iconBig); restored != 0 {
		t.Fatalf("un-captured big icon restore = %d, want 0", restored)
	}
	if restored := profileWindowMarkerRestoreIcon(state, iconSmall); restored != 0 {
		t.Fatalf("un-captured small icon restore = %d, want 0", restored)
	}
	if restored := profileWindowMarkerRestoreIcon(nil, iconBig); restored != 0 {
		t.Fatalf("nil state restore = %d, want 0", restored)
	}
}

func TestCaptureProfileWindowMarkerOriginalIconsIgnoresInvalidWindow(t *testing.T) {
	state := profileWindowMarkerWindowState{}
	captureProfileWindowMarkerOriginalIcons(0, &state, 0)
	if state.originalBigIconCaptured || state.originalSmallIconCaptured {
		t.Fatalf("invalid window must not capture icons: %#v", state)
	}
}

func TestMergeProfileWindowMarkerWindowsIncludesUserDataWindow(t *testing.T) {
	merged := mergeProfileWindowMarkerWindows(
		[]profileWindowMarkerWindow{{hwnd: 101, processID: 11, visible: false}},
		[]profileWindowMarkerWindow{{hwnd: 202, processID: 22, visible: true, titled: true}},
	)
	if len(merged) != 2 {
		t.Fatalf("merged windows = %d, want 2", len(merged))
	}
	if merged[0].hwnd != 202 {
		t.Fatalf("primary window = %d, want user-data window 202", merged[0].hwnd)
	}
}

func TestClassifyProfileMarkerProcessCandidatesPrefersMatchingPort(t *testing.T) {
	processes := []browserUserDataProcess{
		{PID: 101, DebugPort: 9222},
		{PID: 102, DebugPort: 0},
		{PID: 103, DebugPort: 9333},
		{PID: 101, DebugPort: 9222},
		{PID: 0, DebugPort: 0},
	}
	matched, fallback := classifyProfileMarkerProcessCandidates(processes, 9222)
	if len(matched) != 1 || matched[0] != 101 {
		t.Fatalf("matched pids = %v, want [101]", matched)
	}
	if len(fallback) != 1 || fallback[0] != 102 {
		t.Fatalf("fallback pids = %v, want [102]", fallback)
	}
}

func TestClassifyProfileMarkerProcessCandidatesUsesAllWhenPortUnknown(t *testing.T) {
	processes := []browserUserDataProcess{
		{PID: 201, DebugPort: 0},
		{PID: 202, DebugPort: 9222},
	}
	matched, fallback := classifyProfileMarkerProcessCandidates(processes, 0)
	if len(matched) != 2 {
		t.Fatalf("matched pids = %v, want both processes", matched)
	}
	if len(fallback) != 0 {
		t.Fatalf("fallback pids = %v, want none", fallback)
	}
}

func TestClassifyProfileMarkerProcessCandidatesFallbackOnlyWhenNoMatch(t *testing.T) {
	processes := []browserUserDataProcess{
		{PID: 301, DebugPort: 0},
		{PID: 302, DebugPort: 9333},
	}
	matched, fallback := classifyProfileMarkerProcessCandidates(processes, 9222)
	if len(matched) != 0 {
		t.Fatalf("matched pids = %v, want none", matched)
	}
	if len(fallback) != 1 || fallback[0] != 301 {
		t.Fatalf("fallback pids = %v, want [301]", fallback)
	}
}

func TestProfileWindowMarkerOverlayIconDrawsCrispBadge(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerOverlayIconWidth*profileWindowMarkerOverlayIconHeight)
	drawProfileWindowMarkerOverlayBadge(pixels, `A`)

	badgeColor := profileWindowMarkerColor(`A`)
	badgePixels := 0
	for pixelY := 0; pixelY < profileWindowMarkerOverlayIconHeight; pixelY++ {
		for pixelX := 0; pixelX < profileWindowMarkerOverlayIconWidth; pixelX++ {
			if pixels[pixelY*profileWindowMarkerOverlayIconWidth+pixelX] == badgeColor {
				badgePixels++
			}
		}
	}
	if badgePixels < 20 {
		t.Fatalf(`instance marker overlay badge is too small or missing: pixels=%d`, badgePixels)
	}

	if pixels[0] != 0 || pixels[profileWindowMarkerOverlayIconWidth-1] != 0 {
		t.Fatal(`overlay icon corners must remain transparent`)
	}

	geometry := profileWindowMarkerBadgeGeometryFor(profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight)
	glyphPixels := 0
	innerRadiusSquared := geometry.innerRadius * geometry.innerRadius
	for pixelY := 0; pixelY < profileWindowMarkerOverlayIconHeight; pixelY++ {
		for pixelX := 0; pixelX < profileWindowMarkerOverlayIconWidth; pixelX++ {
			deltaX := pixelX - geometry.centerX
			deltaY := pixelY - geometry.centerY
			if deltaX*deltaX+deltaY*deltaY > innerRadiusSquared {
				continue
			}
			if isProfileWindowMarkerGlyphInkPixel(pixels[pixelY*profileWindowMarkerOverlayIconWidth+pixelX]) {
				glyphPixels++
			}
		}
	}
	if glyphPixels == 0 {
		t.Fatal(`instance marker overlay glyph was not drawn`)
	}
}

func TestProfileWindowMarkerSmallBadgeUsesReadableSingleGlyph(t *testing.T) {
	for _, size := range []int{profileWindowMarkerOverlayIconWidth, profileWindowMarkerSmallIconWidth} {
		pixels := make([]uint32, size*size)
		drawProfileWindowMarkerDecoratedBadge(pixels, size, size, `10`)

		geometry := profileWindowMarkerBadgeGeometryFor(size, size)
		innerRadiusSquared := geometry.innerRadius * geometry.innerRadius
		allowanceSquared := (geometry.shadowRadius + 1) * (geometry.shadowRadius + 1)
		glyphPixels := 0
		for pixelY := 0; pixelY < size; pixelY++ {
			for pixelX := 0; pixelX < size; pixelX++ {
				if !isProfileWindowMarkerGlyphInkPixel(pixels[pixelY*size+pixelX]) {
					continue
				}
				deltaX := pixelX - geometry.centerX
				deltaY := pixelY - geometry.centerY
				distanceSquared := deltaX*deltaX + deltaY*deltaY
				if distanceSquared > allowanceSquared {
					t.Fatalf(`small badge ink escaped its circle at size %d: (%d, %d)`, size, pixelX, pixelY)
				}
				if distanceSquared <= innerRadiusSquared {
					glyphPixels++
				}
			}
		}
		if glyphPixels == 0 {
			t.Fatalf(`small badge has no readable glyph at size %d`, size)
		}
	}
}

func TestProfileWindowMarkerDecoratedBadgeStaysInTopRight(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerFallbackIconWidth*profileWindowMarkerFallbackIconHeight)
	drawProfileWindowMarkerDecoratedBadge(pixels, profileWindowMarkerFallbackIconWidth, profileWindowMarkerFallbackIconHeight, `A`)

	badgeColor := profileWindowMarkerColor(`A`)
	badgePixels := 0
	for pixelY := 0; pixelY < 15; pixelY++ {
		for pixelX := 18; pixelX < profileWindowMarkerFallbackIconWidth; pixelX++ {
			if pixels[pixelY*profileWindowMarkerFallbackIconWidth+pixelX] == badgeColor {
				badgePixels++
			}
		}
	}
	if badgePixels == 0 {
		t.Fatal(`decorated instance icon lost its top-right badge`)
	}

	for pixelY := 26; pixelY < profileWindowMarkerFallbackIconHeight; pixelY++ {
		for pixelX := 18; pixelX < profileWindowMarkerFallbackIconWidth; pixelX++ {
			if pixels[pixelY*profileWindowMarkerFallbackIconWidth+pixelX] == badgeColor {
				t.Fatalf(`badge color leaked below the top-right corner at (%d, %d)`, pixelX, pixelY)
			}
		}
	}
}

func TestProfileWindowMarkerFallbackIconUsesEmbeddedLogo(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerFallbackIconWidth*profileWindowMarkerFallbackIconHeight)
	drawProfileWindowMarkerFallbackIcon(pixels, `B`)

	if pixels[0] != 0 {
		t.Fatal(`fallback instance icon should keep a transparent background`)
	}

	whitePixels := 0
	bluePixels := 0
	for _, pixel := range pixels {
		if isProfileWindowMarkerWhiteLogoPixel(pixel) {
			whitePixels++
		}
		if isProfileWindowMarkerBlueLineLogoPixel(pixel) {
			bluePixels++
		}
	}
	if whitePixels == 0 {
		t.Fatal(`fallback instance icon lost the embedded logo white base`)
	}
	if bluePixels == 0 {
		t.Fatal(`fallback instance icon lost the embedded logo blue lines`)
	}
	badgePixels := 0
	for pixelY := 0; pixelY <= 14; pixelY++ {
		for pixelX := 18; pixelX < profileWindowMarkerFallbackIconWidth; pixelX++ {
			if pixels[pixelY*profileWindowMarkerFallbackIconWidth+pixelX] == profileWindowMarkerColor(`B`) {
				badgePixels++
			}
		}
	}
	if badgePixels == 0 {
		t.Fatal(`fallback instance icon lost its clear corner badge`)
	}
}

func TestProfileWindowMarkerEmbeddedLogoKeepsWhiteBaseAndBlueLines(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerFallbackIconWidth*profileWindowMarkerFallbackIconHeight)
	if !drawProfileWindowMarkerEmbeddedLogoIcon(pixels, profileWindowMarkerFallbackIconWidth, profileWindowMarkerFallbackIconHeight) {
		t.Fatal(`embedded logo could not be drawn`)
	}
	if pixels[0] != 0 {
		t.Fatal(`embedded logo should keep a transparent canvas outside the fingerprint`)
	}

	whitePixels := 0
	bluePixels := 0
	for _, pixel := range pixels {
		if isProfileWindowMarkerWhiteLogoPixel(pixel) {
			whitePixels++
		}
		if isProfileWindowMarkerBlueLineLogoPixel(pixel) {
			bluePixels++
		}
	}
	if whitePixels == 0 {
		t.Fatal(`embedded logo lost its white base before badge decoration`)
	}
	if bluePixels == 0 {
		t.Fatal(`embedded logo lost its blue ant lines before badge decoration`)
	}
}

func TestProfileWindowMarkerBadgeRendersClearPixelGlyph(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerFallbackIconWidth*profileWindowMarkerFallbackIconHeight)
	drawProfileWindowMarkerDecoratedBadge(pixels, profileWindowMarkerFallbackIconWidth, profileWindowMarkerFallbackIconHeight, `7`)

	geometry := profileWindowMarkerBadgeGeometryFor(profileWindowMarkerFallbackIconWidth, profileWindowMarkerFallbackIconHeight)
	glyphPixels := 0
	innerRadiusSquared := geometry.innerRadius * geometry.innerRadius
	for pixelY := 0; pixelY < profileWindowMarkerFallbackIconHeight; pixelY++ {
		for pixelX := 0; pixelX < profileWindowMarkerFallbackIconWidth; pixelX++ {
			deltaX := pixelX - geometry.centerX
			deltaY := pixelY - geometry.centerY
			if deltaX*deltaX+deltaY*deltaY > innerRadiusSquared {
				continue
			}
			if isProfileWindowMarkerGlyphInkPixel(pixels[pixelY*profileWindowMarkerFallbackIconWidth+pixelX]) {
				glyphPixels++
			}
		}
	}
	if glyphPixels < 20 {
		t.Fatalf(`badge pixel glyph is too small or missing: glyphPixels=%d`, glyphPixels)
	}
}

func TestProfileWindowMarkerSmallInstanceIconKeepsLogoAndBadge(t *testing.T) {
	pixels := make([]uint32, profileWindowMarkerOverlayIconWidth*profileWindowMarkerOverlayIconHeight)
	drawProfileWindowMarkerInstanceIcon(pixels, profileWindowMarkerOverlayIconWidth, profileWindowMarkerOverlayIconHeight, `A`)

	badgePixels := 0
	bluePixels := 0
	for pixelY := 0; pixelY < profileWindowMarkerOverlayIconHeight; pixelY++ {
		for pixelX := 0; pixelX < profileWindowMarkerOverlayIconWidth; pixelX++ {
			pixel := pixels[pixelY*profileWindowMarkerOverlayIconWidth+pixelX]
			if pixelY <= 10 && pixelX >= 7 && pixel == profileWindowMarkerColor(`A`) {
				badgePixels++
			}
			if isProfileWindowMarkerBlueLineLogoPixel(pixel) {
				bluePixels++
			}
		}
	}
	if bluePixels == 0 {
		t.Fatal(`small instance icon lost its embedded logo fingerprint`)
	}
	if badgePixels == 0 {
		t.Fatal(`small instance icon lost its top-right badge`)
	}
}

func isProfileWindowMarkerWhiteLogoPixel(pixel uint32) bool {
	_, red, green, blue := splitProfileWindowMarkerARGB(pixel)
	return red >= 230 && green >= 230 && blue >= 230
}

func isProfileWindowMarkerGlyphInkPixel(pixel uint32) bool {
	alpha, red, green, blue := splitProfileWindowMarkerARGB(pixel)
	return alpha >= 200 && red <= 96 && green <= 96 && blue <= 96
}

func isProfileWindowMarkerBlueLineLogoPixel(pixel uint32) bool {
	_, red, green, blue := splitProfileWindowMarkerARGB(pixel)
	return blue >= 200 && green <= 90 && red <= 35
}

func splitProfileWindowMarkerARGB(pixel uint32) (uint8, uint8, uint8, uint8) {
	return uint8(pixel >> 24), uint8(pixel >> 16), uint8(pixel >> 8), uint8(pixel)
}
