package backend

import (
	"strings"
	"testing"
)

func TestNextProfileWindowMarkerCodeUsesLettersThenNumbers(t *testing.T) {
	if code := nextProfileWindowMarkerCode(nil); code != "A" {
		t.Fatalf("first marker code = %q, want A", code)
	}

	used := make(map[string]struct{}, len(profileWindowMarkerLetters))
	for _, code := range profileWindowMarkerLetters {
		used[string(code)] = struct{}{}
	}
	if code := nextProfileWindowMarkerCode(used); code != "1" {
		t.Fatalf("marker code after letters = %q, want 1", code)
	}
	used["1"] = struct{}{}
	if code := nextProfileWindowMarkerCode(used); code != "2" {
		t.Fatalf("marker code after first number = %q, want 2", code)
	}
}

func TestNextProfileWindowMarkerCodeReusesAvailableLetter(t *testing.T) {
	used := map[string]struct{}{"A": {}, "C": {}}
	if code := nextProfileWindowMarkerCode(used); code != "B" {
		t.Fatalf("available marker code = %q, want B", code)
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
		markerBigIconApplied:    true,
	})
	marker.rememberWindow(42, 100)
	state, ok := marker.windowState(42)
	if !ok || !state.originalBigIconCaptured || !state.markerBigIconApplied {
		t.Fatalf("window state was not preserved for the same process: %+v", state)
	}

	marker.rememberWindow(42, 200)
	state, ok = marker.windowState(42)
	if !ok {
		t.Fatal("window state was lost after process change")
	}
	if state.processID != 200 || state.originalBigIconCaptured || state.markerBigIconApplied {
		t.Fatalf("window state was not reset after process change: %+v", state)
	}
}
