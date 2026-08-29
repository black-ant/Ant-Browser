package automation

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

// 常驻页面会话的协议层测试。
//
// 用内存管道替代真实的 Node 进程：分帧、id 关联、断线判定这些逻辑与
// Playwright 无关，没必要为了测它们去装一整套运行时。

// fakeSessionProcess 模拟 Node 侧：收到指令后按 respond 生成响应行。
// respond 返回空串表示「不回复」，用于制造超时。
type fakeSessionProcess struct {
	pr      *io.PipeReader
	pw      *io.PipeWriter
	respond func(id int64, action string, args map[string]any) string

	mu       sync.Mutex
	commands []string
	killed   bool
}

func newFakeProcess(respond func(id int64, action string, args map[string]any) string) *fakeSessionProcess {
	pr, pw := io.Pipe()
	return &fakeSessionProcess{pr: pr, pw: pw, respond: respond}
}

func (p *fakeSessionProcess) Write(data []byte) (int, error) {
	p.mu.Lock()
	if p.killed {
		p.mu.Unlock()
		return 0, errors.New("process is closed")
	}
	line := strings.TrimSpace(string(data))
	p.commands = append(p.commands, line)
	p.mu.Unlock()

	var command struct {
		ID     int64          `json:"id"`
		Action string         `json:"action"`
		Args   map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(line), &command); err != nil {
		return 0, err
	}

	if response := p.respond(command.ID, command.Action, command.Args); response != "" {
		// 必须异步写：io.Pipe 的写会阻塞到有人读，而读取发生在 Write 返回之后。
		go func() { _, _ = io.WriteString(p.pw, response+"\n") }()
	}
	return len(data), nil
}

func (p *fakeSessionProcess) Stdout() io.Reader { return p.pr }

func (p *fakeSessionProcess) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.killed {
		return nil
	}
	p.killed = true
	_ = p.pw.CloseWithError(io.EOF)
	return nil
}

func (p *fakeSessionProcess) wasKilled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.killed
}

func (p *fakeSessionProcess) sentCommands() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.commands...)
}

// push 让测试在指令之外主动写入一行（例如 ready 事件）。
func (p *fakeSessionProcess) push(line string) {
	go func() { _, _ = io.WriteString(p.pw, line+"\n") }()
}

func newTestSession(proc *fakeSessionProcess) *pageSession {
	return &pageSession{
		profileID: "p1",
		proc:      proc,
		reader:    bufio.NewReaderSize(proc.Stdout(), 4096),
		lastUsed:  time.Now(),
	}
}

func okResponse(id int64, result string) string {
	return `{"id":` + itoaTest(id) + `,"ok":true,"result":` + result + `}`
}

func itoaTest(value int64) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func TestPageSessionCallRoundTrip(t *testing.T) {
	proc := newFakeProcess(func(id int64, action string, _ map[string]any) string {
		return okResponse(id, `{"url":"https://example.com/","title":"Example","action":"`+action+`"}`)
	})
	session := newTestSession(proc)

	step, err := session.call(PageCommand{
		Action: "goto",
		Args:   map[string]any{"url": "https://example.com/"},
	}, time.Second)
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	if !step.OK {
		t.Errorf("step.OK = false, want true (error: %s)", step.Error)
	}
	if step.Action != "goto" {
		t.Errorf("step.Action = %q, want goto", step.Action)
	}
	if got, _ := step.Result["url"].(string); got != "https://example.com/" {
		t.Errorf("result url = %q, want https://example.com/", got)
	}

	sent := proc.sentCommands()
	if len(sent) != 1 {
		t.Fatalf("sent %d commands, want 1", len(sent))
	}
	// 指令必须带自增 id，否则响应无法归属。
	if !strings.Contains(sent[0], `"id":1`) || !strings.Contains(sent[0], `"action":"goto"`) {
		t.Errorf("command line = %s, want id=1 and action=goto", sent[0])
	}
	if !strings.Contains(sent[0], `"url":"https://example.com/"`) {
		t.Errorf("command line = %s, want args passed through", sent[0])
	}
}

// 指令 id 必须逐次递增，否则迟到的响应会被错误地当成当前指令的结果。
func TestPageSessionAssignsIncrementingIDs(t *testing.T) {
	proc := newFakeProcess(func(id int64, _ string, _ map[string]any) string {
		return okResponse(id, `{}`)
	})
	session := newTestSession(proc)

	for i := 0; i < 3; i++ {
		if _, err := session.call(PageCommand{Action: "snapshot"}, time.Second); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	for index, line := range proc.sentCommands() {
		want := `"id":` + itoaTest(int64(index+1))
		if !strings.Contains(line, want) {
			t.Errorf("command %d = %s, want %s", index, line, want)
		}
	}
}

// 事件行（ready 等）没有 id，不能被误当成指令响应消费掉。
func TestPageSessionSkipsUnrelatedLines(t *testing.T) {
	proc := newFakeProcess(func(id int64, _ string, _ map[string]any) string {
		return `{"type":"ready","session":{"cdpUrl":"http://127.0.0.1:1"}}` + "\n" + okResponse(id, `{"title":"Done"}`)
	})
	session := newTestSession(proc)

	step, err := session.call(PageCommand{Action: "snapshot"}, time.Second)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got, _ := step.Result["title"].(string); got != "Done" {
		t.Errorf("result title = %q, want Done", got)
	}
}

// 指令自身失败不代表会话坏了：返回 OK=false 但不返回 error，会话可继续使用。
func TestPageSessionStepFailureKeepsSessionAlive(t *testing.T) {
	proc := newFakeProcess(func(id int64, _ string, _ map[string]any) string {
		return `{"id":` + itoaTest(id) + `,"ok":false,"error":"selector not found"}`
	})
	session := newTestSession(proc)

	step, err := session.call(PageCommand{Action: "click"}, time.Second)
	if err != nil {
		t.Fatalf("call returned session error for a step failure: %v", err)
	}
	if step.OK {
		t.Error("step.OK = true, want false")
	}
	if step.Error != "selector not found" {
		t.Errorf("step.Error = %q, want selector not found", step.Error)
	}
	if session.isClosed() {
		t.Error("session was closed by a step-level failure")
	}
}

// closed 事件意味着浏览器断开，会话必须立刻作废。
func TestPageSessionClosedEventFailsAndClosesSession(t *testing.T) {
	proc := newFakeProcess(func(int64, string, map[string]any) string {
		return `{"type":"closed","reason":"browser disconnected"}`
	})
	session := newTestSession(proc)

	if _, err := session.call(PageCommand{Action: "snapshot"}, time.Second); err == nil {
		t.Fatal("call succeeded, want error after closed event")
	} else if !strings.Contains(err.Error(), "browser disconnected") {
		t.Errorf("error = %v, want it to mention the disconnect reason", err)
	}

	if !session.isClosed() {
		t.Error("session is not marked closed after a closed event")
	}
	if !proc.wasKilled() {
		t.Error("process was not killed after a closed event")
	}
}

func TestPageSessionAwaitReady(t *testing.T) {
	proc := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	session := newTestSession(proc)
	proc.push(`{"type":"ready","session":{"cdpUrl":"http://127.0.0.1:9222"},"page":{"url":"about:blank"}}`)

	if err := session.awaitReady(2 * time.Second); err != nil {
		t.Fatalf("awaitReady: %v", err)
	}
	if got, _ := session.session["cdpUrl"].(string); got != "http://127.0.0.1:9222" {
		t.Errorf("session cdpUrl = %q, want http://127.0.0.1:9222", got)
	}
}

func TestPageSessionAwaitReadyTimesOut(t *testing.T) {
	proc := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	session := newTestSession(proc)

	err := session.awaitReady(80 * time.Millisecond)
	if err == nil {
		t.Fatal("awaitReady succeeded, want timeout")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("error = %v, want a timeout message", err)
	}
}

// 管道上不能有两条指令同时在飞，否则响应无法可靠归属。
func TestPageSessionSerializesConcurrentCalls(t *testing.T) {
	var inFlight, maxInFlight int
	var mu sync.Mutex

	proc := newFakeProcess(func(id int64, action string, _ map[string]any) string {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(5 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		return okResponse(id, `{"action":"`+action+`"}`)
	})
	session := newTestSession(proc)

	const callers = 6
	var wg sync.WaitGroup
	results := make([]string, callers)
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			action := "snapshot"
			if index%2 == 1 {
				action = "extract"
			}
			step, err := session.call(PageCommand{Action: action}, 2*time.Second)
			errs[index] = err
			if err == nil {
				results[index], _ = step.Result["action"].(string)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
		want := "snapshot"
		if i%2 == 1 {
			want = "extract"
		}
		// 拿到别人的响应说明关联错乱了。
		if results[i] != want {
			t.Errorf("caller %d got action %q, want %q", i, results[i], want)
		}
	}
	if maxInFlight != 1 {
		t.Errorf("max concurrent commands = %d, want 1", maxInFlight)
	}
}

func TestPageSessionWriteFailureClosesSession(t *testing.T) {
	proc := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	session := newTestSession(proc)
	_ = proc.Kill()

	if _, err := session.call(PageCommand{Action: "snapshot"}, time.Second); err == nil {
		t.Fatal("call succeeded on a dead process, want error")
	}
	if !session.isClosed() {
		t.Error("session is not marked closed after a write failure")
	}
}

// ---------------------------------------------------------------------------
// Manager 层
// ---------------------------------------------------------------------------

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir(), config.DefaultConfig(), func(string, any) {}, Options{})
}

func registerTestSession(m *Manager, profileID string, proc *fakeSessionProcess) *pageSession {
	session := newTestSession(proc)
	session.profileID = profileID

	m.pageMu.Lock()
	m.pageSessions[profileID] = session
	m.pageMu.Unlock()
	return session
}

func TestClosePageSessionRemovesAndKills(t *testing.T) {
	m := newTestManager(t)
	proc := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	registerTestSession(m, "p1", proc)

	m.ClosePageSession("p1")

	if got := m.PageSessionProfiles(); len(got) != 0 {
		t.Errorf("page sessions = %v, want empty", got)
	}
	if !proc.wasKilled() {
		t.Error("process was not killed")
	}

	// 重复关闭必须是空操作，退出路径上会被多次触发。
	m.ClosePageSession("p1")
}

func TestStopAllTasksClosesPageSessions(t *testing.T) {
	m := newTestManager(t)
	first := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	second := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	registerTestSession(m, "p1", first)
	registerTestSession(m, "p2", second)

	m.StopAllTasks()

	if got := m.PageSessionProfiles(); len(got) != 0 {
		t.Errorf("page sessions = %v, want empty after StopAllTasks", got)
	}
	if !first.wasKilled() || !second.wasKilled() {
		t.Error("StopAllTasks left a page session process running")
	}
}

func TestRunPageCommandsRejectsInvalidInput(t *testing.T) {
	m := newTestManager(t)

	cases := []struct {
		name string
		req  PageCommandRequest
		want string
	}{
		{
			name: "missing profile",
			req:  PageCommandRequest{Commands: []PageCommand{{Action: "goto"}}, LaunchBaseURL: "http://127.0.0.1:1"},
			want: "profileId",
		},
		{
			name: "no commands",
			req:  PageCommandRequest{ProfileID: "p1", LaunchBaseURL: "http://127.0.0.1:1"},
			want: "页面指令",
		},
		{
			name: "missing launch base url",
			req:  PageCommandRequest{ProfileID: "p1", Commands: []PageCommand{{Action: "goto"}}},
			want: "launchBaseUrl",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := m.RunPageCommands(nil, tc.req); err == nil {
				t.Fatal("RunPageCommands succeeded, want error")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// 脚本任务开跑前必须让位：两个 Playwright 客户端连同一个浏览器时，
// 脚本结束的连接清理会把常驻会话一起带走，留下一个已死却仍在表里的条目。
func TestRunScriptTaskClosesPageSession(t *testing.T) {
	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Automation.NodeSource = config.AutomationNodeSourceBundled
	m := NewManager(appRoot, cfg, func(string, any) {}, Options{})

	fakeReadyRuntime(t, m, cfg.Automation.RuntimeVersion)
	if !m.CurrentState().Ready {
		t.Fatal("faked runtime is not reported as ready")
	}

	proc := newFakeProcess(func(int64, string, map[string]any) string { return "" })
	registerTestSession(m, "p1", proc)

	// 这次执行注定失败（node 是个空文件），但让位发生在启动子进程之前。
	_, _ = m.RunScriptTask(nil, ScriptTaskRequest{
		TaskKey:       "p1",
		ScriptPath:    filepath.Join(appRoot, "script.cjs"),
		LaunchBaseURL: "http://127.0.0.1:19876",
	})

	if got := m.PageSessionProfiles(); len(got) != 0 {
		t.Errorf("page sessions = %v, want the session evicted by the script run", got)
	}
	if !proc.wasKilled() {
		t.Error("page session process survived a script run on the same profile")
	}
}

// fakeReadyRuntime 摆出 CurrentState 判定 Ready 所需的三个文件。
func fakeReadyRuntime(t *testing.T, m *Manager, runtimeVersion string) {
	t.Helper()

	runtimeDir := m.runtimeDir(runtimeVersion)
	nodePath := filepath.Join(runtimeDir, "node", "bin", "node")
	if runtime.GOOS == "windows" {
		nodePath = filepath.Join(runtimeDir, "node", "node.exe")
	}

	files := []string{
		nodePath,
		filepath.Join(runtimeDir, "node_modules", "playwright-core", "package.json"),
		filepath.Join(runtimeDir, runnerScriptFileName),
	}
	for _, path := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o755); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
