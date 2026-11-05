package backend

import (
	"ant-chrome/backend/internal/snapshot"
)

// snapshotDir 返回指定实例的快照目录路径（存放在 data/snapshots 下）
func (a *App) snapshotDir(profileId string) (string, error) {
	return snapshot.EnsureDir(a.resolveAppPath("data"), profileId)
}
