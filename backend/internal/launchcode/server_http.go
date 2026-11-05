package launchcode

import "net/http"

func (s *LaunchServer) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/automation/scripts", s.handleAutomationScripts)
	mux.HandleFunc("/api/automation/scripts/", s.handleAutomationScriptByID)
	mux.Handle("/api/automation/scripts/run", s.trackLaunchTask(http.HandlerFunc(s.handleAutomationScriptRun)))
	mux.HandleFunc("/api/automation/scripts/runs", s.handleAutomationScriptRuns)
	mux.Handle("/api/automation/hooks/", s.trackLaunchTask(http.HandlerFunc(s.handleAutomationPublicHook)))
	mux.HandleFunc("/api/profiles", s.handleProfiles)
	mux.HandleFunc("/api/profiles/", s.handleProfileByID)
	mux.HandleFunc("/api/runtime/active", s.handleRuntimeActive)
	mux.Handle("/api/runtime/session", s.trackLaunchTask(http.HandlerFunc(s.handleRuntimeSession)))
	mux.Handle("/api/runtime/status", s.trackLaunchTask(http.HandlerFunc(s.handleRuntimeStatus)))
	mux.Handle("/api/runtime/stop", s.trackLaunchTask(http.HandlerFunc(s.handleRuntimeStop)))
	mux.Handle("/api/launch", s.trackLaunchTask(http.HandlerFunc(s.handleLaunchWithBody)))
	mux.HandleFunc("/api/launch/logs", s.handleLaunchLogs)
	mux.HandleFunc("/api/launch/tasks", s.handleLaunchTasks)
	mux.Handle("/api/launch/", s.trackLaunchTask(http.HandlerFunc(s.handleLaunch)))
	mux.HandleFunc("/", s.handleCDPProxy)
	return mux
}

func (s *LaunchServer) buildHandler(includeLocalhost bool) http.Handler {
	var handler http.Handler = s.buildMux()
	handler = s.apiAuthMiddleware(handler)
	if includeLocalhost {
		handler = s.localhostMiddleware(handler)
	}
	return handler
}

// handleHealth GET /api/health
func (s *LaunchServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// NewTestHandler 返回不含 localhost 限制的 handler，仅供测试使用
func NewTestHandler(s *LaunchServer) http.Handler {
	return s.buildHandler(false)
}
