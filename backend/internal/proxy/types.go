package proxy

import (
	"os/exec"
	"sync"
	"time"
)

// XrayBridge Xray 桥接进程
type XrayBridge struct {
	NodeKey    string
	Port       int
	Cmd        *exec.Cmd
	Pid        int
	Running    bool
	LastError  string
	RefCount   int
	LastUsedAt time.Time
	Stopping   bool
	Restarting bool
	Outbounds  []interface{}
	Routes     []interface{}
	DNSServers string
	ExitDone   chan struct{}
	ExitErr    error
	exitMu     sync.Mutex
	waitOnce   sync.Once
}

// ProxyResult 代理解析结果
type ProxyResult struct {
	StandardProxy string                 // 标准代理 URL (http/socks5)
	Outbound      map[string]interface{} // Xray outbound 配置
}
