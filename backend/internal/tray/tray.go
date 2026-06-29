//go:build windows

package tray

import (
	_ "embed"
	"runtime"

	"github.com/energye/systray"
)

//go:embed icon.ico
var iconData []byte

// Callbacks 托盘回调
type Callbacks struct {
	OnShow        func()
	OnQuitAppOnly func()
	OnQuit        func()
}

// Run 启动系统托盘（阻塞，需在独立 goroutine 中调用）。
// Windows 托盘依赖消息循环，必须固定在同一个 OS 线程上。
func Run(cb Callbacks) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Maka Browser")
		systray.SetTooltip("Maka Browser")

		mShow := systray.AddMenuItem("显示窗口", "显示主窗口")
		systray.AddSeparator()
		mQuitAppOnly := systray.AddMenuItem("仅退出应用", "关闭客户端，保留已打开的浏览器")
		mQuit := systray.AddMenuItem("退出应用与浏览器", "退出应用并关闭当前打开的浏览器")

		systray.SetOnClick(func(menu systray.IMenu) {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		systray.SetOnDClick(func(menu systray.IMenu) {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		systray.SetOnRClick(func(menu systray.IMenu) {
			if menu != nil {
				_ = menu.ShowMenu()
			}
		})

		mShow.Click(func() {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		mQuitAppOnly.Click(func() {
			systray.Quit()
			if cb.OnQuitAppOnly != nil {
				cb.OnQuitAppOnly()
			}
		})

		mQuit.Click(func() {
			systray.Quit()
			if cb.OnQuit != nil {
				cb.OnQuit()
			}
		})
	}, func() {
		// onExit: 托盘退出时什么都不做，由 OnQuit 回调处理
	})
}

// Quit 主动退出托盘循环
func Quit() {
	systray.Quit()
}
