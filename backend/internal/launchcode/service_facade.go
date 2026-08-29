package launchcode

import (
	"net/http"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
)

// 本文件是 LaunchServer 的协议无关服务门面。
//
// 现有 /api/* handler 把「解码 -> 校验 -> 执行 -> 写 JSON」耦合在一起，
// 非 HTTP 的调用方（例如 MCP）无法复用。这里对内部业务函数做一层纯增量封装：
// 只导出能力，不改动任何既有行为，因此 /api/* 的响应保持逐字节不变。
//
// 约定：返回 ServiceError 而不是 (status, errMsg) 二元组，让调用方既能拿到
// 可读信息，也能按需还原出原始 HTTP 语义。

// ServiceError 表示一次服务调用失败，同时携带原始 HTTP 状态码。
// 状态码保留下来是为了让不同协议的调用方各自决定如何映射，
// 而不是在这一层就把语义压扁。
type ServiceError struct {
	Status  int
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// NotFound 用于调用方区分「目标不存在」和其他失败。
func (e *ServiceError) NotFound() bool {
	return e.Status == http.StatusNotFound
}

// Ambiguous 表示 selector 命中多个实例且未声明 matchMode=first。
func (e *ServiceError) Ambiguous() bool {
	return e.Status == http.StatusConflict
}

// Unavailable 表示所需能力未注入（宿主未实现对应接口）。
func (e *ServiceError) Unavailable() bool {
	return e.Status == http.StatusServiceUnavailable
}

func newServiceError(status int, message string) *ServiceError {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	return &ServiceError{Status: status, Message: message}
}

// serviceErrorFrom 把内部惯用的 (status, errMsg) 约定转换为 error。
// errMsg 为空表示成功。
func serviceErrorFrom(status int, errMsg string) *ServiceError {
	if errMsg == "" {
		return nil
	}
	return newServiceError(status, errMsg)
}

// RuntimeSession 描述一次实例接管会话的结果。
type RuntimeSession struct {
	Profile *browser.Profile
	// Ready 为 true 表示 CDP 已可接管，可直接使用 CDPURL。
	Ready bool
	// LaunchCode 是本次解析到的快捷启动码。
	LaunchCode string
	// CDPURL 是统一 CDP 入口地址，Ready 为 false 时为空。
	CDPURL string
}

// ---------------------------------------------------------------------------
// 实例查询
// ---------------------------------------------------------------------------

// ListProfiles 返回全部实例快照，并补全 launchCode。
func (s *LaunchServer) ListProfiles() ([]browser.Profile, error) {
	items, status, errMsg := s.listProfiles()
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return items, nil
}

// FindProfiles 按 selector 解析出全部匹配实例。
func (s *LaunchServer) FindProfiles(selector LaunchSelector) ([]browser.Profile, error) {
	items, status, errMsg := s.findProfilesBySelector(normalizeLaunchSelector(selector))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return items, nil
}

// FindProfile 按 selector 解析出唯一实例。
// selector 命中多个且 matchMode 不是 first 时返回 Ambiguous 错误。
func (s *LaunchServer) FindProfile(selector LaunchSelector) (*browser.Profile, error) {
	profile, status, errMsg := s.findProfileBySelector(normalizeLaunchSelector(selector))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return &profile, nil
}

// ---------------------------------------------------------------------------
// 实例写操作
// ---------------------------------------------------------------------------

// CreateProfile 创建实例，requestedCode 为空时自动分配 launch code。
func (s *LaunchServer) CreateProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	profile, launchCode, status, errMsg := s.createProfile(input, requestedCode)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	return profile, launchCode, nil
}

// UpdateProfile 更新实例配置。launch code 设置失败时会回滚实例更新。
func (s *LaunchServer) UpdateProfile(profileID string, input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	previous, status, errMsg := s.profileSnapshotByID(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}

	profile, launchCode, status, errMsg := s.updateProfile(profileID, input, requestedCode, previous)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	return profile, launchCode, nil
}

// DeleteProfile 删除实例。运行中的实例不允许删除。
func (s *LaunchServer) DeleteProfile(profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return newServiceError(http.StatusNotFound, "profile not found")
	}

	snapshot, status, errMsg := s.profileSnapshotByID(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return err
	}
	if snapshot != nil && snapshot.Running {
		return newServiceError(http.StatusConflict, "running profile cannot be deleted")
	}

	if err := s.deleteProfileInternal(profileID); err != nil {
		return newServiceError(mapProfileWriteErrorStatus(err), err.Error())
	}
	return nil
}

// ---------------------------------------------------------------------------
// 运行时控制
// ---------------------------------------------------------------------------

// StartProfile 按 selector 启动实例，不等待 CDP 就绪。
// 需要接管浏览器时请改用 OpenRuntimeSession。
func (s *LaunchServer) StartProfile(selector LaunchSelector, params LaunchRequestParams) (*browser.Profile, string, error) {
	profile, launchCode, status, errMsg := s.launchBySelector(normalizeLaunchSelector(selector), normalizeLaunchRequestParams(params))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, launchCode, err
	}
	return profile, launchCode, nil
}

// StatusProfile 查询实例运行态，不触发启动。
func (s *LaunchServer) StatusProfile(profileID string) (*browser.Profile, error) {
	profile, status, errMsg := s.statusProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return profile, nil
}

// StopProfile 停止实例，并在需要时清空统一 CDP 入口。
func (s *LaunchServer) StopProfile(profileID string) (*browser.Profile, error) {
	profile, status, errMsg := s.stopProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return profile, nil
}

// OpenRuntimeSession 启动实例并等待 CDP 就绪，是外部工具接管浏览器的推荐入口。
// 等待超时不算失败：返回 Ready=false，调用方可稍后重试。
func (s *LaunchServer) OpenRuntimeSession(selector LaunchSelector, params LaunchRequestParams, timeout time.Duration) (*RuntimeSession, error) {
	normalized := normalizeRuntimeSelector(selector)
	if normalized.IsEmpty() {
		return nil, newServiceError(http.StatusBadRequest, "selector is required")
	}
	if err := validateRuntimeSelector(normalized); err != nil {
		return nil, newServiceError(http.StatusBadRequest, err.Error())
	}

	profile, launchCode, status, errMsg := s.launchBySelector(normalized, normalizeLaunchRequestParams(params))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}

	if timeout <= 0 {
		timeout = defaultRuntimeSessionTimeout
	}
	profile, ready, err := s.prepareRuntimeSession(profile, timeout)
	if err != nil {
		return nil, newServiceError(mapProfileWriteErrorStatus(err), err.Error())
	}
	if profile == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "runtime session is not available")
	}
	if launchCode != "" && profile.LaunchCode == "" {
		profile.LaunchCode = launchCode
	}

	session := &RuntimeSession{Profile: profile, Ready: ready, LaunchCode: profile.LaunchCode}
	if ready {
		session.CDPURL = s.CDPURL()
	}
	return session, nil
}

// ActiveRuntimeSession 返回当前挂在统一 CDP 入口上的实例。
// 没有活动目标时返回 nil，不算错误。
func (s *LaunchServer) ActiveRuntimeSession() (*RuntimeSession, error) {
	profileID, _, port := s.ActiveProfile()
	if strings.TrimSpace(profileID) == "" || port <= 0 {
		return nil, nil
	}

	profile, status, errMsg := s.statusProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return &RuntimeSession{
		Profile:    profile,
		Ready:      profile != nil && profile.DebugReady,
		LaunchCode: profile.LaunchCode,
		CDPURL:     s.CDPURL(),
	}, nil
}

// ---------------------------------------------------------------------------
// 自动化脚本
// ---------------------------------------------------------------------------

// ListScripts 返回全部自动化脚本元数据（不含脚本正文）。
func (s *LaunchServer) ListScripts() ([]automation.ScriptRecord, error) {
	lister, ok := s.starter.(AutomationScriptLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}

	items, err := lister.AutomationScriptList()
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	return items, nil
}

// GetScript 返回单个自动化脚本详情。
func (s *LaunchServer) GetScript(scriptID string) (*automation.ScriptRecord, error) {
	scriptID = strings.TrimSpace(scriptID)
	if scriptID == "" {
		return nil, newServiceError(http.StatusBadRequest, "scriptId is required")
	}

	getter, ok := s.starter.(AutomationScriptGetter)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}

	record, err := getter.AutomationScriptGet(scriptID)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	if record == nil {
		return nil, newServiceError(http.StatusNotFound, "automation script not found")
	}
	return record, nil
}

// RunScript 执行自动化脚本，阻塞直到脚本结束或超时。
func (s *LaunchServer) RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	if strings.TrimSpace(input.ScriptID) == "" {
		return nil, newServiceError(http.StatusBadRequest, "scriptId is required")
	}

	runner, ok := s.starter.(AutomationScriptRunner)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}

	record, err := runner.AutomationScriptRunWithOptions(input)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	if record == nil {
		return nil, newServiceError(http.StatusInternalServerError, "automation script run returned no record")
	}
	return record, nil
}

// ListScriptRuns 返回最近的脚本执行记录。
func (s *LaunchServer) ListScriptRuns(limit int) ([]automation.ScriptRunRecord, error) {
	lister, ok := s.starter.(AutomationScriptRunLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}

	items, err := lister.AutomationScriptRunList(limit)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	return items, nil
}
