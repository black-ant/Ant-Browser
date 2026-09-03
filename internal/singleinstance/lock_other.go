//go:build !windows

package singleinstance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type singleInstanceFileLock struct {
	file *os.File
}

func tryLockSingleInstanceFile(path string) (*singleInstanceFileLock, bool, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
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
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("unlock single instance file: %w", unlockErr)
	}
	return closeErr
}
