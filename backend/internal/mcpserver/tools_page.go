package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPageTools(srv *mcp.Server, p PageProvider) {
	registerPageNavigationTools(srv, p)
	registerPageInteractionTools(srv, p)
	registerPageReadTools(srv, p)
	registerPageSessionTools(srv, p)
}

func registerPageNavigationTools(srv *mcp.Server, p PageProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_goto",
		Description: "在目标实例里导航到指定 URL。首次调用会自动启动实例并接管 CDP，" +
			"之后的页面操作复用同一个会话，无需重复启动。",
		Annotations: mutating("页面导航"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in gotoInput) (*mcp.CallToolResult, gotoOutput, error) {
		args := map[string]any{"url": strings.TrimSpace(in.URL)}
		if waitUntil := strings.TrimSpace(in.WaitUntil); waitUntil != "" {
			args["waitUntil"] = waitUntil
		}

		fail, out, payload := runPageAction(p, in.Selector, in.TimeoutMs, "goto", args)
		if fail != nil {
			return fail, gotoOutput{pageOutput: out}, nil
		}

		result := gotoOutput{pageOutput: out}
		if status, ok := payload["status"].(float64); ok {
			result.Status = int(status)
		}
		return nil, result, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_page_wait",
		Description: "等待页面达到某个状态：元素出现或消失、URL 变化、加载完成。都不指定时按 timeoutMs 单纯等待。",
		Annotations: readOnly("等待页面状态"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in waitInput) (*mcp.CallToolResult, pageOutput, error) {
		args := map[string]any{}
		if element := strings.TrimSpace(in.Element); element != "" {
			args["selector"] = element
			if state := strings.TrimSpace(in.State); state != "" {
				args["state"] = state
			}
		}
		if url := strings.TrimSpace(in.URL); url != "" {
			args["url"] = url
		}
		if loadState := strings.TrimSpace(in.LoadState); loadState != "" {
			args["loadState"] = loadState
		}

		fail, out, _ := runPageAction(p, in.Selector, in.TimeoutMs, "waitFor", args)
		if fail != nil {
			return fail, out, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_tabs",
		Description: "管理标签页：list 列出全部、new 新建、select 切换、close 关闭。" +
			"切换后的页面操作都作用于新选中的标签页。",
		Annotations: mutating("管理标签页"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in tabsInput) (*mcp.CallToolResult, tabsOutput, error) {
		op := strings.TrimSpace(in.Op)
		if op == "" {
			op = "list"
		}
		args := map[string]any{"op": op}
		if in.Index != nil {
			args["index"] = *in.Index
		}
		if url := strings.TrimSpace(in.URL); url != "" {
			args["url"] = url
		}

		result, err := pageCall(p, in.Selector, in.TimeoutMs, "tabs", args)
		if err != nil {
			return toolError(err), tabsOutput{}, nil
		}
		payload, stepErr := firstStep(result)
		if stepErr != nil {
			return toolError(stepErr), tabsOutput{ProfileID: result.ProfileID}, nil
		}

		out := tabsOutput{ProfileID: result.ProfileID, OK: true, Page: pageStateOf(payload)}
		if count, ok := payload["count"].(float64); ok {
			out.Count = int(count)
		}
		if items, ok := payload["items"].([]any); ok {
			out.Items = decodeTabs(items)
			out.Count = len(out.Items)
		}
		return nil, out, nil
	})
}

func registerPageInteractionTools(srv *mcp.Server, p PageProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_click",
		Description: "点击页面元素。element 建议直接用 ant_page_snapshot 返回的 selector，" +
			"那是从真实 DOM 算出来的，比自己猜 CSS 可靠。",
		Annotations: mutating("点击元素"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in clickInput) (*mcp.CallToolResult, pageOutput, error) {
		extra := map[string]any{}
		if button := strings.TrimSpace(in.Button); button != "" {
			extra["button"] = button
		}
		if in.ClickCount > 0 {
			extra["clickCount"] = in.ClickCount
		}
		if in.Force {
			extra["force"] = true
		}

		fail, out, _ := runPageAction(p, in.Selector, in.TimeoutMs, "click", in.args(extra))
		if fail != nil {
			return fail, out, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_fill",
		Description: "填写输入框。一次可以传多个字段，整张表单在一轮里填完，比逐个字段调用快得多。" +
			"会先清空原有内容再填入。",
		Annotations: mutating("填写表单"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in fillInput) (*mcp.CallToolResult, fillOutput, error) {
		if len(in.Fields) == 0 {
			return toolError(newInputError("fields 不能为空")), fillOutput{}, nil
		}

		fields := make([]map[string]any, 0, len(in.Fields))
		for _, field := range in.Fields {
			element := strings.TrimSpace(field.Element)
			if element == "" {
				return toolError(newInputError("每个字段都必须提供 element")), fillOutput{}, nil
			}
			entry := map[string]any{"selector": element, "value": field.Value}
			if frame := strings.TrimSpace(field.FrameSelector); frame != "" {
				entry["frameSelector"] = frame
			}
			fields = append(fields, entry)
		}

		fail, out, payload := runPageAction(p, in.Selector, in.TimeoutMs, "fill", map[string]any{"fields": fields})
		if fail != nil {
			return fail, fillOutput{pageOutput: out}, nil
		}

		result := fillOutput{pageOutput: out}
		if filled, ok := payload["filled"].([]any); ok {
			result.Filled = decodeStrings(filled)
		}
		return nil, result, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_page_press",
		Description: "发送键盘按键，例如提交表单的 Enter、关闭弹层的 Escape。可先聚焦到指定元素再按。",
		Annotations: mutating("键盘输入"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pressInput) (*mcp.CallToolResult, pageOutput, error) {
		key := strings.TrimSpace(in.Key)
		if key == "" {
			return toolError(newInputError("key 不能为空")), pageOutput{}, nil
		}

		args := map[string]any{"key": key}
		if element := strings.TrimSpace(in.Element); element != "" {
			args["selector"] = element
			if frame := strings.TrimSpace(in.FrameSelector); frame != "" {
				args["frameSelector"] = frame
			}
		}

		fail, out, _ := runPageAction(p, in.Selector, in.TimeoutMs, "press", args)
		if fail != nil {
			return fail, out, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_page_select",
		Description: "在下拉框（select 元素）里选中选项。values 传选项的 value 属性值。",
		Annotations: mutating("选择下拉项"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in selectInput) (*mcp.CallToolResult, selectOutput, error) {
		if len(in.Values) == 0 {
			return toolError(newInputError("values 不能为空")), selectOutput{}, nil
		}

		values := make([]any, 0, len(in.Values))
		for _, value := range in.Values {
			values = append(values, value)
		}

		fail, out, payload := runPageAction(p, in.Selector, in.TimeoutMs, "selectOption", in.args(map[string]any{"values": values}))
		if fail != nil {
			return fail, selectOutput{pageOutput: out}, nil
		}

		result := selectOutput{pageOutput: out}
		if selected, ok := payload["selected"].([]any); ok {
			result.Selected = decodeStrings(selected)
		}
		return nil, result, nil
	})
}

func registerPageReadTools(srv *mcp.Server, p PageProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_snapshot",
		Description: "获取页面快照：当前 URL、标题，以及所有可交互元素及其选择器。" +
			"这是决定下一步操作的首选工具——它只返回能点能填的元素，比截图省上下文，" +
			"给出的 selector 可以直接回传给 ant_page_click / ant_page_fill。",
		Annotations: readOnly("页面快照"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in snapshotInput) (*mcp.CallToolResult, snapshotOutput, error) {
		args := map[string]any{}
		if in.Limit > 0 {
			args["limit"] = in.Limit
		}
		if in.IncludeText {
			args["includeText"] = true
		}

		fail, out, payload := runPageAction(p, in.Selector, in.TimeoutMs, "snapshot", args)
		if fail != nil {
			return fail, snapshotOutput{pageOutput: out}, nil
		}

		result := snapshotOutput{pageOutput: out, Elements: []PageElement{}}
		if elements, ok := payload["elements"].([]any); ok {
			result.Elements = decodeElements(elements)
		}
		if text, ok := payload["text"].(string); ok {
			result.Text = text
		}
		if truncated, ok := payload["truncated"].(bool); ok {
			result.Truncated = truncated
		}
		return nil, result, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_screenshot",
		Description: "截取页面图像。默认只截视口且用 jpeg 压缩——整页 png 动辄数 MB，会挤掉上下文。" +
			"确认视觉布局时用它，判断可点元素请优先用 ant_page_snapshot。",
		Annotations: readOnly("页面截图"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in screenshotInput) (*mcp.CallToolResult, pageOutput, error) {
		args := map[string]any{}
		if element := strings.TrimSpace(in.Element); element != "" {
			args["selector"] = element
		}
		if in.FullPage {
			args["fullPage"] = true
		}
		if imageType := strings.TrimSpace(in.Type); imageType != "" {
			args["type"] = imageType
		}
		if in.Quality > 0 {
			args["quality"] = in.Quality
		}

		result, err := pageCall(p, in.Selector, in.TimeoutMs, "screenshot", args)
		if err != nil {
			return toolError(err), pageOutput{}, nil
		}
		payload, stepErr := firstStep(result)
		if stepErr != nil {
			return toolError(stepErr), pageOutput{ProfileID: result.ProfileID}, nil
		}

		out := pageOutput{ProfileID: result.ProfileID, OK: true, Page: pageStateOf(payload)}

		// 图片走 Content 而不是 structured output：后者会被序列化成 JSON 文本，
		// base64 塞进去既浪费又无法被客户端识别为图像。
		name, _ := payload["screenshot"].(string)
		shot, ok := result.Screenshots[name]
		if !ok || len(shot.Data) == 0 {
			message, _ := payload["screenshotError"].(string)
			if strings.TrimSpace(message) == "" {
				message = "截图未能读取"
			}
			return toolError(&launchcode.ServiceError{Status: 500, Message: message}), out, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{Data: shot.Data, MIMEType: shot.MIMEType}},
		}, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_extract",
		Description: "抽取页面内容：元素的文本、HTML 或指定属性。设 all=true 可一次取回所有命中元素，" +
			"适合抓列表、表格这类重复结构。",
		Annotations: readOnly("抽取页面内容"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in extractInput) (*mcp.CallToolResult, extractOutput, error) {
		mode := strings.TrimSpace(in.Mode)
		if mode == "" {
			mode = "text"
		}
		if mode == "attribute" && strings.TrimSpace(in.Attribute) == "" {
			return toolError(newInputError("mode=attribute 时必须提供 attribute")), extractOutput{}, nil
		}

		extra := map[string]any{"mode": mode}
		if attribute := strings.TrimSpace(in.Attribute); attribute != "" {
			extra["attribute"] = attribute
		}
		if in.All {
			extra["all"] = true
		}

		result, err := pageCall(p, in.Selector, in.TimeoutMs, "extract", in.args(extra))
		if err != nil {
			return toolError(err), extractOutput{}, nil
		}
		payload, stepErr := firstStep(result)
		if stepErr != nil {
			return toolError(stepErr), extractOutput{ProfileID: result.ProfileID}, nil
		}

		out := extractOutput{ProfileID: result.ProfileID, OK: true, Mode: mode}
		if value, ok := payload["value"].(string); ok {
			out.Value = value
		}
		if values, ok := payload["values"].([]any); ok {
			out.Values = decodeStrings(values)
			out.Count = len(out.Values)
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_evaluate",
		Description: "在页面上下文里执行 JavaScript 并返回结果。" +
			"用于快照和抽取覆盖不到的场景，例如读取 window 上的变量或触发自定义事件。" +
			"expression 可以是表达式，也可以是接收 arg 的箭头函数。",
		Annotations: mutating("执行页面脚本"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in evaluateInput) (*mcp.CallToolResult, evaluateOutput, error) {
		expression := strings.TrimSpace(in.Expression)
		if expression == "" {
			return toolError(newInputError("expression 不能为空")), evaluateOutput{}, nil
		}

		args := map[string]any{"expression": expression}
		if in.Arg != nil {
			args["arg"] = in.Arg
		}

		result, err := pageCall(p, in.Selector, in.TimeoutMs, "evaluate", args)
		if err != nil {
			return toolError(err), evaluateOutput{}, nil
		}
		payload, stepErr := firstStep(result)
		if stepErr != nil {
			return toolError(stepErr), evaluateOutput{ProfileID: result.ProfileID}, nil
		}

		return nil, evaluateOutput{
			ProfileID: result.ProfileID,
			OK:        true,
			Value:     payload["value"],
		}, nil
	})
}

func registerPageSessionTools(srv *mcp.Server, p PageProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_page_release",
		Description: "释放实例的常驻页面会话，断开 CDP 连接。浏览器本身不会关闭，页面也保持原样。" +
			"操作完成后调用可以立即回收资源；不调用的话空闲一段时间后也会自动回收。",
		Annotations: destructive("释放页面会话"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in releaseInput) (*mcp.CallToolResult, releaseOutput, error) {
		var selector launchcode.LaunchSelector
		if in.Selector != nil {
			selector = in.Selector.toLaunchSelector()
		}

		profileID, err := p.ClosePageSession(selector)
		if err != nil {
			return toolError(err), releaseOutput{}, nil
		}
		return nil, releaseOutput{ProfileID: profileID, Released: true}, nil
	})
}

// ---------------------------------------------------------------------------
// 解码辅助
//
// Node 侧回传的是 map[string]any，这里做一次窄化。用 json 往返而不是逐字段
// 断言，是因为元素字段较多且可选，手写断言又长又容易漏。
// ---------------------------------------------------------------------------

func decodeElements(items []any) []PageElement {
	out := make([]PageElement, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var element PageElement
		if err := json.Unmarshal(data, &element); err != nil {
			continue
		}
		out = append(out, element)
	}
	return out
}

func decodeTabs(items []any) []PageTab {
	out := make([]PageTab, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var tab PageTab
		if err := json.Unmarshal(data, &tab); err != nil {
			continue
		}
		out = append(out, tab)
	}
	return out
}

func decodeStrings(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
