package mcpserver

import (
	"context"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Selector 是暴露给模型的实例选择条件。
//
// 有意只保留 launchcode.LaunchSelector 中语义清晰的字段：内部的
// Keyword/Tag 单数形式是历史兼容项，与复数形式重复，会让模型困惑。
type Selector struct {
	Code        string   `json:"code,omitempty" jsonschema:"快捷启动码，最精确的定位方式"`
	ProfileID   string   `json:"profileId,omitempty" jsonschema:"实例的稳定 ID"`
	ProfileName string   `json:"profileName,omitempty" jsonschema:"实例名称，需完全匹配（忽略大小写）"`
	Keywords    []string `json:"keywords,omitempty" jsonschema:"关键字，全部命中才算匹配"`
	Tags        []string `json:"tags,omitempty" jsonschema:"标签，全部命中才算匹配"`
	GroupID     string   `json:"groupId,omitempty" jsonschema:"分组 ID"`
	MatchMode   string   `json:"matchMode,omitempty" jsonschema:"命中多个实例时的处理方式：unique 报错（默认），first 取第一个"`
}

func (s Selector) toLaunchSelector() launchcode.LaunchSelector {
	return launchcode.LaunchSelector{
		Code:        strings.TrimSpace(s.Code),
		ProfileID:   strings.TrimSpace(s.ProfileID),
		ProfileName: strings.TrimSpace(s.ProfileName),
		Keywords:    s.Keywords,
		Tags:        s.Tags,
		GroupID:     strings.TrimSpace(s.GroupID),
		MatchMode:   strings.TrimSpace(s.MatchMode),
	}
}

// InstanceSummary 是实例的精简视图。
//
// 刻意不返回完整的 browser.Profile：它有 30+ 字段，其中启动参数、
// 指纹参数、代理绑定来源等对模型决策没有帮助，只会挤占上下文。
// 需要完整配置时用 ant_instance_get。
type InstanceSummary struct {
	ProfileID   string   `json:"profileId"`
	ProfileName string   `json:"profileName"`
	LaunchCode  string   `json:"launchCode"`
	GroupID     string   `json:"groupId,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	ProxyID     string   `json:"proxyId,omitempty"`
	CoreID      string   `json:"coreId,omitempty"`
	Running     bool     `json:"running"`
	DebugReady  bool     `json:"debugReady"`
}

func toSummary(p *browser.Profile) InstanceSummary {
	if p == nil {
		return InstanceSummary{}
	}
	return InstanceSummary{
		ProfileID:   p.ProfileId,
		ProfileName: p.ProfileName,
		LaunchCode:  p.LaunchCode,
		GroupID:     p.GroupId,
		Tags:        p.Tags,
		Keywords:    p.Keywords,
		ProxyID:     p.ProxyId,
		CoreID:      p.CoreId,
		Running:     p.Running,
		DebugReady:  p.DebugReady,
	}
}

func toSummaries(items []browser.Profile) []InstanceSummary {
	out := make([]InstanceSummary, 0, len(items))
	for i := range items {
		out = append(out, toSummary(&items[i]))
	}
	return out
}

// RuntimeState 描述实例的运行态。
type RuntimeState struct {
	ProfileID      string `json:"profileId"`
	ProfileName    string `json:"profileName"`
	LaunchCode     string `json:"launchCode,omitempty"`
	Running        bool   `json:"running"`
	DebugReady     bool   `json:"debugReady"`
	Pid            int    `json:"pid,omitempty"`
	DebugPort      int    `json:"debugPort,omitempty"`
	RuntimeWarning string `json:"runtimeWarning,omitempty"`
	LastError      string `json:"lastError,omitempty"`
}

func toRuntimeState(p *browser.Profile) RuntimeState {
	if p == nil {
		return RuntimeState{}
	}
	return RuntimeState{
		ProfileID:      p.ProfileId,
		ProfileName:    p.ProfileName,
		LaunchCode:     p.LaunchCode,
		Running:        p.Running,
		DebugReady:     p.DebugReady,
		Pid:            p.Pid,
		DebugPort:      p.DebugPort,
		RuntimeWarning: p.RuntimeWarning,
		LastError:      p.LastError,
	}
}

func registerInstanceTools(srv *mcp.Server, p InstanceProvider) {
	registerInstanceQueryTools(srv, p)
	registerInstanceWriteTools(srv, p)
	registerRuntimeTools(srv, p)
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

type listInstancesInput struct {
	Tag         string `json:"tag,omitempty" jsonschema:"只返回带该标签的实例"`
	GroupID     string `json:"groupId,omitempty" jsonschema:"只返回该分组下的实例"`
	Keyword     string `json:"keyword,omitempty" jsonschema:"按名称或关键字做子串匹配（忽略大小写）"`
	RunningOnly bool   `json:"runningOnly,omitempty" jsonschema:"只返回正在运行的实例"`
}

type listInstancesOutput struct {
	Count int               `json:"count"`
	Items []InstanceSummary `json:"items"`
}

type getInstanceInput struct {
	Selector Selector `json:"selector" jsonschema:"实例选择条件"`
}

type getInstanceOutput struct {
	Instance InstanceSummary  `json:"instance"`
	Profile  *browser.Profile `json:"profile" jsonschema:"实例的完整配置"`
}

func registerInstanceQueryTools(srv *mcp.Server, p InstanceProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_list",
		Description: "列出浏览器实例，可按标签、分组、关键字和运行状态过滤。返回精简信息；需要完整配置请用 ant_instance_get。",
		Annotations: readOnly("列出实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listInstancesInput) (*mcp.CallToolResult, listInstancesOutput, error) {
		items, err := p.ListProfiles()
		if err != nil {
			return toolError(err), listInstancesOutput{}, nil
		}

		filtered := filterInstances(items, in)
		return nil, listInstancesOutput{Count: len(filtered), Items: toSummaries(filtered)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_get",
		Description: "按 selector 查询单个实例的完整配置。selector 命中多个实例时报错，除非设置 matchMode=first。",
		Annotations: readOnly("查询实例详情"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getInstanceInput) (*mcp.CallToolResult, getInstanceOutput, error) {
		profile, err := p.FindProfile(in.Selector.toLaunchSelector())
		if err != nil {
			return toolError(err), getInstanceOutput{}, nil
		}
		return nil, getInstanceOutput{Instance: toSummary(profile), Profile: profile}, nil
	})
}

// filterInstances 在内存里做过滤。
//
// 没有复用 findProfilesBySelector 是因为语义不同：selector 是「精确定位单个目标」，
// 命中多个会报歧义错误；而列表查询要的是「按条件筛选一批」，关键字也应支持子串匹配。
func filterInstances(items []browser.Profile, in listInstancesInput) []browser.Profile {
	tag := strings.TrimSpace(in.Tag)
	groupID := strings.TrimSpace(in.GroupID)
	keyword := strings.ToLower(strings.TrimSpace(in.Keyword))

	out := make([]browser.Profile, 0, len(items))
	for _, item := range items {
		if in.RunningOnly && !item.Running {
			continue
		}
		if groupID != "" && strings.TrimSpace(item.GroupId) != groupID {
			continue
		}
		if tag != "" && !containsFold(item.Tags, tag) {
			continue
		}
		if keyword != "" && !matchesKeyword(item, keyword) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func matchesKeyword(item browser.Profile, keyword string) bool {
	if strings.Contains(strings.ToLower(item.ProfileName), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(item.LaunchCode), keyword) {
		return true
	}
	for _, value := range item.Keywords {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 写操作
// ---------------------------------------------------------------------------

// InstanceConfig 是创建/更新实例时可设置的字段。
//
// 与 browser.ProfileInput 相比省略了 fingerprintArgs 和 launchArgs：
// 这两个是底层 Chrome 命令行参数，写错会让实例起不来，不适合让模型自由填写。
// 需要调整时应通过 GUI 或直接改配置。
type InstanceConfig struct {
	ProfileName   string   `json:"profileName" jsonschema:"实例名称"`
	UserDataDir   string   `json:"userDataDir,omitempty" jsonschema:"用户数据目录，相对路径基于数据根目录；留空自动生成"`
	CoreID        string   `json:"coreId,omitempty" jsonschema:"浏览器内核 ID，留空使用默认内核"`
	ProxyID       string   `json:"proxyId,omitempty" jsonschema:"绑定的代理节点 ID"`
	GroupID       string   `json:"groupId,omitempty" jsonschema:"所属分组 ID"`
	Tags          []string `json:"tags,omitempty" jsonschema:"标签"`
	Keywords      []string `json:"keywords,omitempty" jsonschema:"检索关键字"`
	MemoryLimitMB int      `json:"memoryLimitMb,omitempty" jsonschema:"实例最大内存（MB），0 表示不限制"`
}

func (c InstanceConfig) toProfileInput() browser.ProfileInput {
	return browser.ProfileInput{
		ProfileName:   strings.TrimSpace(c.ProfileName),
		UserDataDir:   strings.TrimSpace(c.UserDataDir),
		CoreId:        strings.TrimSpace(c.CoreID),
		ProxyId:       strings.TrimSpace(c.ProxyID),
		GroupId:       strings.TrimSpace(c.GroupID),
		Tags:          c.Tags,
		Keywords:      c.Keywords,
		MemoryLimitMB: c.MemoryLimitMB,
	}
}

type createInstanceInput struct {
	Config     InstanceConfig `json:"config" jsonschema:"实例配置"`
	LaunchCode string         `json:"launchCode,omitempty" jsonschema:"自定义快捷启动码，留空自动分配"`
}

type createInstanceOutput struct {
	Instance InstanceSummary `json:"instance"`
}

type updateInstanceInput struct {
	Selector   Selector       `json:"selector" jsonschema:"要更新的实例"`
	Config     InstanceConfig `json:"config" jsonschema:"新的实例配置。注意这是整体替换，未填写的字段会被清空"`
	LaunchCode string         `json:"launchCode,omitempty" jsonschema:"自定义快捷启动码，留空保持不变"`
}

type deleteInstanceInput struct {
	Selector Selector `json:"selector" jsonschema:"要删除的实例。运行中的实例不能删除，需先调用 ant_instance_stop"`
}

type deleteInstanceOutput struct {
	Deleted     bool   `json:"deleted"`
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
}

func registerInstanceWriteTools(srv *mcp.Server, p InstanceProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_create",
		Description: "创建一个新的浏览器实例。",
		Annotations: mutating("创建实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in createInstanceInput) (*mcp.CallToolResult, createInstanceOutput, error) {
		profile, launchCode, err := p.CreateProfile(in.Config.toProfileInput(), strings.TrimSpace(in.LaunchCode))
		if err != nil {
			return toolError(err), createInstanceOutput{}, nil
		}

		summary := toSummary(profile)
		summary.LaunchCode = launchCode
		return nil, createInstanceOutput{Instance: summary}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_update",
		Description: "更新实例配置。config 是整体替换而非增量合并，建议先用 ant_instance_get 读出当前配置再改。",
		Annotations: mutating("更新实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in updateInstanceInput) (*mcp.CallToolResult, createInstanceOutput, error) {
		target, err := p.FindProfile(in.Selector.toLaunchSelector())
		if err != nil {
			return toolError(err), createInstanceOutput{}, nil
		}

		profile, launchCode, err := p.UpdateProfile(target.ProfileId, in.Config.toProfileInput(), strings.TrimSpace(in.LaunchCode))
		if err != nil {
			return toolError(err), createInstanceOutput{}, nil
		}

		summary := toSummary(profile)
		summary.LaunchCode = launchCode
		return nil, createInstanceOutput{Instance: summary}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_delete",
		Description: "删除实例。这会移除实例配置，运行中的实例必须先停止。",
		Annotations: destructive("删除实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in deleteInstanceInput) (*mcp.CallToolResult, deleteInstanceOutput, error) {
		target, err := p.FindProfile(in.Selector.toLaunchSelector())
		if err != nil {
			return toolError(err), deleteInstanceOutput{}, nil
		}
		if err := p.DeleteProfile(target.ProfileId); err != nil {
			return toolError(err), deleteInstanceOutput{}, nil
		}
		return nil, deleteInstanceOutput{
			Deleted:     true,
			ProfileID:   target.ProfileId,
			ProfileName: target.ProfileName,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// 运行时
// ---------------------------------------------------------------------------

type startInstanceInput struct {
	Selector             Selector `json:"selector" jsonschema:"要启动的实例"`
	StartURLs            []string `json:"startUrls,omitempty" jsonschema:"本次启动额外打开的地址"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls,omitempty" jsonschema:"是否跳过实例配置的默认启动地址"`
	ProxyID              string   `json:"proxyId,omitempty" jsonschema:"本次启动临时使用的代理，不会修改实例的代理绑定"`
}

type startInstanceOutput struct {
	Runtime RuntimeState `json:"runtime"`
}

type runtimeSessionInput struct {
	Selector             Selector `json:"selector" jsonschema:"要接管的实例"`
	StartURLs            []string `json:"startUrls,omitempty" jsonschema:"本次启动额外打开的地址"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls,omitempty" jsonschema:"是否跳过实例配置的默认启动地址"`
	TimeoutMs            int      `json:"timeoutMs,omitempty" jsonschema:"等待 CDP 就绪的超时毫秒数，默认 45000，上限 120000"`
}

type runtimeSessionOutput struct {
	Ready   bool         `json:"ready" jsonschema:"为 true 时才可以使用 cdpUrl 接管浏览器"`
	CDPURL  string       `json:"cdpUrl,omitempty" jsonschema:"统一 CDP 入口地址，交给浏览器自动化工具使用"`
	Runtime RuntimeState `json:"runtime"`
	Hint    string       `json:"hint,omitempty" jsonschema:"未就绪时的后续建议"`
}

type stopInstanceInput struct {
	Selector Selector `json:"selector" jsonschema:"要停止的实例"`
}

type stopInstanceOutput struct {
	Stopped bool         `json:"stopped"`
	Runtime RuntimeState `json:"runtime"`
}

type runtimeStatusInput struct {
	Selector Selector `json:"selector" jsonschema:"要查询的实例"`
}

type runtimeStatusOutput struct {
	Runtime RuntimeState `json:"runtime"`
}

type activeSessionOutput struct {
	Active  bool         `json:"active"`
	CDPURL  string       `json:"cdpUrl,omitempty"`
	Runtime RuntimeState `json:"runtime,omitzero"`
}

func registerRuntimeTools(srv *mcp.Server, p InstanceProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_start",
		Description: "启动实例但不等待调试端口就绪。如果接下来要用 CDP 接管浏览器，请改用 ant_runtime_session。",
		Annotations: mutating("启动实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in startInstanceInput) (*mcp.CallToolResult, startInstanceOutput, error) {
		profile, launchCode, err := p.StartProfile(in.Selector.toLaunchSelector(), launchcode.LaunchRequestParams{
			StartURLs:            in.StartURLs,
			SkipDefaultStartURLs: in.SkipDefaultStartURLs,
			ProxyId:              strings.TrimSpace(in.ProxyID),
		})
		if err != nil {
			return toolError(err), startInstanceOutput{}, nil
		}

		state := toRuntimeState(profile)
		if state.LaunchCode == "" {
			state.LaunchCode = launchCode
		}
		return nil, startInstanceOutput{Runtime: state}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_runtime_session",
		Description: "启动实例并等待其可被 CDP 接管，返回统一 CDP 入口地址。" +
			"这是把浏览器交给自动化工具前的推荐入口。等待超时不算失败，会返回 ready=false，可稍后重试。" +
			"注意同一时刻只有一个实例挂在统一入口上。",
		Annotations: mutating("接管实例会话"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in runtimeSessionInput) (*mcp.CallToolResult, runtimeSessionOutput, error) {
		session, err := p.OpenRuntimeSession(
			in.Selector.toLaunchSelector(),
			launchcode.LaunchRequestParams{
				StartURLs:            in.StartURLs,
				SkipDefaultStartURLs: in.SkipDefaultStartURLs,
			},
			time.Duration(in.TimeoutMs)*time.Millisecond,
		)
		if err != nil {
			return toolError(err), runtimeSessionOutput{}, nil
		}

		out := runtimeSessionOutput{
			Ready:   session.Ready,
			CDPURL:  session.CDPURL,
			Runtime: toRuntimeState(session.Profile),
		}
		if !session.Ready {
			out.Hint = "浏览器已启动但调试端口尚未就绪，请稍后重试本工具，或用 ant_runtime_status 观察状态。"
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_runtime_status",
		Description: "查询实例当前运行态，不会触发启动。",
		Annotations: readOnly("查询运行态"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in runtimeStatusInput) (*mcp.CallToolResult, runtimeStatusOutput, error) {
		target, err := p.FindProfile(in.Selector.toLaunchSelector())
		if err != nil {
			return toolError(err), runtimeStatusOutput{}, nil
		}

		profile, err := p.StatusProfile(target.ProfileId)
		if err != nil {
			return toolError(err), runtimeStatusOutput{}, nil
		}
		return nil, runtimeStatusOutput{Runtime: toRuntimeState(profile)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_runtime_active",
		Description: "查询当前挂在统一 CDP 入口上的实例。切换接管目标前建议先确认这里。",
		Annotations: readOnly("查询活动会话"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, activeSessionOutput, error) {
		session, err := p.ActiveRuntimeSession()
		if err != nil {
			return toolError(err), activeSessionOutput{}, nil
		}
		if session == nil {
			return nil, activeSessionOutput{Active: false}, nil
		}
		return nil, activeSessionOutput{
			Active:  true,
			CDPURL:  session.CDPURL,
			Runtime: toRuntimeState(session.Profile),
		}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_instance_stop",
		Description: "停止运行中的实例。这会关闭真实的浏览器进程。",
		Annotations: destructive("停止实例"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in stopInstanceInput) (*mcp.CallToolResult, stopInstanceOutput, error) {
		target, err := p.FindProfile(in.Selector.toLaunchSelector())
		if err != nil {
			return toolError(err), stopInstanceOutput{}, nil
		}

		profile, err := p.StopProfile(target.ProfileId)
		if err != nil {
			return toolError(err), stopInstanceOutput{}, nil
		}
		return nil, stopInstanceOutput{Stopped: true, Runtime: toRuntimeState(profile)}, nil
	})
}
