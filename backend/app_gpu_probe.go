package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// GPU 硬解巡检。
//
// 背景:视频硬解是多开 CPU 大盘的决定因素 —— 一旦 GPU 被 Chromium 驱动黑名单拉黑而回落软解,
// 每实例 CPU 会翻 2~5 倍,比本项目所有启动参数优化加起来的影响都大。
// 而 exe 是 mac 交叉编译、跑在各种客户机/云主机上,被拉黑非常常见,必须能自动发现。
//
// 做法:开一个 chrome://gpu 标签读取 "Graphics Feature Status",判断 Video Decode 是否硬件加速,
// 读完立即关闭。这是**机器级**信息,每个 app 会话跑一次即可,不必每实例跑。

// gpuProbeExpression 取 chrome://gpu 的全部可见文本。
// 该页部分版本用 shadow DOM 渲染,因此在 innerText 之外再递归收集各 shadowRoot 的文本。
const gpuProbeExpression = `(() => {
  const parts = [document.body ? document.body.innerText : ''];
  const walk = (root) => {
    root.querySelectorAll('*').forEach((el) => {
      if (el.shadowRoot) { parts.push(el.shadowRoot.textContent || ''); walk(el.shadowRoot); }
    });
  };
  walk(document);
  return parts.join('\n');
})()`

// parseGPUFeatureStatus 把 chrome://gpu 的文本解析成 特性名→状态 映射。
// 同名特性只取第一次出现(页面下方的问题列表可能重复提及同名条目)。
func parseGPUFeatureStatus(text string) map[string]string {
	status := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx <= 0 || idx == len(line)-1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key == "" || value == "" {
			continue
		}
		if _, exists := status[key]; !exists {
			status[key] = value
		}
	}
	return status
}

// gpuHardwareDecodeOK 判定视频硬解是否可用。
// 缺失条目按失败处理(fail-closed):"读不到"和"没有硬解"对运维的处置一样 —— 必须人工核查。
func gpuHardwareDecodeOK(status map[string]string) bool {
	value, ok := status["Video Decode"]
	if !ok {
		return false
	}
	lower := strings.ToLower(value)
	return strings.Contains(lower, "hardware accelerated") && !strings.Contains(lower, "software only")
}

// ProbeGPUCapability 在指定实例里开一个 chrome://gpu 标签,读取图形特性状态后立即关闭(Wails 导出)。
func (a *App) ProbeGPUCapability(debugPort int) (map[string]string, error) {
	created, err := cdpBrowserCallResult(debugPort, "Target.createTarget", map[string]any{"url": "chrome://gpu"})
	if err != nil {
		return nil, fmt.Errorf("创建 GPU 探测标签失败: %w", err)
	}
	targetID, _ := created["targetId"].(string)
	if targetID == "" {
		return nil, fmt.Errorf("GPU 探测标签未返回 targetId")
	}
	defer func() {
		_, _ = cdpBrowserCallResult(debugPort, "Target.closeTarget", map[string]any{"targetId": targetID})
	}()

	wsURL, err := waitGPUProbeTargetWS(debugPort, targetID, 5*time.Second)
	if err != nil {
		return nil, err
	}
	result, err := cdpCallOnTarget(wsURL, "Runtime.evaluate", map[string]any{
		"expression":    gpuProbeExpression,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	inner, _ := result["result"].(map[string]any)
	text, _ := inner["value"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("chrome://gpu 文本为空")
	}
	return parseGPUFeatureStatus(text), nil
}

// waitGPUProbeTargetWS 轮询 /json 直到出现该 targetId 的 WebSocket 地址(页面需要一点时间初始化)。
func waitGPUProbeTargetWS(debugPort int, targetID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, err := cdpGetEndpointBody(debugPort, "/json")
		if err == nil {
			var targets []cdpTarget
			if json.Unmarshal(body, &targets) == nil {
				for _, t := range targets {
					if t.Id == targetID && t.WebSocketDebuggerUrl != "" {
						return t.WebSocketDebuggerUrl, nil
					}
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("等待 GPU 探测标签的调试地址超时")
}

// cdpCallOnTarget 在指定 target 的 WebSocket 上发一次命令并读回 result。
// 与 cdpCall 的区别:后者固定连"第一个 page",这里需要连指定标签。
func cdpCallOnTarget(wsURL string, method string, params map[string]any) (map[string]any, error) {
	conn, err := cdpDialWebSocket(wsURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(cdpWebSocketReadTimeout))

	if err := conn.WriteJSON(cdpMessage{Id: 1, Method: method, Params: params}); err != nil {
		return nil, err
	}
	var resp cdpResponse
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("CDP 错误: %s", resp.Error.Message)
	}
	return resp.Result, nil
}

// LogGPUCapability 探测并记录结果;软解时打 Error 级日志,便于运维立刻发现。
func (a *App) LogGPUCapability(debugPort int) {
	log := logger.New("GPUProbe")
	status, err := a.ProbeGPUCapability(debugPort)
	if err != nil {
		log.Warn("GPU 能力探测失败", logger.F("port", debugPort), logger.F("error", err.Error()))
		return
	}
	if gpuHardwareDecodeOK(status) {
		log.Info("视频硬解已启用", logger.F("video_decode", status["Video Decode"]))
		return
	}
	log.Error("视频硬解未启用,多开 CPU 将显著升高",
		logger.F("video_decode", status["Video Decode"]),
		logger.F("hint", "检查 GPU 驱动黑名单(chrome://gpu);必要时评估 --ignore-gpu-blocklist"),
	)
}
