//go:build !windows

package windowsizing

func getDesktopWorkArea() (DesktopWorkArea, bool) {
	return DesktopWorkArea{}, false
}
