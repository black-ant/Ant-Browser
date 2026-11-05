//go:build windows

package windowsizing

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const spiGetWorkArea = 0x0030

const defaultWindowsDPI = 96

type windowsRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func getDesktopWorkArea() (DesktopWorkArea, bool) {
	var workArea windowsRect
	result, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW").Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&workArea)),
		0,
	)
	if result == 0 {
		return DesktopWorkArea{}, false
	}

	width := windowsPhysicalToLogicalPixels(int(workArea.Right - workArea.Left))
	height := windowsPhysicalToLogicalPixels(int(workArea.Bottom - workArea.Top))
	if width <= 0 || height <= 0 {
		return DesktopWorkArea{}, false
	}

	return DesktopWorkArea{Width: width, Height: height}, true
}

func windowsPhysicalToLogicalPixels(value int) int {
	if value <= 0 {
		return 0
	}
	dpi := getWindowsSystemDPI()
	if dpi <= 0 {
		return value
	}
	return value * defaultWindowsDPI / dpi
}

func getWindowsSystemDPI() int {
	getDPIForSystem := windows.NewLazySystemDLL("user32.dll").NewProc("GetDpiForSystem")
	if err := getDPIForSystem.Find(); err == nil {
		result, _, _ := getDPIForSystem.Call()
		if result > 0 {
			return int(result)
		}
	}

	user32 := windows.NewLazySystemDLL("user32.dll")
	getDC := user32.NewProc("GetDC")
	releaseDC := user32.NewProc("ReleaseDC")
	gdi32 := windows.NewLazySystemDLL("gdi32.dll")
	getDeviceCaps := gdi32.NewProc("GetDeviceCaps")
	if getDC.Find() != nil || releaseDC.Find() != nil || getDeviceCaps.Find() != nil {
		return defaultWindowsDPI
	}

	deviceContext, _, _ := getDC.Call(0)
	if deviceContext == 0 {
		return defaultWindowsDPI
	}
	defer releaseDC.Call(0, deviceContext)

	const logPixelsX = 88
	result, _, _ := getDeviceCaps.Call(deviceContext, uintptr(logPixelsX))
	if result <= 0 {
		return defaultWindowsDPI
	}
	return int(result)
}
