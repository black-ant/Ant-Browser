//go:build windows

package backend

import (
	"os/exec"
	"syscall"
)

func configureProxyGatewayWorkerProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200,
	}
}
