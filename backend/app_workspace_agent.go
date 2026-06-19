package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

const (
	defaultWorkspaceAgentListenHost   = "127.0.0.1"
	defaultWorkspaceAgentListenPort   = "47831"
	defaultWorkspaceAntRuntimeBaseURL = "http://127.0.0.1:19876"
	defaultWorkspaceServerOrigin      = "http://127.0.0.1:4174"
	workspaceBootstrapTimeout         = 8 * time.Second
)

type workspaceServerConnectionConfig struct {
	ServerOrigin string `json:"serverOrigin"`
	ApiBaseURL   string `json:"apiBaseUrl"`
	ServerIP     string `json:"serverIp"`
	ServerPort   int    `json:"serverPort"`
}

type workspaceServerOriginResolution struct {
	Origin     string
	Source     string
	ConfigPath string
}

type workspaceServerConfigCandidate struct {
	ConfigPath string
	Origin     string
	ModifiedAt time.Time
}

type workspaceAuthIdentity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type workspaceMeEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID          string                `json:"id"`
		DisplayName string                `json:"displayName"`
		UserSummary workspaceAuthIdentity `json:"userSummary"`
		User        workspaceAuthIdentity `json:"user"`
	} `json:"data"`
}

type workspaceBootstrapUser struct {
	UserID      string
	DisplayName string
}

type workspaceBootstrapEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AgentSessionID string `json:"agentSessionId"`
	} `json:"data"`
}

type workspaceShopsEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []json.RawMessage `json:"items"`
	} `json:"data"`
}

func (a *App) ensureWorkspaceAgentBootstrapped() error {
	log := logger.New("WorkspaceAgent")

	installRoot, err := resolveWorkspaceInstallRootWithConfig(a.appRoot, a.config)
	if err != nil {
		appendWorkspaceHostLog(resolveWorkspaceRuntimeDir(), "workspace install root unavailable: app_root=%s error=%v", a.appRoot, err)
		log.Warn("workspace install root unavailable", logger.F("app_root", a.appRoot), logger.F("error", err.Error()))
		return err
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	serverOrigin := a.resolveWorkspaceServerOrigin(runtimeDir)
	appendWorkspaceHostLog(runtimeDir, "bootstrap begin: app_root=%s install_root=%s runtime_dir=%s server_origin=%s", a.appRoot, installRoot, runtimeDir, serverOrigin)
	if strings.TrimSpace(serverOrigin) == "" {
		appendWorkspaceHostLog(runtimeDir, "workspace server origin unavailable")
		log.Warn("workspace server origin unavailable", logger.F("runtime_dir", runtimeDir))
		return fmt.Errorf("workspace server origin unavailable")
	}

	agentBaseURL := a.resolveWorkspaceAgentBaseURL()
	appendWorkspaceHostLog(runtimeDir, "resolved agent base url: %s", agentBaseURL)
	if !isHTTPReachable(agentBaseURL + "/health") {
		cmd, resolvedAgentBaseURL, err := a.startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin)
		if err != nil {
			appendWorkspaceHostLog(runtimeDir, "start workspace agent failed: %v", err)
			log.Error("start workspace agent failed", logger.F("error", err.Error()))
			return fmt.Errorf("start workspace agent: %w", err)
		}
		agentBaseURL = resolvedAgentBaseURL
		a.workspaceAgentURL = resolvedAgentBaseURL
		if a.workspaceService != nil {
			a.workspaceService.SetBaseURL(resolvedAgentBaseURL)
		}
		a.workspaceAgentCmd = cmd
		if !waitForHTTPReachable(agentBaseURL+"/health", workspaceBootstrapTimeout) {
			appendWorkspaceHostLog(runtimeDir, "workspace agent health timeout: url=%s", agentBaseURL+"/health")
			log.Error("workspace agent health timeout", logger.F("url", agentBaseURL+"/health"))
			return fmt.Errorf("workspace agent health timeout: %s", agentBaseURL+"/health")
		}
	}

	authSession, err := a.LoadDesktopAuthSession()
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "load desktop auth session failed: %v", err)
		log.Error("load desktop auth session failed", logger.F("error", err.Error()))
		return fmt.Errorf("load desktop auth session: %w", err)
	}
	accessToken := ""
	if authSession != nil {
		accessToken = strings.TrimSpace(authSession.AccessToken)
	}
	if accessToken == "" {
		appendWorkspaceHostLog(runtimeDir, "workspace agent bootstrap skipped: persisted desktop access token unavailable")
		log.Warn("workspace agent bootstrap skipped", logger.F("reason", "persisted desktop access token unavailable"))
		return fmt.Errorf("persisted desktop access token unavailable")
	}

	bootstrapUser, err := fetchWorkspaceBootstrapUser(serverOrigin, accessToken)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "fetch workspace bootstrap user failed: %v", err)
		log.Error("fetch workspace bootstrap user failed", logger.F("error", err.Error()))
		return fmt.Errorf("fetch workspace bootstrap user: %w", err)
	}

	if err := bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin, accessToken, bootstrapUser); err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent bootstrap failed: %v", err)
		log.Error("workspace agent bootstrap failed", logger.F("error", err.Error()))
		return fmt.Errorf("bootstrap workspace agent session: %w", err)
	}

	shopCount, err := warmWorkspaceAuthorizedShops(agentBaseURL)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace shops warmup failed: %v", err)
		log.Error("workspace shops warmup failed", logger.F("error", err.Error()))
		return fmt.Errorf("warm workspace authorized shops: %w", err)
	}

	appendWorkspaceHostLog(runtimeDir, "workspace agent ready: agent_base_url=%s server_origin=%s shop_count=%d", agentBaseURL, serverOrigin, shopCount)
	log.Info("workspace agent ready",
		logger.F("agent_base_url", agentBaseURL),
		logger.F("server_origin", serverOrigin),
		logger.F("shop_count", shopCount),
	)
	return nil
}

func (a *App) startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin string) (*exec.Cmd, string, error) {
	nodeExe, err := resolveWorkspaceNodeExecutable(installRoot)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "resolve node executable failed: %v", err)
		return nil, "", err
	}

	agentEntry := workspaceAgentEntryPath(installRoot)
	if _, statErr := os.Stat(agentEntry); statErr != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent entry missing: %v", statErr)
		return nil, "", fmt.Errorf("workspace agent entry missing: %w", statErr)
	}

	listenPort, agentBaseURL, err := reserveWorkspaceAgentPort()
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "reserve workspace agent port failed: %v", err)
		return nil, "", err
	}

	cleanedCount, cleanupErr := cleanupWorkspaceBootstrapProcesses(installRoot)
	if cleanupErr != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace bootstrap cleanup failed: %v", cleanupErr)
		return nil, "", cleanupErr
	}
	if cleanedCount > 0 {
		appendWorkspaceHostLog(runtimeDir, "workspace bootstrap cleanup removed stale processes: count=%d", cleanedCount)
	}

	logFile, err := openWorkspaceHostLogFile(runtimeDir)
	if err != nil {
		return nil, "", err
	}
	if a.workspaceAgentLog != nil && a.workspaceAgentLog != logFile {
		_ = a.workspaceAgentLog.Close()
	}
	a.workspaceAgentLog = logFile

	antRuntimeBaseURL := a.resolveWorkspaceAntRuntimeBaseURL()
	cmd := exec.Command(nodeExe, "--enable-source-maps", agentEntry)
	hideWindow(cmd)
	cmd.Dir = installRoot
	cmd.Env = withWorkspaceAgentEnv(os.Environ(), runtimeDir, serverOrigin, listenPort, agentBaseURL, antRuntimeBaseURL)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	appendWorkspaceHostLog(runtimeDir,
		"starting workspace agent: node=%s entry=%s cwd=%s listen=%s:%s server_origin=%s ant_runtime=%s",
		nodeExe,
		agentEntry,
		cmd.Dir,
		defaultWorkspaceAgentListenHost,
		listenPort,
		serverOrigin,
		antRuntimeBaseURL,
	)

	if err := cmd.Start(); err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent process start error: %v", err)
		return nil, "", err
	}
	appendWorkspaceHostLog(runtimeDir, "workspace agent process started: pid=%d", cmd.Process.Pid)
	return cmd, agentBaseURL, nil
}

func stopWorkspaceAgentProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return stopProcessCmdForShutdown(cmd)
}

func (a *App) resetWorkspaceAgentSessionRuntimeHook(reason string) error {
	if a == nil {
		return nil
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
		appendWorkspaceHostLog(runtimeDir, "workspace agent session/runtime hook reset: reason=%s", trimmedReason)
	}

	var resetErr error
	if err := stopWorkspaceAgentProcess(a.workspaceAgentCmd); err != nil {
		resetErr = err
		appendWorkspaceHostLog(runtimeDir, "workspace agent session/runtime hook stop failed: %v", err)
	}
	a.workspaceAgentCmd = nil

	if a.workspaceAgentLog != nil {
		_ = a.workspaceAgentLog.Close()
		a.workspaceAgentLog = nil
	}

	a.workspaceAgentURL = ""
	if a.workspaceService != nil {
		a.workspaceService.SetBaseURL(resolveWorkspaceAgentBaseURLWithConfig(a.config))
	}

	return resetErr
}

func (a *App) ensureWorkspaceAgentReachableForRequest(reason string) {
	if a == nil {
		return
	}
	if strings.TrimSpace(a.workspaceAgentURL) == "" {
		return
	}

	agentBaseURL := a.resolveWorkspaceAgentBaseURL()
	if strings.TrimSpace(agentBaseURL) != "" && isHTTPReachable(agentBaseURL+"/health") {
		return
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	appendWorkspaceHostLog(runtimeDir, "workspace agent unavailable before request: reason=%s url=%s", strings.TrimSpace(reason), agentBaseURL)
	log := logger.New("WorkspaceAgent")
	log.Warn("workspace agent unavailable before request",
		logger.F("reason", strings.TrimSpace(reason)),
		logger.F("url", agentBaseURL),
	)

	_ = a.resetWorkspaceAgentSessionRuntimeHook("workspace agent unreachable before " + strings.TrimSpace(reason))
	if err := a.ensureWorkspaceAgentBootstrapped(); err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent request recovery failed: reason=%s error=%v", strings.TrimSpace(reason), err)
		log.Warn("workspace agent request recovery failed",
			logger.F("reason", strings.TrimSpace(reason)),
			logger.F("error", err.Error()),
		)
	}
}

func resolveWorkspaceInstallRoot(appRoot string) (string, error) {
	return resolveWorkspaceInstallRootWithConfig(appRoot, nil)
}

func resolveWorkspaceInstallRootWithConfig(appRoot string, cfg *config.Config) (string, error) {
	if root, configured, err := resolveWorkspaceInstallRootFromEnv(); configured {
		if err != nil {
			return "", err
		}
		return root, nil
	}
	if root, configured, err := resolveWorkspaceInstallRootFromConfig(cfg); configured {
		if err != nil {
			return "", err
		}
		return root, nil
	}

	current := strings.TrimSpace(appRoot)
	if current == "" {
		return "", fmt.Errorf("app root is empty")
	}

	for i := 0; i < 5; i++ {
		if workspaceInstallRootLooksValid(current) {
			return current, nil
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", fmt.Errorf("workspace agent entry not found from app root: %s", strings.TrimSpace(appRoot))
}

func resolveWorkspaceInstallRootFromConfig(cfg *config.Config) (string, bool, error) {
	if cfg == nil {
		return "", false, nil
	}
	root := strings.TrimSpace(cfg.Workspace.InstallRoot)
	if root == "" {
		return "", false, nil
	}
	root = strings.TrimRight(root, string(os.PathSeparator))
	if workspaceInstallRootLooksValid(root) {
		return root, true, nil
	}
	return "", true, fmt.Errorf("workspace install root configured by config.workspace.install_root is invalid: %s", cfg.Workspace.InstallRoot)
}

func resolveWorkspaceInstallRootFromEnv() (string, bool, error) {
	for _, key := range []string{"ANT_BROWSER_WORKSPACE_INSTALL_ROOT", "WORKSPACE_INSTALL_ROOT"} {
		rawValue := strings.TrimSpace(os.Getenv(key))
		if rawValue == "" {
			continue
		}
		root := strings.TrimRight(rawValue, string(os.PathSeparator))
		if workspaceInstallRootLooksValid(root) {
			return root, true, nil
		}
		return "", true, fmt.Errorf("workspace install root configured by %s is invalid: %s", key, rawValue)
	}
	return "", false, nil
}

func workspaceInstallRootLooksValid(root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	agentEntry := filepath.Join(root, "apps", "agent", "src", "server", "index.mjs")
	_, err := os.Stat(agentEntry)
	return err == nil
}

func resolveWorkspaceRuntimeDir() string {
	return resolveWorkspaceRuntimeDirWithConfig(nil)
}

func resolveWorkspaceRuntimeDirWithConfig(cfg *config.Config) string {
	for _, candidate := range []string{
		configuredWorkspaceRuntimeDir(cfg),
		strings.TrimSpace(os.Getenv("DESKTOP_RUNTIME_DIR")),
		strings.TrimSpace(os.Getenv("AGENT_STATE_DIR")),
	} {
		if candidate != "" {
			return candidate
		}
	}

	if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
		return filepath.Join(programData, "1688shop-agent", "runtime")
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".1688shop-desktop-runtime")
	}

	return filepath.Join(os.TempDir(), "1688shop-desktop-runtime")
}

func resolveWorkspaceServerOrigin(runtimeDir string) string {
	return resolveWorkspaceServerOriginWithConfig(runtimeDir, nil)
}

func resolveWorkspaceServerOriginWithConfig(runtimeDir string, cfg *config.Config) string {
	return resolveWorkspaceServerOriginDetails(runtimeDir, cfg).Origin
}

func resolveWorkspaceServerOriginDetails(runtimeDir string, cfg *config.Config) workspaceServerOriginResolution {
	return resolveWorkspaceServerOriginDetailsForApp("", runtimeDir, cfg)
}

func (a *App) resolveWorkspaceServerOrigin(runtimeDir string) string {
	return a.resolveWorkspaceServerOriginDetails(runtimeDir).Origin
}

func (a *App) resolveWorkspaceServerOriginDetails(runtimeDir string) workspaceServerOriginResolution {
	if a == nil {
		return resolveWorkspaceServerOriginDetails(runtimeDir, nil)
	}
	return resolveWorkspaceServerOriginDetailsForApp(a.appRoot, runtimeDir, a.config)
}

func resolveWorkspaceServerOriginDetailsForApp(appRoot, runtimeDir string, cfg *config.Config) workspaceServerOriginResolution {
	if candidate, ok := newestWorkspaceServerConfigCandidate(workspaceServerConfigCandidatePaths(appRoot, runtimeDir)); ok {
		return workspaceServerOriginResolution{
			Origin:     strings.TrimRight(candidate.Origin, "/"),
			Source:     "runtime-config",
			ConfigPath: candidate.ConfigPath,
		}
	}

	if value := strings.TrimSpace(os.Getenv("DESKTOP_SERVER_BASE_URL")); value != "" {
		return workspaceServerOriginResolution{
			Origin: strings.TrimRight(value, "/"),
			Source: "env:DESKTOP_SERVER_BASE_URL",
		}
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.Workspace.ServerOrigin); value != "" {
			return workspaceServerOriginResolution{
				Origin: strings.TrimRight(value, "/"),
				Source: "config.yaml",
			}
		}
	}

	return workspaceServerOriginResolution{
		Origin: defaultWorkspaceServerOrigin,
		Source: "default",
	}
}

func workspaceServerConfigCandidatePaths(appRoot, runtimeDir string) []string {
	paths := []string{}
	for _, dir := range []string{
		strings.TrimSpace(runtimeDir),
		workspaceInstallRuntimeDir(appRoot),
	} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		path := filepath.Join(dir, "config", "server-connection.json")
		if !containsCleanPath(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths
}

func newestWorkspaceServerConfigCandidate(paths []string) (workspaceServerConfigCandidate, bool) {
	var newest workspaceServerConfigCandidate
	found := false
	for _, path := range paths {
		candidate, ok := readWorkspaceServerConfigCandidate(path)
		if !ok {
			continue
		}
		if !found || candidate.ModifiedAt.After(newest.ModifiedAt) {
			newest = candidate
			found = true
		}
	}
	return newest, found
}

func readWorkspaceServerConfigCandidate(configPath string) (workspaceServerConfigCandidate, bool) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return workspaceServerConfigCandidate{}, false
	}
	info, err := os.Stat(configPath)
	if err != nil || info.IsDir() {
		return workspaceServerConfigCandidate{}, false
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return workspaceServerConfigCandidate{}, false
	}
	var config workspaceServerConnectionConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return workspaceServerConfigCandidate{}, false
	}
	origin := strings.TrimSpace(config.ServerOrigin)
	if origin == "" {
		return workspaceServerConfigCandidate{}, false
	}
	return workspaceServerConfigCandidate{
		ConfigPath: configPath,
		Origin:     origin,
		ModifiedAt: info.ModTime(),
	}, true
}

func workspaceInstallRuntimeDir(appRoot string) string {
	appRoot = strings.TrimSpace(appRoot)
	if appRoot == "" {
		return ""
	}
	return filepath.Join(appRoot, "runtime")
}

func containsCleanPath(paths []string, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	for _, path := range paths {
		if filepath.Clean(strings.TrimSpace(path)) == target {
			return true
		}
	}
	return false
}

func resolveWorkspaceLocalAgentBaseURL() string {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL")),
		strings.TrimSpace(os.Getenv("AGENT_BASE_URL")),
		defaultWorkspaceAgentBaseURL,
	} {
		if candidate != "" {
			return strings.TrimRight(candidate, "/")
		}
	}
	return defaultWorkspaceAgentBaseURL
}

func configuredWorkspaceRuntimeDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Workspace.RuntimeDir)
}

func reserveWorkspaceAgentPort() (string, string, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(defaultWorkspaceAgentListenHost, "0"))
	if err != nil {
		return "", "", fmt.Errorf("reserve workspace agent port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return "", "", fmt.Errorf("workspace agent reserved port unavailable")
	}
	port := fmt.Sprintf("%d", addr.Port)
	baseURL := fmt.Sprintf("http://%s:%s", defaultWorkspaceAgentListenHost, port)
	return port, baseURL, nil
}

func resolveWorkspaceNodeExecutable(installRoot string) (string, error) {
	candidates := []string{
		bundledWorkspaceNodePath(installRoot),
		filepath.Join(installRoot, "runtime", "node", "node.exe"),
		filepath.Join(installRoot, "runtime", "node", "win-x64", "node.exe"),
		filepath.Join(installRoot, "runtime", "node", "bin", "node"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if requiresBundledWorkspaceAgentRuntime(installRoot) {
		return "", fmt.Errorf("bundled node executable not found under %s", filepath.Join(installRoot, "runtime", "node"))
	}

	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("node executable not found")
}

func requiresBundledWorkspaceAgentRuntime(installRoot string) bool {
	if strings.TrimSpace(installRoot) == "" {
		return false
	}
	for _, marker := range []string{"ant-chrome.exe", "ant-chrome"} {
		if _, err := os.Stat(filepath.Join(installRoot, marker)); err == nil {
			return true
		}
	}
	return false
}

func workspaceAgentEntryPath(installRoot string) string {
	return filepath.Join(installRoot, "apps", "agent", "src", "server", "index.mjs")
}

func bundledWorkspaceNodePath(installRoot string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(installRoot, "runtime", "node", "node.exe")
	}
	return filepath.Join(installRoot, "runtime", "node", "bin", "node")
}

func validateBundledWorkspaceAgentPayload(installRoot string) error {
	if !requiresBundledWorkspaceAgentRuntime(installRoot) {
		return nil
	}
	agentEntry := workspaceAgentEntryPath(installRoot)
	if _, err := os.Stat(agentEntry); err != nil {
		return fmt.Errorf("workspace agent entry missing: %s", agentEntry)
	}
	if _, err := os.Stat(bundledWorkspaceNodePath(installRoot)); err != nil {
		return fmt.Errorf("bundled node runtime missing: %s", bundledWorkspaceNodePath(installRoot))
	}
	return nil
}

func openWorkspaceHostLogFile(runtimeDir string) (*os.File, error) {
	logPath := resolveWorkspaceHostLogPath(runtimeDir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create workspace host log dir failed: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open workspace host log failed: %w", err)
	}
	return file, nil
}

func resolveWorkspaceHostLogPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, "logs", "host-workspace-agent.log")
}

func appendWorkspaceHostLog(runtimeDir, format string, args ...interface{}) {
	if strings.TrimSpace(runtimeDir) == "" {
		runtimeDir = resolveWorkspaceRuntimeDir()
	}
	file, err := openWorkspaceHostLogFile(runtimeDir)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", time.Now().Format(time.RFC3339Nano), fmt.Sprintf(format, args...))
}

func (a *App) resolveWorkspaceAntRuntimeBaseURL() string {
	if a != nil && a.launchServer != nil {
		if port := a.launchServer.Port(); port > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", port)
		}
	}
	return defaultWorkspaceAntRuntimeBaseURL
}

func withWorkspaceAgentEnv(base []string, runtimeDir, serverOrigin, listenPort, agentBaseURL, antRuntimeBaseURL string) []string {
	envMap := make(map[string]string, len(base)+8)
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		envMap[key] = value
	}

	envMap["DESKTOP_RUNTIME_DIR"] = runtimeDir
	envMap["AGENT_STATE_DIR"] = runtimeDir
	envMap["DESKTOP_SERVER_BASE_URL"] = serverOrigin
	envMap["AGENT_LISTEN_HOST"] = defaultWorkspaceAgentListenHost
	envMap["AGENT_LISTEN_PORT"] = listenPort
	envMap["AGENT_BASE_URL"] = agentBaseURL
	if strings.TrimSpace(antRuntimeBaseURL) == "" {
		antRuntimeBaseURL = defaultWorkspaceAntRuntimeBaseURL
	}
	envMap["ANT_RUNTIME_BASE_URL"] = strings.TrimRight(strings.TrimSpace(antRuntimeBaseURL), "/")
	envMap["ANT_RUNTIME_AUTO_BOOT"] = "false"

	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	return result
}

func fetchWorkspaceBootstrapUser(serverOrigin, accessToken string) (workspaceBootstrapUser, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return workspaceBootstrapUser{}, fmt.Errorf("workspace bootstrap access token is required")
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, strings.TrimRight(serverOrigin, "/")+"/api/auth/me", nil)
	if err != nil {
		return workspaceBootstrapUser{}, err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("authorization", "Bearer "+accessToken)

	var me workspaceMeEnvelope
	if err := doWorkspaceJSON(request, &me); err != nil {
		return workspaceBootstrapUser{}, err
	}

	user := workspaceBootstrapUser{
		UserID: strings.TrimSpace(firstNonEmptyWorkspaceString(
			me.Data.UserSummary.ID,
			me.Data.User.ID,
			me.Data.ID,
		)),
		DisplayName: strings.TrimSpace(firstNonEmptyWorkspaceString(
			me.Data.UserSummary.DisplayName,
			me.Data.User.DisplayName,
			me.Data.DisplayName,
		)),
	}
	if user.UserID == "" {
		return workspaceBootstrapUser{}, fmt.Errorf("workspace bootstrap user id is required")
	}
	return user, nil
}

func bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin, accessToken string, user workspaceBootstrapUser) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return fmt.Errorf("workspace bootstrap access token is required")
	}

	userID := strings.TrimSpace(user.UserID)
	if userID == "" {
		return fmt.Errorf("workspace bootstrap user id is required")
	}
	displayName := strings.TrimSpace(user.DisplayName)

	requestBody := map[string]any{
		"accessToken": accessToken,
		"user": map[string]string{
			"userId":      userID,
			"displayName": displayName,
		},
		"serverBaseUrl": serverOrigin,
	}

	var bootstrap workspaceBootstrapEnvelope
	if err := postWorkspaceJSON(agentBaseURL+"/local/session/bootstrap", requestBody, &bootstrap); err != nil {
		return err
	}
	return nil
}

func warmWorkspaceAuthorizedShops(agentBaseURL string) (int, error) {
	var shops workspaceShopsEnvelope
	if err := getWorkspaceJSON(agentBaseURL+"/local/shops", &shops); err != nil {
		return 0, err
	}
	return len(shops.Data.Items), nil
}

func firstNonEmptyWorkspaceString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func waitForHTTPReachable(target string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHTTPReachable(target) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func isHTTPReachable(target string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 500
}

func postWorkspaceJSON(url string, body any, dest any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json")
	request.Header.Set("accept", "application/json")

	return doWorkspaceJSON(request, dest)
}

func getWorkspaceJSON(url string, dest any) error {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("accept", "application/json")
	return doWorkspaceJSON(request, dest)
}

func doWorkspaceJSON(request *http.Request, dest any) error {
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("request failed: %s (%d): %s", request.URL.String(), response.StatusCode, strings.TrimSpace(string(body)))
	}

	if dest == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dest)
}
