package mcpserver

import (
	"strings"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 页面操作工具：让模型直接驱动浏览器，而不是只能执行预先写好的脚本。
//
// 每个工具在协议层是独立的一次调用，但底层复用同一个常驻会话——CDP 握手只在
// 首次发生，后续调用只是往管道里写一行指令。因此工具粒度可以按「模型好理解」
// 来切，不必为了省启动开销把动作揉在一起。
//
// 所有工具的 selector 都可省略：省略时作用于当前挂在统一 CDP 入口上的实例，
// 这也是连续操作同一个页面时的常态。

// PageState 是每次页面操作后回传的页面状态。
// 统一带上 url/title，模型不必为了确认「现在在哪」再多调一次快照。
type PageState struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

type pageOutput struct {
	ProfileID string    `json:"profileId"`
	OK        bool      `json:"ok"`
	Page      PageState `json:"page"`
	Error     string    `json:"error,omitempty"`
}

// pageCall 是所有页面工具的公共执行路径。
//
// 把 selector 解析、单步指令下发、结果拆包收敛到一处，各工具只负责
// 把自己的入参翻译成一条指令的 args。
func pageCall(p PageProvider, selector *Selector, timeoutMs int, action string, args map[string]any) (*launchcode.PageResult, error) {
	req := launchcode.PageRequest{
		Steps:     []launchcode.PageStep{{Action: action, Args: args}},
		TimeoutMs: timeoutMs,
	}
	if selector != nil {
		req.Selector = selector.toLaunchSelector()
	}
	return p.RunPageSteps(req)
}

// firstStep 取出单步调用的结果。
//
// 会话级失败（连不上、进程断了）和指令级失败（选择器没命中）对模型来说
// 都是「这一步没成」，但错误文案来源不同，这里统一成一个 error。
func firstStep(result *launchcode.PageResult) (map[string]any, error) {
	if result == nil {
		return nil, &launchcode.ServiceError{Status: 500, Message: "页面操作没有返回结果"}
	}
	if len(result.Steps) == 0 {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "页面操作没有返回结果"
		}
		return nil, &launchcode.ServiceError{Status: 500, Message: message}
	}

	step := result.Steps[0]
	if !step.OK {
		message := strings.TrimSpace(step.Error)
		if message == "" {
			message = "页面操作失败"
		}
		return step.Result, &launchcode.ServiceError{Status: 500, Message: message}
	}
	return step.Result, nil
}

func pageStateOf(payload map[string]any) PageState {
	if payload == nil {
		return PageState{}
	}
	url, _ := payload["url"].(string)
	title, _ := payload["title"].(string)
	return PageState{URL: url, Title: title}
}

// runPageAction 执行一条指令并组装标准的 pageOutput。
func runPageAction(p PageProvider, selector *Selector, timeoutMs int, action string, args map[string]any) (*mcp.CallToolResult, pageOutput, map[string]any) {
	result, err := pageCall(p, selector, timeoutMs, action, args)
	if err != nil {
		return toolError(err), pageOutput{}, nil
	}

	payload, stepErr := firstStep(result)
	if stepErr != nil {
		return toolError(stepErr), pageOutput{ProfileID: result.ProfileID, Error: stepErr.Error()}, payload
	}

	return nil, pageOutput{
		ProfileID: result.ProfileID,
		OK:        true,
		Page:      pageStateOf(payload),
	}, payload
}

// ---------------------------------------------------------------------------
// 入参 / 出参
// ---------------------------------------------------------------------------

// targetInput 是所有页面工具共享的目标与超时参数。
type targetInput struct {
	Selector  *Selector `json:"selector,omitempty" jsonschema:"目标实例；省略则作用于当前活动实例"`
	TimeoutMs int       `json:"timeoutMs,omitempty" jsonschema:"单步超时毫秒数，默认 30000，上限 300000"`
}

// elementInput 是需要定位元素的工具共享的参数。
type elementInput struct {
	targetInput
	Element       string `json:"element" jsonschema:"元素选择器，支持 CSS 以及 text= / role= 等 Playwright 选择器；建议直接用 ant_page_snapshot 返回的 selector"`
	FrameSelector string `json:"frameSelector,omitempty" jsonschema:"元素在 iframe 内时，指向该 iframe 的选择器"`
	Index         *int   `json:"index,omitempty" jsonschema:"选择器命中多个元素时取第几个，从 0 开始；省略取第一个"`
}

func (in elementInput) args(extra map[string]any) map[string]any {
	args := map[string]any{"selector": strings.TrimSpace(in.Element)}
	if frame := strings.TrimSpace(in.FrameSelector); frame != "" {
		args["frameSelector"] = frame
	}
	if in.Index != nil {
		args["index"] = *in.Index
	}
	for key, value := range extra {
		args[key] = value
	}
	return args
}

type gotoInput struct {
	targetInput
	URL       string `json:"url" jsonschema:"要打开的完整 URL"`
	WaitUntil string `json:"waitUntil,omitempty" jsonschema:"等待时机：domcontentloaded（默认）/ load / networkidle / commit"`
}

type gotoOutput struct {
	pageOutput
	Status int `json:"status,omitempty" jsonschema:"导航响应的 HTTP 状态码"`
}

type snapshotInput struct {
	targetInput
	Limit       int  `json:"limit,omitempty" jsonschema:"最多返回多少个可交互元素，默认 200，上限 500"`
	IncludeText bool `json:"includeText,omitempty" jsonschema:"是否附带页面正文文本，默认否"`
}

// PageElement 是快照里的一个可交互元素。
type PageElement struct {
	Role     string `json:"role"`
	Tag      string `json:"tag,omitempty"`
	Name     string `json:"name,omitempty" jsonschema:"元素的可读名称：aria-label、placeholder 或可见文本"`
	Selector string `json:"selector" jsonschema:"可直接回传给 ant_page_click / ant_page_fill 的选择器"`
	Type     string `json:"type,omitempty"`
	Value    string `json:"value,omitempty"`
	Href     string `json:"href,omitempty"`
	Checked  *bool  `json:"checked,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type snapshotOutput struct {
	pageOutput
	Elements  []PageElement `json:"elements"`
	Text      string        `json:"text,omitempty"`
	Truncated bool          `json:"truncated,omitempty" jsonschema:"元素数量达到上限被截断"`
}

type screenshotInput struct {
	targetInput
	Element  string `json:"element,omitempty" jsonschema:"只截取该元素；省略则截整个视口"`
	FullPage bool   `json:"fullPage,omitempty" jsonschema:"截取完整页面而非视口，默认否；长页面会显著变大"`
	Type     string `json:"type,omitempty" jsonschema:"图片格式：jpeg（默认，体积小）或 png"`
	Quality  int    `json:"quality,omitempty" jsonschema:"jpeg 质量 1-100，默认 60"`
}

type clickInput struct {
	elementInput
	Button     string `json:"button,omitempty" jsonschema:"鼠标键：left（默认）/ right / middle"`
	ClickCount int    `json:"clickCount,omitempty" jsonschema:"点击次数，默认 1"`
	Force      bool   `json:"force,omitempty" jsonschema:"跳过可点击性检查，默认否"`
}

// FillField 是一个待填字段。
type FillField struct {
	Element       string `json:"element" jsonschema:"输入框选择器"`
	Value         string `json:"value" jsonschema:"要填入的值"`
	FrameSelector string `json:"frameSelector,omitempty" jsonschema:"字段在 iframe 内时指向该 iframe 的选择器"`
}

type fillInput struct {
	targetInput
	Fields []FillField `json:"fields" jsonschema:"要填写的字段列表；一次传多个可以在一轮里填完整张表单"`
}

type fillOutput struct {
	pageOutput
	Filled []string `json:"filled" jsonschema:"实际填写的选择器列表"`
}

type pressInput struct {
	targetInput
	Key           string `json:"key" jsonschema:"按键名，例如 Enter、Escape、Tab、Control+A"`
	Element       string `json:"element,omitempty" jsonschema:"先聚焦到该元素再按键；省略则作用于当前焦点"`
	FrameSelector string `json:"frameSelector,omitempty"`
}

type selectInput struct {
	elementInput
	Values []string `json:"values" jsonschema:"要选中的选项值；单选传一个元素的数组"`
}

type selectOutput struct {
	pageOutput
	Selected []string `json:"selected" jsonschema:"实际选中的选项值"`
}

type waitInput struct {
	targetInput
	Element   string `json:"element,omitempty" jsonschema:"等待该元素达到目标状态"`
	State     string `json:"state,omitempty" jsonschema:"元素目标状态：visible（默认）/ hidden / attached / detached"`
	URL       string `json:"url,omitempty" jsonschema:"等待 URL 匹配该值，支持 glob 通配"`
	LoadState string `json:"loadState,omitempty" jsonschema:"等待加载态：load / domcontentloaded / networkidle"`
}

type extractInput struct {
	elementInput
	Mode      string `json:"mode,omitempty" jsonschema:"抽取内容：text（默认）/ html / attribute"`
	Attribute string `json:"attribute,omitempty" jsonschema:"要读取的属性名，仅在 mode 为 attribute 时使用"`
	All       bool   `json:"all,omitempty" jsonschema:"抽取所有命中元素而非第一个，默认否"`
}

type extractOutput struct {
	ProfileID string   `json:"profileId"`
	OK        bool     `json:"ok"`
	Mode      string   `json:"mode"`
	Value     string   `json:"value,omitempty" jsonschema:"单个结果，仅在 all 为 false 时返回"`
	Values    []string `json:"values,omitempty" jsonschema:"全部结果，仅在 all 为 true 时返回"`
	Count     int      `json:"count,omitempty"`
}

type evaluateInput struct {
	targetInput
	Expression string `json:"expression" jsonschema:"在页面上下文里执行的 JS：表达式或箭头函数，例如 () => document.title"`
	Arg        any    `json:"arg,omitempty" jsonschema:"传给箭头函数的参数，必须可 JSON 序列化"`
}

type evaluateOutput struct {
	ProfileID string `json:"profileId"`
	OK        bool   `json:"ok"`
	Value     any    `json:"value" jsonschema:"表达式的返回值；不可序列化的部分会被转成字符串描述"`
}

type tabsInput struct {
	targetInput
	Op    string `json:"op,omitempty" jsonschema:"操作：list（默认）/ new / select / close"`
	Index *int   `json:"index,omitempty" jsonschema:"select / close 的目标标签页序号，从 0 开始"`
	URL   string `json:"url,omitempty" jsonschema:"新标签页要打开的 URL，仅在 op 为 new 时使用"`
}

// PageTab 是一个标签页。
type PageTab struct {
	Index  int    `json:"index"`
	URL    string `json:"url,omitempty"`
	Title  string `json:"title,omitempty"`
	Active bool   `json:"active,omitempty"`
}

type tabsOutput struct {
	ProfileID string    `json:"profileId"`
	OK        bool      `json:"ok"`
	Count     int       `json:"count"`
	Items     []PageTab `json:"items,omitempty"`
	Page      PageState `json:"page,omitempty"`
}

type releaseInput struct {
	Selector *Selector `json:"selector,omitempty" jsonschema:"目标实例；省略则释放当前活动实例的会话"`
}

type releaseOutput struct {
	ProfileID string `json:"profileId"`
	Released  bool   `json:"released"`
}
