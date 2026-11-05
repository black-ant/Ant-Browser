package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/launchcode"
)

const automationScriptPublicAPIInvokeDefaultTimeout = 31 * time.Minute

type AutomationScriptPublicAPIInvokeInput struct {
	URL        string `json:"url"`
	Method     string `json:"method"`
	BodyText   string `json:"bodyText"`
	APIKey     string `json:"apiKey"`
	AuthHeader string `json:"authHeader"`
	TimeoutMs  int    `json:"timeoutMs"`
}

type AutomationScriptPublicAPIInvokeResult struct {
	OK         bool        `json:"ok"`
	Status     int         `json:"status"`
	StatusText string      `json:"statusText"`
	BodyText   string      `json:"bodyText"`
	BodyJSON   interface{} `json:"bodyJson"`
}

func (a *App) AutomationScriptInvokePublicAPI(input AutomationScriptPublicAPIInvokeInput) (*AutomationScriptPublicAPIInvokeResult, error) {
	requestURL, err := normalizeAutomationScriptInvokeURL(input.URL)
	if err != nil {
		return nil, err
	}

	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodPost
	}

	bodyText := strings.TrimSpace(input.BodyText)
	if bodyText == "" {
		bodyText = "{}"
	}

	timeout := automationScriptPublicAPIInvokeDefaultTimeout
	if input.TimeoutMs > 0 {
		timeout = time.Duration(input.TimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		requestURL.String(),
		bytes.NewBufferString(bodyText),
	)
	if err != nil {
		return nil, fmt.Errorf("create invoke request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	authHeader := strings.TrimSpace(input.AuthHeader)
	if authHeader == "" {
		authHeader = launchcode.DefaultAPIKeyHeader
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		if autoHeader, autoKey := a.resolveAutomationScriptInvokeAuth(requestURL); autoKey != "" {
			authHeader = autoHeader
			apiKey = autoKey
		}
	}
	if authHeader != "" && apiKey != "" {
		req.Header.Set(authHeader, apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("invoke public api failed: %w", ctxErr)
		}
		return nil, fmt.Errorf("invoke public api failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read public api response failed: %w", err)
	}

	result := &AutomationScriptPublicAPIInvokeResult{
		OK:         resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		BodyText:   string(rawBody),
		BodyJSON:   nil,
	}

	trimmedBody := bytes.TrimSpace(rawBody)
	if len(trimmedBody) > 0 {
		var decoded interface{}
		if err := json.Unmarshal(trimmedBody, &decoded); err == nil {
			result.BodyJSON = decoded
		}
	}

	return result, nil
}

func normalizeAutomationScriptInvokeURL(rawURL string) (*url.URL, error) {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return nil, fmt.Errorf("接口地址不能为空")
	}

	parsedURL, err := url.Parse(normalizedURL)
	if err != nil {
		return nil, fmt.Errorf("接口地址不合法: %w", err)
	}
	if !parsedURL.IsAbs() {
		return nil, fmt.Errorf("接口地址必须是完整 URL")
	}

	switch strings.ToLower(parsedURL.Scheme) {
	case "http", "https":
	default:
		return nil, fmt.Errorf("接口地址仅支持 http 或 https")
	}

	if strings.TrimSpace(parsedURL.Host) == "" {
		return nil, fmt.Errorf("接口地址缺少主机")
	}

	return parsedURL, nil
}

func (a *App) resolveAutomationScriptInvokeAuth(targetURL *url.URL) (string, string) {
	if targetURL == nil || a.launchServer == nil || a.config == nil {
		return "", ""
	}
	if !a.launchServer.APIAuthEnabled() {
		return "", ""
	}

	apiKey := strings.TrimSpace(a.config.LaunchServer.Auth.APIKey)
	if apiKey == "" {
		return "", ""
	}

	launchPort := a.launchServer.Port()
	if launchPort <= 0 {
		return "", ""
	}

	requestPort := targetURL.Port()
	if requestPort == "" {
		switch strings.ToLower(targetURL.Scheme) {
		case "https":
			requestPort = "443"
		default:
			requestPort = "80"
		}
	}
	if requestPort != strconv.Itoa(launchPort) {
		return "", ""
	}

	host := strings.TrimSpace(strings.ToLower(targetURL.Hostname()))
	if host == "" {
		return "", ""
	}
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		if !parsedIP.IsLoopback() {
			return "", ""
		}
	} else if host != "localhost" {
		return "", ""
	}

	return a.launchServer.APIAuthHeader(), apiKey
}
