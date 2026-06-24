package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/cdp"
	"ant-chrome/backend/internal/logger"

	"github.com/gorilla/websocket"
)

const (
	antGeolocationArgPrefix           = "--ant-geolocation="
	antGeolocationModeArgPrefix       = "--ant-geolocation-mode="
	antGeolocationPermissionArgPrefix = "--ant-geolocation-permission="
)

type profileGeolocationOverride struct {
	Latitude    float64
	Longitude   float64
	Accuracy    float64
	HasPosition bool
	Permission  string
	Explicit    bool
	Source      string
}

func (g profileGeolocationOverride) shouldApply() bool {
	return g.HasPosition || g.Permission == "allow" || g.Permission == "block"
}

func (g profileGeolocationOverride) cdpParams() map[string]any {
	return map[string]any{
		"latitude":  g.Latitude,
		"longitude": g.Longitude,
		"accuracy":  g.Accuracy,
	}
}

func extractProfileGeolocationArgs(args []string) ([]string, profileGeolocationOverride, []string) {
	if len(args) == 0 {
		return nil, profileGeolocationOverride{Permission: "prompt"}, nil
	}

	filtered := make([]string, 0, len(args))
	geo := profileGeolocationOverride{Permission: "prompt"}
	warnings := make([]string, 0, 2)

	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}

		switch {
		case strings.HasPrefix(arg, antGeolocationArgPrefix):
			geo.Explicit = true
			geo.Source = "profile"
			lat, lon, accuracy, err := parseAntGeolocationValue(strings.TrimPrefix(arg, antGeolocationArgPrefix))
			if err != nil {
				warnings = append(warnings, err.Error())
				continue
			}
			geo.Latitude = lat
			geo.Longitude = lon
			geo.Accuracy = accuracy
			geo.HasPosition = true
		case strings.HasPrefix(arg, antGeolocationModeArgPrefix):
			geo.Explicit = true
			geo.Source = "profile"
		case strings.HasPrefix(arg, antGeolocationPermissionArgPrefix):
			geo.Explicit = true
			geo.Permission = normalizeGeolocationPermission(strings.TrimPrefix(arg, antGeolocationPermissionArgPrefix))
		default:
			filtered = append(filtered, arg)
		}
	}

	return filtered, geo, warnings
}

func parseAntGeolocationValue(value string) (float64, float64, float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation value: expected lat,lon[,accuracy]")
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation longitude: %w", err)
	}

	accuracy := 100.0
	if len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
		accuracy, err = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid internal geolocation accuracy: %w", err)
		}
	}

	if lat < -90 || lat > 90 {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation latitude: %.6f out of range", lat)
	}
	if lon < -180 || lon > 180 {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation longitude: %.6f out of range", lon)
	}
	if accuracy <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid internal geolocation accuracy: %.2f must be positive", accuracy)
	}

	return lat, lon, accuracy, nil
}

func normalizeGeolocationPermission(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "granted":
		return "allow"
	case "block", "deny", "denied":
		return "block"
	default:
		return "prompt"
	}
}

func proxyGeolocationOverride(healthJSON string) (profileGeolocationOverride, bool) {
	geo, ok := browser.DeriveProxyGeo(healthJSON)
	if !ok || !geo.HasLatLon {
		return profileGeolocationOverride{}, false
	}
	return profileGeolocationOverride{
		Latitude:    geo.Lat,
		Longitude:   geo.Lon,
		Accuracy:    100,
		HasPosition: true,
		Permission:  "prompt",
		Source:      "proxy",
	}, true
}

func (a *App) applyProfileGeolocation(profileId string, geo profileGeolocationOverride) error {
	if !geo.shouldApply() {
		return nil
	}

	pc, err := a.profileCDP(profileId)
	if err != nil {
		return err
	}

	switch geo.Permission {
	case "allow":
		if err := pc.callBrowser("Browser.grantPermissions", map[string]any{
			"permissions": []string{"geolocation"},
		}); err != nil {
			return err
		}
	case "block":
		if err := pc.callBrowser("Browser.setPermission", map[string]any{
			"permission": map[string]any{"name": "geolocation"},
			"setting":    "denied",
		}); err != nil {
			return err
		}
	}

	if !geo.HasPosition {
		return nil
	}

	return applyGeolocationToPageTargets(pc, geo.cdpParams())
}

func (a *App) applyAndWatchProfileGeolocation(profileId string, debugPort int, geo profileGeolocationOverride) error {
	err := a.applyProfileGeolocation(profileId, geo)
	a.startProfileGeolocationWatcher(profileId, debugPort, geo)
	return err
}

func applyGeolocationToPageTargets(pc *profileCDP, params map[string]any) error {
	if pc.pipe != nil {
		return applyGeolocationToPipePageTargets(pc.pipe, params)
	}
	return applyGeolocationToPortPageTargets(pc.debugPort, params)
}

func applyGeolocationToPipePageTargets(conn *cdp.PipeConn, params map[string]any) error {
	res, err := conn.SendCommand("", "Target.getTargets", nil)
	if err != nil {
		return err
	}
	infos, ok := res["targetInfos"].([]interface{})
	if !ok {
		return fmt.Errorf("CDP target list has unexpected format")
	}

	applied := 0
	var firstErr error
	for _, item := range infos {
		info, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if targetType, _ := info["type"].(string); targetType != "page" {
			continue
		}
		targetID, _ := info["targetId"].(string)
		if strings.TrimSpace(targetID) == "" {
			continue
		}
		attachResult, err := conn.SendCommand("", "Target.attachToTarget", map[string]interface{}{
			"targetId": targetID,
			"flatten":  true,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sessionID, _ := attachResult["sessionId"].(string)
		if strings.TrimSpace(sessionID) == "" {
			if firstErr == nil {
				firstErr = fmt.Errorf("CDP attach did not return sessionId")
			}
			continue
		}
		if _, err := conn.SendCommand(sessionID, "Emulation.setGeolocationOverride", params); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			_, _ = conn.SendCommand("", "Target.detachFromTarget", map[string]interface{}{"sessionId": sessionID})
			continue
		}
		applied++
		_, _ = conn.SendCommand("", "Target.detachFromTarget", map[string]interface{}{"sessionId": sessionID})
	}

	if applied == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("no page target available for geolocation override")
	}
	return nil
}

func applyGeolocationToPortPageTargets(debugPort int, params map[string]any) error {
	targets, err := cdpPortPageTargets(debugPort)
	if err != nil {
		return err
	}

	applied := 0
	var firstErr error
	for _, target := range targets {
		if target.Type != "page" || strings.TrimSpace(target.WebSocketDebuggerUrl) == "" {
			continue
		}
		if err := cdpWebSocketCall(target.WebSocketDebuggerUrl, "Emulation.setGeolocationOverride", params, 5*time.Second); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		applied++
	}

	if applied == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("no page target available for geolocation override")
	}
	return nil
}

func cdpPortPageTargets(debugPort int) ([]cdpTarget, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", debugPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

type targetAttachCDPMessage struct {
	ID        int            `json:"id,omitempty"`
	Method    string         `json:"method,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *App) startProfileGeolocationWatcher(profileId string, debugPort int, geo profileGeolocationOverride) {
	if !geo.HasPosition || debugPort <= 0 {
		return
	}
	go a.watchPortGeolocationTargets(profileId, debugPort, geo.cdpParams())
}

func (a *App) watchPortGeolocationTargets(profileId string, debugPort int, params map[string]any) {
	log := logger.New("Browser")
	wsURL, err := cdpBrowserWebSocketURL(debugPort)
	if err != nil {
		log.Warn("地理定位新标签监听启动失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()))
		return
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Warn("地理定位新标签监听连接失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()))
		return
	}
	defer conn.Close()

	stop := make(chan struct{})
	go a.closeGeolocationWatcherWhenInactive(profileId, debugPort, conn, stop)
	defer close(stop)

	nextID := 1
	pending := make(map[int]string)
	if err := writeTargetAttachCDPCommand(conn, nextID, "", "Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": false,
		"flatten":                true,
	}); err != nil {
		log.Warn("地理定位新标签监听启用失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()))
		return
	}
	pending[nextID] = "Target.setAutoAttach"
	nextID++

	for {
		var msg targetAttachCDPMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if a.profileDebugPortActive(profileId, debugPort) {
				log.Warn("地理定位新标签监听中断",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
					logger.F("error", err.Error()))
			}
			return
		}

		if msg.ID > 0 {
			method := pending[msg.ID]
			delete(pending, msg.ID)
			if msg.Error != nil {
				log.Warn("地理定位新标签 CDP 命令失败",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
					logger.F("method", method),
					logger.F("error", msg.Error.Message))
				if method == "Target.setAutoAttach" {
					return
				}
			}
			continue
		}

		sessionID, ok := attachedPageSessionID(msg)
		if !ok {
			continue
		}
		method := "Emulation.setGeolocationOverride"
		if err := writeTargetAttachCDPCommand(conn, nextID, sessionID, method, params); err != nil {
			log.Warn("新标签地理定位覆盖失败",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
				logger.F("session_id", sessionID),
				logger.F("error", err.Error()))
			return
		}
		pending[nextID] = method
		nextID++
	}
}

func attachedPageSessionID(msg targetAttachCDPMessage) (string, bool) {
	if msg.Method != "Target.attachedToTarget" || msg.Params == nil {
		return "", false
	}
	sessionID, _ := msg.Params["sessionId"].(string)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", false
	}
	targetInfo, _ := msg.Params["targetInfo"].(map[string]any)
	if targetInfo == nil {
		return "", false
	}
	targetType, _ := targetInfo["type"].(string)
	if targetType != "page" {
		return "", false
	}
	return sessionID, true
}

func writeTargetAttachCDPCommand(conn *websocket.Conn, id int, sessionID string, method string, params map[string]any) error {
	msg := targetAttachCDPMessage{
		ID:        id,
		Method:    method,
		Params:    params,
		SessionID: strings.TrimSpace(sessionID),
	}
	return conn.WriteJSON(msg)
}

func (a *App) closeGeolocationWatcherWhenInactive(profileId string, debugPort int, conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !a.profileDebugPortActive(profileId, debugPort) {
				_ = conn.Close()
				return
			}
		}
	}
}

func (a *App) profileDebugPortActive(profileId string, debugPort int) bool {
	if a == nil || a.browserMgr == nil || debugPort <= 0 {
		return false
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, ok := a.browserMgr.Profiles[profileId]
	return ok && profile != nil && profile.Running && profile.DebugReady && profile.DebugPort == debugPort
}

func cdpBrowserWebSocketURL(debugPort int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", debugPort))
	if err != nil {
		return "", fmt.Errorf("CDP /json/version request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var version cdpBrowserVersion
	if err := json.Unmarshal(body, &version); err != nil {
		return "", fmt.Errorf("CDP browser target parse failed: %w", err)
	}
	wsURL := strings.TrimSpace(version.WebSocketDebuggerUrl)
	if wsURL == "" {
		return "", fmt.Errorf("browser-level WebSocket debugger URL is empty")
	}
	return wsURL, nil
}

func cdpWebSocketCall(wsURL string, method string, params map[string]any, timeout time.Duration) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(timeout))

	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}

	for {
		var resp cdpResponse
		if err := conn.ReadJSON(&resp); err != nil {
			return err
		}
		if resp.Id != msg.Id {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("CDP error: %s", resp.Error.Message)
		}
		return nil
	}
}
