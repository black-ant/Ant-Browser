package backend

import (
	"ant-chrome/backend/internal/backup"
	"fmt"
	"strings"
	"time"
)

func (a *App) backupExportPackageToPath(savePath string) (map[string]interface{}, error) {
	if strings.TrimSpace(savePath) == `` {
		return nil, fmt.Errorf(`backup export path is empty`)
	}
	a.backupEmitExportProgress(`starting`, 0, `checking running profiles`)
	if runningNames := a.backupRunningProfileNames(); len(runningNames) > 0 {
		message := fmt.Sprintf(`stop running profiles before export: %s`, strings.Join(runningNames, `, `))
		a.backupEmitExportProgress(`error`, 100, message)
		return nil, fmt.Errorf(`%s`, message)
	}
	a.backupEmitExportProgress(`preparing`, 8, `collecting backup scope`)
	if err := a.backupCheckpointSQLiteWAL(); err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}

	scope, err := a.backupBuildScope()
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}
	manifest := backup.BuildManifest(scope, a.appName(), a.appVersion(), time.Now())
	a.backupEmitExportProgress(`preparing`, 15, `writing backup package`)

	includedEntries, skippedEntries, fileCount, err := backupWritePackageZip(savePath, scope, manifest, a.backupEmitExportProgressMeta)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}

	return map[string]interface{}{
		`cancelled`:       false,
		`zipPath`:         savePath,
		`includedEntries`: includedEntries,
		`skippedEntries`:  skippedEntries,
		`fileCount`:       fileCount,
		`message`:         `backup export complete`,
	}, nil
}
