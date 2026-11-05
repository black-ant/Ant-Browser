//go:build windows
// +build windows

package automation

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

func prepareTaskCommand(cmd *exec.Cmd) {
	hideWindow(cmd)
}
