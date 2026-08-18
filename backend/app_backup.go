package backend

import (
	"ant-chrome/backend/internal/backup"
	"time"
)

type BackupScope = backup.Scope
type BackupManifest = backup.Manifest

// BackupGetScopeDefinition 返回当前环境下的备份范围定义（第一阶段：范围与包格式）。
func (a *App) BackupGetScopeDefinition() (BackupScope, error) {
	return backup.BuildScope(backup.BuildOptions{
		AppRoot: a.appStateRootAbs(),
		Config:  a.config,
	})
}

// BackupGetManifestTemplate 返回 manifest 结构预览（不执行实际导出）。
func (a *App) BackupGetManifestTemplate() (BackupManifest, error) {
	scope, err := a.BackupGetScopeDefinition()
	if err != nil {
		return BackupManifest{}, err
	}
	return backup.BuildManifest(scope, a.appName(), a.appVersion(), time.Now()), nil
}
