package backend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProfileWindowMarkerTaskbarOverlayReappliesAfterInterval(t *testing.T) {
	state := &profileWindowMarkerWindowState{
		taskbarOverlayIconApplied:   true,
		taskbarOverlayLastAppliedAt: time.Now(),
	}
	if state.taskbarOverlayNeedsReapply() {
		t.Fatal("recently applied overlay must not need a reapply")
	}

	state.taskbarOverlayLastAppliedAt = time.Now().Add(-profileWindowMarkerOverlayReapplyInterval - time.Second)
	if !state.taskbarOverlayNeedsReapply() {
		t.Fatal("overlay must be reapplied after the refresh interval")
	}

	state.taskbarOverlayIconApplied = false
	if !state.taskbarOverlayNeedsReapply() {
		t.Fatal("overlay must be applied when it was never applied")
	}
}

func TestProfileWindowMarkerIconResourcePathPrefersInstalledIcon(t *testing.T) {
	appRoot := t.TempDir()
	installedIcon := filepath.Join(appRoot, "AntBrowser.ico")
	if err := os.WriteFile(installedIcon, []byte("ico"), 0o600); err != nil {
		t.Fatalf("write installed icon: %v", err)
	}

	if got := profileWindowMarkerIconResourcePath(appRoot); got != installedIcon {
		t.Fatalf("icon resource path = %q, want %q", got, installedIcon)
	}
}

func TestNextProfileWindowMarkerCodeUsesNumbersThenLetters(t *testing.T) {
	if code := nextProfileWindowMarkerCode(nil); code != "1" {
		t.Fatalf("first marker code = %q, want 1", code)
	}

	used := make(map[string]struct{}, 12)
	for index := 1; index <= 10; index++ {
		used[strconv.Itoa(index)] = struct{}{}
	}
	if code := nextProfileWindowMarkerCode(used); code != "A" {
		t.Fatalf("marker code after one through ten = %q, want A", code)
	}
	used["A"] = struct{}{}
	if code := nextProfileWindowMarkerCode(used); code != "B" {
		t.Fatalf("marker code after A = %q, want B", code)
	}
}

func TestNextProfileWindowMarkerCodeContinuesThroughTen(t *testing.T) {
	used := make(map[string]struct{}, 9)
	for index := 1; index <= 9; index++ {
		used[strconv.Itoa(index)] = struct{}{}
	}
	if code := nextProfileWindowMarkerCode(used); code != "10" {
		t.Fatalf("marker code after one through nine = %q, want 10", code)
	}
}

func TestNextProfileWindowMarkerCodeContinuesWithNumbersAfterLetters(t *testing.T) {
	used := make(map[string]struct{}, 36)
	for index := 1; index <= 10; index++ {
		used[strconv.Itoa(index)] = struct{}{}
	}
	for _, code := range profileWindowMarkerLetters {
		used[string(code)] = struct{}{}
	}
	if code := nextProfileWindowMarkerCode(used); code != "11" {
		t.Fatalf("marker code after one through ten and A-Z = %q, want 11", code)
	}
}

func TestNextProfileWindowMarkerCodeReusesAvailableNumber(t *testing.T) {
	used := map[string]struct{}{"1": {}, "3": {}}
	if code := nextProfileWindowMarkerCode(used); code != "2" {
		t.Fatalf("available marker code = %q, want 2", code)
	}
}

func TestSanitizeProfileWindowMarkerName(t *testing.T) {
	name := sanitizeProfileWindowMarkerName("  test\tprofile\n" + string(rune(0x00b7)) + "name  ")
	if strings.ContainsAny(name, "\r\n\t") {
		t.Fatalf("sanitized name contains control characters: %q", name)
	}
	if strings.ContainsRune(name, rune(0x00b7)) {
		t.Fatalf("sanitized name contains title separator: %q", name)
	}
	if name != "test profile -name" {
		t.Fatalf("sanitized name = %q, want %q", name, "test profile -name")
	}
}

func TestApplyProfileWindowMarkerTitleIsIdempotent(t *testing.T) {
	markerCode := "A"
	profileName := "Work"
	marked := applyProfileWindowMarkerTitle("Example page", markerCode, profileName)
	if marked != profileWindowMarkerPrefix(markerCode, profileName)+"Example page" {
		t.Fatalf("marked title = %q", marked)
	}
	if again := applyProfileWindowMarkerTitle(marked, markerCode, profileName); again != marked {
		t.Fatalf("marker was duplicated: %q", again)
	}
	updated := applyProfileWindowMarkerTitle(marked, markerCode, profileName+" 2")
	if !strings.HasSuffix(updated, "Example page") {
		t.Fatalf("page title was not preserved: %q", updated)
	}
	if strings.Count(updated, "[") != 1 {
		t.Fatalf("marker prefix was duplicated after profile rename: %q", updated)
	}
}

func TestApplyProfileWindowMarkerTitleReplacesPageTitle(t *testing.T) {
	markerCode := "B"
	profileName := "Navigation"
	first := applyProfileWindowMarkerTitle("First page", markerCode, profileName)
	second := applyProfileWindowMarkerTitle("Second page", markerCode, profileName)
	if !strings.HasSuffix(first, "First page") {
		t.Fatalf("first page title missing: %q", first)
	}
	if strings.HasSuffix(second, "First page") {
		t.Fatalf("old page title remained: %q", second)
	}
	if !strings.HasSuffix(second, "Second page") {
		t.Fatalf("new page title missing: %q", second)
	}
	if strings.Count(second, "[") != 1 {
		t.Fatalf("marker prefix was duplicated: %q", second)
	}
}

func TestApplyProfileWindowMarkerTitleReplacesRuntimeCode(t *testing.T) {
	marked := applyProfileWindowMarkerTitle("Example page", "A", "Work")
	updated := applyProfileWindowMarkerTitle(marked, "1", "Work")
	if !strings.HasPrefix(updated, "[1] ") {
		t.Fatalf("runtime marker code was not replaced: %q", updated)
	}
	if !strings.HasSuffix(updated, "Example page") {
		t.Fatalf("page title was not preserved after code replacement: %q", updated)
	}
}

func TestProfileWindowMarkerWindowStateResetsWhenProcessChanges(t *testing.T) {
	marker := &profileWindowMarker{
		windowRefs: make(map[uintptr]profileWindowMarkerWindowState),
	}

	marker.rememberWindow(42, 100)
	marker.setWindowState(42, profileWindowMarkerWindowState{
		processID:               100,
		originalBigIcon:         1,
		originalBigIconCaptured: true,
		fallbackBigIconApplied:  true,
		decoratedBigIcon:        2,
		decoratedBigIconApplied: true,
	})
	marker.rememberWindow(42, 100)
	state, ok := marker.windowState(42)
	if !ok || !state.originalBigIconCaptured || !state.fallbackBigIconApplied || state.decoratedBigIcon != 2 || !state.decoratedBigIconApplied {
		t.Fatalf("window state was not preserved for the same process: %+v", state)
	}

	marker.rememberWindow(42, 200)
	state, ok = marker.windowState(42)
	if !ok {
		t.Fatal("window state was lost after process change")
	}
	if state.processID != 200 || state.originalBigIconCaptured || state.fallbackBigIconApplied || state.decoratedBigIcon != 0 || state.decoratedBigIconApplied {
		t.Fatalf("window state was not reset after process change: %+v", state)
	}
}

func TestProfileWindowMarkerAppUserModelIDUsesStableProfileID(t *testing.T) {
	appID := profileWindowMarkerAppUserModelID("profile one/中文:A", "B")
	if appID != "AntBrowser.Instance.profile.one.A" {
		t.Fatalf("unexpected AppUserModelID: %q", appID)
	}
}

func TestProfileWindowMarkerAppUserModelIDFallsBackToMarkerCode(t *testing.T) {
	appID := profileWindowMarkerAppUserModelID("", "A")
	if appID != "AntBrowser.Instance.A" {
		t.Fatalf("unexpected fallback AppUserModelID: %q", appID)
	}
}

func TestProfileWindowMarkerAppUserModelIDStaysWithinWindowsLimit(t *testing.T) {
	appID := profileWindowMarkerAppUserModelID(strings.Repeat("a", 200), "A")
	if len(appID) > 128 {
		t.Fatalf("AppUserModelID exceeds Windows limit: len=%d", len(appID))
	}
	if strings.HasSuffix(appID, ".") || strings.HasSuffix(appID, "-") || strings.HasSuffix(appID, "_") {
		t.Fatalf("AppUserModelID has a trailing separator: %q", appID)
	}
}
