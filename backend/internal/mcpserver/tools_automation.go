package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/automation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ScriptSummary 是自动化脚本的精简视图。
// 不包含 scriptText——脚本正文动辄上千行，对选择脚本没有帮助。
type ScriptSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type" jsonschema:"脚本类型，playwright-cdp 或 launch-api"`
	Status      string   `json:"status" jsonschema:"脚本状态：draft / ready / disabled"`
	Tags        []string `json:"tags,omitempty"`
}

func toScriptSummary(r automation.ScriptRecord) ScriptSummary {
	return ScriptSummary{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Type:        r.Type,
		Status:      r.Status,
		Tags:        r.Tags,
	}
}

// ScriptDetail 在精简视图基础上补充默认选择器与参数。
// 这两项决定「不传 selector/params 时会发生什么」，是调用前必须知道的信息。
type ScriptDetail struct {
	ScriptSummary
	Notes           string         `json:"notes,omitempty"`
	EntryFile       string         `json:"entryFile,omitempty"`
	TargetMode      string         `json:"targetMode,omitempty" jsonschema:"目标绑定方式：manual / existing / rotate / create"`
	DefaultParams   map[string]any `json:"defaultParams,omitempty" jsonschema:"脚本自带的默认参数"`
	DefaultSelector map[string]any `json:"defaultSelector,omitempty" jsonschema:"脚本自带的默认目标实例"`
	CreatedAt       string         `json:"createdAt,omitempty"`
	UpdatedAt       string         `json:"updatedAt,omitempty"`
}

// decodeJSONObject 把存储层的 JSON 文本解析成对象。
// 内容为空或不是合法 JSON 对象时返回 nil 而不是报错——
// 脚本元数据损坏不该让整个查询失败。
func decodeJSONObject(text string) map[string]any {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil
	}
	return out
}

// ScriptRun 是一次脚本执行的结果。
type ScriptRun struct {
	ID         string `json:"id"`
	ScriptID   string `json:"scriptId"`
	ScriptName string `json:"scriptName,omitempty"`
	Status     string `json:"status" jsonschema:"执行状态：success 或 failed"`
	Summary    string `json:"summary,omitempty" jsonschema:"脚本自述的执行结果摘要"`
	Error      string `json:"error,omitempty"`
	ResultText string `json:"resultText,omitempty" jsonschema:"脚本返回的结构化结果，通常是 JSON 文本"`
	StartedAt  string `json:"startedAt,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

func toScriptRun(r automation.ScriptRunRecord) ScriptRun {
	return ScriptRun{
		ID:         r.ID,
		ScriptID:   r.ScriptID,
		ScriptName: r.ScriptName,
		Status:     r.Status,
		Summary:    r.Summary,
		Error:      r.Error,
		ResultText: r.ResultText,
		StartedAt:  r.StartedAt,
		DurationMs: r.DurationMs,
	}
}

type listScriptsOutput struct {
	Count int             `json:"count"`
	Items []ScriptSummary `json:"items"`
}

type getScriptInput struct {
	ScriptID string `json:"scriptId" jsonschema:"脚本 ID"`
}

type getScriptOutput struct {
	Script ScriptDetail `json:"script"`
}

type runScriptInput struct {
	ScriptID  string         `json:"scriptId" jsonschema:"要执行的脚本 ID"`
	Selector  *Selector      `json:"selector,omitempty" jsonschema:"覆盖脚本默认的目标实例；留空则使用脚本自带的目标"`
	Params    map[string]any `json:"params,omitempty" jsonschema:"覆盖脚本默认参数的 JSON 对象；留空则使用脚本自带参数"`
	TimeoutMs int            `json:"timeoutMs,omitempty" jsonschema:"执行超时毫秒数，默认 300000，范围 1000-1800000"`
}

type runScriptOutput struct {
	Run ScriptRun `json:"run"`
}

type listScriptRunsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"返回条数，默认 20，上限 200"`
}

type listScriptRunsOutput struct {
	Count int         `json:"count"`
	Items []ScriptRun `json:"items"`
}

func registerAutomationTools(srv *mcp.Server, p AutomationProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_script_list",
		Description: "列出已导入的自动化脚本。执行脚本前先用这个确认可用的 scriptId。",
		Annotations: readOnly("列出自动化脚本"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listScriptsOutput, error) {
		items, err := p.ListScripts()
		if err != nil {
			return toolError(err), listScriptsOutput{}, nil
		}

		out := make([]ScriptSummary, 0, len(items))
		for _, item := range items {
			out = append(out, toScriptSummary(item))
		}
		return nil, listScriptsOutput{Count: len(out), Items: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_script_get",
		Description: "查询单个脚本的详情，包含它自带的默认目标实例和默认参数。",
		Annotations: readOnly("查询脚本详情"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getScriptInput) (*mcp.CallToolResult, getScriptOutput, error) {
		record, err := p.GetScript(in.ScriptID)
		if err != nil {
			return toolError(err), getScriptOutput{}, nil
		}

		detail := ScriptDetail{
			ScriptSummary:   toScriptSummary(*record),
			Notes:           record.Notes,
			EntryFile:       record.EntryFile,
			TargetMode:      record.TargetConfig.Mode,
			DefaultParams:   decodeJSONObject(record.ParamsText),
			DefaultSelector: decodeJSONObject(record.SelectorText),
			CreatedAt:       record.CreatedAt,
			UpdatedAt:       record.UpdatedAt,
		}
		return nil, getScriptOutput{Script: detail}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "ant_script_run",
		Description: "执行自动化脚本并等待结果。脚本会在目标实例里通过 Playwright 驱动浏览器，" +
			"可能耗时较久。省略 selector 或 params 时使用脚本自带的默认值，这也是推荐用法。",
		Annotations: mutating("执行自动化脚本"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in runScriptInput) (*mcp.CallToolResult, runScriptOutput, error) {
		request, err := buildScriptRunRequest(in)
		if err != nil {
			return toolError(err), runScriptOutput{}, nil
		}

		record, err := p.RunScript(request)
		if err != nil {
			return toolError(err), runScriptOutput{}, nil
		}

		run := toScriptRun(*record)
		// 脚本自身执行失败与调用失败是两回事，但对模型来说都需要看到失败信号，
		// 因此这里也标记 IsError，同时保留完整的结构化结果供其排查。
		if strings.EqualFold(run.Status, "failed") {
			result := toolError(errorFromRun(run))
			return result, runScriptOutput{Run: run}, nil
		}
		return nil, runScriptOutput{Run: run}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_script_runs",
		Description: "查询最近的脚本执行记录。脚本执行失败后可以用它回溯原因。",
		Annotations: readOnly("查询脚本执行记录"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listScriptRunsInput) (*mcp.CallToolResult, listScriptRunsOutput, error) {
		items, err := p.ListScriptRuns(in.Limit)
		if err != nil {
			return toolError(err), listScriptRunsOutput{}, nil
		}

		out := make([]ScriptRun, 0, len(items))
		for _, item := range items {
			out = append(out, toScriptRun(item))
		}
		return nil, listScriptRunsOutput{Count: len(out), Items: out}, nil
	})
}

// buildScriptRunRequest 把工具入参翻译成执行层请求。
//
// 关键语义：selector / params 省略时置 UseScriptSelector / UseScriptParams 为 true，
// 表示「用脚本自带的默认值」——这与 HTTP API 的行为一致，见 skills 里的 API 合约。
func buildScriptRunRequest(in runScriptInput) (automation.ScriptRunRequest, error) {
	request := automation.ScriptRunRequest{
		ScriptID:  strings.TrimSpace(in.ScriptID),
		TimeoutMs: in.TimeoutMs,
	}

	if in.Selector == nil {
		request.UseScriptSelector = true
	} else {
		encoded, err := json.Marshal(in.Selector)
		if err != nil {
			return request, newInputError("selector 无法序列化：" + err.Error())
		}
		request.SelectorText = string(encoded)
	}

	if len(in.Params) == 0 {
		request.UseScriptParams = true
	} else {
		encoded, err := json.Marshal(in.Params)
		if err != nil {
			return request, newInputError("params 无法序列化：" + err.Error())
		}
		request.ParamsText = string(encoded)
	}

	return request, nil
}
