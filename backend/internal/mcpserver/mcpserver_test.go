package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeProvider 是可控的能力实现。
// 真实的 LaunchServer 需要 SQLite、浏览器进程和代理内核，无法在单测里构造。
type fakeProvider struct {
	profiles []browser.Profile
	scripts  []automation.ScriptRecord
	runs     []automation.ScriptRunRecord
	proxies  []config.BrowserProxy
	cores    []config.BrowserCore

	// 记录最后一次收到的调用参数，用于断言翻译逻辑。
	lastRunRequest automation.ScriptRunRequest
	lastSelector   launchcode.LaunchSelector

	sessionReady bool
	failWith     error
}

func (f *fakeProvider) ListProfiles() ([]browser.Profile, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.profiles, nil
}

func (f *fakeProvider) FindProfiles(selector launchcode.LaunchSelector) ([]browser.Profile, error) {
	f.lastSelector = selector
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.profiles, nil
}

func (f *fakeProvider) FindProfile(selector launchcode.LaunchSelector) (*browser.Profile, error) {
	f.lastSelector = selector
	if f.failWith != nil {
		return nil, f.failWith
	}
	if len(f.profiles) == 0 {
		return nil, &launchcode.ServiceError{Status: 404, Message: "profile not found"}
	}
	return &f.profiles[0], nil
}

func (f *fakeProvider) CreateProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	if f.failWith != nil {
		return nil, "", f.failWith
	}
	code := requestedCode
	if code == "" {
		code = "AUTO_CODE"
	}
	return &browser.Profile{
		ProfileId:   "new-profile",
		ProfileName: input.ProfileName,
		Tags:        input.Tags,
		LaunchCode:  code,
	}, code, nil
}

func (f *fakeProvider) UpdateProfile(profileID string, input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	if f.failWith != nil {
		return nil, "", f.failWith
	}
	return &browser.Profile{ProfileId: profileID, ProfileName: input.ProfileName}, requestedCode, nil
}

func (f *fakeProvider) DeleteProfile(string) error { return f.failWith }

func (f *fakeProvider) StartProfile(selector launchcode.LaunchSelector, _ launchcode.LaunchRequestParams) (*browser.Profile, string, error) {
	f.lastSelector = selector
	if f.failWith != nil {
		return nil, "", f.failWith
	}
	return &browser.Profile{ProfileId: "p1", Running: true, DebugPort: 9333}, "CODE1", nil
}

func (f *fakeProvider) StopProfile(profileID string) (*browser.Profile, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &browser.Profile{ProfileId: profileID, Running: false}, nil
}

func (f *fakeProvider) StatusProfile(profileID string) (*browser.Profile, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &browser.Profile{ProfileId: profileID, Running: true, DebugReady: true}, nil
}

func (f *fakeProvider) OpenRuntimeSession(selector launchcode.LaunchSelector, _ launchcode.LaunchRequestParams, _ time.Duration) (*launchcode.RuntimeSession, error) {
	f.lastSelector = selector
	if f.failWith != nil {
		return nil, f.failWith
	}
	session := &launchcode.RuntimeSession{
		Profile: &browser.Profile{ProfileId: "p1", Running: true, DebugReady: f.sessionReady},
		Ready:   f.sessionReady,
	}
	if f.sessionReady {
		session.CDPURL = "http://127.0.0.1:19876"
	}
	return session, nil
}

func (f *fakeProvider) ActiveRuntimeSession() (*launchcode.RuntimeSession, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	if !f.sessionReady {
		return nil, nil
	}
	return &launchcode.RuntimeSession{
		Profile: &browser.Profile{ProfileId: "p1", DebugReady: true},
		Ready:   true,
		CDPURL:  "http://127.0.0.1:19876",
	}, nil
}

func (f *fakeProvider) ListScripts() ([]automation.ScriptRecord, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.scripts, nil
}

func (f *fakeProvider) GetScript(scriptID string) (*automation.ScriptRecord, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	for i := range f.scripts {
		if f.scripts[i].ID == scriptID {
			return &f.scripts[i], nil
		}
	}
	return nil, &launchcode.ServiceError{Status: 404, Message: "automation script not found"}
}

func (f *fakeProvider) RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	f.lastRunRequest = input
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &automation.ScriptRunRecord{
		ID:       "run-1",
		ScriptID: input.ScriptID,
		Status:   "success",
		Summary:  "done",
	}, nil
}

func (f *fakeProvider) ListScriptRuns(int) ([]automation.ScriptRunRecord, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.runs, nil
}

func (f *fakeProvider) ListProxies() ([]config.BrowserProxy, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.proxies, nil
}

func (f *fakeProvider) TestProxySpeed(proxyID string) (*launchcode.ProxySpeedResult, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &launchcode.ProxySpeedResult{ProxyID: proxyID, Ok: true, LatencyMs: 120}, nil
}

func (f *fakeProvider) CheckProxyHealth(proxyID string) (*launchcode.ProxyHealthResult, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &launchcode.ProxyHealthResult{ProxyID: proxyID, Ok: true, IP: "1.2.3.4", Country: "US"}, nil
}

func (f *fakeProvider) ListCores() ([]config.BrowserCore, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.cores, nil
}

// newTestClient 建立一对内存传输的 client/server，返回已完成 initialize 的会话。
func newTestClient(t *testing.T, provider Provider) *mcp.ClientSession {
	t.Helper()

	server := New(provider, "test").MCPServer()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result
}

// decodeStructured 把工具的结构化输出解回具体类型。
func decodeStructured(t *testing.T, result *mcp.CallToolResult, target any) {
	t.Helper()

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
}

// TestToolsAreRegistered 锁定对外暴露的工具集合。
// 工具名是外部契约的一部分，改名会破坏已有客户端配置，因此这里全量断言。
func TestToolsAreRegistered(t *testing.T) {
	session := newTestClient(t, &fakeProvider{})

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	got := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		got[tool.Name] = true
	}

	want := []string{
		"ant_instance_list", "ant_instance_get", "ant_instance_create",
		"ant_instance_update", "ant_instance_delete", "ant_instance_start",
		"ant_instance_stop", "ant_runtime_session", "ant_runtime_status",
		"ant_runtime_active",
		"ant_script_list", "ant_script_get", "ant_script_run", "ant_script_runs",
		"ant_proxy_list", "ant_proxy_test_speed", "ant_proxy_check_health",
		"ant_core_list",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered", name)
		}
	}
	if len(result.Tools) != len(want) {
		t.Errorf("tool count = %d, want %d (registered: %v)", len(result.Tools), len(want), got)
	}
	// ToolCount 是手工维护的常量（SDK 没有枚举 API），这里锁住它不跑偏。
	if ToolCount() != len(result.Tools) {
		t.Errorf("ToolCount() = %d, but %d tools are registered; update the toolCount constant", ToolCount(), len(result.Tools))
	}
}

// TestReadOnlyToolsAnnotated 确认只读工具带 ReadOnlyHint，
// 破坏性工具带 DestructiveHint —— 客户端据此做权限分级。
func TestReadOnlyToolsAnnotated(t *testing.T) {
	session := newTestClient(t, &fakeProvider{})

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	readOnlyTools := map[string]bool{
		"ant_instance_list": true, "ant_instance_get": true,
		"ant_runtime_status": true, "ant_runtime_active": true,
		"ant_script_list": true, "ant_script_get": true, "ant_script_runs": true,
		"ant_proxy_list": true, "ant_core_list": true,
	}
	destructiveTools := map[string]bool{
		"ant_instance_delete": true, "ant_instance_stop": true,
	}

	for _, tool := range result.Tools {
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", tool.Name)
			continue
		}
		if readOnlyTools[tool.Name] && !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q should be marked read-only", tool.Name)
		}
		if !readOnlyTools[tool.Name] && tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q should not be marked read-only", tool.Name)
		}
		if destructiveTools[tool.Name] {
			if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Errorf("tool %q should be marked destructive", tool.Name)
			}
		}
	}
}

func TestInstanceListFiltering(t *testing.T) {
	provider := &fakeProvider{profiles: []browser.Profile{
		{ProfileId: "p1", ProfileName: "Amazon US", Tags: []string{"电商"}, Running: true},
		{ProfileId: "p2", ProfileName: "Twitter", Tags: []string{"社媒"}, Running: false},
		{ProfileId: "p3", ProfileName: "Amazon JP", Tags: []string{"电商"}, Running: false},
	}}
	session := newTestClient(t, provider)

	tests := []struct {
		name string
		args map[string]any
		want int
	}{
		{"no filter", map[string]any{}, 3},
		{"by tag", map[string]any{"tag": "电商"}, 2},
		{"by tag case-insensitive", map[string]any{"tag": "社媒"}, 1},
		{"running only", map[string]any{"runningOnly": true}, 1},
		{"keyword substring", map[string]any{"keyword": "amazon"}, 2},
		{"keyword no match", map[string]any{"keyword": "nonexistent"}, 0},
		{"tag and running", map[string]any{"tag": "电商", "runningOnly": true}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callTool(t, session, "ant_instance_list", tt.args)
			if result.IsError {
				t.Fatalf("unexpected error: %v", result.Content)
			}

			var out listInstancesOutput
			decodeStructured(t, result, &out)
			if out.Count != tt.want {
				t.Errorf("count = %d, want %d", out.Count, tt.want)
			}
			if len(out.Items) != tt.want {
				t.Errorf("len(items) = %d, want %d", len(out.Items), tt.want)
			}
		})
	}
}

// TestInstanceListOmitsSensitiveFields 确认精简视图不会泄漏代理配置。
func TestInstanceListOmitsSensitiveFields(t *testing.T) {
	provider := &fakeProvider{profiles: []browser.Profile{{
		ProfileId:   "p1",
		ProfileName: "test",
		ProxyId:     "proxy-1",
		ProxyConfig: "vmess://secret-credentials-here",
	}}}
	session := newTestClient(t, provider)

	result := callTool(t, session, "ant_instance_list", map[string]any{})
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "secret-credentials-here") {
		t.Errorf("instance list leaked proxy config: %s", raw)
	}
}

// TestProxyListOmitsProxyConfig 确认代理列表不返回凭据。
func TestProxyListOmitsProxyConfig(t *testing.T) {
	provider := &fakeProvider{proxies: []config.BrowserProxy{{
		ProxyId:     "proxy-1",
		ProxyName:   "Node A",
		ProxyConfig: "vmess://user:password@example.com:443",
		LastTestOk:  true,
	}}}
	session := newTestClient(t, provider)

	result := callTool(t, session, "ant_proxy_list", map[string]any{})
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "password") {
		t.Errorf("proxy list leaked credentials: %s", raw)
	}

	var out listProxiesOutput
	decodeStructured(t, result, &out)
	if len(out.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(out.Items))
	}
	if out.Items[0].Protocol != "vmess" {
		t.Errorf("protocol = %q, want vmess", out.Items[0].Protocol)
	}
}

func TestDetectProtocol(t *testing.T) {
	tests := []struct{ input, want string }{
		{"vmess://abc", "vmess"},
		{"socks5://1.2.3.4:1080", "socks5"},
		{"http://proxy:8080", "http"},
		{"", ""},
		{"no-scheme-here", ""},
		{"has space://x", ""},
	}
	for _, tt := range tests {
		if got := detectProtocol(tt.input); got != tt.want {
			t.Errorf("detectProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestScriptRunUsesScriptDefaults 锁定关键语义：
// 省略 selector/params 时必须置 UseScript* 标志，否则脚本会拿到空配置。
func TestScriptRunUsesScriptDefaults(t *testing.T) {
	provider := &fakeProvider{}
	session := newTestClient(t, provider)

	result := callTool(t, session, "ant_script_run", map[string]any{"scriptId": "demo"})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	got := provider.lastRunRequest
	if !got.UseScriptSelector {
		t.Error("UseScriptSelector = false, want true when selector omitted")
	}
	if !got.UseScriptParams {
		t.Error("UseScriptParams = false, want true when params omitted")
	}
	if got.ScriptID != "demo" {
		t.Errorf("ScriptID = %q, want demo", got.ScriptID)
	}
}

func TestScriptRunWithOverrides(t *testing.T) {
	provider := &fakeProvider{}
	session := newTestClient(t, provider)

	result := callTool(t, session, "ant_script_run", map[string]any{
		"scriptId": "demo",
		"selector": map[string]any{"code": "BUYER_001"},
		"params":   map[string]any{"keyword": "test"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", contentText(result))
	}

	got := provider.lastRunRequest
	if got.UseScriptSelector {
		t.Error("UseScriptSelector = true, want false when selector provided")
	}
	if got.UseScriptParams {
		t.Error("UseScriptParams = true, want false when params provided")
	}
	if !strings.Contains(got.SelectorText, "BUYER_001") {
		t.Errorf("SelectorText = %q, want it to contain BUYER_001", got.SelectorText)
	}
	if !strings.Contains(got.ParamsText, "test") {
		t.Errorf("ParamsText = %q, want it to contain test", got.ParamsText)
	}
}

// TestScriptRunFailureIsReportedAsError 确认脚本自身失败也会置 IsError，
// 同时保留结构化结果供排查。
func TestScriptRunFailureIsReportedAsError(t *testing.T) {
	provider := &failingScriptProvider{}
	session := newTestClient(t, provider)

	result := callTool(t, session, "ant_script_run", map[string]any{"scriptId": "demo"})
	if !result.IsError {
		t.Error("IsError = false, want true when script run fails")
	}

	var out runScriptOutput
	decodeStructured(t, result, &out)
	if out.Run.Status != "failed" {
		t.Errorf("Run.Status = %q, want failed", out.Run.Status)
	}
	if out.Run.Error == "" {
		t.Error("Run.Error is empty, want the underlying failure detail preserved")
	}
}

type failingScriptProvider struct{ fakeProvider }

func (f *failingScriptProvider) RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	return &automation.ScriptRunRecord{
		ID:       "run-x",
		ScriptID: input.ScriptID,
		Status:   "failed",
		Error:    "target instance not reachable",
	}, nil
}

// TestRuntimeSessionNotReadyGivesHint 确认超时未就绪时给出可操作建议而非静默成功。
func TestRuntimeSessionNotReadyGivesHint(t *testing.T) {
	session := newTestClient(t, &fakeProvider{sessionReady: false})

	result := callTool(t, session, "ant_runtime_session", map[string]any{
		"selector": map[string]any{"code": "TEST"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var out runtimeSessionOutput
	decodeStructured(t, result, &out)
	if out.Ready {
		t.Error("Ready = true, want false")
	}
	if out.CDPURL != "" {
		t.Errorf("CDPURL = %q, want empty when not ready", out.CDPURL)
	}
	if out.Hint == "" {
		t.Error("Hint is empty, want actionable guidance when not ready")
	}
}

func TestRuntimeSessionReadyReturnsCDPURL(t *testing.T) {
	session := newTestClient(t, &fakeProvider{sessionReady: true})

	result := callTool(t, session, "ant_runtime_session", map[string]any{
		"selector": map[string]any{"code": "TEST"},
	})

	var out runtimeSessionOutput
	decodeStructured(t, result, &out)
	if !out.Ready {
		t.Fatal("Ready = false, want true")
	}
	if out.CDPURL == "" {
		t.Error("CDPURL is empty, want the unified CDP entry when ready")
	}
}

func TestActiveSessionWhenNoneActive(t *testing.T) {
	session := newTestClient(t, &fakeProvider{sessionReady: false})

	result := callTool(t, session, "ant_runtime_active", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var out activeSessionOutput
	decodeStructured(t, result, &out)
	if out.Active {
		t.Error("Active = true, want false when no session is active")
	}
}

// TestServiceErrorsBecomeToolErrors 确认服务层错误转成工具级错误而非协议错误，
// 且错误文案带可操作建议。
func TestServiceErrorsBecomeToolErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantInMsg string
	}{
		{"not found", &launchcode.ServiceError{Status: 404, Message: "profile not found"}, "ant_instance_list"},
		{"ambiguous", &launchcode.ServiceError{Status: 409, Message: "selector matched 3 instances"}, "matchMode=first"},
		{"unavailable", &launchcode.ServiceError{Status: 503, Message: "proxy api is unavailable"}, "不可用"},
		{"bad request", &launchcode.ServiceError{Status: 400, Message: "proxyId is required"}, "参数无效"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newTestClient(t, &fakeProvider{failWith: tt.err})

			result := callTool(t, session, "ant_instance_list", map[string]any{})
			if !result.IsError {
				t.Fatal("IsError = false, want true")
			}

			text := contentText(result)
			if !strings.Contains(text, tt.wantInMsg) {
				t.Errorf("error text = %q, want it to contain %q", text, tt.wantInMsg)
			}
		})
	}
}

// TestConflictWithoutSelectorHintIsNotMisleading 确认状态冲突（如删除运行中实例）
// 不会给出「加 matchMode=first」这种驴唇不对马嘴的建议。
func TestConflictWithoutSelectorHintIsNotMisleading(t *testing.T) {
	err := &launchcode.ServiceError{Status: 409, Message: "running profile cannot be deleted"}
	text := describeError(err)

	if strings.Contains(text, "matchMode") {
		t.Errorf("describeError(%q) = %q, should not suggest matchMode for a state conflict", err.Message, text)
	}
}

func contentText(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}
