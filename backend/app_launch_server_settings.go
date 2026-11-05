package backend

import (
	"fmt"
	"net"
	"strconv"

	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
)

func (a *App) SaveLaunchServerSettings(port int) (map[string]interface{}, error) {
	if a.config == nil {
		return nil, fmt.Errorf("launch server config is not initialized")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("端口必须在 1-65535 之间")
	}

	currentPort := 0
	if a.launchServer != nil {
		currentPort = a.launchServer.Port()
	}
	if currentPort != port {
		if err := ensureLaunchServerPortAvailable(port); err != nil {
			return nil, err
		}
	}

	previousPort := a.config.LaunchServer.Port
	a.config.LaunchServer.Port = port
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.LaunchServer.Port = previousPort
		return nil, err
	}

	if currentPort != port {
		if err := a.restartLaunchServer(port); err != nil {
			a.config.LaunchServer.Port = previousPort
			_ = a.config.Save(a.resolveAppPath("config.yaml"))
			return nil, err
		}
	}

	return a.GetLaunchServerInfo(), nil
}

func ensureLaunchServerPortAvailable(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("端口 %d 不可用: %w", port, err)
	}
	return listener.Close()
}

func (a *App) restartLaunchServer(port int) error {
	log := logger.New("LaunchServer")
	previousServer := a.launchServer
	if previousServer != nil {
		if err := previousServer.Stop(); err != nil {
			return fmt.Errorf("停止 LaunchServer 失败: %w", err)
		}
	}

	server := launchcode.NewLaunchServer(a.launchCodeSvc, a, a.browserMgr, port)
	server.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: a.config.LaunchServer.Auth.Enabled,
		APIKey:  a.config.LaunchServer.Auth.APIKey,
		Header:  a.config.LaunchServer.Auth.Header,
	})
	if err := server.Start(); err != nil {
		if previousServer != nil {
			if restoreErr := previousServer.Start(); restoreErr != nil {
				a.launchServer = nil
				return fmt.Errorf("启动 LaunchServer 失败: %w；恢复旧 LaunchServer 失败: %v", err, restoreErr)
			}
			a.launchServer = previousServer
		}
		return fmt.Errorf("启动 LaunchServer 失败: %w", err)
	}
	a.launchServer = server
	log.Info("LaunchServer 端口已切换", logger.F("port", server.Port()))
	return nil
}
