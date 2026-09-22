package backend

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type browserWindowCDPCall func(debugPort int, method string, params map[string]any) (map[string]any, error)

type browserTargetInfosResult struct {
	TargetInfos []struct {
		TargetID string `json:"targetId"`
		Type     string `json:"type"`
	} `json:"targetInfos"`
}

type browserWindowForTargetResult struct {
	WindowID int `json:"windowId"`
	Bounds   struct {
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		WindowState string `json:"windowState"`
	} `json:"bounds"`
}

// configuredBrowserWindowSize returns the effective --window-size value. Chrome
// can restore browser.window_placement after processing this launch argument, so
// callers must verify the live window rather than trusting the command line.
func configuredBrowserWindowSize(args []string) (int, int, bool) {
	value := browserArgValue(args, "--window-size")
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func decodeBrowserWindowCDPResult(result map[string]any, target any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// syncConfiguredBrowserWindowSize repairs Chrome session-restore placement so
// window.outerWidth/outerHeight keep matching the profile's --window-size. Each
// distinct restored browser window is handled once. A minimized window is sized
// through the required normal state and then minimized again.
func syncConfiguredBrowserWindowSize(debugPort int, args []string) error {
	return syncConfiguredBrowserWindowSizeWithCall(debugPort, args, cdpBrowserCallResult)
}

func syncConfiguredBrowserWindowSizeWithCall(debugPort int, args []string, call browserWindowCDPCall) error {
	width, height, ok := configuredBrowserWindowSize(args)
	if !ok || debugPort <= 0 || call == nil {
		return nil
	}

	targetsResult, err := call(debugPort, "Target.getTargets", nil)
	if err != nil {
		return fmt.Errorf("读取浏览器窗口目标失败: %w", err)
	}
	var targets browserTargetInfosResult
	if err := decodeBrowserWindowCDPResult(targetsResult, &targets); err != nil {
		return fmt.Errorf("解析浏览器窗口目标失败: %w", err)
	}

	seenWindows := map[int]struct{}{}
	for _, target := range targets.TargetInfos {
		if target.Type != "page" || strings.TrimSpace(target.TargetID) == "" {
			continue
		}
		windowResult, err := call(debugPort, "Browser.getWindowForTarget", map[string]any{"targetId": target.TargetID})
		if err != nil {
			return fmt.Errorf("读取浏览器窗口边界失败: %w", err)
		}
		var window browserWindowForTargetResult
		if err := decodeBrowserWindowCDPResult(windowResult, &window); err != nil {
			return fmt.Errorf("解析浏览器窗口边界失败: %w", err)
		}
		if window.WindowID <= 0 {
			continue
		}
		if _, exists := seenWindows[window.WindowID]; exists {
			continue
		}
		seenWindows[window.WindowID] = struct{}{}

		state := strings.ToLower(strings.TrimSpace(window.Bounds.WindowState))
		if window.Bounds.Width == width && window.Bounds.Height == height && (state == "normal" || state == "minimized") {
			continue
		}
		if state != "normal" {
			if _, err := call(debugPort, "Browser.setWindowBounds", map[string]any{
				"windowId": window.WindowID,
				"bounds":   map[string]any{"windowState": "normal"},
			}); err != nil {
				return fmt.Errorf("恢复浏览器窗口状态失败: %w", err)
			}
			// Chromium acknowledges the CDP command before the Linux window
			// manager has finished the state transition. Resizing immediately
			// can therefore be ignored.
			time.Sleep(250 * time.Millisecond)
		}
		if _, err := call(debugPort, "Browser.setWindowBounds", map[string]any{
			"windowId": window.WindowID,
			"bounds":   map[string]any{"width": width, "height": height},
		}); err != nil {
			return fmt.Errorf("校正浏览器窗口尺寸失败: %w", err)
		}
		time.Sleep(250 * time.Millisecond)
		verifiedResult, err := call(debugPort, "Browser.getWindowForTarget", map[string]any{"targetId": target.TargetID})
		if err != nil {
			return fmt.Errorf("验证浏览器窗口尺寸失败: %w", err)
		}
		var verified browserWindowForTargetResult
		if err := decodeBrowserWindowCDPResult(verifiedResult, &verified); err != nil {
			return fmt.Errorf("解析浏览器窗口校正结果失败: %w", err)
		}
		if !browserWindowSizeWithinTolerance(verified.Bounds.Width, verified.Bounds.Height, width, height) || strings.ToLower(strings.TrimSpace(verified.Bounds.WindowState)) != "normal" {
			return fmt.Errorf("浏览器窗口尺寸仍为 %d,%d (%s)，期望 %d,%d", verified.Bounds.Width, verified.Bounds.Height, verified.Bounds.WindowState, width, height)
		}
		if state == "minimized" {
			if _, err := call(debugPort, "Browser.setWindowBounds", map[string]any{
				"windowId": window.WindowID,
				"bounds":   map[string]any{"windowState": "minimized"},
			}); err != nil {
				return fmt.Errorf("恢复浏览器窗口最小化状态失败: %w", err)
			}
		}
	}
	return nil
}

func browserWindowSizeWithinTolerance(actualWidth, actualHeight, expectedWidth, expectedHeight int) bool {
	const linuxWindowManagerTolerance = 4
	widthDelta := actualWidth - expectedWidth
	if widthDelta < 0 {
		widthDelta = -widthDelta
	}
	heightDelta := actualHeight - expectedHeight
	if heightDelta < 0 {
		heightDelta = -heightDelta
	}
	return widthDelta <= linuxWindowManagerTolerance && heightDelta <= linuxWindowManagerTolerance
}
