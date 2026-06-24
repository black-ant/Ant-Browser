package cdp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// DebugPipe 持有 --remote-debugging-pipe 的管道端。
//   - send：父进程写 CDP 命令，对端是子进程 fd 3（子进程从 fd 3 读命令）。
//   - recv：父进程读 CDP 响应/事件，对端是子进程 fd 4（子进程往 fd 4 写）。
//   - childFiles：传给 exec.Cmd.ExtraFiles 的子进程端，[0]→fd3(读端)，[1]→fd4(写端)。
//
// 创建逻辑平台相关（见 pipe_attach_unix.go / pipe_attach_windows.go）：
// Windows 上 os/exec 无法把句柄映射成子进程 fd 3/4，故直接返回不支持错误，由调用方回退端口。
type DebugPipe struct {
	send       *os.File
	recv       *os.File
	childFiles []*os.File
}

// ExtraFiles 返回需要挂到 exec.Cmd.ExtraFiles 上的子进程端（fd3 读端、fd4 写端）。
func (p *DebugPipe) ExtraFiles() []*os.File { return p.childFiles }

// CloseChildEnds 在 cmd.Start() 之后调用：父进程不再持有子进程那一端，
// 否则读循环收不到 EOF、句柄泄露。
func (p *DebugPipe) CloseChildEnds() {
	for _, f := range p.childFiles {
		if f != nil {
			_ = f.Close()
		}
	}
}

// NewConn 从父进程端构造多路复用连接并启动读循环（在 cmd.Start + CloseChildEnds 之后调用）。
func (p *DebugPipe) NewConn() *PipeConn {
	return NewPipeConn(p.send, p.recv)
}

// Close 关闭所有管道端（启动失败时清理）。
func (p *DebugPipe) Close() {
	p.CloseChildEnds()
	if p.send != nil {
		_ = p.send.Close()
	}
	if p.recv != nil {
		_ = p.recv.Close()
	}
}

// PipeConn 是基于 --remote-debugging-pipe 的单连接、多路复用 CDP 传输。
// 一个浏览器进程只有一对管道，因此所有 CDP 消费者（DevTools 会话 / Cookie /
// 用户名扫描 / 就绪探测）都共用同一个 PipeConn，用 sessionId（flatten 模式）区分目标。
type PipeConn struct {
	send io.WriteCloser
	recv io.ReadCloser

	writeMu sync.Mutex // 串行化写

	mu        sync.Mutex
	commandID int
	pending   map[int]chan *CDPMessage

	handlersMu sync.RWMutex
	handlers   map[string]func(*CDPMessage) // key 为 sessionId（"" = 根/浏览器级事件）

	closeOnce sync.Once
	closed    chan struct{}
}

// NewPipeConn 用给定的写/读端建立多路复用连接并启动读循环。
func NewPipeConn(send io.WriteCloser, recv io.ReadCloser) *PipeConn {
	c := &PipeConn{
		send:     send,
		recv:     recv,
		pending:  make(map[int]chan *CDPMessage),
		handlers: make(map[string]func(*CDPMessage)),
		closed:   make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *PipeConn) readLoop() {
	reader := bufio.NewReaderSize(c.recv, 1<<16)
	for {
		// CDP pipe 协议：每条消息以 NUL(0x00) 分隔。
		frame, err := reader.ReadBytes(0)
		if err != nil {
			c.failAll(err)
			c.Close()
			return
		}
		if n := len(frame); n > 0 && frame[n-1] == 0 {
			frame = frame[:n-1]
		}
		if len(frame) == 0 {
			continue
		}
		var msg CDPMessage
		if jerr := json.Unmarshal(frame, &msg); jerr != nil {
			continue
		}
		c.dispatch(&msg)
	}
}

func (c *PipeConn) dispatch(msg *CDPMessage) {
	// 命令响应
	if msg.ID > 0 {
		c.mu.Lock()
		ch, ok := c.pending[msg.ID]
		if ok {
			delete(c.pending, msg.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
		return
	}
	// 事件：按 sessionId 分发
	if msg.Method != "" {
		c.handlersMu.RLock()
		h := c.handlers[msg.SessionID]
		c.handlersMu.RUnlock()
		if h != nil {
			h(msg)
		}
	}
}

// SetEventHandler 注册某 sessionId 的事件处理器（sessionID="" 表示根连接事件）。
func (c *PipeConn) SetEventHandler(sessionID string, h func(*CDPMessage)) {
	c.handlersMu.Lock()
	c.handlers[sessionID] = h
	c.handlersMu.Unlock()
}

func (c *PipeConn) RemoveEventHandler(sessionID string) {
	c.handlersMu.Lock()
	delete(c.handlers, sessionID)
	c.handlersMu.Unlock()
}

// SendCommand 向指定 sessionId（""=根/浏览器级）发送 CDP 命令并等待响应（默认 10s 超时）。
func (c *PipeConn) SendCommand(sessionID, method string, params map[string]interface{}) (map[string]interface{}, error) {
	return c.SendCommandTimeout(sessionID, method, params, 10*time.Second)
}

// SendCommandTimeout 同 SendCommand，但可指定等待响应的超时（用于需要长轮询的页面脚本）。
func (c *PipeConn) SendCommandTimeout(sessionID, method string, params map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	c.mu.Lock()
	c.commandID++
	id := c.commandID
	ch := make(chan *CDPMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	msg := CDPMessage{ID: id, Method: method, Params: params, SessionID: sessionID}
	payload, err := json.Marshal(msg)
	if err != nil {
		c.removePending(id)
		return nil, err
	}
	payload = append(payload, 0) // NUL 分隔符

	c.writeMu.Lock()
	_, werr := c.send.Write(payload)
	c.writeMu.Unlock()
	if werr != nil {
		c.removePending(id)
		return nil, werr
	}

	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case res := <-ch:
		if res.Error != nil {
			return nil, fmt.Errorf("CDP错误: %s", res.Error.Message)
		}
		return res.Result, nil
	case <-time.After(timeout):
		c.removePending(id)
		return nil, fmt.Errorf("命令超时: %s", method)
	case <-c.closed:
		return nil, fmt.Errorf("CDP 管道连接已关闭")
	}
}

func (c *PipeConn) removePending(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *PipeConn) failAll(err error) {
	c.mu.Lock()
	for id, ch := range c.pending {
		select {
		case ch <- &CDPMessage{Error: &CDPError{Message: err.Error()}}:
		default:
		}
		delete(c.pending, id)
	}
	c.mu.Unlock()
}

// Close 关闭连接与底层管道端。可重复调用。
func (c *PipeConn) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		if c.send != nil {
			_ = c.send.Close()
		}
		if c.recv != nil {
			_ = c.recv.Close()
		}
	})
}

// Closed 返回连接关闭信号通道。
func (c *PipeConn) Closed() <-chan struct{} { return c.closed }
