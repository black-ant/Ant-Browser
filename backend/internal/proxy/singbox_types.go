package proxy

import (
	"ant-chrome/backend/internal/config"
	"os/exec"
	"sync"
	"time"
)

const (
	singBoxBridgeIdleTTL         = 45 * time.Second
	singBoxBridgeCleanupInterval = 15 * time.Second
)

// SingBoxBridge sing-box 桥接进程
type SingBoxBridge struct {
	NodeKey      string
	Port         int
	Cmd          *exec.Cmd
	Pid          int
	Running      bool
	Stopping     bool
	LastError    string
	Outbound     map[string]interface{}
	RefCount     int
	LastUsedAt   time.Time
	Restarting   bool
	RestartCount int
	ExitDone     chan struct{}
	ExitErr      error
	exitMu       sync.Mutex
	waitOnce     sync.Once
}

// SingBoxManager sing-box 桥接管理器
type SingBoxManager struct {
	Config       *config.Config
	AppRoot      string // 应用根目录，所有相对路径基于此解析
	Bridges      map[string]*SingBoxBridge
	OnBridgeDied func(key string, err error)
	mu           sync.Mutex
	launchLocks  map[string]*bridgeLaunchLock
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewSingBoxManager 创建 sing-box 管理器
func NewSingBoxManager(cfg *config.Config, appRoot string) *SingBoxManager {
	manager := &SingBoxManager{
		Config:      cfg,
		AppRoot:     appRoot,
		Bridges:     make(map[string]*SingBoxBridge),
		launchLocks: make(map[string]*bridgeLaunchLock),
		stopCh:      make(chan struct{}),
	}
	go manager.cleanupLoop()
	return manager
}
