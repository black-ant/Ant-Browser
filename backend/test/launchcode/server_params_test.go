package launchcode_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
)

type mockStarterWithParams struct {
	profiles    map[string]*browser.Profile
	lastProfile string
	started     []string
	lastParams  launchcode.LaunchRequestParams
}

func newMockStarterWithParams() *mockStarterWithParams {
	return &mockStarterWithParams{profiles: make(map[string]*browser.Profile)}
}

func (m *mockStarterWithParams) addProfile(p *browser.Profile) {
	m.profiles[p.ProfileId] = p
}

func (m *mockStarterWithParams) StartInstance(profileId string) (*browser.Profile, error) {
	m.lastProfile = profileId
	m.started = append(m.started, profileId)
	p, ok := m.profiles[profileId]
	if !ok {
		return nil, http.ErrMissingFile
	}
	return p, nil
}

func (m *mockStarterWithParams) StartInstanceWithParams(profileId string, params launchcode.LaunchRequestParams) (*browser.Profile, error) {
	m.lastProfile = profileId
	m.started = append(m.started, profileId)
	m.lastParams = params
	p, ok := m.profiles[profileId]
	if !ok {
		return nil, http.ErrMissingFile
	}
	return p, nil
}

func TestLaunchWithParams(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarterWithParams()
	starter.addProfile(&browser.Profile{
		ProfileId:   "profile-automation",
		ProfileName: "automation",
		Pid:         321,
		DebugPort:   9555,
	})

	code, err := svc.EnsureCode("profile-automation")
	if err != nil {
		t.Fatalf("EnsureCode 失败: %v", err)
	}

	handler := buildTestHandler(svc, starter)
	body := map[string]interface{}{
		"code":                 code,
		"launchArgs":           []string{"--window-size=1280,800", "--lang=en-US"},
		"startUrls":            []string{"https://example.com"},
		"skipDefaultStartUrls": true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastProfile != "profile-automation" {
		t.Fatalf("profileId 传递错误: %s", starter.lastProfile)
	}
	if len(starter.lastParams.LaunchArgs) != 2 {
		t.Fatalf("launchArgs 传递错误: %+v", starter.lastParams.LaunchArgs)
	}
	if len(starter.lastParams.StartURLs) != 1 || starter.lastParams.StartURLs[0] != "https://example.com" {
		t.Fatalf("startUrls 传递错误: %+v", starter.lastParams.StartURLs)
	}
	if !starter.lastParams.SkipDefaultStartURLs {
		t.Fatal("skipDefaultStartUrls 传递错误")
	}
}

func TestLaunchWithParamsUsingCodeAsKeywordFallback(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarterWithParams()
	profile := &browser.Profile{
		ProfileId:   "profile-automation-keyword-fallback",
		ProfileName: "automation-keyword-fallback",
		Keywords:    []string{"buyer-001", "amazon"},
		Pid:         654,
		DebugPort:   9666,
	}
	starter.addProfile(profile)

	manager := newSelectorTestManager(profile)
	handler := buildTestHandlerWithManager(svc, starter, manager)
	body := map[string]interface{}{
		"code":                 "buyer-001",
		"launchArgs":           []string{"--window-size=1280,800", "--lang=en-US"},
		"startUrls":            []string{"https://example.com"},
		"skipDefaultStartUrls": true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastProfile != profile.ProfileId {
		t.Fatalf("code 关键字兜底命中窗口错误: got=%s want=%s", starter.lastProfile, profile.ProfileId)
	}
	if len(starter.lastParams.LaunchArgs) != 2 {
		t.Fatalf("launchArgs 传递错误: %+v", starter.lastParams.LaunchArgs)
	}
	if len(starter.lastParams.StartURLs) != 1 || starter.lastParams.StartURLs[0] != "https://example.com" {
		t.Fatalf("startUrls 传递错误: %+v", starter.lastParams.StartURLs)
	}
	if !starter.lastParams.SkipDefaultStartURLs {
		t.Fatal("skipDefaultStartUrls 传递错误")
	}
}

func TestLaunchWithParamsUsingCodeAsKeywordFallbackPrefersExactKeywordMatch(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarterWithParams()
	profileFuzzy := &browser.Profile{
		ProfileId:   "profile-params-code-fuzzy",
		ProfileName: "automation-fuzzy",
		Keywords:    []string{"buyer-001-old", "amazon"},
		Pid:         655,
		DebugPort:   9667,
	}
	profileExact := &browser.Profile{
		ProfileId:   "profile-params-code-exact",
		ProfileName: "automation-exact",
		Keywords:    []string{"buyer-001", "amazon"},
		Pid:         656,
		DebugPort:   9668,
	}
	starter.addProfile(profileFuzzy)
	starter.addProfile(profileExact)

	manager := newSelectorTestManager(profileFuzzy, profileExact)
	handler := buildTestHandlerWithManager(svc, starter, manager)
	body := map[string]interface{}{
		"code":                 "buyer-001",
		"launchArgs":           []string{"--window-size=1280,800", "--lang=en-US"},
		"startUrls":            []string{"https://example.com"},
		"skipDefaultStartUrls": true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if starter.lastProfile != profileExact.ProfileId {
		t.Fatalf("code 关键字兜底应优先命中精确关键字窗口: got=%s want=%s", starter.lastProfile, profileExact.ProfileId)
	}
	if len(starter.lastParams.LaunchArgs) != 2 {
		t.Fatalf("launchArgs 传递错误: %+v", starter.lastParams.LaunchArgs)
	}
	if len(starter.lastParams.StartURLs) != 1 || starter.lastParams.StartURLs[0] != "https://example.com" {
		t.Fatalf("startUrls 传递错误: %+v", starter.lastParams.StartURLs)
	}
	if !starter.lastParams.SkipDefaultStartURLs {
		t.Fatal("skipDefaultStartUrls 传递错误")
	}
}

func TestLaunchWithParamsBadRequest(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarterWithParams()
	handler := buildTestHandler(svc, starter)

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewBufferString("{bad json}"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d", w.Code)
		}
	})

	t.Run("missing-code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewBufferString(`{"launchArgs":["--incognito"]}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("期望 400，实际 %d", w.Code)
		}
	})
}

func TestLaunchLogsEndpoint(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarterWithParams()
	starter.addProfile(&browser.Profile{
		ProfileId:   "profile-log-test",
		ProfileName: "log-test",
		Pid:         456,
		DebugPort:   9666,
	})

	code, err := svc.EnsureCode("profile-log-test")
	if err != nil {
		t.Fatalf("EnsureCode 失败: %v", err)
	}

	handler := buildTestHandler(svc, starter)
	payload := bytes.NewBufferString(`{"code":"` + code + `","launchArgs":["--incognito"]}`)

	reqLaunch := httptest.NewRequest(http.MethodPost, "/api/launch", payload)
	reqLaunch.Header.Set("Content-Type", "application/json")
	wLaunch := httptest.NewRecorder()
	handler.ServeHTTP(wLaunch, reqLaunch)
	if wLaunch.Code != http.StatusOK {
		t.Fatalf("调用 launch 失败: %d", wLaunch.Code)
	}

	reqLogs := httptest.NewRequest(http.MethodGet, "/api/launch/logs?limit=10", nil)
	wLogs := httptest.NewRecorder()
	handler.ServeHTTP(wLogs, reqLogs)
	if wLogs.Code != http.StatusOK {
		t.Fatalf("查询 logs 失败: %d", wLogs.Code)
	}

	var resp struct {
		OK    bool                          `json:"ok"`
		Items []launchcode.LaunchCallRecord `json:"items"`
	}
	if err := json.NewDecoder(wLogs.Body).Decode(&resp); err != nil {
		t.Fatalf("解析 logs 响应失败: %v", err)
	}
	if !resp.OK {
		t.Fatal("logs 响应 ok=false")
	}
	if len(resp.Items) == 0 {
		t.Fatal("logs 为空，期望至少一条记录")
	}
	if resp.Items[0].Path != "/api/launch" {
		t.Fatalf("最新记录 path 不正确: %s", resp.Items[0].Path)
	}
}
