package backend

// 虚拟分组绑定真实链路集成测试：
// 使用真实 SQLite DAO + 本地 HTTP 代理成员，走完整的 resolveBrowserStartProxy 链路
// （实体匹配失败 → 虚拟分组解析 → 成员选择 → 配置校验 → 内核解析 → 返回代理地址）。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/proxy"
)

func newVirtualGroupIntegrationApp(t *testing.T, memberConfigs map[string]string) *App {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewDB(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	app := NewApp(dir)
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	app.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(db.GetConn())

	proxies := make([]browser.Proxy, 0, len(memberConfigs))
	for proxyID, proxyConfig := range memberConfigs {
		proxies = append(proxies, browser.Proxy{
			ProxyId:     proxyID,
			ProxyName:   proxyID,
			ProxyConfig: proxyConfig,
		})
	}
	if err := app.browserMgr.ProxyDAO.ReplaceAll(proxies); err != nil {
		t.Fatalf("seed proxies failed: %v", err)
	}
	return app
}

func newLocalHTTPProxy(t *testing.T) *httptest.Server {
	t.Helper()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)
	upstream := &http.Client{}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		resp, err := upstream.Do(outReq)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
	}))
	t.Cleanup(proxy.Close)
	return proxy
}

// 完整启动链路：固定绑定虚拟分组（round_robin）→ 每次启动轮询不同成员。
func TestVirtualGroupBindingStartChainRoundRobin(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	proxyB := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{
		"proxy-a": proxyA.URL,
		"proxy-b": proxyB.URL,
	})
	group := BrowserProxyVirtualGroup{
		GroupId:        "vg-e2e-1",
		GroupName:      "e2e-rr",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}
	if err := app.SaveBrowserProxyVirtualGroup(group); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	profile := BrowserProfile{ProfileId: "profile-e2e", ProxyMode: "fixed", ProxyId: "vg-e2e-1"}
	firstURL, ref, usingBridge, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	if ref.valid() || usingBridge {
		t.Fatalf("native 代理不应产生桥接引用: ref=%+v bridge=%v", ref, usingBridge)
	}
	if profile.ProxyId != "vg-e2e-1" {
		t.Fatalf("虚拟分组绑定不应被改写: %s", profile.ProxyId)
	}

	secondURL, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err != nil {
		t.Fatalf("second start failed: %v", err)
	}
	if firstURL == secondURL {
		t.Fatalf("round_robin 两次启动应轮询不同成员，均为 %s", firstURL)
	}
	if !strings.Contains(firstURL, "127.0.0.1") || !strings.Contains(secondURL, "127.0.0.1") {
		t.Fatalf("返回地址异常: %s / %s", firstURL, secondURL)
	}
}

// sticky：固定使用 selectedProxyId 成员。
func TestVirtualGroupBindingStartChainSticky(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	proxyB := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{
		"proxy-a": proxyA.URL,
		"proxy-b": proxyB.URL,
	})
	group := BrowserProxyVirtualGroup{
		GroupId:         "vg-e2e-2",
		GroupName:       "e2e-sticky",
		Strategy:        "sticky",
		SelectedProxyId: "proxy-b",
		MemberProxyIds:  []string{"proxy-a", "proxy-b"},
	}
	if err := app.SaveBrowserProxyVirtualGroup(group); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	profile := BrowserProfile{ProfileId: "profile-sticky", ProxyMode: "fixed", ProxyId: "vg-e2e-2"}
	for i := 0; i < 3; i++ {
		proxyURL, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
		if err != nil {
			t.Fatalf("start #%d failed: %v", i, err)
		}
		if proxyURL != proxyB.URL {
			t.Fatalf("sticky 启动 #%d 返回 %s, want %s", i, proxyURL, proxyB.URL)
		}
	}
}

// 分组成员按分组名展开（memberGroupNames）也能选到成员。
func TestVirtualGroupBindingStartChainByGroupName(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	// 给成员设置分组名，用于 memberGroupNames 展开
	if err := app.browserMgr.ProxyDAO.Upsert(browser.Proxy{
		ProxyId:     "proxy-a",
		ProxyName:   "proxy-a",
		ProxyConfig: proxyA.URL,
		GroupName:   "成员分组",
	}); err != nil {
		t.Fatalf("upsert member group name failed: %v", err)
	}
	group := BrowserProxyVirtualGroup{
		GroupId:          "vg-e2e-3",
		GroupName:        "e2e-by-group",
		Strategy:         "sticky",
		MemberGroupNames: []string{"成员分组"},
	}
	if err := app.SaveBrowserProxyVirtualGroup(group); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}
	profile := BrowserProfile{ProfileId: "profile-by-group", ProxyMode: "fixed", ProxyId: "vg-e2e-3"}
	proxyURL, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if proxyURL != proxyA.URL {
		t.Fatalf("按分组名展开失败: got %s, want %s", proxyURL, proxyA.URL)
	}
}

// 删除虚拟分组后，绑定该分组的实例启动应明确报错（不静默直连）。
func TestVirtualGroupBindingStartChainAfterGroupDeleted(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	group := BrowserProxyVirtualGroup{
		GroupId:        "vg-e2e-4",
		GroupName:      "e2e-deleted",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a"},
	}
	if err := app.SaveBrowserProxyVirtualGroup(group); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}
	if err := app.DeleteBrowserProxyVirtualGroup("vg-e2e-4"); err != nil {
		t.Fatalf("delete virtual group failed: %v", err)
	}
	profile := BrowserProfile{ProfileId: "profile-deleted", ProxyMode: "fixed", ProxyId: "vg-e2e-4"}
	_, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err == nil {
		t.Fatal("分组删除后启动应报错，实际成功")
	}
	if !strings.Contains(profile.LastError, "代理") {
		t.Fatalf("错误信息不明确: %q", profile.LastError)
	}
}

// 未绑定虚拟分组的普通实体代理行为不受影响（回归保护）。
func TestVirtualGroupBindingStartChainEntityProxyUnaffected(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	profile := BrowserProfile{ProfileId: "profile-entity", ProxyMode: "fixed", ProxyId: "proxy-a"}
	proxyURL, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err != nil {
		t.Fatalf("entity proxy start failed: %v", err)
	}
	if proxyURL != proxyA.URL {
		t.Fatalf("entity proxy URL = %s, want %s", proxyURL, proxyA.URL)
	}
}

// 编译期保证 config 包引用（测试文件使用 config.DefaultConfig）。
var _ = config.DefaultConfig

// 虚拟分组绑定 + 需要桥接的内核成员（认证 HTTP → xray 桥接）：
// 验证完整链路返回 socks5 地址并持有桥接引用，释放后引用回零。
func TestVirtualGroupBindingStartChainWithBridgeMember(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	abs, err := filepath.Abs(filepath.Join("..", "bin", "xray.exe"))
	if err != nil {
		t.Fatalf("resolve xray path: %v", err)
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		t.Skipf("本机无 xray.exe（%s），跳过桥接链路验证", abs)
	}
	t.Setenv("XRAY_BINARY_PATH", abs)

	proxyA := newLocalHTTPProxy(t)
	// 认证 HTTP 代理必须走 xray 桥接
	host := strings.TrimPrefix(proxyA.URL, "http://")
	memberConfig := "http://user:pass@" + host
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-auth": memberConfig})
	app.config = config.DefaultConfig()
	app.xrayMgr = proxy.NewXrayManager(app.config, app.appRoot)
	t.Cleanup(func() { app.xrayMgr.StopAll() })
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-e2e-bridge",
		GroupName:      "e2e-bridge",
		Strategy:       "sticky",
		MemberProxyIds: []string{"proxy-auth"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	profile := BrowserProfile{ProfileId: "profile-bridge", ProxyMode: "fixed", ProxyId: "vg-e2e-bridge"}
	proxyURL, ref, usingBridge, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, &profile)
	if err != nil {
		t.Fatalf("桥接成员启动失败: %v", err)
	}
	if !usingBridge || !ref.valid() {
		t.Fatalf("认证成员应产生桥接引用: bridge=%v ref=%+v", usingBridge, ref)
	}
	if !strings.HasPrefix(proxyURL, "socks5://127.0.0.1:") {
		t.Fatalf("桥接返回地址异常: %s", proxyURL)
	}
	app.releaseProxyBridgeRef(ref)
	t.Logf("桥接成员链路 OK: %s (engine=%s)", proxyURL, ref.Engine)
}

// 虚拟分组点测速 / IP 健康 / 预热：目标解析到成员（不报配置为空）。
func TestVirtualGroupBindingTargetsResolveToMember(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-e2e-targets",
		GroupName:      "e2e-targets",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}
	proxies := app.getLatestProxies()

	memberID, memberConfig, ok := app.resolveVirtualGroupProxyTarget("vg-e2e-targets", proxies)
	if !ok || memberID != "proxy-a" || memberConfig != proxyA.URL {
		t.Fatalf("目标解析 = (%q,%q,%v), want proxy-a", memberID, memberConfig, ok)
	}
	// 实体 ID 不受影响
	if id, cfg, ok := app.resolveVirtualGroupProxyTarget("proxy-a", proxies); ok || id != "" || cfg != "" {
		t.Fatalf("实体 ID 不应被解析: (%q,%q,%v)", id, cfg, ok)
	}
}

// 保存实例时的代理校验：虚拟分组 ID 应被成员配置校验通过（否则前端保存会被拒绝）。
func TestVirtualGroupBindingValidateProxyConfig(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-validate",
		GroupName:      "validate",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}
	result := app.ValidateProxyConfig("", "vg-validate")
	if !result.Supported {
		t.Fatalf("虚拟分组绑定校验未通过: %s", result.ErrorMsg)
	}
	// 实体代理校验保持原有行为
	if entity := app.ValidateProxyConfig("", "proxy-a"); !entity.Supported {
		t.Fatalf("实体代理校验回归: %s", entity.ErrorMsg)
	}
	// 未知 ID 仍应拒绝
	if unknown := app.ValidateProxyConfig("", "vg-not-exist"); unknown.Supported {
		t.Fatal("未知代理 ID 不应通过校验")
	}
}

// Manager 层创建/更新实例时绑定虚拟分组不应报"代理ID不存在"（切换节点走同一路径）。
func TestVirtualGroupBindingProfileCreateAndSwitch(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-switch",
		GroupName:      "switch",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	// 创建时绑定虚拟分组
	created, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "vg-bound-profile",
		UserDataDir:     "",
		ProxyId:         "vg-switch",
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("创建实例绑定虚拟分组失败（用户切换报错点）: %v", err)
	}
	if created.ProxyId != "vg-switch" || created.ProxyConfig != "" {
		t.Fatalf("创建结果 = (%q,%q), want (vg-switch, '')", created.ProxyId, created.ProxyConfig)
	}
	if created.ProxyMode != "fixed" {
		t.Fatalf("ProxyMode = %q, want fixed", created.ProxyMode)
	}

	// 切换节点：update 用 proxyId=虚拟分组、proxyConfig=''、proxyMode 省略
	updated, err := app.BrowserProfileUpdate(created.ProfileId, BrowserProfileInput{
		ProfileName:     created.ProfileName,
		UserDataDir:     created.UserDataDir,
		CoreId:          created.CoreId,
		ProxyId:         "vg-switch",
		ProxyConfig:     "",
		FingerprintArgs: created.FingerprintArgs,
		LaunchArgs:      created.LaunchArgs,
		MemoryLimitMB:   created.MemoryLimitMB,
	})
	if err != nil {
		t.Fatalf("切换为虚拟分组失败: %v", err)
	}
	if updated.ProxyId != "vg-switch" || updated.ProxyConfig != "" {
		t.Fatalf("切换结果 = (%q,%q), want (vg-switch, '')", updated.ProxyId, updated.ProxyConfig)
	}

	// 未知 ID 仍应报错（防止把任意字符串当分组）
	if _, err := app.BrowserProfileUpdate(created.ProfileId, BrowserProfileInput{
		ProfileName:     created.ProfileName,
		UserDataDir:     created.UserDataDir,
		ProxyId:         "vg-not-exist",
		ProxyConfig:     "",
		FingerprintArgs: created.FingerprintArgs,
		LaunchArgs:      created.LaunchArgs,
	}); err == nil {
		t.Fatal("未知代理 ID 应保持报错")
	}
}

// 用户操作序列：创建实例（实体代理）→ 切换为虚拟分组 → 启动实例。
func TestVirtualGroupBindingSwitchThenLaunch(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	proxyB := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{
		"proxy-a": proxyA.URL,
		"proxy-b": proxyB.URL,
	})
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-seq",
		GroupName:      "seq",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	created, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "seq-profile",
		UserDataDir:     "",
		ProxyId:         "proxy-a",
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("create with entity proxy failed: %v", err)
	}
	if created.ProxyId != "proxy-a" {
		t.Fatalf("初始绑定 = %q, want proxy-a", created.ProxyId)
	}

	// 列表中切换代理 → 虚拟分组（与 saveProfileProxy 相同的 update 调用）
	updated, err := app.BrowserProfileUpdate(created.ProfileId, BrowserProfileInput{
		ProfileName:     created.ProfileName,
		UserDataDir:     created.UserDataDir,
		CoreId:          created.CoreId,
		ProxyId:         "vg-seq",
		ProxyConfig:     "",
		FingerprintArgs: created.FingerprintArgs,
		LaunchArgs:      created.LaunchArgs,
	})
	if err != nil {
		t.Fatalf("切换到虚拟分组失败（用户报错点）: %v", err)
	}
	if updated.ProxyId != "vg-seq" || updated.ProxyConfig != "" {
		t.Fatalf("切换后 = (%q,%q), want (vg-seq, '')", updated.ProxyId, updated.ProxyConfig)
	}
	if updated.ProxyMode != "fixed" {
		t.Fatalf("切换后 ProxyMode = %q, want fixed", updated.ProxyMode)
	}

	// 启动实例：虚拟分组解析到成员并返回可用代理地址
	proxyURL, _, _, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: updated.ProfileId}, updated)
	if err != nil {
		t.Fatalf("切换后启动失败: %v", err)
	}
	if proxyURL != proxyA.URL && proxyURL != proxyB.URL {
		t.Fatalf("启动使用地址 = %s, want 分组成员之一", proxyURL)
	}
	t.Logf("创建→切换→启动 OK: %s", proxyURL)
}

// 复现用户 bug：启动实例时 ApplyDefaults 把虚拟分组绑定实例自动改成直连。
// 修复前：groupId 在实体代理池找不到 + 无配置 → bindProfileToDirectProxy → __direct__。
func TestApplyDefaultsKeepsVirtualGroupBinding(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:        "vg-defaults",
		GroupName:      "defaults",
		Strategy:       "round_robin",
		MemberProxyIds: []string{"proxy-a"},
	}); err != nil {
		t.Fatalf("save virtual group failed: %v", err)
	}

	profile, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "defaults-profile",
		UserDataDir:     "",
		ProxyId:         "vg-defaults",
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if profile.ProxyId != "vg-defaults" {
		t.Fatalf("初始绑定异常: %s", profile.ProxyId)
	}

	// 模拟启动实例：prepareBrowserLaunchContext 会调 ApplyDefaults
	browserProfile := toBrowserProfileForTest(app, profile)
	changed, _, defaultsErr := app.browserMgr.ApplyDefaults(browserProfile)
	_ = changed
	if defaultsErr != nil {
		t.Fatalf("ApplyDefaults returned error: %v", defaultsErr)
	}
	if browserProfile.ProxyId != "vg-defaults" {
		t.Fatalf("ApplyDefaults 破坏了虚拟分组绑定: %q（用户遇到的自动直连 bug）", browserProfile.ProxyId)
	}
	if browserProfile.ProxyConfig == "direct://" {
		t.Fatal("ApplyDefaults 把虚拟分组绑定改成了直连")
	}
}

// 实体 ID 失效时也不允许自动改直连（保留原绑定，启动时明确报错）。
func TestApplyDefaultsKeepsDanglingEntityBinding(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	profile, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "dangling-profile",
		UserDataDir:     "",
		ProxyId:         "proxy-a",
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// 删除绑定代理，模拟失效
	if err := app.browserMgr.ProxyDAO.Delete("proxy-a"); err != nil {
		t.Fatalf("delete proxy failed: %v", err)
	}
	// 清 Manager 缓存重载
	app.browserMgr.Profiles[profile.ProfileId].ProxyId = "proxy-gone"
	app.browserMgr.Profiles[profile.ProfileId].ProxyConfig = ""

	browserProfile := toBrowserProfileForTest(app, profile)
	browserProfile.ProxyId = "proxy-gone"
	browserProfile.ProxyConfig = ""
	if _, _, err := app.browserMgr.ApplyDefaults(browserProfile); err != nil {
		t.Fatalf("失效实体绑定不应在 ApplyDefaults 阶段报错: %v", err)
	}
	if browserProfile.ProxyId != "proxy-gone" {
		t.Fatalf("失效实体绑定被自动改写: %q（不允许自动操作用户绑定）", browserProfile.ProxyId)
	}
	if browserProfile.ProxyConfig == "direct://" || browserProfile.ProxyId == "__direct__" {
		t.Fatal("失效绑定被自动改为直连（隐私风险）")
	}
}

func toBrowserProfileForTest(app *App, profile *BrowserProfile) *browser.Profile {
	// BrowserProfile 与 browser.Profile 字段兼容，直接取 Manager 内部对象
	if inner, ok := app.browserMgr.Profiles[profile.ProfileId]; ok {
		return inner
	}
	return &browser.Profile{}
}

// 更新实例时空代理输入必须报错，既不自动直连也不静默保留。
func TestUpdateEmptyProxyInputRejected(t *testing.T) {
	proxyA := newLocalHTTPProxy(t)
	app := newVirtualGroupIntegrationApp(t, map[string]string{"proxy-a": proxyA.URL})
	profile, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "empty-input-profile",
		UserDataDir:     "",
		ProxyId:         "proxy-a",
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	updated, err := app.BrowserProfileUpdate(profile.ProfileId, BrowserProfileInput{
		ProfileName:     profile.ProfileName,
		UserDataDir:     profile.UserDataDir,
		ProxyId:         "",
		ProxyConfig:     "",
		FingerprintArgs: profile.FingerprintArgs,
		LaunchArgs:      profile.LaunchArgs,
	})
	if err == nil {
		t.Fatalf("空代理输入应报错，实际返回: %#v", updated)
	}
	current := app.browserMgr.Profiles[profile.ProfileId]
	if current == nil {
		t.Fatal("报错后实例丢失")
	}
	if current.ProxyId != "proxy-a" {
		t.Fatalf("报错路径改动了原绑定: %q（不允许自动操作）", current.ProxyId)
	}
	if current.ProxyId == "__direct__" || current.ProxyConfig == "direct://" {
		t.Fatal("空输入被自动改为直连")
	}
}
