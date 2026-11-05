//go:build !windows
// +build !windows

package browser

import "os/exec"

func hideExtensionInstallerWindow(command *exec.Cmd) {}
