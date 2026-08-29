package mcpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 错误呈现策略。
//
// 现有 HTTP 层用状态码表达失败语义（404 找不到、409 selector 歧义、
// 503 能力未注入），但 agent 看不懂裸 HTTP 码。这里把状态码翻译成
// 可操作的自然语言提示，告诉模型「下一步该怎么做」，而不是「出了什么错」。
//
// 原始状态码仍通过 structured content 透出，供需要精确判断的调用方使用。

// toolError 把服务层错误转换为 MCP 工具错误结果。
//
// 返回 nil error：MCP 约定工具级失败要放进 CallToolResult.IsError，
// 而不是作为协议级错误抛出——后者会让客户端认为服务不可用。
func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: describeError(err)}},
	}
}

func describeError(err error) string {
	if err == nil {
		return "unknown error"
	}

	var svcErr *launchcode.ServiceError
	if !errors.As(err, &svcErr) {
		return err.Error()
	}

	switch {
	case svcErr.NotFound():
		return fmt.Sprintf("%s。请先用 ant_instance_list 确认目标是否存在。", svcErr.Message)
	case svcErr.Ambiguous():
		// 409 同时用于 selector 歧义和状态冲突（例如删除运行中实例），
		// 靠消息内容区分，避免给出误导性的修复建议。
		if strings.Contains(svcErr.Message, "matched") || strings.Contains(svcErr.Message, "ambiguous") {
			return fmt.Sprintf("%s。请补充更精确的条件（code / profileId），或显式设置 matchMode=first。", svcErr.Message)
		}
		return svcErr.Message
	case svcErr.Unavailable():
		return fmt.Sprintf("%s。该能力在当前运行环境不可用。", svcErr.Message)
	case svcErr.Status == http.StatusBadRequest:
		return fmt.Sprintf("参数无效：%s", svcErr.Message)
	default:
		return svcErr.Message
	}
}

// statusOf 提取错误对应的 HTTP 状态码，非服务层错误统一按 500 处理。
func statusOf(err error) int {
	var svcErr *launchcode.ServiceError
	if errors.As(err, &svcErr) {
		return svcErr.Status
	}
	return http.StatusInternalServerError
}

// newInputError 构造参数校验失败错误。
// SDK 会按 schema 做类型校验，这里处理的是 schema 表达不了的约束
// （例如「必须是 JSON 对象」）。
func newInputError(message string) error {
	return &launchcode.ServiceError{Status: http.StatusBadRequest, Message: message}
}

// errorFromRun 把脚本自身的执行失败转成 error，用于生成错误文案。
func errorFromRun(run ScriptRun) error {
	message := strings.TrimSpace(run.Error)
	if message == "" {
		message = strings.TrimSpace(run.Summary)
	}
	if message == "" {
		message = "脚本执行失败但未返回错误信息"
	}
	return &launchcode.ServiceError{
		Status:  http.StatusInternalServerError,
		Message: "脚本执行失败：" + message,
	}
}

// readOnly 标注不修改任何状态的工具。
func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:        title,
		ReadOnlyHint: true,
	}
}

// mutating 标注会修改状态、但不销毁数据的工具（创建、更新、启动）。
func mutating(title string) *mcp.ToolAnnotations {
	no := false
	return &mcp.ToolAnnotations{
		Title:           title,
		DestructiveHint: &no,
	}
}

// destructive 标注会销毁数据或中断运行的工具（删除、停止）。
// 客户端通常会对这类工具要求用户二次确认。
func destructive(title string) *mcp.ToolAnnotations {
	yes := true
	return &mcp.ToolAnnotations{
		Title:           title,
		DestructiveHint: &yes,
	}
}
