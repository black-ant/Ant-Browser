package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"errors"
	"fmt"
)

func (a *App) ResetManagedSettings() error {
	if a == nil {
		return fmt.Errorf("应用未初始化")
	}
	if a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	return a.resetManagedSettings()
}

func (a *App) resetManagedSettings() error {
	scheduler := a.backupScheduler
	if scheduler != nil {
		if err := scheduler.beginReset(); err != nil {
			return err
		}
		defer scheduler.endReset()
	}

	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	defaults := config.DefaultConfig()
	previous := *a.config
	previousServerPort := 0
	if a.launchServer != nil {
		previousServerPort = a.launchServer.Port()
	}
	targetServerPort := defaults.LaunchServer.Port
	if a.launchServer != nil && previousServerPort != targetServerPort {
		if err := ensureLaunchServerPortAvailable(targetServerPort); err != nil {
			return fmt.Errorf("重置 LaunchServer 端口失败：%w", err)
		}
	}

	next := previous
	next.Automation = defaults.Automation
	next.Backup = defaults.Backup
	next.LaunchServer = defaults.LaunchServer
	*a.config = next

	configPath := a.resolveAppPath("config.yaml")
	if err := a.config.Save(configPath); err != nil {
		*a.config = previous
		return fmt.Errorf("保存重置后的配置失败：%w", err)
	}

	if a.launchServer != nil {
		if previousServerPort != targetServerPort {
			if err := a.restartLaunchServer(targetServerPort); err != nil {
				*a.config = previous
				if rollbackErr := a.config.Save(configPath); rollbackErr != nil {
					return fmt.Errorf("重置 LaunchServer 失败：%w；恢复配置失败：%v", err, rollbackErr)
				}
				return fmt.Errorf("重置 LaunchServer 失败：%w", err)
			}
		} else {
			a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
				Enabled: defaults.LaunchServer.Auth.Enabled,
				APIKey:  defaults.LaunchServer.Auth.APIKey,
				Header:  defaults.LaunchServer.Auth.Header,
			})
		}
	}

	if err := saveBackupLocalConfig(a.resolveAppPath(backupLocalConfigFileName), defaults.Backup); err != nil {
		rollbackErr := a.rollbackManagedSettings(previous, previousServerPort)
		if rollbackErr != nil {
			return fmt.Errorf("删除本地备份凭据失败：%w；恢复设置失败：%v", err, rollbackErr)
		}
		return fmt.Errorf("删除本地备份凭据失败：%w", err)
	}

	if a.automationMgr != nil {
		a.automationMgr.StopAllTasks()
		a.automationMgr.SetConfig(a.config)
	}
	if scheduler != nil {
		scheduler.applyResetSettings(defaults.Backup)
	}
	return nil
}

func (s *backupScheduler) beginReset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("定时备份正在执行，请稍后再试")
	}
	s.resetting = true
	return nil
}

func (s *backupScheduler) endReset() {
	s.mu.Lock()
	s.resetting = false
	s.mu.Unlock()
}

func (s *backupScheduler) applyResetSettings(settings config.BackupConfig) {
	s.mu.Lock()
	s.settings = normalizeBackupSettings(settings)
	s.configurationError = ""
	s.state = backupScheduleState{Status: backupScheduleStatusNever}
	s.lastDate = ""
	s.mu.Unlock()
}

func (a *App) rollbackManagedSettings(previous config.Config, previousServerPort int) error {
	*a.config = previous
	rollbackErrors := make([]error, 0, 2)

	if a.launchServer != nil {
		currentServerPort := a.launchServer.Port()
		if currentServerPort != previousServerPort {
			if err := a.restartLaunchServer(previousServerPort); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复 LaunchServer 失败：%w", err))
			}
		} else {
			a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
				Enabled: previous.LaunchServer.Auth.Enabled,
				APIKey:  previous.LaunchServer.Auth.APIKey,
				Header:  previous.LaunchServer.Auth.Header,
			})
		}
	}

	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复配置文件失败：%w", err))
	}
	return errors.Join(rollbackErrors...)
}
