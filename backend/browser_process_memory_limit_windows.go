package backend

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func applyBrowserProcessMemoryLimit(cmd *exec.Cmd, memoryLimitMB int) (func(), error) {
	if memoryLimitMB <= 0 {
		return nil, nil
	}
	if cmd == nil || cmd.Process == nil {
		return nil, fmt.Errorf("浏览器进程不存在")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
	info.JobMemoryLimit = uintptr(memoryLimitMB) * 1024 * 1024
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}

	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(processHandle)

	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}

	return func() { _ = windows.CloseHandle(job) }, nil
}
