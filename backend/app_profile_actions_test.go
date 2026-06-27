package backend

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseProfileStartupActions(t *testing.T) {
	legacy := parseProfileStartupActions("")
	if legacy.HasExplicitConfig {
		t.Fatalf("empty profile config should not be explicit")
	}
	if !shouldApplyDefaultBookmarks(legacy) {
		t.Fatalf("legacy profiles should keep applying default bookmarks")
	}

	emptyObject := parseProfileStartupActions("{}")
	if emptyObject.HasExplicitConfig {
		t.Fatalf("empty JSON object should behave like legacy config")
	}

	invalid := parseProfileStartupActions("{")
	if invalid.HasExplicitConfig {
		t.Fatalf("invalid profile config should fall back to legacy behavior")
	}

	actions := parseProfileStartupActions(`{
		"version": 1,
		"formState": {
			"randomFingerprintOnStart": true
		},
			"postCreateActions": {
			"importCookies": "sid=abc",
			"applyDefaultBookmarks": false,
			"clearCacheBeforeStart": true,
			"clearCookiesBeforeStart": true,
			"clearLocalStorageBeforeStart": true
		}
	}`)
	if !actions.HasExplicitConfig {
		t.Fatalf("create-window profile config should be explicit")
	}
	if shouldApplyDefaultBookmarks(actions) {
		t.Fatalf("explicit false should skip default bookmarks")
	}
	if !actions.ClearCacheBeforeStart || !actions.ClearCookiesBeforeStart || !actions.ClearLocalStorageBeforeStart {
		t.Fatalf("cleanup flags were not parsed: %+v", actions)
	}
	if actions.ImportCookies != "sid=abc" {
		t.Fatalf("importCookies was not parsed: %+v", actions)
	}
	if !actions.RandomFingerprintOnStart {
		t.Fatalf("randomFingerprintOnStart was not parsed: %+v", actions)
	}
}

func TestWithFingerprintSeedReplacesExistingSeed(t *testing.T) {
	args := withFingerprintSeed([]string{
		"--user-agent=UA",
		"--fingerprint=old",
		"--fingerprint=older",
		"--lang=en-US",
	}, "new-seed")

	if !slices.Contains(args, "--fingerprint=new-seed") {
		t.Fatalf("new seed missing: %#v", args)
	}
	if slices.Contains(args, "--fingerprint=old") || slices.Contains(args, "--fingerprint=older") {
		t.Fatalf("old seeds should be removed: %#v", args)
	}
	if len(args) != 3 {
		t.Fatalf("unexpected args after replacement: %#v", args)
	}
}

func TestProfilePreStartCleanupRelativePaths(t *testing.T) {
	paths := profilePreStartCleanupRelativePaths(profileStartupActions{
		ClearCacheBeforeStart:        true,
		ClearCookiesBeforeStart:      true,
		ClearLocalStorageBeforeStart: true,
	})

	for _, want := range []string{
		"Default/Cache",
		"Default/Code Cache",
		"Default/GPUCache",
		"Default/Network/Cache",
		"Default/Service Worker/CacheStorage",
		"Default/Service Worker/ScriptCache",
		"Default/Network/Cookies",
		"Default/Network/Cookies-journal",
		"Default/Cookies",
		"Default/Cookies-journal",
		"Default/Local Storage",
	} {
		if !slices.Contains(paths, want) {
			t.Fatalf("cleanup paths missing %q in %#v", want, paths)
		}
	}
}

func TestApplyProfilePreStartActionsRemovesSelectedData(t *testing.T) {
	root := t.TempDir()
	createFile := func(rel string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	createFile("Default/Cache/item")
	createFile("Default/Network/Cookies")
	createFile("Default/Local Storage/leveldb/log")
	createFile("Default/History")

	err := applyProfilePreStartActions(root, profileStartupActions{
		ClearCacheBeforeStart:        true,
		ClearCookiesBeforeStart:      true,
		ClearLocalStorageBeforeStart: true,
	})
	if err != nil {
		t.Fatalf("applyProfilePreStartActions returned error: %v", err)
	}

	for _, rel := range []string{
		"Default/Cache",
		"Default/Network/Cookies",
		"Default/Local Storage",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("%s should have been removed, stat err=%v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Default", "History")); err != nil {
		t.Fatalf("unselected profile data should remain: %v", err)
	}
}

func TestSafeChromeProfilePathRejectsOutsidePath(t *testing.T) {
	if _, err := safeChromeProfilePath(t.TempDir(), "../outside"); err == nil {
		t.Fatalf("expected outside path to be rejected")
	}
}

func TestParseCookieImportFormats(t *testing.T) {
	jsonCookies, err := parseCookieImport(`[{"name":"sid","value":"abc","domain":".example.com"}]`, "https://fallback.test/")
	if err != nil {
		t.Fatalf("parse JSON cookies: %v", err)
	}
	if len(jsonCookies) != 1 || jsonCookies[0].Domain != ".example.com" || jsonCookies[0].Path != "/" {
		t.Fatalf("unexpected JSON cookies: %#v", jsonCookies)
	}

	netscapeCookies, err := parseCookieImport(".example.com\tTRUE\t/\tTRUE\t1893456000\ttoken\txyz", "")
	if err != nil {
		t.Fatalf("parse Netscape cookies: %v", err)
	}
	if len(netscapeCookies) != 1 || netscapeCookies[0].Name != "token" || !netscapeCookies[0].Secure {
		t.Fatalf("unexpected Netscape cookies: %#v", netscapeCookies)
	}

	simpleCookies, err := parseCookieImport("session=abc", "https://example.com/start")
	if err != nil {
		t.Fatalf("parse simple cookies: %v", err)
	}
	if len(simpleCookies) != 1 || simpleCookies[0].URL != "https://example.com/start" {
		t.Fatalf("unexpected simple cookies: %#v", simpleCookies)
	}

	if _, err := parseCookieImport("session=abc", ""); err == nil {
		t.Fatalf("simple cookies without fallback URL should fail")
	}
}

func TestClearProfileConfigImportCookies(t *testing.T) {
	next, changed, err := clearProfileConfigImportCookies(`{
		"version": 1,
		"formState": {},
		"postCreateActions": {
			"importCookies": "sid=abc",
			"clearCacheBeforeStart": true
		}
	}`)
	if err != nil {
		t.Fatalf("clearProfileConfigImportCookies returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected profile config to change")
	}
	if strings.Contains(next, "sid=abc") || strings.Contains(next, "importCookies") {
		t.Fatalf("import cookies should be removed: %s", next)
	}
	if !strings.Contains(next, "clearCacheBeforeStart") {
		t.Fatalf("other post-create actions should remain: %s", next)
	}
}
