//go:build windows
// +build windows

package browser

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func getAvailableDiskSpace(path string) (uint64, error) {
	volume := filepath.VolumeName(path)
	if volume == "" {
		volume = path
	} else {
		volume += string(filepath.Separator)
	}

	ptr, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return 0, err
	}

	var freeBytesAvailable uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailable, nil, nil); err != nil {
		return 0, err
	}
	return freeBytesAvailable, nil
}
