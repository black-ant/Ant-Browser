package backend

import (
	"strings"

	"ant-chrome/backend/internal/logger"

	"github.com/gorilla/websocket"
)

// urlAccessFilter 描述一个窗口的网址访问过滤策略。
// 白名单优先：一旦配置了白名单，只放行匹配白名单的请求，其余全部拦截；
// 否则按黑名单拦截匹配项。两者均为空时不启用过滤。
type urlAccessFilter struct {
	whitelist []string
	blacklist []string
}

// parseURLAccessFilter 把黑 / 白名单文本（每行一个）解析为过滤策略。
// 每条规则归一化为小写、去协议、去首尾空白，便于子串匹配。
func parseURLAccessFilter(blacklist, whitelist string) urlAccessFilter {
	return urlAccessFilter{
		whitelist: parseURLFilterRules(whitelist),
		blacklist: parseURLFilterRules(blacklist),
	}
}

func parseURLFilterRules(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, 8)
	for _, rawLine := range strings.Split(text, "\n") {
		rule := normalizeURLFilterToken(rawLine)
		if rule == "" || seen[rule] {
			continue
		}
		seen[rule] = true
		out = append(out, rule)
	}
	return out
}

// normalizeURLFilterToken 把单条规则或被检测 URL 归一化：小写、去协议头、去首尾斜杠/空白。
func normalizeURLFilterToken(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	return strings.TrimSpace(strings.Trim(s, "/"))
}

func (f urlAccessFilter) enabled() bool {
	return len(f.whitelist) > 0 || len(f.blacklist) > 0
}

// shouldBlock 判定给定 URL 是否应被拦截。
//   - 配置了白名单：不匹配任何白名单项 → 拦截（白名单模式默认拒绝）。
//   - 否则：匹配任一黑名单项 → 拦截。
//   - 无法解析的 URL（如 data:、about:）在白名单模式下放行，避免误伤浏览器内部页。
func (f urlAccessFilter) shouldBlock(rawURL string) bool {
	token := normalizeURLFilterToken(rawURL)
	if token == "" {
		return false
	}

	if len(f.whitelist) > 0 {
		// 浏览器内部页（newtab/blank/扩展页）在白名单模式下不拦截。
		if isInternalURL(rawURL) {
			return false
		}
		for _, rule := range f.whitelist {
			if matchURLRule(token, rule) {
				return false
			}
		}
		return true
	}

	for _, rule := range f.blacklist {
		if matchURLRule(token, rule) {
			return true
		}
	}
	return false
}

func matchURLRule(token, rule string) bool {
	if rule == "" {
		return false
	}
	return strings.Contains(token, rule)
}

func isInternalURL(rawURL string) bool {
	s := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasPrefix(s, "about:") ||
		strings.HasPrefix(s, "chrome:") ||
		strings.HasPrefix(s, "chrome-extension:") ||
		strings.HasPrefix(s, "edge:") ||
		strings.HasPrefix(s, "devtools:") ||
		strings.HasPrefix(s, "data:") ||
		strings.HasPrefix(s, "blob:")
}

// startProfileURLFilterWatcher 在窗口启动后开启网址访问过滤（端口模式）。
// 通过 Target.setAutoAttach 监听所有页面目标，对每个页面启用 Fetch 域并按策略放行/拦截请求。
func (a *App) startProfileURLFilterWatcher(profileId string, debugPort int, filter urlAccessFilter) {
	if !filter.enabled() || debugPort <= 0 {
		return
	}
	go a.watchPortURLFilterTargets(profileId, debugPort, filter)
}

func (a *App) watchPortURLFilterTargets(profileId string, debugPort int, filter urlAccessFilter) {
	log := logger.New("Browser")
	wsURL, err := cdpBrowserWebSocketURL(debugPort)
	if err != nil {
		log.Warn("网址过滤监听启动失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()))
		return
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Warn("网址过滤监听连接失败",
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
	if err := writeTargetAttachCDPCommand(conn, nextID, "", "Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": false,
		"flatten":                true,
	}); err != nil {
		log.Warn("网址过滤监听启用失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()))
		return
	}
	nextID++

	for {
		var msg urlFilterCDPMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if a.profileDebugPortActive(profileId, debugPort) {
				log.Warn("网址过滤监听中断",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
					logger.F("error", err.Error()))
			}
			return
		}

		switch msg.Method {
		case "Target.attachedToTarget":
			sessionID, ok := urlFilterAttachedPageSessionID(msg)
			if !ok {
				continue
			}
			// 对新页面启用 Fetch 拦截（仅 Request 阶段）。
			if err := writeTargetAttachCDPCommand(conn, nextID, sessionID, "Fetch.enable", map[string]any{
				"patterns": []map[string]any{{"urlPattern": "*"}},
			}); err != nil {
				log.Warn("网址过滤启用 Fetch 失败",
					logger.F("profile_id", profileId),
					logger.F("session_id", sessionID),
					logger.F("error", err.Error()))
				return
			}
			nextID++
		case "Fetch.requestPaused":
			a.handleURLFilterRequestPaused(conn, msg, filter, &nextID)
		}
	}
}

// handleURLFilterRequestPaused 对单条暂停的请求决定放行或拦截。
func (a *App) handleURLFilterRequestPaused(conn *websocket.Conn, msg urlFilterCDPMessage, filter urlAccessFilter, nextID *int) {
	if msg.Params == nil {
		return
	}
	requestID, _ := msg.Params["requestId"].(string)
	if strings.TrimSpace(requestID) == "" {
		return
	}
	sessionID := strings.TrimSpace(msg.SessionID)

	reqURL := ""
	if reqInfo, ok := msg.Params["request"].(map[string]any); ok {
		reqURL, _ = reqInfo["url"].(string)
	}

	if filter.shouldBlock(reqURL) {
		_ = writeTargetAttachCDPCommand(conn, *nextID, sessionID, "Fetch.failRequest", map[string]any{
			"requestId":   requestID,
			"errorReason": "BlockedByClient",
		})
		*nextID++
		return
	}

	_ = writeTargetAttachCDPCommand(conn, *nextID, sessionID, "Fetch.continueRequest", map[string]any{
		"requestId": requestID,
	})
	*nextID++
}

type urlFilterCDPMessage struct {
	ID        int            `json:"id,omitempty"`
	Method    string         `json:"method,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func urlFilterAttachedPageSessionID(msg urlFilterCDPMessage) (string, bool) {
	if msg.Params == nil {
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
	if targetType, _ := targetInfo["type"].(string); targetType != "page" {
		return "", false
	}
	return sessionID, true
}
