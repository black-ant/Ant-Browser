package launchcode

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// 页面操作（Playwright / CDP）能力的服务门面。
//
// 与代理池同理：这块能力的实现依赖自动化运行时和浏览器进程，都挂在宿主
// *App 上，但不能用类型断言从 starter 取——Wails 会把 *App 的导出方法
// 全部绑定给前端，接口方法挂上去会凭空多出一批重复的前端 API。
// 因此由宿主显式注入一个小适配器。

// PageStep 是一条页面指令。
type PageStep struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// PageStepOutcome 是单条指令的执行结果。
type PageStepOutcome struct {
	Action string         `json:"action"`
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// PageRequest 描述一次页面操作调用。
type PageRequest struct {
	// Selector 定位目标实例；为空时沿用当前挂在统一 CDP 入口上的实例。
	Selector LaunchSelector
	Steps    []PageStep
	// TimeoutMs 是单条指令的等待上限，<=0 时由实现方取默认值。
	TimeoutMs int
}

// PageResult 是一次页面操作的聚合结果。
type PageResult struct {
	ProfileID  string            `json:"profileId"`
	LaunchCode string            `json:"launchCode,omitempty"`
	OK         bool              `json:"ok"`
	Reused     bool              `json:"reused"`
	Steps      []PageStepOutcome `json:"steps"`
	Error      string            `json:"error,omitempty"`
	// Screenshots 按产物路径缓存已读出的图片字节，供协议层直接投递给客户端。
	// 不参与 JSON 序列化：几百 KB 的二进制没有进结构化输出的道理。
	Screenshots map[string]PageScreenshot `json:"-"`
}

// PageScreenshot 是一张已读入内存的截图。
type PageScreenshot struct {
	Data     []byte
	MIMEType string
}

// PageDriver 提供页面操作能力。由宿主显式注入。
type PageDriver interface {
	RunPageSteps(req PageRequest) (*PageResult, error)
	ClosePageSession(selector LaunchSelector) (string, error)
}

const pageAPIUnavailable = "page automation api is unavailable"

// SetPageDriver 注入页面操作能力。未注入时相关工具返回 Unavailable。
func (s *LaunchServer) SetPageDriver(driver PageDriver) {
	s.pageMu.Lock()
	s.page = driver
	s.pageMu.Unlock()
}

func (s *LaunchServer) pageDriver() PageDriver {
	s.pageMu.RLock()
	defer s.pageMu.RUnlock()
	return s.page
}

// RunPageSteps 在目标实例的常驻页面会话上执行指令。
func (s *LaunchServer) RunPageSteps(req PageRequest) (*PageResult, error) {
	if len(req.Steps) == 0 {
		return nil, newServiceError(http.StatusBadRequest, "至少需要一条页面指令")
	}
	for index, step := range req.Steps {
		if strings.TrimSpace(step.Action) == "" {
			return nil, newServiceError(http.StatusBadRequest, fmt.Sprintf("第 %d 条指令缺少 action", index+1))
		}
	}

	driver := s.pageDriver()
	if driver == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, pageAPIUnavailable)
	}

	req.Selector = normalizeLaunchSelector(req.Selector)
	result, err := driver.RunPageSteps(req)
	if err != nil {
		return nil, asServiceError(err)
	}
	if result == nil {
		return nil, newServiceError(http.StatusInternalServerError, "page automation returned no result")
	}
	return result, nil
}

// ClosePageSession 释放目标实例的常驻页面会话。
func (s *LaunchServer) ClosePageSession(selector LaunchSelector) (string, error) {
	driver := s.pageDriver()
	if driver == nil {
		return "", newServiceError(http.StatusServiceUnavailable, pageAPIUnavailable)
	}

	profileID, err := driver.ClosePageSession(normalizeLaunchSelector(selector))
	if err != nil {
		return "", asServiceError(err)
	}
	return profileID, nil
}

// asServiceError 保留驱动层已经分好类的状态码（例如 404 找不到实例），
// 其余按 500 处理。
func asServiceError(err error) error {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		return svcErr
	}
	return newServiceError(http.StatusInternalServerError, err.Error())
}
