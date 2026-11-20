package launchcode

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
)

func newActiveCDPServer(t *testing.T, enabled bool, apiKey string) (*httptest.Server, *fakeChrome) {
	t.Helper()
	chrome := newFakeChrome(t)
	server := newAuthTestServer(enabled, apiKey)
	server.SetActiveProfile(&browser.Profile{ProfileId: "p-1", ProfileName: "one", DebugPort: chrome.port(t), DebugReady: true})
	front := httptest.NewServer(NewTestHandler(server))
	t.Cleanup(front.Close)
	return front, chrome
}

func TestCDPProxyRequiresAPIKeyWhenAuthEnabled(t *testing.T) {
	front, _ := newActiveCDPServer(t, true, testAPIKey)

	cases := []struct {
		name string
		key  string
		want int
	}{
		{"missing key", "", http.StatusUnauthorized},
		{"wrong key", "nope", http.StatusUnauthorized},
		{"valid key", testAPIKey, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]interface{}
			status := getJSON(t, front.URL+"/json/version", keyHeader(tc.key), &body)
			if status != tc.want {
				t.Fatalf("status = %d, want %d (body %v)", status, tc.want, body)
			}
			if tc.want == http.StatusUnauthorized && body["authHeader"] != DefaultAPIKeyHeader {
				t.Fatalf("401 body should name the auth header, got %v", body)
			}
		})
	}
}

func TestCDPProxyWebSocketUpgradeRequiresAPIKey(t *testing.T) {
	front, _ := newActiveCDPServer(t, true, testAPIKey)
	wsURL := "ws" + strings.TrimPrefix(front.URL, "http") + "/devtools/browser/b-1"

	if status := dialWS(t, wsURL, nil); status != http.StatusUnauthorized {
		t.Fatalf("upgrade without key: status = %d, want 401", status)
	}
	if status := dialWS(t, wsURL, keyHeader("nope")); status != http.StatusUnauthorized {
		t.Fatalf("upgrade with wrong key: status = %d, want 401", status)
	}
	if status := dialWS(t, wsURL, keyHeader(testAPIKey)); status != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade with key: status = %d, want 101", status)
	}
}

func TestCDPProxyWithoutActiveAuthIsUnchanged(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		apiKey  string
	}{
		{"auth disabled", false, testAPIKey},
		{"auth enabled without key configured", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			front, _ := newActiveCDPServer(t, tc.enabled, tc.apiKey)
			if status := getJSON(t, front.URL+"/json/version", nil, nil); status != http.StatusOK {
				t.Fatalf("GET without key: status = %d, want 200", status)
			}
			wsURL := "ws" + strings.TrimPrefix(front.URL, "http") + "/devtools/browser/b-1"
			if status := dialWS(t, wsURL, nil); status != http.StatusSwitchingProtocols {
				t.Fatalf("upgrade without key: status = %d, want 101", status)
			}
		})
	}
}

// /api/* 的鉴权行为不能因为抽出公共校验而改变。
func TestAPIRoutesStillRequireAPIKey(t *testing.T) {
	front, _ := newActiveCDPServer(t, true, testAPIKey)
	if status := getJSON(t, front.URL+"/api/health", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("/api/health without key: status = %d, want 401", status)
	}
	if status := getJSON(t, front.URL+"/api/health", keyHeader(testAPIKey), nil); status != http.StatusOK {
		t.Fatalf("/api/health with key: status = %d, want 200", status)
	}
}
