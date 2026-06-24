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
	Config    *config.Config
	AppRoot   string // 应用根目录，所有相对路径基于此解析
	Processes map[string]*exec.Cmd
	mu        sync.Mutex
}

// NewClashManager 创建 Clash 管理器
func NewClashManager(cfg *config.Config, appRoot string) *ClashManager {
	return &ClashManager{
		Config:    cfg,
		AppRoot:   appRoot,
		Processes: make(map[string]*exec.Cmd),
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
	profileID := profile.GetProfileId()
	m.mu.Lock()
	m.Processes[profileID] = cmd
	m.mu.Unlock()
	go m.watchProcess(profileID, cmd)
	profile.SetClashRunning(true)
	profile.SetClashPid(cmd.Process.Pid)
	profile.SetClashLastError("")
	log.Info("Clash 启动成功", logger.F("profile_id", profileID), logger.F("pid", cmd.Process.Pid), logger.F("port", port))
	return nil
}

// StopForProfile 停止配置的 Clash 进程
func (m *ClashManager) StopForProfile(profile ClashProfile) {
	log := logger.New("Clash")
	profileID := profile.GetProfileId()
	m.mu.Lock()
	cmd := m.Processes[profileID]
	delete(m.Processes, profileID)
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			log.Error("Clash 停止失败", logger.F("profile_id", profileID), logger.F("error", err))
		}
	}
	profile.SetClashRunning(false)
	profile.SetClashPid(0)
	log.Info("Clash 已停止", logger.F("profile_id", profileID))
}

// StopAll 停止所有 Clash 进程
func (m *ClashManager) StopAll() {
	m.mu.Lock()
	processes := make(map[string]*exec.Cmd, len(m.Processes))
	for profileID, cmd := range m.Processes {
		processes[profileID] = cmd
		delete(m.Processes, profileID)
	}
	m.mu.Unlock()

	for _, cmd := range processes {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

func (m *ClashManager) watchProcess(profileID string, cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	_ = cmd.Wait()

	m.mu.Lock()
	if current := m.Processes[profileID]; current == cmd {
		delete(m.Processes, profileID)
	}
	m.mu.Unlock()
}
