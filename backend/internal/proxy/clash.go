package proxy

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ClashManager Clash 进程管理器
type ClashManager struct {
	Config           *config.Config
	AppRoot          string // 应用根目录，所有相对路径基于此解析
	Processes        map[string]*exec.Cmd
	NodeBridges      map[string]*MihomoNodeBridge
	OnNodeBridgeDied func(key string, err error)
	mu               sync.Mutex
	launchLocks      map[string]*bridgeLaunchLock
}

// NewClashManager 创建 Clash 管理器
func NewClashManager(cfg *config.Config, appRoot string) *ClashManager {
	return &ClashManager{
		Config:      cfg,
		AppRoot:     appRoot,
		Processes:   make(map[string]*exec.Cmd),
		NodeBridges: make(map[string]*MihomoNodeBridge),
		launchLocks: make(map[string]*bridgeLaunchLock),
	}
}

// ClashProfile Clash 配置接口
type ClashProfile interface {
	GetProfileId() string
	GetClashEnabled() bool
	GetClashRunning() bool
	GetClashConfigPath() string
	GetClashProxyPort() int
	SetClashRunning(bool)
	SetClashPid(int)
	SetClashProxyPort(int)
	SetClashLastError(string)
}

// StartForProfile 为配置启动 Clash 进程
func (m *ClashManager) StartForProfile(profile ClashProfile, userDataDir string) error {
	log := logger.New("Clash")
	if !profile.GetClashEnabled() {
		return nil
	}
	if profile.GetClashRunning() {
		return nil
	}
	clashBinaryPath := strings.TrimSpace(m.Config.Browser.ClashBinaryPath)
	if clashBinaryPath == "" {
		err := fmt.Errorf("clash binary path not configured")
		profile.SetClashLastError(err.Error())
		log.Error("Clash 启动失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		return err
	}
	if _, err := os.Stat(clashBinaryPath); err != nil {
		profile.SetClashLastError(err.Error())
		log.Error("Clash 启动失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		return err
	}
	templatePath := strings.TrimSpace(profile.GetClashConfigPath())
	if templatePath == "" {
		err := fmt.Errorf("clash config path not configured")
		profile.SetClashLastError(err.Error())
		log.Error("Clash 启动失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		return err
	}
	if _, err := os.Stat(templatePath); err != nil {
		profile.SetClashLastError(err.Error())
		log.Error("Clash 启动失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		return err
	}
	port := profile.GetClashProxyPort()
	if port == 0 {
		p, err := nextAvailablePort()
		if err != nil {
			profile.SetClashLastError(err.Error())
			log.Error("Clash 端口分配失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
			return err
		}
		port = p
		profile.SetClashProxyPort(port)
	}
	args := []string{
		"-f", templatePath,
		"-d", userDataDir,
	}
	cmd := exec.Command(clashBinaryPath, args...)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		profile.SetClashLastError(err.Error())
		log.Error("Clash 启动失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		return err
	}
	m.mu.Lock()
	m.Processes[profile.GetProfileId()] = cmd
	m.mu.Unlock()
	profile.SetClashRunning(true)
	profile.SetClashPid(cmd.Process.Pid)
	profile.SetClashLastError("")
	log.Info("Clash 内核进程已启动", logger.F("engine", "clash"), logger.F("profile_id", profile.GetProfileId()), logger.F("pid", cmd.Process.Pid), logger.F("port", port))
	return nil
}

// StopForProfile 停止配置的 Clash 进程
func (m *ClashManager) StopForProfile(profile ClashProfile) {
	log := logger.New("Clash")
	m.mu.Lock()
	cmd := m.Processes[profile.GetProfileId()]
	delete(m.Processes, profile.GetProfileId())
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			log.Error("Clash 停止失败", logger.F("profile_id", profile.GetProfileId()), logger.F("error", err))
		}
	}
	profile.SetClashRunning(false)
	profile.SetClashPid(0)
	log.Info("Clash 已停止", logger.F("profile_id", profile.GetProfileId()))
}

// StopAll 停止所有 Clash 进程
func (m *ClashManager) StopAll() {
	m.mu.Lock()
	processes := make([]*exec.Cmd, 0, len(m.Processes))
	for profileID, cmd := range m.Processes {
		if cmd != nil {
			processes = append(processes, cmd)
		}
		delete(m.Processes, profileID)
	}
	bridges := make([]*MihomoNodeBridge, 0, len(m.NodeBridges))
	for key, bridge := range m.NodeBridges {
		if bridge != nil {
			bridge.Running = false
			bridges = append(bridges, bridge)
		}
		delete(m.NodeBridges, key)
	}
	m.mu.Unlock()
	for _, cmd := range processes {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	for _, bridge := range bridges {
		if bridge != nil && bridge.Cmd != nil && bridge.Cmd.Process != nil {
			_ = bridge.Cmd.Process.Kill()
		}
	}
}
