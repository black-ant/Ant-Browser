package launchcode_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"ant-chrome/backend/internal/launchcode"
)

func TestLaunchServerStartWithAutoPort(t *testing.T) {
	svc := launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	srv := launchcode.NewLaunchServer(svc, nil, nil, nil, 0)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer func() {
		_ = srv.Stop()
	}()

	port := srv.Port()
	if port <= 0 {
		t.Fatalf("自动端口分配失败: got=%d", port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
	if err != nil {
		t.Fatalf("健康检查请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("健康检查状态码错误: got=%d", resp.StatusCode)
	}
}

func TestLaunchServerReturnsErrorWhenPreferredPortIsBusy(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占用端口失败: %v", err)
	}
	defer occupied.Close()

	busyPort := occupied.Addr().(*net.TCPAddr).Port
	svc := launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	srv := launchcode.NewLaunchServer(svc, nil, nil, nil, busyPort)

	if err := srv.Start(); err == nil {
		defer func() {
			_ = srv.Stop()
		}()
		t.Fatalf("期望固定端口被占用时返回错误，但启动成功了: %d", busyPort)
	}
}
