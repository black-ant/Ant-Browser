package backend

import (
	"net"
	"strconv"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func TestRestartLaunchServerRestoresPreviousServerWhenNewPortFails(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}

	previousServer := launchcode.NewLaunchServer(nil, nil, nil, 0)
	if err := previousServer.Start(); err != nil {
		t.Fatalf("previous LaunchServer.Start returned error: %v", err)
	}
	defer previousServer.Stop()
	app.launchServer = previousServer

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port

	if err := app.restartLaunchServer(occupiedPort); err == nil {
		t.Fatal("restartLaunchServer should fail when the requested port is occupied")
	}
	if app.launchServer != previousServer {
		t.Fatal("restartLaunchServer did not restore the previous LaunchServer pointer")
	}
	if previousServer.Port() <= 0 {
		t.Fatal("restored LaunchServer does not have a bound port")
	}

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(previousServer.Port())))
	if err != nil {
		t.Fatalf("restored LaunchServer is not accepting connections: %v", err)
	}
	_ = conn.Close()
}
