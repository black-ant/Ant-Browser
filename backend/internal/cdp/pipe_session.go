package cdp

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// pipe_session.go 在 PipeConn 之上提供 --remote-debugging-pipe 模式下的高层能力：
// 目标发现 / 附着页面会话（flatten）/ 面向页面与浏览器的命令封装 / 就绪探测。
// 这些方法在 Windows 上同样会被编译（平台无关），但仅在 pipe 模式（Linux/macOS）运行时被调用。

// pageSessionState 缓存已附着的页面会话，避免每次命令都重新发现+附着。
type pageSession struct {
	mu        sync.Mutex
	sessionID string
	targetID  string
}

var pipePageSessions sync.Map // *PipeConn -> *pageSession

func (c *PipeConn) pageSession() *pageSession {
	if v, ok := pipePageSessions.Load(c); ok {
		return v.(*pageSession)
	}
	ps := &pageSession{}
	actual, _ := pipePageSessions.LoadOrStore(c, ps)
	return actual.(*pageSession)
}

// WaitReady 通过根连接发送 Browser.getVersion 探测浏览器是否已就绪。
func (c *PipeConn) WaitReady(timeout time.Duration) error {
	_, err := c.SendCommandTimeout("", "Browser.getVersion", nil, timeout)
	return err
}

// EnsurePageSession 发现一个 page 类型目标并以 flatten 模式附着，返回其 sessionId（带缓存）。
func (c *PipeConn) EnsurePageSession() (string, error) {
	ps := c.pageSession()
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.sessionID != "" {
		return ps.sessionID, nil
	}

	targetID, err := c.findPageTargetID()
	if err != nil {
		return "", err
	}

	res, err := c.SendCommand("", "Target.attachToTarget", map[string]interface{}{
		"targetId": targetID,
		"flatten":  true,
	})
	if err != nil {
		return "", fmt.Errorf("附着页面目标失败: %w", err)
	}
	sid, _ := res["sessionId"].(string)
	if strings.TrimSpace(sid) == "" {
		return "", fmt.Errorf("附着页面目标未返回 sessionId")
	}
	ps.sessionID = sid
	ps.targetID = targetID
	return sid, nil
}

// findPageTargetID 通过 Target.getTargets 找出一个 page 类型目标。
func (c *PipeConn) findPageTargetID() (string, error) {
	res, err := c.SendCommand("", "Target.getTargets", nil)
	if err != nil {
		return "", fmt.Errorf("获取目标列表失败: %w", err)
	}
	infos, ok := res["targetInfos"].([]interface{})
	if !ok {
		return "", fmt.Errorf("目标列表格式异常")
	}
	for _, item := range infos {
		info, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := info["type"].(string); t == "page" {
			if id, _ := info["targetId"].(string); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("未找到 page 类型目标")
}

// invalidatePageSession 在页面会话失效（目标关闭/分离）时清除缓存，下次重新附着。
func (c *PipeConn) invalidatePageSession() {
	ps := c.pageSession()
	ps.mu.Lock()
	ps.sessionID = ""
	ps.targetID = ""
	ps.mu.Unlock()
}

// CallPage 向当前页面目标发送一条 CDP 命令（默认超时）。
func (c *PipeConn) CallPage(method string, params map[string]interface{}) (map[string]interface{}, error) {
	return c.CallPageTimeout(method, params, 10*time.Second)
}

// CallPageTimeout 向当前页面目标发送一条 CDP 命令，可指定超时。
// 若会话失效会清缓存并重试一次（页面可能在两次调用间被导航/关闭）。
func (c *PipeConn) CallPageTimeout(method string, params map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	sid, err := c.EnsurePageSession()
	if err != nil {
		return nil, err
	}
	res, err := c.SendCommandTimeout(sid, method, params, timeout)
	if err != nil && isStaleSessionErr(err) {
		c.invalidatePageSession()
		if sid, err2 := c.EnsurePageSession(); err2 == nil {
			return c.SendCommandTimeout(sid, method, params, timeout)
		}
	}
	return res, err
}

// CallBrowser 发送浏览器级 CDP 命令（根连接，sessionId 为空）。
func (c *PipeConn) CallBrowser(method string, params map[string]interface{}) (map[string]interface{}, error) {
	return c.SendCommand("", method, params)
}

func isStaleSessionErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "session") && (strings.Contains(msg, "not found") || strings.Contains(msg, "detached"))
}
