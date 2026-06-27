package launchcode_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

type managerBackedStarter struct {
	mgr        *browser.Manager
	started    []string
	lastParams launchcode.LaunchRequestParams
}

func (m *managerBackedStarter) StartInstance(profileID string) (*browser.Profile, error) {
	profile, ok := m.mgr.Profiles[profileID]
	if !ok || profile == nil {
		return nil, fmt.Errorf("profile not found: %s", profileID)
	}

	m.started = append(m.started, profileID)
	profile.Running = true
	profile.Pid = 4000 + len(m.started)
	profile.DebugPort = 9300 + len(m.started)
	profile.LastStartAt = time.Now().Format(time.RFC3339)
	return profile, nil
}

func (m *managerBackedStarter) StartInstanceWithParams(profileID string, params launchcode.LaunchRequestParams) (*browser.Profile, error) {
	m.lastParams = params
	return m.StartInstance(profileID)
}

func newProfileCreateTestManager(t *testing.T, configure func(*config.Config)) *browser.Manager {
	t.Helper()

	cfg := config.DefaultConfig()
	if configure != nil {
		configure(cfg)
	}
	return browser.NewManager(cfg, t.TempDir())
}

func TestCreateProfileAPIStoresProxyAndMetadata(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, func(cfg *config.Config) {
		cfg.Browser.Proxies = []config.BrowserProxy{
			{
				ProxyId:     "proxy-us",
				ProxyName:   "US Residential",
				ProxyConfig: "socks5://127.0.0.1:1080",
			},
		}
	})
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	payload := bytes.NewBufferString(`{
		"profile": {
			"profileName": "buyer-001",
			"userDataDir": "buyers/buyer-001",
			"proxyId": "proxy-us",
			"launchArgs": ["--lang=en-US"],
			"tags": ["电商", "北美"],
			"keywords": ["buyer-001", "amazon"],
			"groupId": "group-sales-us"
		},
		"launchCode": "buyer_001"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望 201，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK         bool             `json:"ok"`
		Created    bool             `json:"created"`
		Launched   bool             `json:"launched"`
		ProfileID  string           `json:"profileId"`
		LaunchCode string           `json:"launchCode"`
		Profile    *browser.Profile `json:"profile"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.OK || !resp.Created || resp.Launched {
		t.Fatalf("响应状态错误: %+v", resp)
	}
	if resp.Profile == nil {
		t.Fatalf("响应缺少 profile: %+v", resp)
	}
	if resp.LaunchCode != "BUYER_001" {
		t.Fatalf("launchCode 未归一化: %s", resp.LaunchCode)
	}
	if resp.Profile.ProxyId != "proxy-us" {
		t.Fatalf("proxyId 不正确: %+v", resp.Profile)
	}
	if resp.Profile.ProxyConfig != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxyConfig 未按代理池解析: %+v", resp.Profile)
	}
	if resp.Profile.GroupId != "group-sales-us" {
		t.Fatalf("groupId 不正确: %+v", resp.Profile)
	}
	if len(resp.Profile.Tags) != 2 || len(resp.Profile.Keywords) != 2 {
		t.Fatalf("tags/keywords 不正确: %+v", resp.Profile)
	}

	resolvedProfileID, err := svc.Resolve("BUYER_001")
	if err != nil {
		t.Fatalf("launchCode 未写入服务: %v", err)
	}
	if resolvedProfileID != resp.ProfileID {
		t.Fatalf("launchCode 绑定的 profileId 错误: got=%s want=%s", resolvedProfileID, resp.ProfileID)
	}
}

func TestCreateProfileAPIAutoLaunchPassesStartParams(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	payload := bytes.NewBufferString(`{
		"profile": {
			"profileName": "buyer-002",
			"proxyConfig": "http://user:pass@127.0.0.1:8080",
			"launchArgs": ["--disable-sync"],
			"keywords": ["buyer-002"]
		},
		"autoLaunch": true,
		"start": {
			"launchArgs": ["--window-size=1280,800", "--lang=en-US"],
			"startUrls": ["https://example.com/order"],
			"skipDefaultStartUrls": true
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望 201，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(starter.started) != 1 {
		t.Fatalf("应自动启动 1 次，实际 %+v", starter.started)
	}
	if len(starter.lastParams.LaunchArgs) != 2 {
		t.Fatalf("一次性 launchArgs 未透传: %+v", starter.lastParams)
	}
	if len(starter.lastParams.StartURLs) != 1 || starter.lastParams.StartURLs[0] != "https://example.com/order" {
		t.Fatalf("startUrls 未透传: %+v", starter.lastParams)
	}
	if !starter.lastParams.SkipDefaultStartURLs {
		t.Fatalf("skipDefaultStartUrls 未透传: %+v", starter.lastParams)
	}

	var resp struct {
		OK        bool             `json:"ok"`
		Created   bool             `json:"created"`
		Launched  bool             `json:"launched"`
		CDPURL    string           `json:"cdpUrl"`
		DebugPort int              `json:"debugPort"`
		Profile   *browser.Profile `json:"profile"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.OK || !resp.Created || !resp.Launched {
		t.Fatalf("响应状态错误: %+v", resp)
	}
	if resp.Profile == nil || !resp.Profile.Running {
		t.Fatalf("自动启动后的 profile 状态错误: %+v", resp)
	}
	if resp.DebugPort == 0 || resp.CDPURL == "" {
		t.Fatalf("缺少调试端口/CDP 地址: %+v", resp)
	}
	if resp.Profile.ProxyConfig != "http://user:pass@127.0.0.1:8080" {
		t.Fatalf("直连代理配置未保存: %+v", resp.Profile)
	}
}

func TestCreateProfileAPIRejectsMissingProfile(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", bytes.NewBufferString(`{"launchCode":"buyer_003"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestCreateProfileAPIRollsBackOnDuplicateLaunchCode(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	existing, err := mgr.Create(browser.ProfileInput{ProfileName: "existing"})
	if err != nil {
		t.Fatalf("预创建窗口失败: %v", err)
	}
	if _, err := svc.SetCode(existing.ProfileId, "BUYER_DUP"); err != nil {
		t.Fatalf("预设 launchCode 失败: %v", err)
	}

	beforeCount := len(mgr.List())

	payload := bytes.NewBufferString(`{
		"profile": {
			"profileName": "new-buyer",
			"keywords": ["new-buyer"]
		},
		"launchCode": "BUYER_DUP"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/profiles", payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("期望 409，实际 %d，body=%s", w.Code, w.Body.String())
	}

	afterCount := len(mgr.List())
	if afterCount != beforeCount {
		t.Fatalf("launchCode 冲突后应回滚创建: before=%d after=%d", beforeCount, afterCount)
	}
}
