package automation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 常驻页面会话的生命周期管理。
//
// 关键取舍：
//   - 会话表用独立的 pageMu，不复用 m.mu。m.mu 覆盖安装流程与状态读取，
//     在持有它时去 spawn 进程会把安装也一起阻塞，且容易和 registerTask 形成锁序问题。
//   - 崩溃重拉只对一次调用里的第一条指令生效。后续指令已经产生了副作用
//     （点击、导航），重放会重复执行。
//   - 脚本任务开跑前会主动关掉同实例的页面会话，见 RunScriptTask。

// RunPageCommands 在目标实例的常驻会话上依次执行指令。
//
// 会话不存在时按需拉起；已存在则直接复用，省掉 CDP 握手。
func (m *Manager) RunPageCommands(ctx context.Context, req PageCommandRequest) (PageCommandResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	profileID := strings.TrimSpace(req.ProfileID)
	if profileID == "" {
		return PageCommandResult{}, fmt.Errorf("profileId is required")
	}
	if len(req.Commands) == 0 {
		return PageCommandResult{}, fmt.Errorf("至少需要一条页面指令")
	}
	if strings.TrimSpace(req.LaunchBaseURL) == "" {
		return PageCommandResult{}, fmt.Errorf("launchBaseUrl is required")
	}

	state := m.CurrentState()
	if !state.Ready {
		return PageCommandResult{}, fmt.Errorf("自动化运行时尚未就绪")
	}

	session, reused, err := m.acquirePageSession(state, req)
	if err != nil {
		return PageCommandResult{}, err
	}

	result := PageCommandResult{
		ProfileID: profileID,
		OK:        true,
		Reused:    reused,
		Session:   session.session,
		Results:   make([]PageStepResult, 0, len(req.Commands)),
	}

	for index, command := range req.Commands {
		if err := ctx.Err(); err != nil {
			result.OK = false
			result.Error = taskContextErrorMessage(err, req.Timeout)
			return result, nil
		}

		step, err := session.call(command, req.Timeout)
		if err != nil {
			// 会话已死。第一条指令还没产生任何副作用，可以安全地换个新会话重来；
			// 后续指令重放会重复副作用，只能如实报错。
			m.dropPageSession(profileID, session)
			if index == 0 && !reused {
				result.OK = false
				result.Error = err.Error()
				return result, nil
			}
			if index == 0 {
				fresh, _, spawnErr := m.acquirePageSession(state, req)
				if spawnErr != nil {
					result.OK = false
					result.Error = fmt.Sprintf("页面会话已失效且无法重建：%s", spawnErr.Error())
					return result, nil
				}
				session = fresh
				result.Reused = false
				result.Session = fresh.session
				if step, err = session.call(command, req.Timeout); err != nil {
					m.dropPageSession(profileID, session)
					result.OK = false
					result.Error = err.Error()
					return result, nil
				}
			} else {
				result.OK = false
				result.Error = fmt.Sprintf("页面会话在第 %d 条指令（%s）中断：%s", index+1, command.Action, err.Error())
				return result, nil
			}
		}

		result.Results = append(result.Results, step)
		if !step.OK {
			// 指令级失败不代表会话坏了，但后续指令通常依赖前一步的结果，继续跑没有意义。
			result.OK = false
			result.Error = step.Error
			break
		}
	}

	return result, nil
}

// ClosePageSession 关闭指定实例的常驻会话；会话不存在时是空操作。
func (m *Manager) ClosePageSession(profileID string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}

	m.pageMu.Lock()
	session := m.pageSessions[profileID]
	delete(m.pageSessions, profileID)
	m.pageMu.Unlock()

	if session != nil {
		session.close()
	}
}

// closeAllPageSessions 关闭全部常驻会话，退出流程使用。
func (m *Manager) closeAllPageSessions() {
	m.pageMu.Lock()
	sessions := make([]*pageSession, 0, len(m.pageSessions))
	for _, session := range m.pageSessions {
		sessions = append(sessions, session)
	}
	m.pageSessions = make(map[string]*pageSession)
	m.pageMu.Unlock()

	for _, session := range sessions {
		session.close()
	}
}

// PageSessionProfiles 返回当前持有常驻会话的实例列表，用于状态展示与测试断言。
func (m *Manager) PageSessionProfiles() []string {
	m.pageMu.Lock()
	defer m.pageMu.Unlock()

	items := make([]string, 0, len(m.pageSessions))
	for profileID := range m.pageSessions {
		items = append(items, profileID)
	}
	return items
}

// acquirePageSession 取回已有会话或新建一个。
func (m *Manager) acquirePageSession(state RuntimeState, req PageCommandRequest) (*pageSession, bool, error) {
	profileID := strings.TrimSpace(req.ProfileID)

	m.pageMu.Lock()
	if existing, ok := m.pageSessions[profileID]; ok && existing != nil && !existing.isClosed() {
		m.pageMu.Unlock()
		return existing, true, nil
	}
	m.pageMu.Unlock()

	session, err := m.spawnPageSession(state, req)
	if err != nil {
		return nil, false, err
	}

	m.pageMu.Lock()
	// 双检：spawn 期间没持锁，可能有并发调用先建好了会话。
	if existing, ok := m.pageSessions[profileID]; ok && existing != nil && !existing.isClosed() {
		m.pageMu.Unlock()
		session.close()
		return existing, true, nil
	}
	m.pageSessions[profileID] = session
	m.pageMu.Unlock()

	m.ensurePageSessionReaper(req.IdleTimeout)
	return session, false, nil
}

// dropPageSession 只在会话仍是当前登记的那个时移除，避免误删并发重建出来的新会话。
func (m *Manager) dropPageSession(profileID string, session *pageSession) {
	m.pageMu.Lock()
	if current, ok := m.pageSessions[profileID]; ok && current == session {
		delete(m.pageSessions, profileID)
	}
	m.pageMu.Unlock()

	if session != nil {
		session.close()
	}
}

// spawnPageSession 拉起一个常驻 Node 进程并等待它报 ready。
func (m *Manager) spawnPageSession(state RuntimeState, req PageCommandRequest) (*pageSession, error) {
	payload := pageSessionPayload{
		TaskType:         taskTypePageSession,
		RuntimeDir:       state.RuntimeDir,
		Selector:         req.Selector,
		LaunchBaseURL:    strings.TrimSpace(req.LaunchBaseURL),
		LaunchAuthHeader: strings.TrimSpace(req.LaunchAuthHeader),
		LaunchAuthValue:  strings.TrimSpace(req.LaunchAuthValue),
		ArtifactDir:      strings.TrimSpace(req.ArtifactDir),
	}
	if req.Timeout > 0 {
		payload.DefaultTimeoutMs = req.Timeout.Milliseconds()
	}
	payload.ConnectTimeoutMs = pageSessionReadyTimeout.Milliseconds()

	payloadPath, err := m.writePageSessionPayload(payload)
	if err != nil {
		return nil, err
	}
	// payload 只在启动时读一次，进程起来后即可删除，避免鉴权值长期留在磁盘上。
	defer os.Remove(payloadPath)

	cmd := exec.Command(state.NodePath, state.RunnerPath, payloadPath)
	cmd.Dir = state.RuntimeDir
	prepareTaskCommand(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建页面会话输入管道失败: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建页面会话输出管道失败: %w", err)
	}
	// stderr 是 Node 侧的诊断输出，不参与协议；丢弃即可，避免管道写满阻塞进程。
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动页面会话进程失败: %w", err)
	}

	session := &pageSession{
		profileID: strings.TrimSpace(req.ProfileID),
		proc:      &nodeSessionProcess{cmd: cmd, stdin: stdin, stdout: stdout},
		lastUsed:  time.Now(),
	}
	session.reader = bufio.NewReaderSize(stdout, 64<<10)

	// 进程退出后回收资源，避免僵尸进程。
	go func() {
		_ = cmd.Wait()
	}()

	if err := session.awaitReady(pageSessionReadyTimeout); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

func (m *Manager) writePageSessionPayload(payload pageSessionPayload) (string, error) {
	tempDir := filepath.Join(m.runtimeRoot(), "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("创建页面会话临时目录失败: %w", err)
	}
	file, err := os.CreateTemp(tempDir, "page-session-*.json")
	if err != nil {
		return "", fmt.Errorf("创建页面会话临时文件失败: %w", err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		return "", fmt.Errorf("写入页面会话 payload 失败: %w", err)
	}
	return file.Name(), nil
}

// ensurePageSessionReaper 惰性启动空闲回收协程：没人用页面会话时不产生任何后台开销。
func (m *Manager) ensurePageSessionReaper(idleTimeout time.Duration) {
	if idleTimeout <= 0 {
		idleTimeout = defaultPageSessionIdle
	}

	m.pageMu.Lock()
	if m.pageReaperOn {
		m.pageMu.Unlock()
		return
	}
	m.pageReaperOn = true
	m.pageMu.Unlock()

	go m.reapIdlePageSessions(idleTimeout)
}

func (m *Manager) reapIdlePageSessions(idleTimeout time.Duration) {
	interval := idleTimeout / 4
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		m.pageMu.Lock()
		expired := make([]*pageSession, 0)
		for profileID, session := range m.pageSessions {
			if session == nil || session.isClosed() || now.Sub(session.touchedAt()) > idleTimeout {
				expired = append(expired, session)
				delete(m.pageSessions, profileID)
			}
		}
		remaining := len(m.pageSessions)
		if remaining == 0 {
			// 没有会话就退出协程，下次 acquire 时重新拉起。
			m.pageReaperOn = false
		}
		m.pageMu.Unlock()

		for _, session := range expired {
			if session != nil {
				session.close()
			}
		}
		if remaining == 0 {
			return
		}
	}
}
