package launchcode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
)

// 按实例的 CDP 入口：/cdp/<实例名或ID>/...
//
// 与 "/" 不同，它不依赖“当前活动实例”，直接转发到指定实例自己的调试端口；
// 实例没在运行就按它自己的配置拉起，也不会改动 "/" 绑定的活动实例。
// /json/version、/json/list 等返回里的地址改写成经本入口访问的地址。

const (
	cdpProfilePrefix = "/cdp/"

	defaultCDPProfileTimeout = 60 * time.Second
	// 拉起结束后再挡一小段时间：App 的异步就绪监视可能稍晚才调 SetActiveProfile。
	defaultCDPActivationGrace = 2 * time.Second
)

// cdpProfileRoutes 保存 /cdp 入口的运行状态，零值可用。
type cdpProfileRoutes struct {
	mu        sync.Mutex
	flights   map[string]*cdpReadyFlight
	holds     map[string]int
	holdUntil map[string]time.Time
	timeout   time.Duration // 0 表示 defaultCDPProfileTimeout
	grace     time.Duration // 0 表示 defaultCDPActivationGrace
}

// cdpReadyFlight 是同一实例上一次“确认就绪/拉起”的共享结果，并发请求只跑一次。
type cdpReadyFlight struct {
	done    chan struct{}
	profile *browser.Profile
	err     *cdpProxyError
}

type cdpProxyError struct {
	status int
	msg    string
	extra  map[string]interface{}
}

func (c *cdpProfileRoutes) launchTimeout() time.Duration {
	if c.timeout > 0 {
		return c.timeout
	}
	return defaultCDPProfileTimeout
}

func (c *cdpProfileRoutes) activationGrace() time.Duration {
	if c.grace > 0 {
		return c.grace
	}
	return defaultCDPActivationGrace
}

// join 返回该实例正在进行的 flight；没有就新建一个，leader 为 true 时由调用方负责执行。
func (c *cdpProfileRoutes) join(profileID string) (flight *cdpReadyFlight, leader bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.flights[profileID]; ok {
		return existing, false
	}
	if c.flights == nil {
		c.flights = make(map[string]*cdpReadyFlight)
	}
	flight = &cdpReadyFlight{done: make(chan struct{})}
	c.flights[profileID] = flight
	return flight, true
}

func (c *cdpProfileRoutes) finish(profileID string, flight *cdpReadyFlight) {
	c.mu.Lock()
	if c.flights[profileID] == flight {
		delete(c.flights, profileID)
	}
	c.mu.Unlock()
	close(flight.done)
}

// holdActivation 在返回的 release 调用前挡住该实例的 SetActiveProfile；
// release(true) 会再多挡 activationGrace。
func (c *cdpProfileRoutes) holdActivation(profileID string) (release func(withGrace bool)) {
	c.mu.Lock()
	if c.holds == nil {
		c.holds = make(map[string]int)
	}
	c.holds[profileID]++
	c.mu.Unlock()

	return func(withGrace bool) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.holds[profileID] <= 1 {
			delete(c.holds, profileID)
		} else {
			c.holds[profileID]--
		}
		if withGrace {
			if c.holdUntil == nil {
				c.holdUntil = make(map[string]time.Time)
			}
			c.holdUntil[profileID] = time.Now().Add(c.activationGrace())
		}
	}
}

func (c *cdpProfileRoutes) activationHeld(profileID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.holds[profileID] > 0 {
		return true
	}
	until, ok := c.holdUntil[profileID]
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	delete(c.holdUntil, profileID)
	return false
}

// handleProfileCDP 处理 /cdp/<实例名或ID>/...，HTTP 与 WebSocket 升级都走这里。
func (s *LaunchServer) handleProfileCDP(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAPIKey(w, r) {
		return
	}

	ref, rest, err := parseCDPProfilePath(r.URL.EscapedPath())
	if err != nil {
		writeCDPProxyError(w, &cdpProxyError{status: http.StatusBadRequest, msg: err.Error()})
		return
	}
	profile, perr := s.resolveCDPProfile(ref)
	if perr != nil {
		writeCDPProxyError(w, perr)
		return
	}
	ready, perr := s.ensureCDPProfileReady(r, profile)
	if perr != nil {
		writeCDPProxyError(w, perr)
		return
	}
	s.proxyProfileCDP(w, r, ready, rest)
}

// parseCDPProfilePath 把转义后的路径拆成实例引用和要转发给 Chrome 的路径（仍是转义形式）。
// "/cdp/<ref>" 不带后续路径时转发 "/"。
func parseCDPProfilePath(escapedPath string) (ref string, rest string, err error) {
	tail, ok := strings.CutPrefix(escapedPath, cdpProfilePrefix)
	if !ok {
		return "", "", fmt.Errorf("path must start with %s", cdpProfilePrefix)
	}
	rawRef, rawRest, _ := strings.Cut(tail, "/")
	ref, err = url.PathUnescape(rawRef)
	if err != nil {
		return "", "", fmt.Errorf("invalid profile reference %q: %v", rawRef, err)
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("profile name or id is required: %s<name-or-id>/json/version", cdpProfilePrefix)
	}
	return ref, "/" + rawRest, nil
}

// resolveCDPProfile 先按 ID 精确匹配，再按名称匹配；同名多个时要求改用 ID。
func (s *LaunchServer) resolveCDPProfile(ref string) (browser.Profile, *cdpProxyError) {
	if s.browserMgr == nil {
		return browser.Profile{}, &cdpProxyError{status: http.StatusServiceUnavailable, msg: "profile manager is not available"}
	}

	s.browserMgr.Mutex.Lock()
	if profile, ok := s.browserMgr.Profiles[ref]; ok && profile != nil {
		snapshot := *profile
		s.browserMgr.Mutex.Unlock()
		return snapshot, nil
	}
	var matches []browser.Profile
	for _, profile := range s.browserMgr.Profiles {
		if profile != nil && strings.TrimSpace(profile.ProfileName) == ref {
			matches = append(matches, *profile)
		}
	}
	s.browserMgr.Mutex.Unlock()

	switch len(matches) {
	case 0:
		return browser.Profile{}, &cdpProxyError{status: http.StatusNotFound, msg: fmt.Sprintf("profile not found: %s", ref)}
	case 1:
		return matches[0], nil
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].ProfileId < matches[j].ProfileId })
	candidates := make([]map[string]string, 0, len(matches))
	for _, item := range matches {
		candidates = append(candidates, map[string]string{"profileId": item.ProfileId, "profileName": item.ProfileName})
	}
	return browser.Profile{}, &cdpProxyError{
		status: http.StatusConflict,
		msg:    fmt.Sprintf("profile name %q matches %d profiles, use the profile id instead: %s<profile-id>/...", ref, len(matches), cdpProfilePrefix),
		extra:  map[string]interface{}{"candidates": candidates},
	}
}

// ensureCDPProfileReady 确认实例在运行且调试端口就绪，否则拉起并等待。
// 同一实例的并发请求共享一次 flight；单个请求最多等 launchTimeout，
// 超时后 flight 仍在后台跑完，后续请求可以直接接上。
func (s *LaunchServer) ensureCDPProfileReady(r *http.Request, profile browser.Profile) (*browser.Profile, *cdpProxyError) {
	if s.starter == nil {
		return nil, &cdpProxyError{status: http.StatusServiceUnavailable, msg: "profile runtime control is not available"}
	}

	timeout := s.cdpProfiles.launchTimeout()
	flight, leader := s.cdpProfiles.join(profile.ProfileId)
	if leader {
		go s.runCDPReadyFlight(flight, profile, timeout, r.Method, r.URL.Path, remoteIP(r.RemoteAddr))
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-flight.done:
		return flight.profile, flight.err
	case <-r.Context().Done():
		return nil, &cdpProxyError{status: http.StatusServiceUnavailable, msg: "request canceled while waiting for profile"}
	case <-timer.C:
		return nil, cdpNotReadyError(profile, timeout, nil)
	}
}

func (s *LaunchServer) runCDPReadyFlight(flight *cdpReadyFlight, profile browser.Profile, timeout time.Duration, method, path, clientIP string) {
	profileID := profile.ProfileId
	launched := false
	release := s.cdpProfiles.holdActivation(profileID)
	defer func() {
		release(launched)
		s.cdpProfiles.finish(profileID, flight)
	}()

	snapshot, status, errMsg := s.statusProfile(profileID)
	if errMsg != "" {
		flight.err = &cdpProxyError{status: status, msg: errMsg}
		return
	}
	if cdpProfileReady(snapshot) {
		flight.profile = snapshot
		return
	}

	launched = true
	startAt := time.Now()
	logResult := func(status int, errMsg string) {
		s.appendLaunchLog(method, path, clientIP, "", LaunchSelector{ProfileID: profileID}, LaunchRequestParams{},
			errMsg == "", status, errMsg, profileID, profile.ProfileName, startAt)
	}

	started, err := s.launchProfile(profileID, LaunchRequestParams{})
	if err != nil {
		flight.err = &cdpProxyError{status: http.StatusBadGateway, msg: fmt.Sprintf("failed to start profile %s: %v", cdpProfileLabel(profile), err)}
		logResult(flight.err.status, flight.err.msg)
		return
	}

	waited, ready, err := s.waitRuntimeDebugReady(started, timeout-time.Since(startAt), nil)
	if err != nil {
		flight.err = &cdpProxyError{status: mapProfileWriteErrorStatus(err), msg: err.Error()}
		logResult(flight.err.status, flight.err.msg)
		return
	}
	if !ready || !cdpProfileReady(waited) {
		flight.err = cdpNotReadyError(profile, timeout, waited)
		logResult(flight.err.status, flight.err.msg)
		return
	}
	flight.profile = waited
	logResult(http.StatusOK, "")
}

func cdpProfileReady(profile *browser.Profile) bool {
	return profile != nil && profile.Running && profile.DebugReady && profile.DebugPort > 0
}

func cdpProfileLabel(profile browser.Profile) string {
	if name := strings.TrimSpace(profile.ProfileName); name != "" {
		return fmt.Sprintf("%q (%s)", name, profile.ProfileId)
	}
	return profile.ProfileId
}

func cdpNotReadyError(profile browser.Profile, timeout time.Duration, latest *browser.Profile) *cdpProxyError {
	msg := fmt.Sprintf("debug port of profile %s is not ready (waited up to %s); it may still be starting, retry later", cdpProfileLabel(profile), timeout)
	if latest != nil {
		if detail := strings.TrimSpace(firstNonEmpty(latest.LastError, latest.RuntimeWarning)); detail != "" {
			msg += ": " + detail
		}
	}
	return &cdpProxyError{status: http.StatusGatewayTimeout, msg: msg}
}

func writeCDPProxyError(w http.ResponseWriter, perr *cdpProxyError) {
	body := map[string]interface{}{"ok": false, "error": perr.msg}
	for k, v := range perr.extra {
		body[k] = v
	}
	writeJSON(w, perr.status, body)
}

// proxyProfileCDP 把请求转发到实例的调试端口。
// Host 改成 127.0.0.1:<端口>：Chrome 只接受 IP 或 localhost 的 Host，
// 返回里的地址再由 cdpPublicBase 改写成请求方看到的 Host。
func (s *LaunchServer) proxyProfileCDP(w http.ResponseWriter, r *http.Request, profile *browser.Profile, rest string) {
	target := net.JoinHostPort("127.0.0.1", strconv.Itoa(profile.DebugPort))
	public := newCDPPublicBase(r, profile.ProfileId)
	if public.host == "" {
		public.host = net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port()))
	}
	discovery := isCDPDiscoveryPath(rest)
	authHeader := s.apiAuthConfig().Header

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			pr.Out.URL.Host = target
			pr.Out.Host = target
			pr.Out.URL.Path = rest
			if unescaped, err := url.PathUnescape(rest); err == nil {
				pr.Out.URL.Path = unescaped
			}
			pr.Out.URL.RawPath = rest
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			pr.Out.Header.Del(authHeader)
			if discovery {
				pr.Out.Header.Del("Accept-Encoding")
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			if !discovery || resp.StatusCode != http.StatusOK {
				return nil
			}
			return rewriteCDPDiscoveryResponse(resp, public)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
			writeCDPProxyError(w, &cdpProxyError{
				status: http.StatusBadGateway,
				msg:    fmt.Sprintf("cdp proxy error for profile %s: %v", profile.ProfileId, proxyErr),
			})
		},
	}
	proxy.ServeHTTP(w, r)
}

// isCDPDiscoveryPath 判断是否是返回调试地址的 /json 端点；Playwright 会带尾斜杠请求。
func isCDPDiscoveryPath(rest string) bool {
	switch strings.TrimSuffix(rest, "/") {
	case "/json", "/json/list", "/json/version", "/json/new":
		return true
	}
	return false
}

// cdpPublicBase 描述请求方看到的入口：scheme、Host 和 /cdp/<实例ID> 前缀。
type cdpPublicBase struct {
	secure    bool
	host      string
	profileID string
}

func newCDPPublicBase(r *http.Request, profileID string) cdpPublicBase {
	return cdpPublicBase{
		secure:    r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https"),
		host:      r.Host,
		profileID: profileID,
	}
}

func (b cdpPublicBase) wsScheme() string {
	if b.secure {
		return "wss"
	}
	return "ws"
}

func (b cdpPublicBase) pathPrefix() string {
	return cdpProfilePrefix + b.profileID
}

func (b cdpPublicBase) escapedPathPrefix() string {
	return cdpProfilePrefix + url.PathEscape(b.profileID)
}

// webSocketURL: ws://127.0.0.1:9222/devtools/page/X -> ws://<host>/cdp/<id>/devtools/page/X
func (b cdpPublicBase) webSocketURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	escaped := u.EscapedPath()
	u.Scheme = b.wsScheme()
	u.Host = b.host
	u.Path = b.pathPrefix() + u.Path
	u.RawPath = b.escapedPathPrefix() + escaped
	return u.String()
}

// frontendURL 改写 ws=/wss= 参数；相对路径（Chrome 自带的 /devtools/inspector.html）再加上入口前缀。
func (b cdpPublicBase) frontendURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "" && u.Host == "" && strings.HasPrefix(u.Path, "/") {
		escaped := u.EscapedPath()
		u.Path = b.pathPrefix() + u.Path
		u.RawPath = b.escapedPathPrefix() + escaped
	}

	parts := strings.Split(u.RawQuery, "&")
	for i, part := range parts {
		key, value, _ := strings.Cut(part, "=")
		if key != "ws" && key != "wss" {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			continue
		}
		// 参数值形如 127.0.0.1:9222/devtools/page/X，不带 scheme
		_, path, found := strings.Cut(decoded, "/")
		if !found {
			continue
		}
		parts[i] = b.wsScheme() + "=" + b.host + b.escapedPathPrefix() + "/" + path
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}

func rewriteCDPDiscoveryResponse(resp *http.Response, public cdpPublicBase) error {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	if rewritten, rewriteErr := rewriteCDPDiscoveryBody(body, public); rewriteErr == nil {
		body = rewritten
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

// rewriteCDPDiscoveryBody 改写对象或对象数组里的 webSocketDebuggerUrl 与 devtoolsFrontendUrl。
func rewriteCDPDiscoveryBody(body []byte, public cdpPublicBase) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var doc interface{}
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}

	var items []interface{}
	switch v := doc.(type) {
	case []interface{}:
		items = v
	case map[string]interface{}:
		items = []interface{}{v}
	default:
		return nil, errors.New("unexpected discovery payload")
	}
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if raw, ok := obj["webSocketDebuggerUrl"].(string); ok {
			obj["webSocketDebuggerUrl"] = public.webSocketURL(raw)
		}
		if raw, ok := obj["devtoolsFrontendUrl"].(string); ok {
			obj["devtoolsFrontendUrl"] = public.frontendURL(raw)
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
