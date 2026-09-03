//go:build windows

package singleinstance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type singleInstanceFileLock struct {
	file *os.File
}

func tryLockSingleInstanceFile(path string) (*singleInstanceFileLock, bool, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &singleInstanceFileLock{file: file}, true, nil
}

func (l *singleInstanceFileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &windows.Overlapped{})
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("unlock single instance file: %w", unlockErr)
	}
	return closeErr
}
