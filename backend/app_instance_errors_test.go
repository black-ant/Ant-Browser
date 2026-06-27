package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDescribeChromeProcessStartError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "file not found",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: The system cannot find the file specified."),
			want: "浏览器可执行文件不存在",
		},
		{
			name: "access denied",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: Access is denied."),
			want: "系统拒绝启动浏览器进程",
		},
		{
			name: "invalid win32",
			err:  fmt.Errorf("%%1 is not a valid Win32 application"),
			want: "与系统/架构不兼容",
		},
		{
			name: "linux exec format error",
			err:  fmt.Errorf("fork/exec /opt/chrome/chrome.exe: exec format error"),
			want: "与系统/架构不兼容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeChromeProcessStartError(`C:\chrome.exe`, tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("expected %q to contain %q", got, tt.want)
			}
		})
	}
}

func TestDescribeBrowserReadyTimeout(t *testing.T) {
	got := describeBrowserReadyTimeout(9222, 10*time.Second)
	if !strings.Contains(got, "调试端口 9222 未就绪") {
		t.Fatalf("unexpected timeout message: %q", got)
	}
}

func TestDescribeBrowserReadyTimeoutWithoutPort(t *testing.T) {
	got := describeBrowserReadyTimeout(0, 10*time.Second)
	if !strings.Contains(got, "未获取到调试端口") {
		t.Fatalf("unexpected timeout message: %q", got)
	}
}

func TestDescribeBrowserReadyFailureUsesExitDetail(t *testing.T) {
	err := &browserStartupExitError{
		exitErr:    fmt.Errorf("exit status 5"),
		stderrTail: "sandbox initialization failed",
	}

	got := describeBrowserReadyFailure(`C:\chrome.exe`, 9222, 10*time.Second, err)
	if !strings.Contains(got, "sandbox initialization failed") {
		t.Fatalf("expected exit detail in message, got %q", got)
	}
	if strings.Contains(got, "调试端口 9222 未就绪") {
		t.Fatalf("expected exit detail message instead of timeout, got %q", got)
	}
}

func TestBrowserStartAttemptCountDefault(t *testing.T) {
	if browserStartAttemptCount() != 5 {
		t.Fatalf("expected default browser start attempts to be 5, got %d", browserStartAttemptCount())
	}
}

func TestBrowserDebugPendingMessages(t *testing.T) {
	warning := browserDebugPendingWarning(15 * time.Second)
	if !strings.Contains(warning, "15 秒") || !strings.Contains(warning, "继续在后台连接") {
		t.Fatalf("unexpected pending warning: %q", warning)
	}

	notice := browserDebugPendingStartNotice(15 * time.Second)
	if !strings.Contains(notice, "尚未完成接管") || !strings.Contains(notice, "稍后查看窗口状态") {
		t.Fatalf("unexpected pending start notice: %q", notice)
	}
}

func TestShouldRetryBrowserReadyFailure(t *testing.T) {
	if !shouldRetryBrowserReadyFailure(fmt.Errorf("browser debug port 9222 not ready")) {
		t.Fatal("expected timeout-like ready failure to be retryable")
	}

	if shouldRetryBrowserReadyFailure(&browserStartupExitError{
		exitErr:    fmt.Errorf("exit status 5"),
		stderrTail: "missing libEGL.dll",
	}) {
		t.Fatal("expected process exit before ready to stop retrying")
	}
}
