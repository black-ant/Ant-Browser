package automation

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"time"
)

// 常驻页面会话的类型定义。
//
// 会话按 profileID 维护，一个实例最多一个常驻 Node 进程。协议是 NDJSON：
// 每条指令写一行，读一行响应，靠 id 关联。管道天然不支持并发，因此
// 每个会话内部用互斥锁把「写指令 + 读响应」串成一个原子操作。

const (
	taskTypePageSession = "page-session"

	// defaultPageSessionIdle 是会话空闲多久后被回收。
	//
	// 常驻的意义是省掉 CDP 握手，但一个挂着的 Node 进程 + Playwright 连接
	// 也占资源。5 分钟足以覆盖 agent 连续操作的间隔，又不会让忘记释放的
	// 会话长期驻留。
	defaultPageSessionIdle = 5 * time.Minute

	// pageSessionReadyTimeout 是等待 Node 侧发出 ready 行的上限。
	// 这一步包含 launch 实例 + connectOverCDP，是整个会话最慢的环节。
	pageSessionReadyTimeout = 90 * time.Second

	// pageSessionMaxLine 是单行响应的上限。截图等大产物走文件不走管道，
	// 正常响应远小于此；设上限是为了让异常输出快速失败而不是无限增长。
	pageSessionMaxLine = 8 << 20
)

// PageCommand 是一条页面指令。
type PageCommand struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// PageCommandRequest 描述一次页面指令调用。
type PageCommandRequest struct {
	// ProfileID 决定复用哪个常驻会话。
	ProfileID string
	// Selector 传给 Node 侧的 launch，用于首次建立会话时定位实例。
	Selector map[string]any
	Commands []PageCommand

	LaunchBaseURL    string
	LaunchAuthHeader string
	LaunchAuthValue  string
	ArtifactDir      string

	// Timeout 是单条指令的等待上限，同时透传给 Node 侧作为默认超时。
	Timeout time.Duration
	// IdleTimeout 覆盖默认的空闲回收时间，<=0 时用 defaultPageSessionIdle。
	IdleTimeout time.Duration
}

// PageCommandResult 是一次调用的聚合结果。
type PageCommandResult struct {
	ProfileID string           `json:"profileId"`
	OK        bool             `json:"ok"`
	Results   []PageStepResult `json:"results"`
	Session   map[string]any   `json:"session,omitempty"`
	Reused    bool             `json:"reused"`
	Error     string           `json:"error,omitempty"`
}

// PageStepResult 是单条指令的执行结果。
type PageStepResult struct {
	Action string         `json:"action"`
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// sessionProcess 抽象常驻进程的读写与终止。
//
// 抽成接口是为了让单测用内存管道跑完整的分帧、关联和重拉逻辑，
// 不必依赖真实的 Node 运行时。
type sessionProcess interface {
	io.Writer
	Stdout() io.Reader
	Kill() error
}

// nodeSessionProcess 是 sessionProcess 的真实实现。
type nodeSessionProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
}

func (p *nodeSessionProcess) Write(data []byte) (int, error) { return p.stdin.Write(data) }
func (p *nodeSessionProcess) Stdout() io.Reader              { return p.stdout }

func (p *nodeSessionProcess) Kill() error {
	// 先关 stdin：Node 侧 readline 的 close 事件会触发优雅收尾（断开 CDP 连接）。
	// 随后再杀进程组兜底，避免优雅路径卡住。
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	return stopTaskProcess(p.cmd)
}

// pageSession 是一个常驻会话。
type pageSession struct {
	profileID string
	proc      sessionProcess
	reader    *bufio.Reader
	session   map[string]any

	mu       sync.Mutex // 串行化一问一答；NDJSON 管道不支持并发
	nextID   int64
	lastUsed time.Time
	closed   bool
}

// sessionEnvelope 是 Node 侧回写的一行。
// ready/closed 事件没有 id，指令响应有。
type sessionEnvelope struct {
	Type    string         `json:"type,omitempty"`
	ID      int64          `json:"id,omitempty"`
	OK      bool           `json:"ok,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   string         `json:"error,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Session map[string]any `json:"session,omitempty"`
	Page    map[string]any `json:"page,omitempty"`
}

// pageSessionPayload 是启动常驻会话时写给 Node 的 payload。
type pageSessionPayload struct {
	TaskType         string         `json:"taskType"`
	RuntimeDir       string         `json:"runtimeDir"`
	Selector         map[string]any `json:"selector,omitempty"`
	LaunchBaseURL    string         `json:"launchBaseUrl"`
	LaunchAuthHeader string         `json:"launchAuthHeader,omitempty"`
	LaunchAuthValue  string         `json:"launchAuthValue,omitempty"`
	ArtifactDir      string         `json:"artifactDir,omitempty"`
	DefaultTimeoutMs int64          `json:"defaultTimeoutMs,omitempty"`
	ConnectTimeoutMs int64          `json:"connectTimeoutMs,omitempty"`
}

func marshalCommand(id int64, cmd PageCommand) ([]byte, error) {
	payload := struct {
		ID     int64          `json:"id"`
		Action string         `json:"action"`
		Args   map[string]any `json:"args,omitempty"`
	}{ID: id, Action: cmd.Action, Args: cmd.Args}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
