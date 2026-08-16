package backend

import "ant-chrome/backend/internal/apppath"

// MigrateRuntimeStateRoot 把旧版状态目录(ant-browser)原子迁移到当前名(ZwBrowser)。
// 必须在 EnsureRuntimeLayout(会创建新目录)之前调用。返回 (是否迁移, error)。
func MigrateRuntimeStateRoot(appRoot string) (bool, error) {
	return apppath.MigrateLegacyStateRoot(appRoot)
}

// EnsureRuntimeLayout 为运行时准备已安装应用的用户可写目录。
func EnsureRuntimeLayout(appRoot string) error {
	return apppath.EnsureWritableLayout(appRoot)
}

// ResolveRuntimePath 将相对路径解析到安装目录或用户状态目录。
func ResolveRuntimePath(appRoot, p string) string {
	return apppath.Resolve(appRoot, p)
}

// RuntimeStateRoot 返回当前运行时使用的状态目录。
func RuntimeStateRoot(appRoot string) string {
	return apppath.StateRoot(appRoot)
}

// RuntimeUsesDetachedState 表示当前是否启用了“安装目录只读、状态目录独立”的模式。
func RuntimeUsesDetachedState(appRoot string) bool {
	return apppath.IsDetached(appRoot)
}
