//go:build !windows

package tray

// Callbacks 托盘回调
type Callbacks struct {
	OnShow        func()
	OnQuitAppOnly func()
	OnQuit        func()
	OnExit        func()
}

// Run 非 Windows 平台无托盘实现，保持空操作。
func Run(cb Callbacks) {}

// Quit 非 Windows 平台无托盘实现，保持空操作。
func Quit() {}
