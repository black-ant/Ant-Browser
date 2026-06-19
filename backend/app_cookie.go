package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Cookie 管理 API（通过 CDP）
// ============================================================================

// CookieInfo 表示单条浏览器 Cookie
type CookieInfo struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// cdpTarget 表示 /json 接口返回的调试目标
type cdpTarget struct {
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
	Type                 string `json:"type"`
}

type cdpBrowserVersion struct {
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
}

// cdpMessage 是 CDP 协议消息结构
type cdpMessage struct {
	Id     int            `json:"id"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// cdpResponse 是 CDP 协议响应结构
type cdpResponse struct {
	Id     int            `json:"id"`
	Result map[string]any `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// cdpCall 向指定 debugPort 发送单次 CDP 命令并返回 result 字段
func cdpCall(debugPort int, method string, params map[string]any) (map[string]any, error) {
	// 1. 获取 WebSocket 调试地址
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", debugPort))
	if err != nil {
		return nil, fmt.Errorf("CDP /json 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil || len(targets) == 0 {
		return nil, fmt.Errorf("CDP targets 解析失败或为空")
	}

	wsURL := ""
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
			wsURL = t.WebSocketDebuggerUrl
			break
		}
	}
	if wsURL == "" && targets[0].WebSocketDebuggerUrl != "" {
		wsURL = targets[0].WebSocketDebuggerUrl
	}
	if wsURL == "" {
		return nil, fmt.Errorf("未找到可用的 WebSocket 调试地址")
	}

	// 2. 建立 WebSocket 连接
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 3. 发送 CDP 命令
	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("CDP 命令发送失败: %w", err)
	}

	// 4. 等待响应
	var cdpResp cdpResponse
	if err := conn.ReadJSON(&cdpResp); err != nil {
		return nil, fmt.Errorf("CDP 响应读取失败: %w", err)
	}
	if cdpResp.Error != nil {
		return nil, fmt.Errorf("CDP 错误: %s", cdpResp.Error.Message)
	}
	return cdpResp.Result, nil
}

func cdpBrowserCall(debugPort int, method string, params map[string]any) error {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", debugPort))
	if err != nil {
		return fmt.Errorf("CDP /json/version 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var version cdpBrowserVersion
	if err := json.Unmarshal(body, &version); err != nil {
		return fmt.Errorf("CDP browser target 解析失败: %w", err)
	}
	wsURL := strings.TrimSpace(version.WebSocketDebuggerUrl)
	if wsURL == "" {
		return fmt.Errorf("未找到浏览器级 WebSocket 调试地址")
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("浏览器级 WebSocket 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("浏览器级 CDP 命令发送失败: %w", err)
	}

	var cdpResp cdpResponse
	if err := conn.ReadJSON(&cdpResp); err != nil {
		// Browser.close 可能会直接关闭 websocket，视为成功。
		if strings.EqualFold(method, "Browser.close") {
			return nil
		}
		return fmt.Errorf("浏览器级 CDP 响应读取失败: %w", err)
	}
	if cdpResp.Error != nil {
		return fmt.Errorf("浏览器级 CDP 错误: %s", cdpResp.Error.Message)
	}
	return nil
}

// getDebugPort 获取运行中实例的调试端口
func (a *App) getDebugPort(profileId string) (int, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return 0, fmt.Errorf("profile not found: %s", profileId)
	}
	if !profile.Running {
		return 0, fmt.Errorf("实例未运行")
	}
	if profile.DebugPort == 0 || !profile.DebugReady {
		return 0, fmt.Errorf("实例调试接口尚未就绪，请稍后重试")
	}
	return profile.DebugPort, nil
}

// BrowserGetCookies 通过 CDP 获取实例所有 Cookie
func (a *App) BrowserGetCookies(profileId string) ([]CookieInfo, error) {
	debugPort, err := a.getDebugPort(profileId)
	if err != nil {
		return nil, err
	}

	result, err := cdpCall(debugPort, "Network.getAllCookies", nil)
	if err != nil {
		return nil, err
	}

	cookiesRaw, ok := result["cookies"]
	if !ok {
		return []CookieInfo{}, nil
	}

	// 通过 JSON 二次解析
	data, _ := json.Marshal(cookiesRaw)
	var cookies []CookieInfo
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, fmt.Errorf("Cookie 解析失败: %w", err)
	}
	return cookies, nil
}

// BrowserSetCookies 通过 CDP 将一组 Cookie 写入运行中的实例（回写/恢复）
func (a *App) BrowserSetCookies(profileId string, cookies []CookieInfo) error {
	debugPort, err := a.getDebugPort(profileId)
	if err != nil {
		return err
	}
	if len(cookies) == 0 {
		return nil
	}

	arr := make([]map[string]any, 0, len(cookies))
	for _, c := range cookies {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		m := map[string]any{
			"name":     c.Name,
			"value":    c.Value,
			"path":     c.Path,
			"secure":   c.Secure,
			"httpOnly": c.HttpOnly,
		}
		if strings.TrimSpace(c.Domain) != "" {
			m["domain"] = c.Domain
		}
		if c.Expires > 0 {
			m["expires"] = c.Expires
		}
		switch strings.ToLower(strings.TrimSpace(c.SameSite)) {
		case "strict":
			m["sameSite"] = "Strict"
		case "lax":
			m["sameSite"] = "Lax"
		case "none":
			m["sameSite"] = "None"
		}
		arr = append(arr, m)
	}

	_, err = cdpCall(debugPort, "Network.setCookies", map[string]any{"cookies": arr})
	return err
}

// BrowserClearCookies 通过 CDP 清除实例所有 Cookie
func (a *App) BrowserClearCookies(profileId string) error {
	debugPort, err := a.getDebugPort(profileId)
	if err != nil {
		return err
	}
	_, err = cdpCall(debugPort, "Network.clearBrowserCookies", nil)
	return err
}

// BrowserExportCookies 导出 Netscape 格式 Cookie 字符串
func (a *App) BrowserExportCookies(profileId string) (string, error) {
	cookies, err := a.BrowserGetCookies(profileId)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Netscape HTTP Cookie File\n")
	sb.WriteString("# Generated by BrowserManager\n\n")

	for _, c := range cookies {
		includeSubdomains := "FALSE"
		if strings.HasPrefix(c.Domain, ".") {
			includeSubdomains = "TRUE"
		}
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		expires := int64(c.Expires)
		if expires < 0 {
			expires = 0
		}
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			c.Domain, includeSubdomains, c.Path, secure, expires, c.Name, c.Value))
	}
	return sb.String(), nil
}
