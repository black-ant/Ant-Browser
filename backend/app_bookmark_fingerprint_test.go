package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBookmarkListIncludesFingerprintCheck(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	list := app.BookmarkList()
	if len(list) == 0 || list[0].URL != fingerprintCheckBookmarkURL {
		t.Fatalf("first bookmark = %#v, want fingerprint check", list)
	}
}

func TestBookmarkSavePreservesFingerprintCheck(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)

	if err := app.BookmarkSave([]BrowserBookmark{{Name: "Google", URL: "https://www.google.com/"}}); err != nil {
		t.Fatalf("BookmarkSave() error = %v", err)
	}
	list := app.BookmarkList()
	if len(list) == 0 || list[0].URL != fingerprintCheckBookmarkURL {
		t.Fatalf("first bookmark = %#v, want protected fingerprint check", list)
	}
}

func TestBookmarkSaveKeepsFingerprintCheckOpenOnStart(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)

	if err := app.BookmarkSave([]BrowserBookmark{{Name: "指纹检测", URL: fingerprintCheckBookmarkURL, OpenOnStart: true}}); err != nil {
		t.Fatalf("BookmarkSave() error = %v", err)
	}
	list := app.BookmarkList()
	if len(list) == 0 || list[0].URL != fingerprintCheckBookmarkURL || !list[0].OpenOnStart {
		t.Fatalf("first bookmark = %#v, want protected fingerprint check with openOnStart", list)
	}
}

func TestBookmarkSaveDoesNotRestoreDeletedVerificationBookmark(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)

	if err := app.BookmarkSave([]BrowserBookmark{
		{Name: "Ping0", URL: "https://ping0.cc/"},
		{Name: "Google", URL: "https://www.google.com/"},
	}); err != nil {
		t.Fatalf("initial BookmarkSave() error = %v", err)
	}
	if err := app.BookmarkSave([]BrowserBookmark{{Name: "Google", URL: "https://www.google.com/"}}); err != nil {
		t.Fatalf("updated BookmarkSave() error = %v", err)
	}

	list := app.BookmarkList()
	if len(list) != 2 || list[0].URL != fingerprintCheckBookmarkURL || list[1].URL != "https://www.google.com/" {
		t.Fatalf("bookmarks after deletion = %#v, want protected bookmark and Google only", list)
	}
}

func TestResolveFingerprintCheckStartURLs(t *testing.T) {
	app := NewApp(t.TempDir())
	app.browserMgr = browser.NewManager(nil, app.appRoot)
	app.browserMgr.Profiles["profile-123"] = &browser.Profile{ProfileId: "profile-123"}

	urls := app.resolveFingerprintCheckStartURLs("profile-123", []string{"https://example.com", fingerprintCheckBookmarkURL})
	if len(urls) != 2 {
		t.Fatalf("urls len = %d", len(urls))
	}
	if urls[0] != "https://example.com" {
		t.Fatalf("first url = %q", urls[0])
	}
	if !strings.HasPrefix(urls[1], "file://") || !strings.Contains(urls[1], "profileId=profile-123") {
		t.Fatalf("fingerprint url = %q", urls[1])
	}
}

func TestResolveFingerprintCheckStartURLsWithProfileDoesNotRelockManager(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	profile := &browser.Profile{
		ProfileId:   "profile-locked",
		ProxyId:     "__direct__",
		ProxyConfig: "direct://",
	}
	app.browserMgr.Profiles[profile.ProfileId] = profile

	done := make(chan []string, 1)
	app.browserMgr.Mutex.Lock()
	go func() {
		expectedArgs := app.buildFingerprintCheckExpectedArgs(profile.ProfileId, profile.CoreId, profile.FingerprintArgs, profile.LaunchArgs)
		done <- app.resolveFingerprintCheckStartURLsForExpectedArgsAndProfile(profile.ProfileId, expectedArgs, profile, []string{fingerprintCheckBookmarkURL})
	}()

	select {
	case urls := <-done:
		app.browserMgr.Mutex.Unlock()
		if len(urls) != 1 || !strings.HasPrefix(urls[0], "file://") {
			t.Fatalf("urls = %#v, want fingerprint file URL", urls)
		}
	case <-time.After(time.Second):
		app.browserMgr.Mutex.Unlock()
		t.Fatalf("resolve fingerprint start URLs deadlocked while manager mutex was held")
	}
}

func TestRuntimeBookmarksForProfileUsesFileURL(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.Profiles["profile-123"] = &browser.Profile{ProfileId: "profile-123"}

	bookmarks, fingerprintURL, err := app.runtimeBookmarksForProfile("profile-123", []BrowserBookmark{
		{Name: "指纹检测", URL: fingerprintCheckBookmarkURL},
		{Name: "Ping0", URL: "https://ping0.cc/"},
	})
	if err != nil {
		t.Fatalf("runtimeBookmarksForProfile error = %v", err)
	}
	if !strings.HasPrefix(fingerprintURL, "file://") || strings.Contains(fingerprintURL, "ts=") {
		t.Fatalf("fingerprintURL = %q, want stable file URL", fingerprintURL)
	}
	if len(bookmarks) != 2 || bookmarks[0].URL != fingerprintURL || bookmarks[1].URL != "https://ping0.cc/" {
		t.Fatalf("bookmarks = %#v, fingerprintURL = %q", bookmarks, fingerprintURL)
	}
}

func TestRuntimeBookmarksForProfileIncludesProxyContext(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.config.Browser.Proxies = []browser.Proxy{
		{
			ProxyId:     "proxy-auth",
			ProxyName:   "Auth Proxy",
			ProxyConfig: "http://ant:secret-pass@127.0.0.1:18080",
			GroupName:   "Auth Group",
		},
	}
	app.browserMgr.Profiles["profile-proxy-bookmark"] = &browser.Profile{
		ProfileId:   "profile-proxy-bookmark",
		ProxyId:     "proxy-auth",
		ProxyConfig: "http://ant:secret-pass@127.0.0.1:18080",
	}

	_, fingerprintURL, err := app.runtimeBookmarksForProfile("profile-proxy-bookmark", []BrowserBookmark{{Name: "指纹检测", URL: fingerprintCheckBookmarkURL}})
	if err != nil {
		t.Fatalf("runtimeBookmarksForProfile error = %v", err)
	}
	parsed, err := url.Parse(fingerprintURL)
	if err != nil {
		t.Fatalf("parse fingerprint URL error = %v", err)
	}
	pagePath, err := url.PathUnescape(parsed.Path)
	if err != nil {
		t.Fatalf("decode fingerprint page path error = %v", err)
	}
	if len(pagePath) >= 3 && pagePath[0] == '/' && pagePath[2] == ':' {
		pagePath = pagePath[1:]
	}
	content, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read fingerprint page error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"proxyName": "Auth Proxy"`) || !strings.Contains(text, `"hasAuth": true`) {
		t.Fatalf("fingerprint page missing proxy context: %s", text)
	}
	if strings.Contains(text, "secret-pass") {
		t.Fatalf("fingerprint page leaked proxy password: %s", text)
	}
}
