package backend

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// ============================================================================
// 一键窗口平铺：通过 CDP Browser.setWindowBounds 把运行中实例的窗口按网格排布。
// 尺寸带像素级抖动（同一实例抖动值稳定），使各窗口实际尺寸不完全一致。
// ============================================================================

const (
	tileGapPx       = 6   // 网格间距
	tileMinWidthPx  = 320 // Chromium 窗口最小可设宽度的保守值
	tileMinHeightPx = 240
)

type WindowTileResult struct {
	Total  int      `json:"total"`
	Tiled  int      `json:"tiled"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors"`
}

// BrowserWindowsTile 将运行中实例的窗口平铺到主屏工作区。
// profileIds 为空时平铺所有运行中的实例，否则只平铺其中运行中的部分。
func (a *App) BrowserWindowsTile(profileIds []string) (WindowTileResult, error) {
	result := WindowTileResult{Errors: []string{}}
	if a.browserMgr == nil {
		return result, fmt.Errorf("浏览器管理器未初始化")
	}

	wanted := map[string]bool{}
	for _, id := range profileIds {
		if id = strings.TrimSpace(id); id != "" {
			wanted[id] = true
		}
	}

	type tileTarget struct {
		profileID string
		name      string
		debugPort int
	}
	var targets []tileTarget
	for _, p := range a.browserMgr.List() {
		if !p.Running || !p.DebugReady || p.DebugPort <= 0 {
			continue
		}
		if len(wanted) > 0 && !wanted[p.ProfileId] {
			continue
		}
		targets = append(targets, tileTarget{profileID: p.ProfileId, name: p.ProfileName, debugPort: p.DebugPort})
	}
	if len(targets) == 0 {
		return result, fmt.Errorf("没有运行中的实例可平铺")
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].name < targets[j].name })
	result.Total = len(targets)

	availLeft, availTop, availWidth, availHeight := a.tileWorkArea(targets[0].debugPort)

	n := len(targets)
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := (n + cols - 1) / cols
	cellW := (availWidth - tileGapPx*(cols+1)) / cols
	cellH := (availHeight - tileGapPx*(rows+1)) / rows
	if cellW < tileMinWidthPx {
		cellW = tileMinWidthPx
	}
	if cellH < tileMinHeightPx {
		cellH = tileMinHeightPx
	}

	log := logger.New("Browser")
	for i, t := range targets {
		row, col := i/cols, i%cols
		left := availLeft + tileGapPx + col*(cellW+tileGapPx)
		top := availTop + tileGapPx + row*(cellH+tileGapPx)
		// 像素级抖动：按 profileId 哈希收缩 1~6px，同实例每次一致，人眼不可辨，
		// 且只收缩不扩张 → 不会越出网格互相遮挡。
		h := fnv.New32a()
		_, _ = h.Write([]byte(t.profileID))
		jitter := h.Sum32()
		width := cellW - int(jitter%6) - 1
		height := cellH - int((jitter>>3)%6) - 1

		if err := tileOneWindow(t.debugPort, left, top, width, height); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", t.name, err.Error()))
			log.Warn("窗口平铺失败",
				logger.F("profile_id", t.profileID),
				logger.F("error", err.Error()),
			)
			continue
		}
		result.Tiled++
	}
	return result, nil
}

// tileWorkArea 借任一运行中实例的页面执行 JS 读取真实屏幕工作区（去掉任务栏）。
// 读取失败时退回 1920x1080 全屏假设。
func (a *App) tileWorkArea(debugPort int) (left, top, width, height int) {
	left, top, width, height = 0, 0, 1920, 1080
	res, err := cdpCall(debugPort, "Runtime.evaluate", map[string]any{
		"expression":    "JSON.stringify([screen.availLeft||0,screen.availTop||0,screen.availWidth,screen.availHeight])",
		"returnByValue": true,
	})
	if err != nil {
		return
	}
	inner, _ := res["result"].(map[string]any)
	text, _ := inner["value"].(string)
	var vals []int
	if json.Unmarshal([]byte(text), &vals) == nil && len(vals) == 4 && vals[2] > 0 && vals[3] > 0 {
		left, top, width, height = vals[0], vals[1], vals[2], vals[3]
	}
	return
}

// tileOneWindow 通过浏览器级 CDP 会话把单个实例的窗口恢复 normal 并设置边界。
func tileOneWindow(debugPort int, left, top, width, height int) error {
	targetID, err := tileFindPageTargetID(debugPort)
	if err != nil {
		return err
	}

	body, err := cdpGetEndpointBody(debugPort, "/json/version")
	if err != nil {
		return fmt.Errorf("CDP /json/version 请求失败: %w", err)
	}
	var version cdpBrowserVersion
	if err := json.Unmarshal(body, &version); err != nil || strings.TrimSpace(version.WebSocketDebuggerUrl) == "" {
		return fmt.Errorf("未找到浏览器级 WebSocket 调试地址")
	}
	conn, err := cdpDialWebSocket(version.WebSocketDebuggerUrl)
	if err != nil {
		return fmt.Errorf("浏览器级 WebSocket 连接失败: %w", err)
	}
	defer conn.Close()

	call := func(id int, method string, params map[string]any) (map[string]any, error) {
		conn.SetReadDeadline(time.Now().Add(cdpWebSocketReadTimeout))
		if err := conn.WriteJSON(cdpMessage{Id: id, Method: method, Params: params}); err != nil {
			return nil, fmt.Errorf("CDP 命令发送失败: %w", err)
		}
		for {
			var resp cdpResponse
			if err := conn.ReadJSON(&resp); err != nil {
				return nil, fmt.Errorf("CDP 响应读取失败: %w", err)
			}
			if resp.Id != id {
				continue // 跳过事件/乱序消息
			}
			if resp.Error != nil {
				return nil, fmt.Errorf("CDP 错误: %s", resp.Error.Message)
			}
			return resp.Result, nil
		}
	}

	got, err := call(1, "Browser.getWindowForTarget", map[string]any{"targetId": targetID})
	if err != nil {
		return err
	}
	windowID, ok := got["windowId"].(float64)
	if !ok {
		return fmt.Errorf("CDP 未返回 windowId")
	}

	// 先恢复 normal（最大化/最小化状态下直接设边界会被忽略），再设位置尺寸。
	if _, err := call(2, "Browser.setWindowBounds", map[string]any{
		"windowId": windowID,
		"bounds":   map[string]any{"windowState": "normal"},
	}); err != nil {
		return err
	}
	_, err = call(3, "Browser.setWindowBounds", map[string]any{
		"windowId": windowID,
		"bounds":   map[string]any{"left": left, "top": top, "width": width, "height": height},
	})
	return err
}

// tileFindPageTargetID 从 /json 中找一个 page 目标的 targetId。
func tileFindPageTargetID(debugPort int) (string, error) {
	body, err := cdpGetEndpointBody(debugPort, "/json")
	if err != nil {
		return "", fmt.Errorf("CDP /json 请求失败: %w", err)
	}
	var targets []struct {
		Id   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &targets); err != nil || len(targets) == 0 {
		return "", fmt.Errorf("CDP targets 解析失败或为空")
	}
	for _, t := range targets {
		if t.Type == "page" && t.Id != "" {
			return t.Id, nil
		}
	}
	if targets[0].Id != "" {
		return targets[0].Id, nil
	}
	return "", fmt.Errorf("未找到页面目标")
}
