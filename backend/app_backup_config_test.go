package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"errors"
	"path/filepath"
	"testing"
)

type testBookmarkDeletionDAO struct {
	bookmarks []config.BrowserBookmark
	deleted   []string
	readErr   error
	syncErr   error
}

func (d *testBookmarkDeletionDAO) List() ([]config.BrowserBookmark, error) {
	return append([]config.BrowserBookmark{}, d.bookmarks...), nil
}

func (d *testBookmarkDeletionDAO) ReplaceAll(bookmarks []config.BrowserBookmark) error {
	d.bookmarks = append([]config.BrowserBookmark{}, bookmarks...)
	return nil
}

func (d *testBookmarkDeletionDAO) DeletedURLs() ([]string, error) {
	if d.readErr != nil {
		return nil, d.readErr
	}
	return append([]string{}, d.deleted...), nil
}

func (d *testBookmarkDeletionDAO) SyncDeletedURLs(urls []string) error {
	if d.syncErr != nil {
		return d.syncErr
	}
	d.deleted = append([]string{}, urls...)
	return nil
}

func TestBackupApplyIncomingConfigKeepsCurrentBackupSettings(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.App.Name = ""
	app.config.Backup.Channels.OpenList.BaseURL = "http://current.example/dav"
	app.config.Backup.Channels.OpenList.RemotePath = "current/backups"
	app.config.Backup.Schedule.Enabled = true
	app.config.Backup.Schedule.DailyTime = "03:15"

	incoming := config.DefaultConfig()
	incoming.App.Name = "Imported App"
	incoming.Backup.Channels.OpenList.BaseURL = "http://imported.example/dav"
	incoming.Backup.Channels.OpenList.RemotePath = "imported/backups"
	incoming.Backup.Schedule.Enabled = false

	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}
	if app.config.App.Name != "Imported App" {
		t.Fatalf("app name = %q, want imported app name", app.config.App.Name)
	}
	backupConfig := app.config.Backup
	if backupConfig.Channels.OpenList.BaseURL != "http://current.example/dav" || backupConfig.Channels.OpenList.RemotePath != "current/backups" {
		t.Fatalf("OpenList settings = %+v, want current settings", backupConfig.Channels.OpenList)
	}
	if !backupConfig.Schedule.Enabled || backupConfig.Schedule.DailyTime != "03:15" {
		t.Fatalf("schedule = %+v, want current schedule", backupConfig.Schedule)
	}
}

func TestBackupApplyIncomingConfigDoesNotRestoreDeletedBookmarks(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	bookmarkDAO := browser.NewSQLiteBookmarkDAO(db.GetConn())
	deletedURL := "https://ping0.cc/"
	if err := bookmarkDAO.ReplaceAll([]config.BrowserBookmark{{Name: "Ping0", URL: deletedURL}}); err != nil {
		t.Fatal(err)
	}
	if err := bookmarkDAO.ReplaceAll(nil); err != nil {
		t.Fatal(err)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.BookmarkDAO = bookmarkDAO
	incoming := config.DefaultConfig()
	incoming.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "Ping0", URL: deletedURL},
		{Name: "New", URL: "https://new.example/"},
	}
	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}

	for _, bookmark := range app.config.Browser.DefaultBookmarks {
		if bookmark.URL == deletedURL {
			t.Fatalf("deleted bookmark was restored in config: %#v", bookmark)
		}
	}
	foundNew := false
	for _, bookmark := range app.config.Browser.DefaultBookmarks {
		if bookmark.URL == "https://new.example/" {
			foundNew = true
			break
		}
	}
	if !foundNew {
		t.Fatalf("new incoming bookmark was not merged: %#v", app.config.Browser.DefaultBookmarks)
	}
}

func TestBookmarkConfigFallbackDoesNotRestoreDeletedBookmark(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.config.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "Ping0", URL: "https://ping0.cc/"},
		{Name: "Google", URL: "https://www.google.com/"},
	}

	if err := app.BookmarkSave([]BrowserBookmark{{Name: "Google", URL: "https://www.google.com/"}}); err != nil {
		t.Fatalf("BookmarkSave() error = %v", err)
	}
	savedConfig, err := config.Load(filepath.Join(app.appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("load saved fallback config: %v", err)
	}
	if len(savedConfig.Browser.DeletedBookmarkURLs) != 1 || savedConfig.Browser.DeletedBookmarkURLs[0] != "https://ping0.cc/" {
		t.Fatalf("persisted fallback deletion markers = %#v, want Ping0 marker", savedConfig.Browser.DeletedBookmarkURLs)
	}
	app.config = savedConfig
	incoming := config.DefaultConfig()
	incoming.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "Ping0", URL: "https://ping0.cc/"},
		{Name: "New", URL: "https://new.example/"},
	}
	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}

	for _, bookmark := range app.config.Browser.DefaultBookmarks {
		if bookmark.URL == "https://ping0.cc/" {
			t.Fatalf("deleted bookmark was restored in config fallback: %#v", bookmark)
		}
	}
	if len(app.config.Browser.DeletedBookmarkURLs) != 1 || app.config.Browser.DeletedBookmarkURLs[0] != "https://ping0.cc/" {
		t.Fatalf("fallback deletion markers = %#v, want Ping0 marker", app.config.Browser.DeletedBookmarkURLs)
	}
	foundNew := false
	for _, bookmark := range app.config.Browser.DefaultBookmarks {
		if bookmark.URL == "https://new.example/" {
			foundNew = true
			break
		}
	}
	if !foundNew {
		t.Fatalf("new incoming bookmark was not merged: %#v", app.config.Browser.DefaultBookmarks)
	}
}

func TestBackupApplyIncomingConfigMergesConfigAndSQLiteDeletionMarkers(t *testing.T) {
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
	sqliteURL := "https://sqlite.example/Deleted?Q=SQLite"
	if err := bookmarkDAO.ReplaceAll([]config.BrowserBookmark{{Name: "SQLite", URL: sqliteURL}}); err != nil {
		t.Fatal(err)
	}
	if err := bookmarkDAO.ReplaceAll(nil); err != nil {
		t.Fatal(err)
	}
	targetURL := "https://target.example/active"
	if err := bookmarkDAO.ReplaceAll([]config.BrowserBookmark{{Name: "Target", URL: targetURL}}); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Browser.DefaultBookmarks = []config.BrowserBookmark{{Name: "Target", URL: targetURL}}
	app.config.Browser.DeletedBookmarkURLs = []string{"https://config.example/deleted"}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.BookmarkDAO = bookmarkDAO
	incoming := config.DefaultConfig()
	incoming.Browser.DeletedBookmarkURLs = []string{"https://incoming.example/deleted"}
	incoming.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "Target from backup", URL: targetURL},
		{Name: "Config deleted", URL: "https://config.example/deleted"},
		{Name: "SQLite deleted", URL: sqliteURL},
		{Name: "Incoming deleted", URL: "https://incoming.example/deleted"},
		{Name: "New", URL: "https://new.example/"},
	}

	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}

	deletedURLs, err := bookmarkDAO.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	wantDeleted := map[string]struct{}{
		"https://config.example/deleted":          {},
		"https://incoming.example/deleted":        {},
		"https://sqlite.example/Deleted?Q=SQLite": {},
	}
	if len(deletedURLs) != len(wantDeleted) {
		t.Fatalf("SQLite deletion markers = %#v, want %#v", deletedURLs, wantDeleted)
	}
	for _, deletedURL := range deletedURLs {
		if _, ok := wantDeleted[deletedURL]; !ok {
			t.Fatalf("unexpected SQLite deletion marker %q", deletedURL)
		}
	}
	if !containsBookmarkURL(app.config.Browser.DefaultBookmarks, targetURL) {
		t.Fatalf("target active bookmark was not preserved: %#v", app.config.Browser.DefaultBookmarks)
	}
	if !containsBookmarkURL(app.config.Browser.DefaultBookmarks, "https://new.example/") {
		t.Fatalf("new bookmark was not merged: %#v", app.config.Browser.DefaultBookmarks)
	}
	for deletedURL := range wantDeleted {
		if containsBookmarkURL(app.config.Browser.DefaultBookmarks, deletedURL) {
			t.Fatalf("deleted bookmark %q was restored: %#v", deletedURL, app.config.Browser.DefaultBookmarks)
		}
	}
}

func TestBackupApplyIncomingConfigWithoutSQLiteKeepsDeletionMarkers(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	incoming := config.DefaultConfig()
	incoming.Browser.DeletedBookmarkURLs = []string{"https://config-only.example/deleted"}
	incoming.Browser.DefaultBookmarks = []config.BrowserBookmark{{
		Name: "Deleted",
		URL:  "https://config-only.example/deleted",
	}}

	if err := app.backupApplyIncomingConfig(incoming); err != nil {
		t.Fatalf("apply incoming config: %v", err)
	}
	if len(app.config.Browser.DeletedBookmarkURLs) != 1 || app.config.Browser.DeletedBookmarkURLs[0] != "https://config-only.example/deleted" {
		t.Fatalf("config-only deletion markers = %#v", app.config.Browser.DeletedBookmarkURLs)
	}
	if containsBookmarkURL(app.config.Browser.DefaultBookmarks, "https://config-only.example/deleted") {
		t.Fatalf("config-only deleted bookmark was restored: %#v", app.config.Browser.DefaultBookmarks)
	}
}

func TestBackupApplyIncomingConfigReportsSQLiteDeletionReadFailure(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.BookmarkDAO = &testBookmarkDeletionDAO{readErr: errors.New("read deletion markers")}
	incoming := config.DefaultConfig()
	incoming.Browser.DeletedBookmarkURLs = []string{"https://failure.example/deleted"}

	if err := app.backupApplyIncomingConfig(incoming); err == nil {
		t.Fatal("backupApplyIncomingConfig succeeded despite SQLite deletion read failure")
	}
}

func TestBackupApplyIncomingConfigReportsSQLiteDeletionWriteFailure(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.BookmarkDAO = &testBookmarkDeletionDAO{syncErr: errors.New("write deletion markers")}
	incoming := config.DefaultConfig()
	incoming.Browser.DeletedBookmarkURLs = []string{"https://failure.example/deleted"}

	if err := app.backupApplyIncomingConfig(incoming); err == nil {
		t.Fatal("backupApplyIncomingConfig succeeded despite SQLite deletion write failure")
	}
}

func TestBackupMergeConfigKeepsPathsAndQueriesCaseSensitive(t *testing.T) {
	current := config.DefaultConfig()
	current.Browser.DeletedBookmarkURLs = []string{"https://EXAMPLE.com/Path?Query=One"}
	incoming := config.DefaultConfig()
	incoming.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "Same host and path", URL: "https://example.com/Path?Query=One"},
		{Name: "Different path", URL: "https://example.com/path?Query=One"},
		{Name: "Different query", URL: "https://example.com/Path?query=One"},
	}

	merged := backupMergeConfig(current, incoming)
	if containsBookmarkURL(merged.Browser.DefaultBookmarks, "https://example.com/Path?Query=One") {
		t.Fatalf("same host/path/query bookmark was restored: %#v", merged.Browser.DefaultBookmarks)
	}
	if !containsBookmarkURL(merged.Browser.DefaultBookmarks, "https://example.com/path?Query=One") {
		t.Fatalf("different path bookmark was incorrectly filtered: %#v", merged.Browser.DefaultBookmarks)
	}
	if !containsBookmarkURL(merged.Browser.DefaultBookmarks, "https://example.com/Path?query=One") {
		t.Fatalf("different query bookmark was incorrectly filtered: %#v", merged.Browser.DefaultBookmarks)
	}
}

func containsBookmarkURL(bookmarks []config.BrowserBookmark, expectedURL string) bool {
	for _, bookmark := range bookmarks {
		if bookmarkURLKey(bookmark.URL) == bookmarkURLKey(expectedURL) {
			return true
		}
	}
	return false
}
