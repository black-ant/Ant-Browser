// Command cdp-smoke 对一个已运行实例的调试端口跑通项目实际使用的 CDP 能力。
//
// 用途:每次改动 Chromium 启动参数后必跑 —— 确认新参数没有打断 CDP 通道
// (保活、窗口平铺、Cookie 管理、地理对齐、自动化都依赖它)。
//
// 用法:
//
//	go run ./backend/cmd/cdp-smoke -port 9222
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	port := flag.Int("port", 0, "实例的 remote-debugging-port")
	flag.Parse()
	if *port <= 0 {
		fmt.Fprintln(os.Stderr, "必须指定 -port,例如: go run ./backend/cmd/cdp-smoke -port 9222")
		os.Exit(2)
	}

	failed := 0
	check := func(name string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("FAIL  %-24s %v\n", name, err)
			failed++
			return
		}
		fmt.Printf("OK    %s\n", name)
	}

	fmt.Printf("CDP HTTP 面检查 (127.0.0.1:%d)\n\n", *port)
	for _, endpoint := range []string{"/json/version", "/json", "/json/list"} {
		ep := endpoint
		check("HTTP "+ep, func() error {
			_, err := httpGetJSON(*port, ep)
			return err
		})
	}

	check("存在 page 类型标签", func() error {
		body, err := httpGetJSON(*port, "/json")
		if err != nil {
			return err
		}
		var targets []struct {
			Type                 string `json:"type"`
			WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
		}
		if err := json.Unmarshal(body, &targets); err != nil {
			return err
		}
		for _, t := range targets {
			if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
				return nil
			}
		}
		return fmt.Errorf("未找到带 WebSocket 地址的 page 目标")
	})

	check("存在浏览器级 WebSocket 地址", func() error {
		body, err := httpGetJSON(*port, "/json/version")
		if err != nil {
			return err
		}
		var version struct {
			WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
		}
		if err := json.Unmarshal(body, &version); err != nil {
			return err
		}
		if version.WebSocketDebuggerUrl == "" {
			return fmt.Errorf("/json/version 缺少 webSocketDebuggerUrl")
		}
		return nil
	})

	fmt.Println("\n下列域需在真实实例上人工确认(本工具只验 HTTP 面):")
	for _, m := range []string{
		"Browser.getWindowForTarget / setWindowBounds  → 窗口平铺",
		"Target.createTarget                            → 打开新标签",
		"Page.navigate / Runtime.evaluate               → 窗口信息、GPU 探测",
		"Input.dispatchMouseEvent                       → 直播保活",
		"Network.getAllCookies / clearBrowserCookies    → Cookie 管理",
		"Emulation.setGeolocation/Locale/Timezone       → 地理对齐",
		"Memory.simulatePressureNotification            → 主动内存回收",
	} {
		fmt.Println("  -", m)
	}

	if failed > 0 {
		fmt.Printf("\n%d 项失败\n", failed)
		os.Exit(1)
	}
	fmt.Println("\nCDP HTTP 面全部通过")
}

func httpGetJSON(port int, endpoint string) ([]byte, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("响应非合法 JSON")
	}
	return body, nil
}
