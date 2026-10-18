package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestSQLiteBookmarkDAOTracksDeletionUntilExplicitReadd(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "bookmarks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	dao := NewSQLiteBookmarkDAO(db.GetConn())
	deletedURL := "https://ping0.cc/"
	if err := dao.ReplaceAll([]config.BrowserBookmark{
		{Name: "Ping0", URL: deletedURL},
		{Name: "Google", URL: "https://www.google.com/"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := dao.ReplaceAll([]config.BrowserBookmark{{Name: "Google", URL: "https://www.google.com/"}}); err != nil {
		t.Fatal(err)
	}

	deletedURLs, err := dao.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedURLs) != 1 || deletedURLs[0] != deletedURL {
		t.Fatalf("deletion markers = %#v, want %q", deletedURLs, deletedURL)
	}

	if err := dao.ReplaceAll([]config.BrowserBookmark{
		{Name: "Google", URL: "https://www.google.com/"},
		{Name: "Ping0", URL: deletedURL},
	}); err != nil {
		t.Fatal(err)
	}
	deletedURLs, err = dao.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedURLs) != 0 {
		t.Fatalf("deletion markers after explicit re-add = %#v, want none", deletedURLs)
	}
}

func TestNormalizeBookmarkURLPreservesPathQueryAndFragmentCase(t *testing.T) {
	got := NormalizeBookmarkURL(" HTTPS://EXAMPLE.com/Path?Query=Mixed#Fragment ")
	want := "https://example.com/Path?Query=Mixed#Fragment"
	if got != want {
		t.Fatalf("NormalizeBookmarkURL() = %q, want %q", got, want)
	}
	if NormalizeBookmarkURL("https://example.com/Path?Query=Mixed") == NormalizeBookmarkURL("https://example.com/path?Query=Mixed") {
		t.Fatal("URLs with different path case were treated as identical")
	}
	if NormalizeBookmarkURL("https://example.com/Path?Query=Mixed") == NormalizeBookmarkURL("https://example.com/Path?query=Mixed") {
		t.Fatal("URLs with different query case were treated as identical")
	}
}

func TestSQLiteBookmarkDAOUsesSchemeAndHostCaseOnlyForMatching(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "bookmarks-url-case.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	dao := NewSQLiteBookmarkDAO(db.GetConn())
	if err := dao.ReplaceAll([]config.BrowserBookmark{
		{Name: "Host case", URL: "https://EXAMPLE.com/Path?Query=One"},
		{Name: "Path case", URL: "https://example.com/path?Query=One"},
		{Name: "Query case", URL: "https://example.com/Path?query=One"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := dao.ReplaceAll([]config.BrowserBookmark{{Name: "Host case", URL: "https://example.com/Path?Query=One"}}); err != nil {
		t.Fatal(err)
	}

	deletedURLs, err := dao.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	wantDeleted := map[string]struct{}{
		"https://example.com/Path?query=One": {},
		"https://example.com/path?Query=One": {},
	}
	if len(deletedURLs) != len(wantDeleted) {
		t.Fatalf("deletion markers = %#v, want %#v", deletedURLs, wantDeleted)
	}
	for _, deletedURL := range deletedURLs {
		if _, ok := wantDeleted[deletedURL]; !ok {
			t.Fatalf("unexpected deletion marker %q", deletedURL)
		}
	}
}
