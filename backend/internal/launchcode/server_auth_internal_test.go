package launchcode

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildHandlerRejectsNonLocalRequestBeforeAPIAuth(t *testing.T) {
	srv := NewLaunchServer(NewLaunchCodeService(NewMemoryLaunchCodeDAO()), nil, nil, nil, 0)
	srv.SetAPIAuthConfig(APIAuthConfig{
		Enabled: true,
		APIKey:  "secret-key",
		Header:  "X-Test-Api-Key",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "10.0.0.8:3456"
	w := httptest.NewRecorder()
	srv.buildHandler(true).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("非 localhost 请求应优先返回 403: got=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "forbidden: only localhost is allowed") {
		t.Fatalf("错误信息不正确: %s", w.Body.String())
	}
}
