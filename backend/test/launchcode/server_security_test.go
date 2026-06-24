package launchcode_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ant-chrome/backend/internal/launchcode"
)

// TestRequestBodySizeLimit 验证请求体大小限制中间件
func TestRequestBodySizeLimit(t *testing.T) {
	srv := launchcode.NewLaunchServer(newInMemoryService(), newMockStarter(), nil, nil, 0)
	handler := launchcode.NewTestHandler(srv)

	// 创建一个超过1MB的请求体
	largeBody := strings.Repeat("a", 1024*1024+1) // 1MB + 1字节
	reqBody := map[string]interface{}{
		"code": "test-code",
		"data": largeBody,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// 应该返回 413 Request Entity Too Large 或 400 Bad Request
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Fatalf("超大请求体应被拒绝: got=%d body=%s", w.Code, w.Body.String())
	}
}

// TestRequestBodySizeLimitAllowsNormalRequests 验证正常大小的请求不受影响
func TestRequestBodySizeLimitAllowsNormalRequests(t *testing.T) {
	srv := launchcode.NewLaunchServer(newInMemoryService(), newMockStarter(), nil, nil, 0)
	handler := launchcode.NewTestHandler(srv)

	reqBody := map[string]interface{}{
		"code": "test-code",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/launch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// 正常请求应该通过中间件（即使后续可能因为其他原因失败）
	// 不应该是413
	if w.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("正常大小请求不应被拒绝: got=%d body=%s", w.Code, w.Body.String())
	}
}

// TestCDPProxyRateLimiting 验证CDP代理的速率限制
func TestCDPProxyRateLimiting(t *testing.T) {
	srv := launchcode.NewLaunchServer(newInMemoryService(), newMockStarter(), nil, nil, 0)
	handler := launchcode.NewTestHandler(srv)

	// 模拟大量快速请求
	successCount := 0
	rateLimitCount := 0
	totalRequests := 250 // 超过突发容量(200)

	for i := 0; i < totalRequests; i++ {
		req := httptest.NewRequest(http.MethodGet, "/json/version", nil)
		req.Host = "127.0.0.1:19876" // 满足CDP代理的来源校验
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			rateLimitCount++
		} else {
			successCount++
		}
	}

	// 应该有一些请求被限流
	if rateLimitCount == 0 {
		t.Fatalf("速率限制未生效: %d个请求全部通过", totalRequests)
	}

	// 验证限流响应格式
	req := httptest.NewRequest(http.MethodGet, "/json/version", nil)
	req.Host = "127.0.0.1:19876"
	req.RemoteAddr = "127.0.0.1:12345"

	// 发送足够多的请求以触发限流
	for i := 0; i < 300; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("解析限流响应失败: %v", err)
			}
			if resp["error"] != "rate limit exceeded" {
				t.Fatalf("限流错误信息不正确: %+v", resp)
			}
			if resp["ok"] != false {
				t.Fatalf("限流响应应该包含 ok=false: %+v", resp)
			}
			return // 测试通过
		}
	}
	// 如果循环结束还没触发，说明有问题
	t.Logf("警告: 300次请求后仍未触发限流 (successCount=%d, rateLimitCount=%d)", successCount, rateLimitCount)
}

// TestCDPProxyOriginValidation 验证CDP代理的Origin校验
func TestCDPProxyOriginValidation(t *testing.T) {
	srv := launchcode.NewLaunchServer(newInMemoryService(), newMockStarter(), nil, nil, 0)
	handler := launchcode.NewTestHandler(srv)

	tests := []struct {
		name           string
		host           string
		origin         string
		expectedStatus int
		shouldAllow    bool
	}{
		{
			name:           "允许本地无Origin请求",
			host:           "127.0.0.1:19876",
			origin:         "",
			shouldAllow:    true,
			expectedStatus: http.StatusServiceUnavailable, // 无活动浏览器
		},
		{
			name:           "拒绝外部Origin",
			host:           "127.0.0.1:19876",
			origin:         "http://evil.com",
			shouldAllow:    false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "拒绝外部Host",
			host:           "evil.com:19876",
			origin:         "",
			shouldAllow:    false,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/json/version", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Fatalf("期望状态码 %d, 实际 %d, body=%s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if !tt.shouldAllow && w.Code == http.StatusForbidden {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("解析响应失败: %v", err)
				}
				if !strings.Contains(resp["error"].(string), "forbidden") {
					t.Fatalf("应返回forbidden错误: %+v", resp)
				}
			}
		})
	}
}
