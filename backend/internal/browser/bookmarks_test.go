package browser

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceBookmarkURLUpdatesExistingBookmark(t *testing.T) {
	userDataDir := t.TempDir()
	oldURL := "ant://fingerprint-check"
	newURL := "file:///tmp/fingerprint-check/index.html?profileId=profile-123"

	if err := EnsureDefaultBookmarks(userDataDir, []config.BrowserBookmark{{Name: "指纹检测", URL: oldURL}}); err != nil {
		t.Fatalf("EnsureDefaultBookmarks error = %v", err)
	}
	changed, err := ReplaceBookmarkURL(userDataDir, oldURL, newURL)
	if err != nil {
		t.Fatalf("ReplaceBookmarkURL error = %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}

	data, err := os.ReadFile(filepath.Join(userDataDir, "Default", "Bookmarks"))
	if err != nil {
		t.Fatalf("read bookmarks error = %v", err)
	}
	content := string(data)
	if strings.Contains(content, oldURL) {
		t.Fatalf("bookmarks still contains old URL: %s", content)
	}
	if !strings.Contains(content, newURL) {
		t.Fatalf("bookmarks missing new URL: %s", content)
	}
}
