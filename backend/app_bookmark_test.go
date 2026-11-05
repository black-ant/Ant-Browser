package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestMigrateToSQLiteFiltersDeletedDefaultBookmarks(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	bookmarkDAO := browser.NewSQLiteBookmarkDAO(db.GetConn())
	if err := bookmarkDAO.ReplaceAll([]config.BrowserBookmark{{Name: "Ping0", URL: "https://ping0.cc/"}}); err != nil {
		t.Fatal(err)
	}
	if err := bookmarkDAO.ReplaceAll(nil); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = db
	app.config = config.DefaultConfig()
	app.config.Browser.DefaultBookmarks = nil
	app.config.Browser.DeletedBookmarkURLs = []string{"https://www.google.com/"}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	app.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	app.browserMgr.BookmarkDAO = bookmarkDAO
	app.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(db.GetConn())

	app.migrateToSQLite()
	bookmarks, err := bookmarkDAO.List()
	if err != nil {
		t.Fatal(err)
	}
	if containsBookmarkURL(bookmarks, "https://www.google.com/") {
		t.Fatalf("config-deleted default bookmark was initialized: %#v", bookmarks)
	}
	if containsBookmarkURL(bookmarks, "https://ping0.cc/") {
		t.Fatalf("SQLite-deleted default bookmark was initialized: %#v", bookmarks)
	}
	if !containsBookmarkURL(bookmarks, fingerprintCheckBookmarkURL) {
		t.Fatalf("protected fingerprint bookmark was not initialized: %#v", bookmarks)
	}

	deletedURLs, err := bookmarkDAO.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedURLs) != 2 || !containsStringValue(deletedURLs, "https://ping0.cc/") || !containsStringValue(deletedURLs, "https://www.google.com/") {
		t.Fatalf("deletion markers after migration = %#v", deletedURLs)
	}
}

func TestBookmarkSaveAllowsVerificationBookmarksToBeRemovedAndReset(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Browser.DefaultBookmarks = append([]config.BrowserBookmark{}, defaultBookmarkList...)
	app.browserMgr = browser.NewManager(app.config, app.appRoot)

	if err := app.BookmarkSave([]BrowserBookmark{{Name: "Google", URL: "https://www.google.com/"}}); err != nil {
		t.Fatal(err)
	}
	for _, verificationURL := range []string{
		"https://ippure.com/",
		"https://iplark.com/",
		"https://ping0.cc/",
	} {
		if containsBookmarkURL(app.BookmarkList(), verificationURL) {
			t.Fatalf("verification bookmark %q was automatically restored: %#v", verificationURL, app.BookmarkList())
		}
	}

	savedConfig, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	restartedApp := NewApp(root)
	restartedApp.config = savedConfig
	restartedApp.browserMgr = browser.NewManager(savedConfig, restartedApp.appRoot)
	if containsBookmarkURL(restartedApp.BookmarkList(), "https://ping0.cc/") {
		t.Fatal("verification bookmark returned after config reload")
	}

	if err := restartedApp.BookmarkReset(); err != nil {
		t.Fatal(err)
	}
	if !containsBookmarkURL(restartedApp.BookmarkList(), "https://ping0.cc/") {
		t.Fatal("reset did not restore the default verification bookmark")
	}
	savedAfterReset, err := config.Load(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(savedAfterReset.Browser.DeletedBookmarkURLs) != 0 {
		t.Fatalf("deletion markers after reset = %#v, want none", savedAfterReset.Browser.DeletedBookmarkURLs)
	}
}

func containsStringValue(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
