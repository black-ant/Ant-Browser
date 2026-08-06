//go:build windows
// +build windows

package browser

import (
	"os/exec"
	"syscall"
)

const extensionCreateNoWindow = 0x08000000

func hideExtensionInstallerWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: extensionCreateNoWindow}
}
