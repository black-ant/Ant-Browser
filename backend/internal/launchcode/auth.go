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

func (s *LaunchServer) apiAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if !s.authorizeAPIKey(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authorizeAPIKey 在 API 认证生效时校验请求头里的 key，不通过就写 401 并返回 false。
// 认证未生效时直接放行。WebSocket 升级请求同样走这里，key 放在升级请求头里。
func (s *LaunchServer) authorizeAPIKey(w http.ResponseWriter, r *http.Request) bool {
	cfg := s.apiAuthConfig()
	if !cfg.Active() {
		return true
	}

	providedKey := strings.TrimSpace(r.Header.Get(cfg.Header))
	if subtle.ConstantTimeCompare([]byte(providedKey), []byte(cfg.APIKey)) == 1 {
		return true
	}
	writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
		"ok":         false,
		"error":      "unauthorized: invalid api key",
		"authHeader": cfg.Header,
	})
	return false
}
