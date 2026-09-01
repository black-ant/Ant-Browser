package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
)

func (a *App) ensureBackupScheduler() (*backupScheduler, error) {
	if a == nil {
		return nil, fmt.Errorf("应用未初始化")
	}
	if a.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}
	if a.backupScheduler == nil {
		a.startupInitBackupScheduler()
	}
	if a.backupScheduler == nil {
		return nil, fmt.Errorf("备份服务未初始化")
	}
	return a.backupScheduler, nil
}

func (a *App) BackupOpenListGetSettings() (map[string]interface{}, error) {
	scheduler, err := a.ensureBackupScheduler()
	if err != nil {
		return nil, err
	}
	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	return backupOpenListSettingsResult(scheduler.settings.Channels.OpenList), nil
}

func (a *App) BackupOpenListRevealToken() (string, error) {
	scheduler, err := a.ensureBackupScheduler()
	if err != nil {
		return ``, err
	}

	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	return scheduler.settings.Channels.OpenList.Token, nil
}

func (a *App) BackupOpenListSaveSettings(input map[string]string) (map[string]interface{}, error) {
	scheduler, err := a.ensureBackupScheduler()
	if err != nil {
		return nil, err
	}
	return scheduler.saveOpenList(input)
}

func backupOpenListSettingsResult(settings config.OpenListChannelConfig) map[string]interface{} {
	return map[string]interface{}{
		"baseURL":             settings.BaseURL,
		"remotePath":          settings.RemotePath,
		"tokenConfigured":     settings.Token != "",
		"uploadRateLimitMBps": settings.UploadRateLimitMBps,
	}
}
