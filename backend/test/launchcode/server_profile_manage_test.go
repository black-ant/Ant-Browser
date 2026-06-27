package launchcode_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestListProfilesAPIIncludesLaunchCodes(t *testing.T) {
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

	first, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-a",
		ProxyId:     "proxy-us",
		Tags:        []string{"电商"},
		Keywords:    []string{"buyer-a"},
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	second, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-b",
		ProxyConfig: "http://127.0.0.1:8080",
		Keywords:    []string{"buyer-b"},
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	if _, err := svc.SetCode(first.ProfileId, "BUYER_A"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK    bool              `json:"ok"`
		Count int               `json:"count"`
		Items []browser.Profile `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Count != 2 || len(resp.Items) != 2 {
		t.Fatalf("列表响应错误: %+v", resp)
	}

	seen := make(map[string]browser.Profile, len(resp.Items))
	for _, item := range resp.Items {
		if item.LaunchCode == "" {
			t.Fatalf("列表应返回 launchCode: %+v", resp.Items)
		}
		seen[item.ProfileId] = item
	}
	if _, ok := seen[first.ProfileId]; !ok {
		t.Fatalf("列表缺少第一个窗口: %+v", resp.Items)
	}
	if _, ok := seen[second.ProfileId]; !ok {
		t.Fatalf("列表缺少第二个窗口: %+v", resp.Items)
	}
}

func TestGetProfileAPIReturnsProfileByID(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	profile, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-get",
		ProxyConfig: "http://127.0.0.1:8080",
		Tags:        []string{"北美"},
		Keywords:    []string{"buyer-get"},
		GroupId:     "group-get",
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	if _, err := svc.SetCode(profile.ProfileId, "BUYER_GET"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/profiles/"+profile.ProfileId, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK         bool             `json:"ok"`
		LaunchCode string           `json:"launchCode"`
		Profile    *browser.Profile `json:"profile"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.OK || resp.Profile == nil {
		t.Fatalf("查询响应错误: %+v", resp)
	}
	if resp.LaunchCode != "BUYER_GET" || resp.Profile.GroupId != "group-get" {
		t.Fatalf("查询字段错误: %+v", resp)
	}
}

func TestUpdateProfileAPIUpdatesFieldsAndAutoLaunches(t *testing.T) {
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

	profile, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-old",
		ProxyConfig: "http://127.0.0.1:8080",
		Keywords:    []string{"buyer-old"},
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	if _, err := svc.SetCode(profile.ProfileId, "BUYER_OLD"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}

	payload := bytes.NewBufferString(`{
		"profile": {
			"profileName": "buyer-new",
			"userDataDir": "buyers/buyer-new",
			"proxyId": "proxy-us",
			"launchArgs": ["--lang=en-US"],
			"tags": ["电商", "北美"],
			"keywords": ["buyer-new", "amazon"],
			"groupId": "group-sales-us"
		},
		"launchCode": "BUYER_NEW",
		"autoLaunch": true,
		"start": {
			"launchArgs": ["--window-size=1280,800"],
			"startUrls": ["https://example.com/order"],
			"skipDefaultStartUrls": true
		}
	}`)

	req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+profile.ProfileId, payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(starter.started) != 1 {
		t.Fatalf("更新后应自动启动 1 次，实际 %+v", starter.started)
	}
	if len(starter.lastParams.StartURLs) != 1 || starter.lastParams.StartURLs[0] != "https://example.com/order" {
		t.Fatalf("startUrls 未透传: %+v", starter.lastParams)
	}

	updated, status, errMsg := handlerProfileSnapshot(t, mgr, svc, profile.ProfileId)
	if errMsg != "" || status != http.StatusOK {
		t.Fatalf("读取更新后窗口失败: status=%d err=%s", status, errMsg)
	}
	if updated.ProfileName != "buyer-new" || updated.ProxyId != "proxy-us" || updated.ProxyConfig != "socks5://127.0.0.1:1080" {
		t.Fatalf("更新未生效: %+v", updated)
	}
	if updated.GroupId != "group-sales-us" || updated.LaunchCode != "BUYER_NEW" || !updated.Running {
		t.Fatalf("更新后的分组/launchCode/运行状态错误: %+v", updated)
	}
}

func TestUpdateProfileAPIRollsBackOnDuplicateLaunchCode(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	first, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-first",
		ProxyConfig: "http://127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	second, err := mgr.Create(browser.ProfileInput{
		ProfileName: "buyer-second",
		ProxyConfig: "http://127.0.0.1:9090",
	})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	if _, err := svc.SetCode(first.ProfileId, "BUYER_FIRST"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}
	if _, err := svc.SetCode(second.ProfileId, "BUYER_SECOND"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}

	payload := bytes.NewBufferString(`{
		"profile": {
			"profileName": "buyer-first-updated",
			"proxyConfig": "http://127.0.0.1:10080",
			"keywords": ["buyer-first-updated"]
		},
		"launchCode": "BUYER_SECOND"
	}`)

	req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+first.ProfileId, payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("期望 409，实际 %d，body=%s", w.Code, w.Body.String())
	}

	current, status, errMsg := handlerProfileSnapshot(t, mgr, svc, first.ProfileId)
	if errMsg != "" || status != http.StatusOK {
		t.Fatalf("读取回滚后窗口失败: status=%d err=%s", status, errMsg)
	}
	if current.ProfileName != "buyer-first" || current.ProxyConfig != "http://127.0.0.1:8080" || current.LaunchCode != "BUYER_FIRST" {
		t.Fatalf("launchCode 冲突后应回滚更新: %+v", current)
	}
}

func TestDeleteProfileAPIRemovesProfileAndLaunchCode(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	profile, err := mgr.Create(browser.ProfileInput{ProfileName: "buyer-delete"})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	if _, err := svc.SetCode(profile.ProfileId, "BUYER_DELETE"); err != nil {
		t.Fatalf("设置 launchCode 失败: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/profiles/"+profile.ProfileId, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if _, ok := mgr.Profiles[profile.ProfileId]; ok {
		t.Fatalf("窗口删除后仍存在于内存: %s", profile.ProfileId)
	}
	if _, err := svc.Resolve("BUYER_DELETE"); err == nil {
		t.Fatal("删除后 launchCode 仍可解析")
	}
}

func TestDeleteProfileAPIRejectsRunningProfile(t *testing.T) {
	svc := newInMemoryService()
	mgr := newProfileCreateTestManager(t, nil)
	starter := &managerBackedStarter{mgr: mgr}
	handler := buildTestHandlerWithManager(svc, starter, mgr)

	profile, err := mgr.Create(browser.ProfileInput{ProfileName: "buyer-running"})
	if err != nil {
		t.Fatalf("创建测试窗口失败: %v", err)
	}
	mgr.Profiles[profile.ProfileId].Running = true

	req := httptest.NewRequest(http.MethodDelete, "/api/profiles/"+profile.ProfileId, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("期望 409，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if _, ok := mgr.Profiles[profile.ProfileId]; !ok {
		t.Fatalf("运行中窗口不应被删除: %s", profile.ProfileId)
	}
}

func handlerProfileSnapshot(t *testing.T, mgr *browser.Manager, svc interface {
	EnsureCode(profileID string) (string, error)
}, profileID string) (*browser.Profile, int, string) {
	t.Helper()

	mgr.Mutex.Lock()
	profile, ok := mgr.Profiles[profileID]
	var snapshot browser.Profile
	if ok && profile != nil {
		snapshot = *profile
	}
	mgr.Mutex.Unlock()
	if !ok {
		return nil, http.StatusNotFound, "profile not found"
	}
	if snapshot.LaunchCode == "" {
		if code, err := svc.EnsureCode(snapshot.ProfileId); err == nil {
			snapshot.LaunchCode = code
		}
	}
	return &snapshot, http.StatusOK, ""
}
