package launchcode

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const DefaultAPIKeyHeader = "X-Ant-Api-Key"

// APIAuthConfig 定义 LaunchServer 对 /api/* 请求的可选认证配置。
type APIAuthConfig struct {
	Enabled bool
	APIKey  string
	Header  string
}

func normalizeAPIAuthConfig(cfg APIAuthConfig) APIAuthConfig {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Header = strings.TrimSpace(cfg.Header)
	if cfg.Header == "" {
		cfg.Header = DefaultAPIKeyHeader
	}
	return cfg
}

func (cfg APIAuthConfig) Requested() bool {
	return cfg.Enabled
}

func (cfg APIAuthConfig) Configured() bool {
	return cfg.APIKey != ""
}

func (cfg APIAuthConfig) Active() bool {
	return cfg.Requested() && cfg.Configured()
}

func (s *LaunchServer) SetAPIAuthConfig(cfg APIAuthConfig) {
	s.authMu.Lock()
	s.apiAuth = normalizeAPIAuthConfig(cfg)
	s.authMu.Unlock()
}

func (s *LaunchServer) apiAuthConfig() APIAuthConfig {
	s.authMu.RLock()
	cfg := s.apiAuth
	s.authMu.RUnlock()
	return cfg
}

func (s *LaunchServer) APIAuthHeader() string {
	return s.apiAuthConfig().Header
}

func (s *LaunchServer) APIAuthRequested() bool {
	return s.apiAuthConfig().Requested()
}

func (s *LaunchServer) APIAuthConfigured() bool {
	return s.apiAuthConfig().Configured()
}

func (s *LaunchServer) APIAuthEnabled() bool {
	return s.apiAuthConfig().Active()
}

// requiresAPIAuth 判断某个路径是否受 API Key 保护。
//
// 除了历史上的 /api/ 前缀，MCP 端点也必须纳入：它暴露的是与 /api/* 同等
// （甚至更完整）的能力面，漏掉就等于开了一个绕过鉴权的后门。
// 兜底路由 "/" 上的 CDP 反向代理不在此列——它沿用原有行为，仅靠 localhost 限制。
func (s *LaunchServer) requiresAPIAuth(path string) bool {
	if strings.HasPrefix(path, "/api/") {
		return true
	}

	mcpPath, handler := s.mcpMount()
	if handler == nil || mcpPath == "" {
		return false
	}
	return path == mcpPath || strings.HasPrefix(path, mcpPath+"/")
}

func (s *LaunchServer) apiAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.requiresAPIAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		cfg := s.apiAuthConfig()
		if !cfg.Active() {
			next.ServeHTTP(w, r)
			return
		}

		providedKey := strings.TrimSpace(r.Header.Get(cfg.Header))
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(cfg.APIKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"ok":         false,
				"error":      "unauthorized: invalid api key",
				"authHeader": cfg.Header,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
