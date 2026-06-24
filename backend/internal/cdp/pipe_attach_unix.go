//go:build !windows

package cdp

import "os"

// NewDebugPipe 创建 --remote-debugging-pipe 所需的两对管道（POSIX）。
//
//	cmd 管道：父进程写 cmdW  → 子进程从 fd 3 读 cmdR
//	resp 管道：子进程往 fd 4 写 respW → 父进程读 respR
//
// 子进程端按顺序挂到 exec.Cmd.ExtraFiles 上：ExtraFiles[i] 即子进程 fd 3+i，
// 因此 [0]=cmdR(fd3 读端)、[1]=respW(fd4 写端)。
func NewDebugPipe() (*DebugPipe, error) {
	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	respR, respW, err := os.Pipe()
	if err != nil {
		_ = cmdR.Close()
		_ = cmdW.Close()
		return nil, err
	}
	return &DebugPipe{
		send:       cmdW,
		recv:       respR,
		childFiles: []*os.File{cmdR, respW},
	}, nil
}
