package backend

import (
	"errors"
	"testing"
	"time"
)

func TestTerminateBrowserUserDataOrphansKillsPortlessMain(t *testing.T) {
	oldFind := findBrowserUserDataProcesses
	oldTerminate := terminateBrowserUserDataProcess
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{
			{PID: 100, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data" --type=renderer`},
			{PID: 101, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data"`},
			{PID: 102, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data" --type=gpu-process`},
		}, nil
	}
	terminatedPIDs := make([]int, 0)
	terminateBrowserUserDataProcess = func(pid int, _ time.Duration) error {
		terminatedPIDs = append(terminatedPIDs, pid)
		return nil
	}
	t.Cleanup(func() {
		findBrowserUserDataProcesses = oldFind
		terminateBrowserUserDataProcess = oldTerminate
	})

	terminated, err := terminateBrowserUserDataOrphans("C:\\data", 5*time.Second)
	if err != nil {
		t.Fatalf("terminateBrowserUserDataOrphans returned error: %v", err)
	}
	if !terminated {
		t.Fatal("terminated = false, want true for portless main browser")
	}
	if len(terminatedPIDs) != 1 || terminatedPIDs[0] != 101 {
		t.Fatalf("terminated PIDs = %v, want [101]", terminatedPIDs)
	}
}

func TestTerminateBrowserUserDataOrphansSkipsManagedBrowser(t *testing.T) {
	oldFind := findBrowserUserDataProcesses
	oldTerminate := terminateBrowserUserDataProcess
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{
			{PID: 101, DebugPort: 9222, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data" --remote-debugging-port=9222`},
			{PID: 102, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data" --type=gpu-process`},
		}, nil
	}
	terminated := false
	terminateBrowserUserDataProcess = func(pid int, _ time.Duration) error {
		terminated = true
		return nil
	}
	t.Cleanup(func() {
		findBrowserUserDataProcesses = oldFind
		terminateBrowserUserDataProcess = oldTerminate
	})

	got, err := terminateBrowserUserDataOrphans("C:\\data", 5*time.Second)
	if err != nil {
		t.Fatalf("terminateBrowserUserDataOrphans returned error: %v", err)
	}
	if got {
		t.Fatal("terminated = true, want false for managed browser")
	}
	if terminated {
		t.Fatal("terminateBrowserUserDataProcess called for managed browser")
	}
}

func TestTerminateBrowserUserDataOrphansPropagatesTerminateError(t *testing.T) {
	oldFind := findBrowserUserDataProcesses
	oldTerminate := terminateBrowserUserDataProcess
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{
			{PID: 101, CommandLine: `"C:\chrome.exe" --user-data-dir="C:\data"`},
		}, nil
	}
	terminateBrowserUserDataProcess = func(pid int, _ time.Duration) error {
		return errors.New("kill failed")
	}
	t.Cleanup(func() {
		findBrowserUserDataProcesses = oldFind
		terminateBrowserUserDataProcess = oldTerminate
	})

	if _, err := terminateBrowserUserDataOrphans("C:\\data", 5*time.Second); err == nil {
		t.Fatal("expected error from terminateBrowserUserDataProcess")
	}
}
