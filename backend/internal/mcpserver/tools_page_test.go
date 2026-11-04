package mcpserver

import (
	"context"
	"strings"
	"testing"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 页面操作工具的契约测试。
//
// 重点是「协议层翻译」：工具入参 → PageStep.Args，以及 Node 侧回传的
// map[string]any → 结构化输出。真正的浏览器行为由端到端验证覆盖。

func callPageTool(t *testing.T, provider Provider, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	session := newTestClient(t, provider)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result
}

// 每个页面工具都必须把自己的入参翻译成一条正确的指令。
// 动作名是 Go 与 Node 之间的契约，写错了只会在真机上才暴露。
func TestPageToolsTranslateToActions(t *testing.T) {
	cases := []struct {
		tool       string
		args       map[string]any
		wantAction string
		check      func(t *testing.T, args map[string]any)
	}{
		{
			tool:       "ant_page_goto",
			args:       map[string]any{"url": "https://example.com/", "waitUntil": "load"},
			wantAction: "goto",
			check: func(t *testing.T, args map[string]any) {
				if args["url"] != "https://example.com/" {
					t.Errorf("url = %v", args["url"])
				}
				if args["waitUntil"] != "load" {
					t.Errorf("waitUntil = %v", args["waitUntil"])
				}
			},
		},
		{
			tool:       "ant_page_click",
			args:       map[string]any{"element": "#submit", "button": "right"},
			wantAction: "click",
			check: func(t *testing.T, args map[string]any) {
				// element 在协议层叫 element，到 Node 侧统一叫 selector。
				if args["selector"] != "#submit" {
					t.Errorf("selector = %v, want #submit", args["selector"])
				}
				if args["button"] != "right" {
					t.Errorf("button = %v", args["button"])
				}
			},
		},
		{
			tool:       "ant_page_press",
			args:       map[string]any{"key": "Enter"},
			wantAction: "press",
			check: func(t *testing.T, args map[string]any) {
				if args["key"] != "Enter" {
					t.Errorf("key = %v", args["key"])
				}
				// 没给 element 时不应下发空 selector，否则 Node 侧会当成定位失败。
				if _, ok := args["selector"]; ok {
					t.Errorf("selector should be absent, got %v", args["selector"])
				}
			},
		},
		{
			tool:       "ant_page_select",
			args:       map[string]any{"element": "#country", "values": []any{"cn"}},
			wantAction: "selectOption",
			check: func(t *testing.T, args map[string]any) {
				values, ok := args["values"].([]any)
				if !ok || len(values) != 1 || values[0] != "cn" {
					t.Errorf("values = %v", args["values"])
				}
			},
		},
		{
			tool:       "ant_page_wait",
			args:       map[string]any{"element": ".result", "state": "hidden"},
			wantAction: "waitFor",
			check: func(t *testing.T, args map[string]any) {
				if args["selector"] != ".result" || args["state"] != "hidden" {
					t.Errorf("args = %v", args)
				}
			},
		},
		{
			tool:       "ant_page_snapshot",
			args:       map[string]any{"limit": float64(50), "includeText": true},
			wantAction: "snapshot",
			check: func(t *testing.T, args map[string]any) {
				if args["limit"] != 50 || args["includeText"] != true {
					t.Errorf("args = %v", args)
				}
			},
		},
		{
			tool:       "ant_page_extract",
			args:       map[string]any{"element": ".row", "mode": "attribute", "attribute": "href", "all": true},
			wantAction: "extract",
			check: func(t *testing.T, args map[string]any) {
				if args["mode"] != "attribute" || args["attribute"] != "href" || args["all"] != true {
					t.Errorf("args = %v", args)
				}
			},
		},
		{
			tool:       "ant_page_evaluate",
			args:       map[string]any{"expression": "() => document.title"},
			wantAction: "evaluate",
			check: func(t *testing.T, args map[string]any) {
				if args["expression"] != "() => document.title" {
					t.Errorf("expression = %v", args["expression"])
				}
			},
		},
		{
			tool:       "ant_page_tabs",
			args:       map[string]any{"op": "select", "index": float64(2)},
			wantAction: "tabs",
			check: func(t *testing.T, args map[string]any) {
				if args["op"] != "select" || args["index"] != 2 {
					t.Errorf("args = %v", args)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			provider := &fakeProvider{}
			result := callPageTool(t, provider, tc.tool, tc.args)
			if result.IsError {
				t.Fatalf("tool returned error: %v", result.Content)
			}

			steps := provider.lastPageRequest.Steps
			if len(steps) != 1 {
				t.Fatalf("sent %d steps, want 1", len(steps))
			}
			if steps[0].Action != tc.wantAction {
				t.Errorf("action = %q, want %q", steps[0].Action, tc.wantAction)
			}
			if tc.check != nil {
				tc.check(t, steps[0].Args)
			}
		})
	}
}

// 省略 selector 是常态（连续操作同一页面），不能被当成参数错误。
func TestPageToolsAllowOmittedSelector(t *testing.T) {
	provider := &fakeProvider{}
	result := callPageTool(t, provider, "ant_page_snapshot", map[string]any{})
	if result.IsError {
		t.Fatalf("snapshot without selector failed: %v", result.Content)
	}
	if !provider.lastPageRequest.Selector.IsEmpty() {
		t.Errorf("selector = %+v, want empty", provider.lastPageRequest.Selector)
	}
}

// 工具层只做原样透传，大小写等规范化由 LaunchServer 门面统一处理，
// 避免同一条规则在两处各写一份。
func TestPageToolPassesSelectorThrough(t *testing.T) {
	provider := &fakeProvider{}
	result := callPageTool(t, provider, "ant_page_goto", map[string]any{
		"url":      "https://example.com/",
		"selector": map[string]any{"code": "abc", "profileId": "p9"},
	})
	if result.IsError {
		t.Fatalf("goto failed: %v", result.Content)
	}
	if provider.lastPageRequest.Selector.Code != "abc" {
		t.Errorf("selector code = %q, want abc", provider.lastPageRequest.Selector.Code)
	}
	if provider.lastPageRequest.Selector.ProfileID != "p9" {
		t.Errorf("selector profileId = %q, want p9", provider.lastPageRequest.Selector.ProfileID)
	}
}

// fill 支持一次多字段，这是省往返的关键，必须原样传下去。
func TestPageFillSendsAllFields(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "fill",
				OK:     true,
				Result: map[string]any{
					"url":    "https://example.com/login",
					"filled": []any{"#user", "#pass"},
				},
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_fill", map[string]any{
		"fields": []any{
			map[string]any{"element": "#user", "value": "alice"},
			map[string]any{"element": "#pass", "value": "secret"},
		},
	})
	if result.IsError {
		t.Fatalf("fill failed: %v", result.Content)
	}

	fields, ok := provider.lastPageRequest.Steps[0].Args["fields"].([]map[string]any)
	if !ok {
		t.Fatalf("fields has unexpected type %T", provider.lastPageRequest.Steps[0].Args["fields"])
	}
	if len(fields) != 2 {
		t.Fatalf("sent %d fields, want 2", len(fields))
	}
	if fields[0]["selector"] != "#user" || fields[0]["value"] != "alice" {
		t.Errorf("first field = %v", fields[0])
	}
	if fields[1]["selector"] != "#pass" || fields[1]["value"] != "secret" {
		t.Errorf("second field = %v", fields[1])
	}

	var out fillOutput
	decodeStructured(t, result, &out)
	if len(out.Filled) != 2 || out.Filled[0] != "#user" {
		t.Errorf("filled = %v", out.Filled)
	}
}

func TestPageFillRejectsEmptyFields(t *testing.T) {
	result := callPageTool(t, &fakeProvider{}, "ant_page_fill", map[string]any{"fields": []any{}})
	if !result.IsError {
		t.Fatal("empty fields accepted, want an input error")
	}
}

// 快照是模型的「眼睛」，元素列表必须被解码成结构化输出而不是塞回原始 map。
func TestPageSnapshotDecodesElements(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "snapshot",
				OK:     true,
				Result: map[string]any{
					"url":   "https://example.com/",
					"title": "Example",
					"elements": []any{
						map[string]any{"role": "button", "tag": "button", "name": "登录", "selector": "#login"},
						map[string]any{"role": "textbox", "tag": "input", "name": "用户名", "selector": "#user", "type": "text"},
					},
					"truncated": true,
				},
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_snapshot", map[string]any{})
	if result.IsError {
		t.Fatalf("snapshot failed: %v", result.Content)
	}

	var out snapshotOutput
	decodeStructured(t, result, &out)
	if out.Page.Title != "Example" || out.Page.URL != "https://example.com/" {
		t.Errorf("page = %+v", out.Page)
	}
	if len(out.Elements) != 2 {
		t.Fatalf("elements = %d, want 2", len(out.Elements))
	}
	if out.Elements[0].Selector != "#login" || out.Elements[0].Name != "登录" {
		t.Errorf("first element = %+v", out.Elements[0])
	}
	if !out.Truncated {
		t.Error("truncated = false, want true")
	}
}

// 截图必须走 ImageContent：塞进结构化输出会被序列化成 base64 文本，
// 客户端无法识别成图像。
func TestPageScreenshotReturnsImageContent(t *testing.T) {
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "screenshot",
				OK:     true,
				Result: map[string]any{
					"url":        "https://example.com/",
					"screenshot": "shot.png",
					"bytes":      float64(len(pngBytes)),
				},
			}},
			Screenshots: map[string]launchcode.PageScreenshot{
				"shot.png": {Data: pngBytes, MIMEType: "image/png"},
			},
		},
	}

	result := callPageTool(t, provider, "ant_page_screenshot", map[string]any{})
	if result.IsError {
		t.Fatalf("screenshot failed: %v", result.Content)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content items = %d, want 1", len(result.Content))
	}
	image, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.ImageContent", result.Content[0])
	}
	if image.MIMEType != "image/png" {
		t.Errorf("mime = %q, want image/png", image.MIMEType)
	}
	if string(image.Data) != string(pngBytes) {
		t.Errorf("image bytes = %v, want %v", image.Data, pngBytes)
	}
}

// 截图文件读不出来时要明确报错，而不是回一张空图。
func TestPageScreenshotMissingBytesIsError(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "screenshot",
				OK:     true,
				Result: map[string]any{"screenshotError": "读取截图失败: no such file"},
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_screenshot", map[string]any{})
	if !result.IsError {
		t.Fatal("screenshot without bytes succeeded, want an error")
	}
	if !strings.Contains(renderContent(result), "读取截图失败") {
		t.Errorf("error text = %q, want it to surface the read failure", renderContent(result))
	}
}

// 指令级失败（选择器没命中）要标成工具错误，否则模型会以为点成功了。
func TestPageStepFailureIsToolError(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        false,
			Steps: []launchcode.PageStepOutcome{{
				Action: "click",
				OK:     false,
				Error:  "locator.click: Timeout 30000ms exceeded",
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_click", map[string]any{"element": "#missing"})
	if !result.IsError {
		t.Fatal("failed click reported success")
	}
	if !strings.Contains(renderContent(result), "Timeout") {
		t.Errorf("error text = %q, want the underlying failure", renderContent(result))
	}
}

// 能力未注入时门面返回 503，工具应给出「环境不支持」而不是裸错误。
func TestPageToolMapsUnavailableError(t *testing.T) {
	provider := &fakeProvider{
		failWith: &launchcode.ServiceError{Status: 503, Message: "page automation api is unavailable"},
	}

	result := callPageTool(t, provider, "ant_page_snapshot", map[string]any{})
	if !result.IsError {
		t.Fatal("snapshot succeeded, want an error")
	}
	if !strings.Contains(renderContent(result), "当前运行环境不可用") {
		t.Errorf("error text = %q, want the unavailable hint", renderContent(result))
	}
}

func TestPageReleaseClosesSession(t *testing.T) {
	provider := &fakeProvider{}
	result := callPageTool(t, provider, "ant_page_release", map[string]any{
		"selector": map[string]any{"profileId": "p1"},
	})
	if result.IsError {
		t.Fatalf("release failed: %v", result.Content)
	}

	var out releaseOutput
	decodeStructured(t, result, &out)
	if !out.Released || out.ProfileID != "p1" {
		t.Errorf("release output = %+v", out)
	}
	if provider.lastSelector.ProfileID != "p1" {
		t.Errorf("selector profileId = %q, want p1", provider.lastSelector.ProfileID)
	}
}

func TestPageEvaluateReturnsValue(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "evaluate",
				OK:     true,
				Result: map[string]any{"value": map[string]any{"count": float64(3)}},
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_evaluate", map[string]any{
		"expression": "() => ({count: 3})",
	})
	if result.IsError {
		t.Fatalf("evaluate failed: %v", result.Content)
	}

	var out evaluateOutput
	decodeStructured(t, result, &out)
	value, ok := out.Value.(map[string]any)
	if !ok || value["count"] != float64(3) {
		t.Errorf("value = %v", out.Value)
	}
}

func TestPageEvaluateRejectsEmptyExpression(t *testing.T) {
	result := callPageTool(t, &fakeProvider{}, "ant_page_evaluate", map[string]any{"expression": "  "})
	if !result.IsError {
		t.Fatal("empty expression accepted, want an input error")
	}
}

func TestPageTabsDecodesList(t *testing.T) {
	provider := &fakeProvider{
		pageResult: &launchcode.PageResult{
			ProfileID: "p1",
			OK:        true,
			Steps: []launchcode.PageStepOutcome{{
				Action: "tabs",
				OK:     true,
				Result: map[string]any{
					"count": float64(2),
					"items": []any{
						map[string]any{"index": float64(0), "url": "https://a.test/", "title": "A", "active": true},
						map[string]any{"index": float64(1), "url": "https://b.test/", "title": "B"},
					},
				},
			}},
		},
	}

	result := callPageTool(t, provider, "ant_page_tabs", map[string]any{})
	if result.IsError {
		t.Fatalf("tabs failed: %v", result.Content)
	}

	var out tabsOutput
	decodeStructured(t, result, &out)
	if out.Count != 2 || len(out.Items) != 2 {
		t.Fatalf("tabs output = %+v", out)
	}
	if !out.Items[0].Active || out.Items[1].Title != "B" {
		t.Errorf("items = %+v", out.Items)
	}
}

func TestPageExtractRejectsAttributeModeWithoutAttribute(t *testing.T) {
	result := callPageTool(t, &fakeProvider{}, "ant_page_extract", map[string]any{
		"element": ".row",
		"mode":    "attribute",
	})
	if !result.IsError {
		t.Fatal("attribute mode without attribute accepted, want an input error")
	}
}

// renderContent 把工具返回的文本内容拼成一个串，便于断言错误文案。
func renderContent(result *mcp.CallToolResult) string {
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
