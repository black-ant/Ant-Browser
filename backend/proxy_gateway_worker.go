package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/gateway"
	"ant-chrome/backend/internal/proxy"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	proxyGatewayMaxRequestBytes = 8 << 20
	proxyGatewayIdleTimeout     = 30 * time.Second
	proxyGatewayUnattachedTTL   = 2 * time.Minute
)

type proxyGatewayWorkerProfile struct {
	gateway    *gateway.ProfileGateway
	browserPID int
	debugPort  int
	monitorSeq uint64
	preparedAt time.Time
}

type proxyGatewayWorker struct {
	appRoot  string
	token    string
	config   *config.Config
	xray     *proxy.XrayManager
	clash    *proxy.ClashManager
	singbox  *proxy.SingBoxManager
	server   *http.Server
	listener net.Listener

	mu           sync.Mutex
	profiles     map[string]*proxyGatewayWorkerProfile
	lastActivity time.Time
	closed       bool
	closeOnce    sync.Once
	done         chan struct{}
}

func RunProxyGatewayWorkerFromArgs(appRoot string, args []string) (bool, error) {
	if len(args) < 2 || strings.TrimSpace(args[0]) != proxyGatewayWorkerArg {
		return false, nil
	}
	launchPath := strings.TrimSpace(args[1])
	if launchPath == "" {
		return true, fmt.Errorf("proxy gateway worker launch file is missing")
	}
	data, err := os.ReadFile(launchPath)
	if err != nil {
		return true, err
	}
	var launch proxyGatewayWorkerLaunchConfig
	if err := json.Unmarshal(data, &launch); err != nil {
		return true, err
	}
	if strings.TrimSpace(launch.AppRoot) != "" {
		appRoot = strings.TrimSpace(launch.AppRoot)
	}
	worker, err := newProxyGatewayWorker(appRoot, launch.ControlPort, launch.Token)
	if err != nil {
		return true, err
	}
	defer func() {
		worker.close()
		removeWorkerLaunchFile(launchPath, launch.Token)
	}()
	return true, worker.run()
}

func newProxyGatewayWorker(appRoot string, controlPort int, token string) (*proxyGatewayWorker, error) {
	if controlPort < 1 || controlPort > 65535 {
		return nil, fmt.Errorf("invalid proxy gateway control port")
	}
	if len(strings.TrimSpace(token)) < 32 {
		return nil, fmt.Errorf("invalid proxy gateway control token")
	}
	cfg, err := LoadConfig(ResolveRuntimePath(appRoot, "config.yaml"))
	if err != nil {
		cfg = config.DefaultConfig()
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", controlPort))
	if err != nil {
		return nil, err
	}
	worker := &proxyGatewayWorker{
		appRoot:      appRoot,
		token:        token,
		config:       cfg,
		xray:         proxy.NewXrayManager(cfg, appRoot),
		clash:        proxy.NewClashManager(cfg, appRoot),
		singbox:      proxy.NewSingBoxManager(cfg, appRoot),
		listener:     listener,
		profiles:     make(map[string]*proxyGatewayWorkerProfile),
		lastActivity: time.Now(),
		done:         make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", worker.authorize(worker.handleHealth))
	mux.HandleFunc("/v1/profiles/prepare", worker.authorize(worker.handlePrepare))
	mux.HandleFunc("/v1/profiles/routing", worker.authorize(worker.handleRouting))
	mux.HandleFunc("/v1/profiles/attach", worker.authorize(worker.handleAttach))
	mux.HandleFunc("/v1/profiles/stop", worker.authorize(worker.handleStopProfile))
	mux.HandleFunc("/v1/profiles/status", worker.authorize(worker.handleProfileStatus))
	mux.HandleFunc("/v1/shutdown", worker.authorize(worker.handleShutdown))
	worker.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	return worker, nil
}

func (w *proxyGatewayWorker) run() error {
	go w.idleLoop()
	err := w.server.Serve(w.listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (w *proxyGatewayWorker) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		provided := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(w.token)) != 1 {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(response, request)
	}
}

func (w *proxyGatewayWorker) handleHealth(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.mu.Lock()
	profiles := make([]gateway.Status, 0, len(w.profiles))
	for _, item := range w.profiles {
		status := item.gateway.Status()
		status.BrowserPID = item.browserPID
		status.BrowserDebugPort = item.debugPort
		profiles = append(profiles, status)
	}
	w.mu.Unlock()
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ProfileID < profiles[j].ProfileID })
	writeGatewayJSON(response, http.StatusOK, proxyGatewayStatusResponse{OK: true, PID: os.Getpid(), Profiles: profiles})
}

func (w *proxyGatewayWorker) handlePrepare(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	input, err := decodeGatewayProfileRequest(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	route, err := w.prepareRoute(input)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadGateway)
		return
	}

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		releaseGatewayRoute(route)
		http.Error(response, "proxy gateway worker is closed", http.StatusServiceUnavailable)
		return
	}
	item := w.profiles[input.ProfileID]
	if item == nil {
		profileGateway, startErr := gateway.StartProfileGateway(input.ProfileID, route, input.Routing)
		if startErr != nil {
			w.mu.Unlock()
			releaseGatewayRoute(route)
			http.Error(response, startErr.Error(), http.StatusBadGateway)
			return
		}
		item = &proxyGatewayWorkerProfile{gateway: profileGateway, preparedAt: time.Now()}
		w.profiles[input.ProfileID] = item
		w.lastActivity = time.Now()
		status := item.gateway.Status()
		w.mu.Unlock()
		writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: status})
		return
	}
	w.lastActivity = time.Now()
	if item.browserPID <= 0 && item.debugPort <= 0 {
		item.preparedAt = time.Now()
	}
	w.mu.Unlock()
	status, switchErr := item.gateway.Switch(route, input.Routing, input.Force)
	if switchErr != nil {
		releaseGatewayRoute(route)
		http.Error(response, switchErr.Error(), http.StatusBadGateway)
		return
	}
	writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: status})
}

func (w *proxyGatewayWorker) handleRouting(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	input, err := decodeGatewayProfileRequest(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	w.mu.Lock()
	item := w.profiles[input.ProfileID]
	if item != nil {
		w.lastActivity = time.Now()
	}
	w.mu.Unlock()
	if item == nil {
		http.Error(response, "profile gateway is not running", http.StatusNotFound)
		return
	}
	status, err := item.gateway.UpdateRouting(input.Routing, input.Force)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadGateway)
		return
	}
	writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: status})
}

func (w *proxyGatewayWorker) handleAttach(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	input, err := decodeGatewayProfileRequest(request)
	if err != nil || (input.BrowserPID <= 0 && input.BrowserDebugPort <= 0) {
		http.Error(response, "invalid browser process", http.StatusBadRequest)
		return
	}
	w.mu.Lock()
	item := w.profiles[input.ProfileID]
	if item == nil {
		w.mu.Unlock()
		http.Error(response, "profile gateway is not running", http.StatusNotFound)
		return
	}
	item.browserPID = input.BrowserPID
	item.debugPort = input.BrowserDebugPort
	item.preparedAt = time.Time{}
	item.monitorSeq++
	sequence := item.monitorSeq
	w.lastActivity = time.Now()
	status := item.gateway.Status()
	status.BrowserPID = item.browserPID
	status.BrowserDebugPort = item.debugPort
	w.mu.Unlock()
	go w.monitorBrowser(input.ProfileID, input.BrowserPID, input.BrowserDebugPort, sequence)
	writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: status})
}

func (w *proxyGatewayWorker) handleStopProfile(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	input, err := decodeGatewayProfileRequest(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	w.stopProfile(input.ProfileID)
	writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: gateway.Status{ProfileID: input.ProfileID}})
}

func (w *proxyGatewayWorker) handleProfileStatus(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	profileID := strings.TrimSpace(request.URL.Query().Get("profileId"))
	w.mu.Lock()
	item := w.profiles[profileID]
	browserPID := 0
	browserDebugPort := 0
	if item != nil {
		browserPID = item.browserPID
		browserDebugPort = item.debugPort
	}
	w.mu.Unlock()
	if item == nil {
		http.Error(response, "profile gateway is not running", http.StatusNotFound)
		return
	}
	status := item.gateway.Status()
	status.BrowserPID = browserPID
	status.BrowserDebugPort = browserDebugPort
	writeGatewayJSON(response, http.StatusOK, proxyGatewayProfileResponse{Status: status})
}

func (w *proxyGatewayWorker) handleShutdown(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeGatewayJSON(response, http.StatusOK, map[string]bool{"ok": true})
	go w.close()
}

func (w *proxyGatewayWorker) prepareRoute(input proxyGatewayProfileRequest) (gateway.RouteSpec, error) {
	profileID := strings.TrimSpace(input.ProfileID)
	proxyID := strings.TrimSpace(input.ProxyID)
	proxyConfig := strings.TrimSpace(input.ProxyConfig)
	if profileID == "" {
		return gateway.RouteSpec{}, fmt.Errorf("profile id is required")
	}
	if supported, message := proxy.ValidateProxyConfig(proxyConfig, input.Proxies, proxyID); !supported {
		return gateway.RouteSpec{}, fmt.Errorf("invalid proxy configuration: %s", message)
	}
	resolvedConfig := strings.TrimSpace(resolveWorkerProxyConfig(proxyConfig, input.Proxies, proxyID))
	routeID := proxyID
	if routeID == "" {
		routeID = "custom"
	}
	if strings.EqualFold(resolvedConfig, "direct://") {
		return gateway.RouteSpec{ID: routeID, Dial: gateway.DirectDialer()}, nil
	}
	connector := config.NormalizeBrowserConnectorType(input.ConnectorType)
	resolution, err := proxy.ResolveProxyKernelForConnector(resolvedConfig, input.Proxies, proxyID, connector)
	if err != nil {
		return gateway.RouteSpec{}, err
	}
	var endpoint string
	var release func()
	switch resolution.Kernel {
	case proxy.ProxyKernelMihomo:
		var key string
		endpoint, key, err = w.clash.AcquireNodeBridge(resolvedConfig, input.Proxies, proxyID)
		if key != "" {
			release = func() { w.clash.ReleaseNodeBridge(key) }
		}
	case proxy.ProxyKernelSingBox:
		var key string
		endpoint, key, err = w.singbox.AcquireBridge(resolvedConfig, input.Proxies, proxyID)
		if key != "" {
			release = func() { w.singbox.ReleaseBridge(key) }
		}
	case proxy.ProxyKernelXray:
		var key string
		endpoint, key, err = w.xray.AcquireBridge(resolvedConfig, input.Proxies, proxyID)
		if key != "" {
			release = func() { w.xray.ReleaseBridge(key) }
		}
	case proxy.ProxyKernelNative:
		endpoint = resolvedConfig
	default:
		err = fmt.Errorf("unsupported proxy kernel: %s", resolution.Kernel)
	}
	if err != nil {
		if release != nil {
			release()
		}
		return gateway.RouteSpec{}, err
	}
	if w.isGatewayLoop(endpoint) {
		if release != nil {
			release()
		}
		return gateway.RouteSpec{}, fmt.Errorf("proxy gateway loop detected")
	}
	dial, err := gateway.URLDialer(endpoint)
	if err != nil {
		if release != nil {
			release()
		}
		return gateway.RouteSpec{}, err
	}
	return gateway.RouteSpec{ID: routeID, Dial: dial, Release: release}, nil
}

func (w *proxyGatewayWorker) isGatewayLoop(endpoint string) bool {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return false
	}
	target := strings.ToLower(parsed.Host)
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, item := range w.profiles {
		gatewayURL, parseErr := url.Parse(item.gateway.ProxyURL())
		if parseErr == nil && strings.ToLower(gatewayURL.Host) == target {
			return true
		}
	}
	return false
}

func resolveWorkerProxyConfig(proxyConfig string, proxies []config.BrowserProxy, proxyID string) string {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID != "" {
		for _, item := range proxies {
			if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyID) {
				return strings.TrimSpace(item.ProxyConfig)
			}
		}
	}
	return strings.TrimSpace(proxyConfig)
}

func releaseGatewayRoute(route gateway.RouteSpec) {
	if route.Release != nil {
		route.Release()
	}
}

func (w *proxyGatewayWorker) monitorBrowser(profileID string, pid int, debugPort int, sequence uint64) {
	misses := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		w.mu.Lock()
		item := w.profiles[profileID]
		valid := item != nil && item.browserPID == pid && item.debugPort == debugPort && item.monitorSeq == sequence && !w.closed
		w.mu.Unlock()
		if !valid {
			return
		}
		alive := false
		if debugPort > 0 {
			alive = canConnectDebugPort(debugPort, 500*time.Millisecond)
		} else if pid > 0 {
			alive = isProcessAlive(pid)
		}
		if alive {
			misses = 0
			continue
		}
		misses++
		if misses >= 3 {
			w.stopProfile(profileID)
			return
		}
	}
}

func (w *proxyGatewayWorker) stopProfile(profileID string) {
	profileID = strings.TrimSpace(profileID)
	w.mu.Lock()
	item := w.profiles[profileID]
	delete(w.profiles, profileID)
	w.lastActivity = time.Now()
	w.mu.Unlock()
	if item != nil {
		_ = item.gateway.Close()
	}
}

func (w *proxyGatewayWorker) idleLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		w.mu.Lock()
		expired := w.detachExpiredUnattachedProfilesLocked(time.Now(), proxyGatewayUnattachedTTL)
		shouldClose := !w.closed && len(w.profiles) == 0 && time.Since(w.lastActivity) >= proxyGatewayIdleTimeout
		w.mu.Unlock()
		for _, item := range expired {
			if item.gateway != nil {
				_ = item.gateway.Close()
			}
		}
		if shouldClose {
			w.close()
			return
		}
	}
}

func (w *proxyGatewayWorker) detachExpiredUnattachedProfilesLocked(now time.Time, ttl time.Duration) []*proxyGatewayWorkerProfile {
	if ttl <= 0 {
		return nil
	}
	expired := make([]*proxyGatewayWorkerProfile, 0)
	for profileID, item := range w.profiles {
		if item == nil || item.browserPID > 0 || item.debugPort > 0 || item.preparedAt.IsZero() {
			continue
		}
		if now.Sub(item.preparedAt) >= ttl {
			expired = append(expired, item)
			delete(w.profiles, profileID)
		}
	}
	if len(expired) > 0 {
		w.lastActivity = now
	}
	return expired
}

func (w *proxyGatewayWorker) close() {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		profiles := make([]*proxyGatewayWorkerProfile, 0, len(w.profiles))
		for _, item := range w.profiles {
			profiles = append(profiles, item)
		}
		w.profiles = make(map[string]*proxyGatewayWorkerProfile)
		w.mu.Unlock()
		for _, item := range profiles {
			_ = item.gateway.Close()
		}
		if w.xray != nil {
			w.xray.StopAll()
		}
		if w.clash != nil {
			w.clash.StopAll()
		}
		if w.singbox != nil {
			w.singbox.StopAll()
		}
		if w.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = w.server.Shutdown(ctx)
			cancel()
		}
		close(w.done)
	})
}

func decodeGatewayProfileRequest(request *http.Request) (proxyGatewayProfileRequest, error) {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, proxyGatewayMaxRequestBytes))
	var input proxyGatewayProfileRequest
	if err := decoder.Decode(&input); err != nil {
		return input, err
	}
	input.ProfileID = strings.TrimSpace(input.ProfileID)
	if input.ProfileID == "" {
		return input, fmt.Errorf("profile id is required")
	}
	return input, nil
}

func writeGatewayJSON(response http.ResponseWriter, status int, value interface{}) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func removeWorkerLaunchFile(path string, token string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var current proxyGatewayWorkerLaunchConfig
	if json.Unmarshal(data, &current) == nil && current.Token == token {
		_ = os.Remove(path)
		_ = os.Remove(filepath.Join(filepath.Dir(path), "worker.pid"))
	}
}
