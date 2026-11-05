package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	automationDemoHealthPath         = "/api/health"
	automationDemoProfilesPath       = "/api/profiles"
	automationDemoLaunchPath         = "/api/launch"
	automationDemoRuntimeSessionPath = "/api/runtime/session"
	automationDemoTimeout            = 10 * time.Second
)

type automationDemoResultOptions struct {
	RequestedCode       string
	StoppedBeforeDelete bool
	StopError           string
}

type automationDemoCreateOptions struct {
	ProfileName          string   `json:"profileName"`
	LaunchCode           string   `json:"launchCode"`
	StartURL             string   `json:"startUrl"`
	LaunchArgs           []string `json:"launchArgs"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls"`
	AutoLaunch           bool     `json:"autoLaunch"`
}

func (a *App) AutomationDemoHealthCheck() (map[string]interface{}, error) {
	status, payload, err := a.automationDemoRequest(http.MethodGet, automationDemoHealthPath, nil)
	if err != nil {
		return nil, err
	}
	return a.newAutomationDemoPayload(http.MethodGet, automationDemoHealthPath, status, payload, automationDemoResultOptions{}), nil
}

func (a *App) AutomationDemoCreateProfile() (map[string]interface{}, error) {
	return a.automationDemoCreateProfile(automationDemoCreateOptions{})
}

func (a *App) AutomationDemoCreateProfileWithOptions(optionsJSON string) (map[string]interface{}, error) {
	options, err := decodeAutomationDemoCreateOptions(optionsJSON)
	if err != nil {
		return nil, err
	}
	return a.automationDemoCreateProfile(options)
}

func (a *App) automationDemoCreateProfile(options automationDemoCreateOptions) (map[string]interface{}, error) {
	requestedCode, requestBody := buildAutomationDemoCreateRequest(options)

	status, payload, err := a.automationDemoRequest(http.MethodPost, automationDemoProfilesPath, requestBody)
	if err != nil {
		return nil, err
	}
	return a.newAutomationDemoPayload(http.MethodPost, automationDemoProfilesPath, status, payload, automationDemoResultOptions{
		RequestedCode: requestedCode,
	}), nil
}

func (a *App) AutomationDemoLaunchProfile(code string) (map[string]interface{}, error) {
	requestedCode := strings.ToUpper(strings.TrimSpace(code))
	if requestedCode == "" {
		return nil, fmt.Errorf("launch code is required")
	}

	status, payload, err := a.automationDemoRequest(http.MethodPost, automationDemoLaunchPath, map[string]interface{}{
		"code":                 requestedCode,
		"startUrls":            []string{"about:blank"},
		"skipDefaultStartUrls": true,
	})
	if err != nil {
		return nil, err
	}
	return a.newAutomationDemoPayload(http.MethodPost, automationDemoLaunchPath, status, payload, automationDemoResultOptions{
		RequestedCode: requestedCode,
	}), nil
}

func (a *App) AutomationDemoDeleteProfile(profileId string) (map[string]interface{}, error) {
	normalizedProfileID := strings.TrimSpace(profileId)
	if normalizedProfileID == "" {
		return nil, fmt.Errorf("profileId is required")
	}

	apiPath := automationDemoProfilesPath + "/" + url.PathEscape(normalizedProfileID)
	status, payload, err := a.automationDemoRequest(http.MethodDelete, apiPath, nil)
	if err != nil {
		return nil, err
	}

	options := automationDemoResultOptions{}
	if status == http.StatusConflict {
		if _, stopErr := a.BrowserInstanceStop(normalizedProfileID); stopErr != nil {
			options.StopError = stopErr.Error()
			return a.newAutomationDemoPayload(http.MethodDelete, apiPath, status, payload, options), nil
		}
		options.StoppedBeforeDelete = true

		status, payload, err = a.automationDemoRequest(http.MethodDelete, apiPath, nil)
		if err != nil {
			return nil, err
		}
	}

	return a.newAutomationDemoPayload(http.MethodDelete, apiPath, status, payload, options), nil
}

func (a *App) automationDemoRequest(method string, apiPath string, body any) (int, map[string]interface{}, error) {
	return a.automationDemoRequestWithContext(context.Background(), method, apiPath, body)
}

func automationDemoRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, automationDemoTimeout)
}

func (a *App) automationDemoRequestWithContext(ctx context.Context, method string, apiPath string, body any) (int, map[string]interface{}, error) {
	reqCtx, cancel := automationDemoRequestContext(ctx)
	defer cancel()

	baseURL, authHeader, authValue, err := a.automationDemoEndpoint()
	if err != nil {
		return 0, nil, err
	}

	requestURL := strings.TrimRight(baseURL, "/") + apiPath

	var reader io.Reader
	if body != nil {
		raw, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return 0, nil, fmt.Errorf("marshal demo request failed: %w", marshalErr)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, requestURL, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("create demo request failed: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" && authValue != "" {
		req.Header.Set(authHeader, authValue)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctxErr := reqCtx.Err(); ctxErr != nil {
			return 0, nil, fmt.Errorf("call launch api failed: %w", ctxErr)
		}
		return 0, nil, fmt.Errorf("call launch api failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := decodeAutomationDemoBody(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, payload, nil
}

func (a *App) automationDemoEndpoint() (string, string, string, error) {
	if a.launchServer == nil {
		return "", "", "", fmt.Errorf("launch server is not initialized")
	}

	port := a.launchServer.Port()
	if port <= 0 {
		return "", "", "", fmt.Errorf("launch server is not ready")
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if !a.launchServer.APIAuthEnabled() {
		return baseURL, "", "", nil
	}
	if a.config == nil {
		return "", "", "", fmt.Errorf("launch server auth config is not initialized")
	}

	apiKey := strings.TrimSpace(a.config.LaunchServer.Auth.APIKey)
	if apiKey == "" {
		return "", "", "", fmt.Errorf("launch server api key is empty")
	}

	return baseURL, a.launchServer.APIAuthHeader(), apiKey, nil
}

func (a *App) newAutomationDemoPayload(method string, apiPath string, status int, response map[string]interface{}, options automationDemoResultOptions) map[string]interface{} {
	if response == nil {
		response = map[string]interface{}{}
	}

	baseURL := ""
	if a.launchServer != nil && a.launchServer.Port() > 0 {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", a.launchServer.Port())
	}

	ok := status >= http.StatusOK && status < http.StatusMultipleChoices
	if rawOK, exists := response["ok"]; exists {
		if value, valid := rawOK.(bool); valid {
			ok = ok && value
		}
	}

	payload := map[string]interface{}{
		"ok":          ok,
		"status":      status,
		"method":      method,
		"path":        apiPath,
		"baseUrl":     baseURL,
		"requestedAt": time.Now().Format(time.RFC3339),
		"response":    response,
	}

	if errMsg := mapStringValue(response, "error"); errMsg != "" {
		payload["error"] = errMsg
	}

	for _, key := range []string{
		"profileId",
		"profileName",
		"launchCode",
		"cdpUrl",
		"cdpPort",
		"debugPort",
		"debugReady",
		"pid",
		"created",
		"updated",
		"launched",
		"deleted",
		"runtimeWarning",
		"authHeader",
	} {
		if value, exists := response[key]; exists {
			payload[key] = value
		}
	}

	if options.RequestedCode != "" {
		payload["requestedCode"] = options.RequestedCode
		if _, exists := payload["launchCode"]; !exists && status >= http.StatusOK && status < http.StatusMultipleChoices {
			payload["launchCode"] = options.RequestedCode
		}
	}
	if options.StoppedBeforeDelete {
		payload["stoppedBeforeDelete"] = true
	}
	if options.StopError != "" {
		payload["stopError"] = options.StopError
	}

	return payload
}

func decodeAutomationDemoBody(body io.Reader) (map[string]interface{}, error) {
	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read demo response failed: %w", err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]interface{}{"rawBody": string(raw)}, nil
	}

	if payload, ok := decoded.(map[string]interface{}); ok {
		return payload, nil
	}
	return map[string]interface{}{"data": decoded}, nil
}

func decodeAutomationDemoCreateOptions(optionsJSON string) (automationDemoCreateOptions, error) {
	normalizedJSON := strings.TrimSpace(optionsJSON)
	if normalizedJSON == "" {
		return automationDemoCreateOptions{}, nil
	}

	var options automationDemoCreateOptions
	if err := json.Unmarshal([]byte(normalizedJSON), &options); err != nil {
		return automationDemoCreateOptions{}, fmt.Errorf("decode demo create options failed: %w", err)
	}
	return options, nil
}

func buildAutomationDemoCreateRequest(options automationDemoCreateOptions) (string, map[string]interface{}) {
	requestedCode := strings.ToUpper(strings.TrimSpace(options.LaunchCode))
	if requestedCode == "" {
		requestedCode = automationDemoLaunchCode()
	}

	profileName := strings.TrimSpace(options.ProfileName)
	if profileName == "" {
		profileName = fmt.Sprintf("自动化 Demo %s", requestedCode)
	}

	launchArgs := normalizeAutomationDemoLaunchArgs(options.LaunchArgs)
	requestBody := map[string]interface{}{
		"profile": map[string]interface{}{
			"profileName": profileName,
			"userDataDir": fmt.Sprintf("automation-demo-%s", strings.ToLower(strings.ReplaceAll(requestedCode, "_", "-"))),
			"launchArgs":  launchArgs,
			"tags":        []string{"自动化", "Demo"},
			"keywords":    []string{"automation-demo", "launch-api-demo"},
		},
		"launchCode": requestedCode,
		"autoLaunch": options.AutoLaunch,
	}

	if options.AutoLaunch {
		requestBody["start"] = buildAutomationDemoStartPayload(options.StartURL, launchArgs, options.SkipDefaultStartURLs)
	}

	return requestedCode, requestBody
}

func normalizeAutomationDemoLaunchArgs(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func buildAutomationDemoStartPayload(startURL string, launchArgs []string, skipDefaultStartURLs bool) map[string]interface{} {
	payload := map[string]interface{}{}
	if len(launchArgs) > 0 {
		payload["launchArgs"] = launchArgs
	}

	normalizedStartURL := strings.TrimSpace(startURL)
	if normalizedStartURL != "" {
		payload["startUrls"] = []string{normalizedStartURL}
	}

	if skipDefaultStartURLs {
		payload["skipDefaultStartUrls"] = true
	}

	if len(payload) == 0 {
		payload["startUrls"] = []string{"about:blank"}
		payload["skipDefaultStartUrls"] = true
	}

	return payload
}

func automationDemoLaunchCode() string {
	token := strings.ToUpper(strings.ReplaceAll(generateUUID(), "-", ""))
	if len(token) > 6 {
		token = token[:6]
	}
	return "DEMO_" + token
}

func mapStringValue(payload map[string]interface{}, key string) string {
	value, exists := payload[key]
	if !exists || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}
