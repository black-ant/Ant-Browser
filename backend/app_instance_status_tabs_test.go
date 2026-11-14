package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBrowserInstanceGetTabsReadsPageTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/json/list" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode([]browserDebugTabTarget{
			{ID: "page-1", Type: "page", Title: "控制台", URL: "https://console.example.test"},
			{ID: "worker-1", Type: "service_worker", Title: "后台服务", URL: "https://worker.example.test"},
			{ID: "page-2", Type: "page", Title: "", URL: "https://fallback.example.test"},
		})
	}))
	defer server.Close()

	debugPort := server.Listener.Addr().(*net.TCPAddr).Port
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	manager.Profiles["profile-1"] = &browser.Profile{
		ProfileId:  "profile-1",
		Running:    true,
		DebugReady: true,
		DebugPort:  debugPort,
	}
	app := NewApp(root)
	app.browserMgr = manager

	got := app.BrowserInstanceGetTabs("profile-1")
	want := []BrowserTab{
		{TabId: "page-1", Title: "控制台", Url: "https://console.example.test", Active: false},
		{TabId: "page-2", Title: "https://fallback.example.test", Url: "https://fallback.example.test", Active: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BrowserInstanceGetTabs() = %#v, want %#v", got, want)
	}
}

func TestBrowserInstanceGetTabsReturnsEmptyBeforeDebugReady(t *testing.T) {
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	manager.Profiles["profile-1"] = &browser.Profile{
		ProfileId: "profile-1",
		Running:   true,
		DebugPort: 9222,
	}
	app := NewApp(root)
	app.browserMgr = manager

	if tabs := app.BrowserInstanceGetTabs("profile-1"); len(tabs) != 0 {
		t.Fatalf("BrowserInstanceGetTabs() = %#v, want no tabs before debug is ready", tabs)
	}
}
