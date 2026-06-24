package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/usernamescan"

	"github.com/gorilla/websocket"
)

const (
	usernameScanProviderMock    = "mock"
	usernameScanProviderBrowser = "browser"

	defaultUsernameScanWaitAfterSubmitMs = 1200
	defaultUsernameScanTimeoutMs         = 8000
)

func (a *App) validateUsernameScanStart(request usernamescan.StartRequest) error {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" || provider == usernameScanProviderMock {
		return nil
	}
	if provider != usernameScanProviderBrowser {
		return fmt.Errorf("unsupported username scan provider: %s", request.Provider)
	}

	options := normalizeUsernameBrowserOptions(request.Browser)
	if options.ProfileID == "" {
		return fmt.Errorf("select a running browser profile")
	}
	if _, err := a.getDebugPort(options.ProfileID); err != nil {
		return err
	}
	if options.InputSelector == "" {
		return fmt.Errorf("input selector is required")
	}
	if options.AvailableText == "" && options.TakenText == "" && options.HoldText == "" {
		return fmt.Errorf("at least one result rule is required")
	}
	if options.URL != "" {
		parsed, err := url.Parse(options.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("target URL is invalid")
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
		default:
			return fmt.Errorf("target URL must use http or https")
		}
	}
	return nil
}

func (a *App) scanUsernameInBrowser(ctx context.Context, request usernamescan.StartRequest, name string) usernamescan.Result {
	options := normalizeUsernameBrowserOptions(request.Browser)
	profileID := options.ProfileID

	// 验证实例运行中（pipe 或端口）
	if _, err := a.profileCDP(profileID); err != nil {
		return usernameScanErrorResult(name, err)
	}

	if err := ctx.Err(); err != nil {
		return usernameScanErrorResult(name, err)
	}

	if options.URL != "" {
		if err := usernameScanNavigate(ctx, a, profileID, options.URL, options.TimeoutMs); err != nil {
			return usernameScanErrorResult(name, err)
		}
	}

	if err := usernameScanWaitForInput(ctx, a, profileID, options.InputSelector, options.TimeoutMs); err != nil {
		return usernameScanErrorResult(name, err)
	}

	pageText, err := usernameScanFillSubmitAndRead(ctx, a, profileID, options, name)
	if err != nil {
		return usernameScanErrorResult(name, err)
	}

	status, message := classifyUsernameBrowserResult(name, pageText, options)
	return usernamescan.Result{
		Name:      name,
		Status:    status,
		Message:   message,
		CheckedAt: time.Now().Format(time.RFC3339),
	}
}

func normalizeUsernameBrowserOptions(options usernamescan.BrowserScanOptions) usernamescan.BrowserScanOptions {
	options.ProfileID = strings.TrimSpace(options.ProfileID)
	options.URL = strings.TrimSpace(options.URL)
	options.InputSelector = strings.TrimSpace(options.InputSelector)
	options.SubmitSelector = strings.TrimSpace(options.SubmitSelector)
	options.ResultSelector = strings.TrimSpace(options.ResultSelector)
	options.AvailableText = strings.TrimSpace(options.AvailableText)
	options.TakenText = strings.TrimSpace(options.TakenText)
	options.HoldText = strings.TrimSpace(options.HoldText)
	options.WaitAfterSubmitMs = clampUsernameScanInt(options.WaitAfterSubmitMs, 100, 30000, defaultUsernameScanWaitAfterSubmitMs)
	options.TimeoutMs = clampUsernameScanInt(options.TimeoutMs, 1000, 60000, defaultUsernameScanTimeoutMs)
	return options
}

func usernameScanNavigate(ctx context.Context, a *App, profileID string, targetURL string, timeoutMs int) error {
	pc, err := a.profileCDP(profileID)
	if err != nil {
		return err
	}
	_, _ = pc.callPage("Page.enable", nil)
	if _, err := pc.callPage("Page.navigate", map[string]any{
		"url": targetURL,
	}); err != nil {
		return err
	}
	return usernameScanWaitForPageReady(ctx, a, profileID, timeoutMs)
}

func usernameScanWaitForPageReady(ctx context.Context, a *App, profileID string, timeoutMs int) error {
	expression := fmt.Sprintf(`(async () => {
  const deadline = Date.now() + %d;
  while (Date.now() < deadline) {
    if (document.readyState === "interactive" || document.readyState === "complete") {
      return { ok: true };
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  return { ok: false, message: "page did not become ready" };
})()`, timeoutMs)

	value, err := usernameScanEvaluateViaProfile(ctx, a, profileID, expression, time.Duration(timeoutMs+2000)*time.Millisecond)
	if err != nil {
		return err
	}
	if !boolValue(value["ok"]) {
		return fmt.Errorf("%s", stringValue(value["message"], "page did not become ready"))
	}
	return nil
}

func usernameScanWaitForInput(ctx context.Context, a *App, profileID string, inputSelector string, timeoutMs int) error {
	expression := fmt.Sprintf(`(async () => {
  const selector = %s;
  const deadline = Date.now() + %d;
  while (Date.now() < deadline) {
    if (document.querySelector(selector)) {
      return { ok: true };
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  return { ok: false, message: "input selector not found: " + selector };
})()`, jsString(inputSelector), timeoutMs)

	value, err := usernameScanEvaluateViaProfile(ctx, a, profileID, expression, time.Duration(timeoutMs+2000)*time.Millisecond)
	if err != nil {
		return err
	}
	if !boolValue(value["ok"]) {
		return fmt.Errorf("%s", stringValue(value["message"], "input selector not found"))
	}
	return nil
}

func usernameScanFillSubmitAndRead(ctx context.Context, a *App, profileID string, options usernamescan.BrowserScanOptions, name string) (string, error) {
	expression := fmt.Sprintf(`(async () => {
  const inputSelector = %s;
  const submitSelector = %s;
  const resultSelector = %s;
  const value = %s;
  const input = document.querySelector(inputSelector);
  if (!input) {
    return { ok: false, message: "input selector not found: " + inputSelector };
  }

  input.focus();
  if ("value" in input) {
    const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype;
    const descriptor = Object.getOwnPropertyDescriptor(proto, "value") || Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value");
    if (descriptor && descriptor.set) {
      descriptor.set.call(input, value);
    } else {
      input.value = value;
    }
  } else if (input.isContentEditable) {
    input.textContent = value;
  } else {
    input.setAttribute("value", value);
  }
  input.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: value }));
  input.dispatchEvent(new Event("change", { bubbles: true }));

  if (submitSelector) {
    const submit = document.querySelector(submitSelector);
    if (!submit) {
      return { ok: false, message: "submit selector not found: " + submitSelector };
    }
    submit.click();
  } else {
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true }));
    input.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter", code: "Enter", bubbles: true }));
  }

  await new Promise(resolve => setTimeout(resolve, %d));
  const scope = resultSelector ? document.querySelector(resultSelector) : document.body;
  if (!scope) {
    return { ok: false, message: "result selector not found: " + resultSelector };
  }
  const text = String(scope.innerText || scope.textContent || "").trim();
  return { ok: true, text: text.slice(0, 20000) };
})()`,
		jsString(options.InputSelector),
		jsString(options.SubmitSelector),
		jsString(options.ResultSelector),
		jsString(name),
		options.WaitAfterSubmitMs,
	)

	value, err := usernameScanEvaluateViaProfile(ctx, a, profileID, expression, time.Duration(options.TimeoutMs+options.WaitAfterSubmitMs+2000)*time.Millisecond)
	if err != nil {
		return "", err
	}
	if !boolValue(value["ok"]) {
		return "", fmt.Errorf("%s", stringValue(value["message"], "browser scan script failed"))
	}
	return stringValue(value["text"], ""), nil
}

func usernameScanPageCall(ctx context.Context, debugPort int, method string, params map[string]any, timeout time.Duration) (map[string]any, error) {
	wsURL, err := usernameScanPageWebSocketURL(ctx, debugPort, timeout)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{HandshakeTimeout: minDuration(timeout, 5*time.Second)}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("CDP websocket connection failed: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	_ = conn.SetReadDeadline(deadline)
	message := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(message); err != nil {
		return nil, fmt.Errorf("CDP command send failed: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var response cdpResponse
		if err := conn.ReadJSON(&response); err != nil {
			return nil, fmt.Errorf("CDP response read failed: %w", err)
		}
		if response.Id != message.Id {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("CDP error: %s", response.Error.Message)
		}
		return response.Result, nil
	}
}

func usernameScanPageWebSocketURL(ctx context.Context, debugPort int, timeout time.Duration) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json", debugPort), nil)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("CDP target list request failed: %w", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(response.Body)
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil || len(targets) == 0 {
		return "", fmt.Errorf("CDP target list is empty or invalid")
	}

	for _, target := range targets {
		if target.Type == "page" && strings.TrimSpace(target.WebSocketDebuggerUrl) != "" {
			return target.WebSocketDebuggerUrl, nil
		}
	}
	if strings.TrimSpace(targets[0].WebSocketDebuggerUrl) != "" {
		return targets[0].WebSocketDebuggerUrl, nil
	}
	return "", fmt.Errorf("no usable CDP page websocket URL found")
}

func usernameScanEvaluate(ctx context.Context, debugPort int, expression string, timeout time.Duration) (map[string]any, error) {
	result, err := usernameScanPageCall(ctx, debugPort, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
		"userGesture":   true,
	}, timeout)
	if err != nil {
		return nil, err
	}
	if details, ok := result["exceptionDetails"]; ok {
		return nil, fmt.Errorf("page script exception: %v", details)
	}

	remoteObject, ok := result["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Runtime.evaluate returned an invalid result")
	}
	value, ok := remoteObject["value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Runtime.evaluate returned no object value")
	}
	return value, nil
}

func classifyUsernameBrowserResult(name string, pageText string, options usernamescan.BrowserScanOptions) (string, string) {
	if matchUsernameScanRule(pageText, options.TakenText) {
		return "taken", fmt.Sprintf("%s matched the taken rule.", name)
	}
	if matchUsernameScanRule(pageText, options.AvailableText) {
		return "available", fmt.Sprintf("%s matched the available rule.", name)
	}
	if matchUsernameScanRule(pageText, options.HoldText) {
		return "hold", fmt.Sprintf("%s matched the hold rule.", name)
	}
	return "hold", fmt.Sprintf("%s checked, no result rule matched.", name)
}

func matchUsernameScanRule(pageText string, ruleText string) bool {
	pageText = strings.ToLower(pageText)
	for _, token := range splitUsernameScanRuleTerms(ruleText) {
		if strings.Contains(pageText, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func splitUsernameScanRuleTerms(value string) []string {
	var terms []string
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == '|'
	}) {
		token = strings.TrimSpace(token)
		if token != "" {
			terms = append(terms, token)
		}
	}
	return terms
}

func usernameScanErrorResult(name string, err error) usernamescan.Result {
	return usernamescan.Result{
		Name:      name,
		Status:    "error",
		Message:   err.Error(),
		CheckedAt: time.Now().Format(time.RFC3339),
	}
}

func jsString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func stringValue(value any, fallback string) string {
	result, ok := value.(string)
	if !ok || result == "" {
		return fallback
	}
	return result
}

func clampUsernameScanInt(value int, minValue int, maxValue int, fallback int) int {
	if value <= 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minDuration(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// usernameScanEvaluateViaProfile 通过 profileCDP 执行页面 JS 表达式（支持 pipe/端口）。
func usernameScanEvaluateViaProfile(ctx context.Context, a *App, profileID string, expression string, timeout time.Duration) (map[string]any, error) {
	pc, err := a.profileCDP(profileID)
	if err != nil {
		return nil, err
	}
	result, err := pc.callPageTimeout("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
		"userGesture":   true,
	}, timeout)
	if err != nil {
		return nil, err
	}
	if details, ok := result["exceptionDetails"]; ok {
		return nil, fmt.Errorf("page script exception: %v", details)
	}

	remoteObject, ok := result["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Runtime.evaluate returned an invalid result")
	}
	value, ok := remoteObject["value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Runtime.evaluate returned no object value")
	}
	return value, nil
}
