package backend

import (
	appconfig "ant-chrome/backend/internal/config"
	apptray "ant-chrome/backend/internal/tray"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config = appconfig.Config
type TrayCallbacks = apptray.Callbacks

func LoadConfig(path string) (*Config, error) {
	cfg, err := appconfig.Load(path)
	repairedConfig := false
	if err == nil {
		return cfg, nil
	}

	// 配置文件存在但内容损坏时，自动备份并重建默认配置，避免启动阶段反复报错。
	if data, readErr := os.ReadFile(path); readErr == nil && len(data) > 0 {
		backupPath := fmt.Sprintf("%s.broken-%s", path, time.Now().Format("20060102-150405"))
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			return appconfig.DefaultConfig(), fmt.Errorf("加载配置失败: %w；备份损坏配置失败: %v", err, writeErr)
		}
	}

	defaultCfg := appconfig.DefaultConfig()
	repairedConfig = true
	if repairedConfig {
		if saveErr := os.MkdirAll(filepath.Dir(path), 0755); saveErr != nil {
			return defaultCfg, fmt.Errorf("加载配置失败: %w；创建配置目录失败: %v", err, saveErr)
		}
		if saveErr := defaultCfg.Save(path); saveErr != nil {
			return defaultCfg, fmt.Errorf("加载配置失败: %w；重建默认配置失败: %v", err, saveErr)
		}
	}
	return defaultCfg, nil
}

func DefaultConfig() *Config {
	return appconfig.DefaultConfig()
}

func RunTray(cb TrayCallbacks) {
	apptray.Run(cb)
}

func QuitTray() {
	apptray.Quit()
}
