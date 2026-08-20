//go:build !windows

package backend

import (
	"os/exec"
	"syscall"
)

func configureProxyGatewayWorkerProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
