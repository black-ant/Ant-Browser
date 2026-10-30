package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// 模拟"慢而非死"的节点：首个请求（建连）超过客户端超时，后续请求正常响应。
// 超时重试后必须成功，而不是把节点误判为失败。
func TestSpeedTestRequestRetriesOnTimeout(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			time.Sleep(300 * time.Millisecond)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	latency, statusCode, err := doSpeedTestRequest(client, server.URL)
	if err != nil {
		t.Fatalf("超时重试后仍失败: %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("statusCode = %d, want 204", statusCode)
	}
	if latency < 0 {
		t.Fatalf("latency = %d", latency)
	}
	if got := requests.Load(); got < 2 {
		t.Fatalf("请求数 = %d, want >= 2（应触发超时重试）", got)
	}
}

// 无超时类错误（如真实的 HTTP 5xx）不得触发重试：502 返回状态码而非错误，
// 请求数应为 2（首次 HEAD + 热连接复测 HEAD），而不是重试后的更多次。
func TestSpeedTestRequestDoesNotRetryOnNonTimeout(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	_, statusCode, err := doSpeedTestRequest(client, server.URL)
	if err != nil {
		t.Fatalf("502 不是请求级错误: %v", err)
	}
	if statusCode != http.StatusBadGateway {
		t.Fatalf("statusCode = %d, want 502", statusCode)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("请求数 = %d, want 2（非超时错误不重试）", got)
	}
}

func TestIsSpeedTestTimeoutError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"普通错误", errors.New("connection refused"), false},
		{"上下文截止", context.DeadlineExceeded, true},
		{"客户端超时包装", &url.Error{Op: "Head", URL: "http://www.gstatic.com/generate_204", Err: errors.New("context deadline exceeded (Client.Timeout exceeded while awaiting headers)")}, true},
		{"i/o 超时", errors.New("read tcp 1.2.3.4:5->1.2.3.4:6: i/o timeout"), true},
	}
	for _, tc := range cases {
		if got := isSpeedTestTimeoutError(tc.err); got != tc.want {
			t.Fatalf("isSpeedTestTimeoutError(%v) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
