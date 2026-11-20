package launchcode

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
)

// fakeCDPStarter 模拟 App：启动时把实例标成运行并指向 fakeChrome，
// 并像 App 的启动流程一样顺手调 SetActiveProfile。
type fakeCDPStarter struct {
	mgr        *browser.Manager
	server     *LaunchServer
	debugPort  int
	delay      time.Duration
	neverReady bool
	startErr   error
	starts     atomic.Int32
}

func (f *fakeCDPStarter) StartInstance(profileID string) (*browser.Profile, error) {
	f.starts.Add(1)
	time.Sleep(f.delay)
	if f.startErr != nil {
		return nil, f.startErr
	}

	f.mgr.Mutex.Lock()
	profile := f.mgr.Profiles[profileID]
	profile.Running = true
	profile.Pid = 4242
	profile.DebugPort = f.debugPort
	profile.DebugReady = !f.neverReady
	if f.neverReady {
		profile.RuntimeWarning = "debug port pending"
	}
	snapshot := *profile
	f.mgr.Mutex.Unlock()

	f.server.SetActiveProfile(&snapshot)
	return &snapshot, nil
}

func (f *fakeCDPStarter) StatusInstance(profileID string) (*browser.Profile, error) {
	f.mgr.Mutex.Lock()
	defer f.mgr.Mutex.Unlock()
	profile, ok := f.mgr.Profiles[profileID]
	if !ok {
		return nil, errors.New("profile not found")
	}
	snapshot := *profile
	return &snapshot, nil
}

type cdpProfileFixture struct {
	server  *LaunchServer
	starter *fakeCDPStarter
	chrome  *fakeChrome
	front   *httptest.Server
}

func newCDPProfileFixture(t *testing.T, profiles ...*browser.Profile) *cdpProfileFixture {
	t.Helper()
	chrome := newFakeChrome(t)
	mgr := &browser.Manager{Profiles: map[string]*browser.Profile{}}
	for _, p := range profiles {
		if p.Running {
			p.DebugPort = chrome.port(t)
			p.DebugReady = true
		}
		mgr.Profiles[p.ProfileId] = p
	}
	starter := &fakeCDPStarter{mgr: mgr, debugPort: chrome.port(t)}
	server := NewLaunchServer(nil, starter, mgr, 0)
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: testAPIKey})
	starter.server = server
	front := httptest.NewServer(NewTestHandler(server))
	t.Cleanup(front.Close)
	return &cdpProfileFixture{server: server, starter: starter, chrome: chrome, front: front}
}

func (f *cdpProfileFixture) get(t *testing.T, path, host string, out interface{}) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.front.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(DefaultAPIKeyHeader, testAPIKey)
	if host != "" {
		req.Host = host
	}
	return doJSON(t, req, out)
}

func doJSON(t *testing.T, req *http.Request, out interface{}) int {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", req.URL, err)
		}
	}
	return resp.StatusCode
}

func TestParseCDPProfilePath(t *testing.T) {
	cases := []struct {
		path     string
		wantRef  string
		wantRest string
		wantErr  bool
	}{
		{path: "/cdp/shop-a/json/version/", wantRef: "shop-a", wantRest: "/json/version/"},
		{path: "/cdp/shop-a/json/version", wantRef: "shop-a", wantRest: "/json/version"},
		{path: "/cdp/shop-a", wantRef: "shop-a", wantRest: "/"},
		{path: "/cdp/shop-a/", wantRef: "shop-a", wantRest: "/"},
		{path: "/cdp/%E5%BA%97%E9%93%BA/json", wantRef: "店铺", wantRest: "/json"},
		{path: "/cdp/a%2Fb/devtools/page/X", wantRef: "a/b", wantRest: "/devtools/page/X"},
		{path: "/cdp/%20shop-a%20/json", wantRef: "shop-a", wantRest: "/json"},
		{path: "/cdp/", wantErr: true},
		{path: "/cdp//json/version", wantErr: true},
		{path: "/cdp/%zz/json", wantErr: true},
		{path: "/json/version", wantErr: true},
	}
	for _, tc := range cases {
		ref, rest, err := parseCDPProfilePath(tc.path)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseCDPProfilePath(%q) = (%q, %q), want error", tc.path, ref, rest)
			}
			continue
		}
		if err != nil || ref != tc.wantRef || rest != tc.wantRest {
			t.Errorf("parseCDPProfilePath(%q) = (%q, %q, %v), want (%q, %q)", tc.path, ref, rest, err, tc.wantRef, tc.wantRest)
		}
	}
}

func TestIsCDPDiscoveryPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/json": true, "/json/": true, "/json/list": true, "/json/version": true, "/json/version/": true, "/json/new": true,
		"/json/protocol": false, "/devtools/page/X": false, "/": false,
	} {
		if got := isCDPDiscoveryPath(path); got != want {
			t.Errorf("isCDPDiscoveryPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRewriteCDPDiscoveryBody(t *testing.T) {
	base := cdpPublicBase{host: "browser.example:9000", profileID: "p-1"}
	secure := cdpPublicBase{secure: true, host: "browser.example", profileID: "p-1"}

	t.Run("version", func(t *testing.T) {
		body := `{"Browser":"Chrome/146","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/browser/b-1"}`
		got := rewriteForTest(t, body, base)
		if got[0]["webSocketDebuggerUrl"] != "ws://browser.example:9000/cdp/p-1/devtools/browser/b-1" {
			t.Fatalf("webSocketDebuggerUrl = %v", got[0]["webSocketDebuggerUrl"])
		}
		if got[0]["Browser"] != "Chrome/146" {
			t.Fatalf("other fields must be kept, got %v", got[0])
		}
		if got := rewriteForTest(t, body, secure); got[0]["webSocketDebuggerUrl"] != "wss://browser.example/cdp/p-1/devtools/browser/b-1" {
			t.Fatalf("secure webSocketDebuggerUrl = %v", got[0]["webSocketDebuggerUrl"])
		}
	})

	t.Run("list", func(t *testing.T) {
		body := `[
			{"id":"page-1","title":"A & B","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/page-1",
			 "devtoolsFrontendUrl":"/devtools/inspector.html?ws=127.0.0.1:9222/devtools/page/page-1"},
			{"id":"page-2","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/page-2",
			 "devtoolsFrontendUrl":"https://chrome-devtools-frontend.appspot.com/serve_rev/@rev/inspector.html?ws=127.0.0.1:9222/devtools/page/page-2&panel=network"}
		]`
		got := rewriteForTest(t, body, base)
		want := []map[string]string{
			{
				"webSocketDebuggerUrl": "ws://browser.example:9000/cdp/p-1/devtools/page/page-1",
				"devtoolsFrontendUrl":  "/cdp/p-1/devtools/inspector.html?ws=browser.example:9000/cdp/p-1/devtools/page/page-1",
			},
			{
				"webSocketDebuggerUrl": "ws://browser.example:9000/cdp/p-1/devtools/page/page-2",
				"devtoolsFrontendUrl":  "https://chrome-devtools-frontend.appspot.com/serve_rev/@rev/inspector.html?ws=browser.example:9000/cdp/p-1/devtools/page/page-2&panel=network",
			},
		}
		for i := range want {
			for key, value := range want[i] {
				if got[i][key] != value {
					t.Errorf("item %d %s = %v, want %s", i, key, got[i][key], value)
				}
			}
		}
		if got[0]["title"] != "A & B" {
			t.Errorf("title = %v", got[0]["title"])
		}

		secureList := rewriteForTest(t, body, secure)
		if v := secureList[0]["devtoolsFrontendUrl"]; v != "/cdp/p-1/devtools/inspector.html?wss=browser.example/cdp/p-1/devtools/page/page-1" {
			t.Errorf("secure devtoolsFrontendUrl = %v", v)
		}
	})

	t.Run("escaped profile id", func(t *testing.T) {
		body := `{"webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/browser/b-1"}`
		got := rewriteForTest(t, body, cdpPublicBase{host: "h:1", profileID: "id with space"})
		if got[0]["webSocketDebuggerUrl"] != "ws://h:1/cdp/id%20with%20space/devtools/browser/b-1" {
			t.Fatalf("webSocketDebuggerUrl = %v", got[0]["webSocketDebuggerUrl"])
		}
	})

	t.Run("not json", func(t *testing.T) {
		if _, err := rewriteCDPDiscoveryBody([]byte("Target activated"), base); err == nil {
			t.Fatal("plain text should not be rewritten")
		}
	})
}

func rewriteForTest(t *testing.T, body string, base cdpPublicBase) []map[string]interface{} {
	t.Helper()
	out, err := rewriteCDPDiscoveryBody([]byte(body), base)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if strings.Contains(string(out), `\u0026`) {
		t.Fatalf("rewritten body should not HTML-escape: %s", out)
	}
	var items []map[string]interface{}
	if strings.HasPrefix(strings.TrimSpace(string(out)), "[") {
		err = json.Unmarshal(out, &items)
	} else {
		var item map[string]interface{}
		err = json.Unmarshal(out, &item)
		items = append(items, item)
	}
	if err != nil {
		t.Fatalf("decode rewritten body %s: %v", out, err)
	}
	return items
}

func TestCDPProfileProxyForwardsToProfileAndRewritesForRequestHost(t *testing.T) {
	fx := newCDPProfileFixture(t,
		&browser.Profile{ProfileId: "p-0", ProfileName: "bound", Running: true},
		&browser.Profile{ProfileId: "p-1", ProfileName: "shop-a", Running: true},
	)
	fx.server.SetActiveProfile(&browser.Profile{ProfileId: "p-0", DebugPort: 1, DebugReady: true})

	var version map[string]interface{}
	if status := fx.get(t, "/cdp/shop-a/json/version/", "browser.example:9000", &version); status != http.StatusOK {
		t.Fatalf("GET /json/version/ status = %d, body %v", status, version)
	}
	if got := version["webSocketDebuggerUrl"]; got != "ws://browser.example:9000/cdp/p-1/devtools/browser/b-1" {
		t.Fatalf("webSocketDebuggerUrl = %v", got)
	}
	host, path, key := fx.chrome.lastRequest()
	if host != fx.chrome.server.Listener.Addr().String() {
		t.Errorf("chrome saw Host %q, want its own loopback address", host)
	}
	if path != "/json/version/" {
		t.Errorf("chrome saw path %q, want /json/version/", path)
	}
	if key != "" {
		t.Errorf("api key leaked to chrome: %q", key)
	}

	var list []map[string]interface{}
	if status := fx.get(t, "/cdp/p-1/json/list", "browser.example:9000", &list); status != http.StatusOK {
		t.Fatalf("GET /json/list status = %d", status)
	}
	if len(list) != 2 || list[0]["devtoolsFrontendUrl"] != "/cdp/p-1/devtools/inspector.html?ws=browser.example:9000/cdp/p-1/devtools/page/page-1" {
		t.Fatalf("list = %v", list)
	}

	if id, _, _ := fx.server.ActiveProfile(); id != "p-0" {
		t.Fatalf("active profile = %q, want p-0 unchanged", id)
	}
	if n := fx.starter.starts.Load(); n != 0 {
		t.Fatalf("running profile should not be started, starts = %d", n)
	}
}

func TestCDPProfileProxyWebSocketThroughRewrittenURL(t *testing.T) {
	fx := newCDPProfileFixture(t, &browser.Profile{ProfileId: "p-1", ProfileName: "shop-a", Running: true})

	var version map[string]interface{}
	if status := fx.get(t, "/cdp/shop-a/json/version/", "", &version); status != http.StatusOK {
		t.Fatalf("GET /json/version/ status = %d", status)
	}
	wsURL, _ := version["webSocketDebuggerUrl"].(string)
	if !strings.HasPrefix(wsURL, "ws://"+fx.front.Listener.Addr().String()+"/cdp/p-1/") {
		t.Fatalf("webSocketDebuggerUrl = %q", wsURL)
	}

	if status := dialWS(t, wsURL, nil); status != http.StatusUnauthorized {
		t.Fatalf("upgrade without key: status = %d, want 401", status)
	}
	if status := dialWS(t, wsURL, keyHeader(testAPIKey)); status != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade with key: status = %d, want 101", status)
	}
}

func TestCDPProfileProxyRequiresAPIKeyBeforeLaunching(t *testing.T) {
	fx := newCDPProfileFixture(t, &browser.Profile{ProfileId: "p-1", ProfileName: "shop-a"})

	req, _ := http.NewRequest(http.MethodGet, fx.front.URL+"/cdp/shop-a/json/version", nil)
	if status := doJSON(t, req, nil); status != http.StatusUnauthorized {
		t.Fatalf("status without key = %d, want 401", status)
	}
	req, _ = http.NewRequest(http.MethodGet, fx.front.URL+"/cdp/shop-a/json/version", nil)
	req.Header.Set(DefaultAPIKeyHeader, "nope")
	if status := doJSON(t, req, nil); status != http.StatusUnauthorized {
		t.Fatalf("status with wrong key = %d, want 401", status)
	}
	if n := fx.starter.starts.Load(); n != 0 {
		t.Fatalf("unauthenticated request started the profile, starts = %d", n)
	}

	fx.server.SetAPIAuthConfig(APIAuthConfig{})
	req, _ = http.NewRequest(http.MethodGet, fx.front.URL+"/cdp/shop-a/json/version", nil)
	if status := doJSON(t, req, nil); status != http.StatusOK {
		t.Fatalf("status with auth off = %d, want 200", status)
	}
}

func TestCDPProfileProxyResolvesByIDAndRejectsAmbiguousName(t *testing.T) {
	fx := newCDPProfileFixture(t,
		&browser.Profile{ProfileId: "p-1", ProfileName: "dup", Running: true},
		&browser.Profile{ProfileId: "p-2", ProfileName: "dup", Running: true},
	)

	var body map[string]interface{}
	if status := fx.get(t, "/cdp/dup/json/version", "", &body); status != http.StatusConflict {
		t.Fatalf("ambiguous name status = %d, want 409", status)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "profile id") {
		t.Fatalf("error should point to the profile id, got %q", msg)
	}
	if candidates, _ := body["candidates"].([]interface{}); len(candidates) != 2 {
		t.Fatalf("candidates = %v", body["candidates"])
	}

	if status := fx.get(t, "/cdp/p-2/json/version", "", nil); status != http.StatusOK {
		t.Fatalf("lookup by id status = %d, want 200", status)
	}
	if status := fx.get(t, "/cdp/missing/json/version", "", nil); status != http.StatusNotFound {
		t.Fatalf("unknown profile status = %d, want 404", status)
	}
	if status := fx.get(t, "/cdp/", "", nil); status != http.StatusBadRequest {
		t.Fatalf("missing profile status = %d, want 400", status)
	}
}

func TestCDPProfileProxyLaunchesStoppedProfileOnce(t *testing.T) {
	fx := newCDPProfileFixture(t,
		&browser.Profile{ProfileId: "p-0", ProfileName: "bound", Running: true},
		&browser.Profile{ProfileId: "p-1", ProfileName: "shop-a"},
	)
	fx.starter.delay = 150 * time.Millisecond
	fx.server.cdpProfiles.grace = 100 * time.Millisecond
	fx.server.SetActiveProfile(&browser.Profile{ProfileId: "p-0", DebugPort: 1, DebugReady: true})

	const workers = 8
	var wg sync.WaitGroup
	statuses := make([]int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, fx.front.URL+"/cdp/shop-a/json/version/", nil)
			req.Header.Set(DefaultAPIKeyHeader, testAPIKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				statuses[i] = -1
				return
			}
			resp.Body.Close()
			statuses[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, status)
		}
	}
	if n := fx.starter.starts.Load(); n != 1 {
		t.Fatalf("starts = %d, want 1", n)
	}
	if id, _, _ := fx.server.ActiveProfile(); id != "p-0" {
		t.Fatalf("active profile = %q, want p-0 unchanged", id)
	}

	if status := fx.get(t, "/cdp/shop-a/json/version/", "", nil); status != http.StatusOK || fx.starter.starts.Load() != 1 {
		t.Fatalf("follow-up request: status %d, starts %d; want 200 and no new start", status, fx.starter.starts.Load())
	}

	// 挡切换只针对 /cdp 拉起过程，过了宽限期显式切换照常生效
	time.Sleep(150 * time.Millisecond)
	fx.server.SetActiveProfile(&browser.Profile{ProfileId: "p-1", DebugPort: 2, DebugReady: true})
	if id, _, _ := fx.server.ActiveProfile(); id != "p-1" {
		t.Fatalf("explicit activation after grace: active = %q, want p-1", id)
	}

	logs := fx.server.listLaunchLogs(10)
	if len(logs) != 1 || !logs[0].OK || logs[0].ProfileID != "p-1" {
		t.Fatalf("launch logs = %+v, want one successful entry for p-1", logs)
	}
}

func TestCDPProfileProxyLaunchTimeout(t *testing.T) {
	fx := newCDPProfileFixture(t, &browser.Profile{ProfileId: "p-1", ProfileName: "shop-a"})
	fx.starter.neverReady = true
	fx.server.cdpProfiles.timeout = 300 * time.Millisecond

	var body map[string]interface{}
	if status := fx.get(t, "/cdp/shop-a/json/version", "", &body); status != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504 (body %v)", status, body)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "not ready") {
		t.Fatalf("error = %q, want a not-ready message", msg)
	}
}

func TestCDPProfileProxyLaunchFailure(t *testing.T) {
	fx := newCDPProfileFixture(t, &browser.Profile{ProfileId: "p-1", ProfileName: "shop-a"})
	fx.starter.startErr = errors.New("proxy bridge failed")

	var body map[string]interface{}
	if status := fx.get(t, "/cdp/shop-a/json/version", "", &body); status != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", status)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "failed to start profile") || !strings.Contains(msg, "proxy bridge failed") {
		t.Fatalf("error = %q", msg)
	}
}

func TestCDPActivationHoldIsPerProfile(t *testing.T) {
	routes := &cdpProfileRoutes{grace: 50 * time.Millisecond}

	release := routes.holdActivation("p-1")
	if !routes.activationHeld("p-1") || routes.activationHeld("p-2") {
		t.Fatal("hold must only cover p-1")
	}
	release(false)
	if routes.activationHeld("p-1") {
		t.Fatal("hold released without grace should end immediately")
	}

	release = routes.holdActivation("p-1")
	release(true)
	if !routes.activationHeld("p-1") {
		t.Fatal("hold released with grace should last for the grace period")
	}
	time.Sleep(80 * time.Millisecond)
	if routes.activationHeld("p-1") {
		t.Fatal("hold should end after the grace period")
	}
}

// runtime session 抽出等待逻辑后，仍然要在就绪时切换统一入口。
func TestPrepareRuntimeSessionStillActivatesProfile(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	profile, ready, err := server.prepareRuntimeSession(&browser.Profile{ProfileId: "p-1", DebugPort: 9222, DebugReady: true, Running: true}, time.Second)
	if err != nil || !ready || profile == nil {
		t.Fatalf("prepareRuntimeSession = (%v, %v, %v)", profile, ready, err)
	}
	if id, _, port := server.ActiveProfile(); id != "p-1" || port != 9222 {
		t.Fatalf("active = (%q, %d), want (p-1, 9222)", id, port)
	}
}
