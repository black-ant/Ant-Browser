package launchcode_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
)

func mustDebugPortFromURL(t *testing.T, rawURL string) int {
	t.Helper()

	hostPort := strings.TrimPrefix(rawURL, "http://")
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("解析测试 URL 失败: %v", err)
	}
	if host == "" {
		t.Fatalf("测试 URL host 为空: %s", rawURL)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("解析测试端口失败: %v", err)
	}
	return port
}

func TestCDPProxyReturnsUnavailableWithoutActiveTarget(t *testing.T) {
	handler := buildTestHandler(newInMemoryService(), newMockStarter())

	req := httptest.NewRequest(http.MethodGet, "/json/version", nil)
	req.Host = "127.0.0.1:19876"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("期望 503，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["error"] != "no active browser debug target" {
		t.Fatalf("错误信息不正确: %+v", resp)
	}
}

func TestCDPProxySwitchesToLatestLaunchedProfile(t *testing.T) {
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"Browser":"Mock-A"}`))
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"Browser":"Mock-B"}`))
	}))
	defer serverB.Close()

	svc := newInMemoryService()
	starter := newMockStarter()
	profileA := &browser.Profile{
		ProfileId:   "profile-a",
		ProfileName: "Profile A",
		Pid:         1001,
		DebugPort:   mustDebugPortFromURL(t, serverA.URL),
	}
	profileB := &browser.Profile{
		ProfileId:   "profile-b",
		ProfileName: "Profile B",
		Pid:         1002,
		DebugPort:   mustDebugPortFromURL(t, serverB.URL),
	}
	starter.addProfile(profileA)
	starter.addProfile(profileB)

	codeA, err := svc.EnsureCode(profileA.ProfileId)
	if err != nil {
		t.Fatalf("EnsureCode(A) 失败: %v", err)
	}
	codeB, err := svc.EnsureCode(profileB.ProfileId)
	if err != nil {
		t.Fatalf("EnsureCode(B) 失败: %v", err)
	}

	handler := buildTestHandler(svc, starter)

	for _, tc := range []struct {
		code       string
		wantMarker string
	}{
		{code: codeA, wantMarker: "Mock-A"},
		{code: codeB, wantMarker: "Mock-B"},
	} {
		launchReq := httptest.NewRequest(http.MethodGet, "/api/launch/"+tc.code, nil)
		launchReq.Header.Set("X-Launch-Request", "1")
		launchResp := httptest.NewRecorder()
		handler.ServeHTTP(launchResp, launchReq)
		if launchResp.Code != http.StatusOK {
			t.Fatalf("启动请求失败: code=%s status=%d body=%s", tc.code, launchResp.Code, launchResp.Body.String())
		}

		proxyReq := httptest.NewRequest(http.MethodGet, "/json/version", nil)
		proxyReq.Host = "127.0.0.1:19876"
		proxyResp := httptest.NewRecorder()
		handler.ServeHTTP(proxyResp, proxyReq)
		if proxyResp.Code != http.StatusOK {
			t.Fatalf("代理请求失败: code=%s status=%d body=%s", tc.code, proxyResp.Code, proxyResp.Body.String())
		}
		if !strings.Contains(proxyResp.Body.String(), tc.wantMarker) {
			t.Fatalf("代理未切换到最新窗口: want=%s body=%s", tc.wantMarker, proxyResp.Body.String())
		}
	}
}

func TestCDPProxySkipsPendingDebugProfile(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarter()
	profile := &browser.Profile{
		ProfileId:      "profile-pending",
		ProfileName:    "Profile Pending",
		Running:        true,
		Pid:            2001,
		DebugPort:      9777,
		DebugReady:     false,
		RuntimeWarning: "debug pending",
	}
	starter.addProfile(profile)

	code, err := svc.EnsureCode(profile.ProfileId)
	if err != nil {
		t.Fatalf("EnsureCode 失败: %v", err)
	}

	handler := buildTestHandler(svc, starter)

	launchReq := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
	launchReq.Header.Set("X-Launch-Request", "1")
	launchResp := httptest.NewRecorder()
	handler.ServeHTTP(launchResp, launchReq)
	if launchResp.Code != http.StatusOK {
		t.Fatalf("启动请求失败: status=%d body=%s", launchResp.Code, launchResp.Body.String())
	}

	var launchPayload map[string]interface{}
	if err := json.NewDecoder(launchResp.Body).Decode(&launchPayload); err != nil {
		t.Fatalf("解析启动响应失败: %v", err)
	}
	if ready, _ := launchPayload["debugReady"].(bool); ready {
		t.Fatalf("pending 窗口不应被标记为 debugReady: %+v", launchPayload)
	}

	proxyReq := httptest.NewRequest(http.MethodGet, "/json/version", nil)
	proxyReq.Host = "127.0.0.1:19876"
	proxyResp := httptest.NewRecorder()
	handler.ServeHTTP(proxyResp, proxyReq)
	if proxyResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("pending 窗口不应成为活动 CDP target: status=%d body=%s", proxyResp.Code, proxyResp.Body.String())
	}
}

// TestCDPProxyRejectsBrowserOriginatedRequests 验证 CDP 代理对浏览器发起的
// DNS-rebinding / localhost-CSRF 攻击的防护：非回环 Host 或跨源 Origin 一律拒绝，
// 而合法的命令行自动化工具（无 Origin、Host 指向回环）不受影响。
func TestCDPProxyRejectsBrowserOriginatedRequests(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarter()
	profile := &browser.Profile{
		ProfileId:   "profile-sec",
		ProfileName: "Profile Sec",
		Pid:         3001,
		DebugPort:   9778,
	}
	starter.addProfile(profile)

	code, err := svc.EnsureCode(profile.ProfileId)
	if err != nil {
		t.Fatalf("EnsureCode 失败: %v", err)
	}

	handler := buildTestHandler(svc, starter)

	launchReq := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
	launchReq.Header.Set("X-Launch-Request", "1")
	launchResp := httptest.NewRecorder()
	handler.ServeHTTP(launchResp, launchReq)
	if launchResp.Code != http.StatusOK {
		t.Fatalf("启动请求失败: status=%d body=%s", launchResp.Code, launchResp.Body.String())
	}

	cases := []struct {
		name   string
		host   string
		origin string
	}{
		{name: "rebinding host", host: "attacker.example.com", origin: ""},
		{name: "cross-origin", host: "127.0.0.1:19876", origin: "https://evil.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/json/version", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != http.StatusForbidden {
				t.Fatalf("期望 403，实际 %d，body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}
