package backend

import (
	"ant-chrome/backend/internal/gateway"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type proxyGatewayClient struct {
	baseURL    string
	token      string
	launchPath string
	httpClient *http.Client
}

func (a *App) ensureProxyGatewayClient() (*proxyGatewayClient, error) {
	if a == nil {
		return nil, fmt.Errorf("app is unavailable")
	}
	a.proxyGatewayMu.Lock()
	defer a.proxyGatewayMu.Unlock()
	if a.proxyGateway != nil && a.proxyGateway.health() == nil {
		return a.proxyGateway, nil
	}
	launchPath := a.resolveAppPath(filepath.Join("data", "_gateway", "worker.json"))
	if client := loadProxyGatewayClient(launchPath); client != nil && client.health() == nil {
		a.proxyGateway = client
		return client, nil
	}
	client, err := a.startProxyGatewayWorker(launchPath)
	if err != nil {
		return nil, err
	}
	a.proxyGateway = client
	return client, nil
}

func loadProxyGatewayClient(launchPath string) *proxyGatewayClient {
	data, err := os.ReadFile(launchPath)
	if err != nil {
		return nil
	}
	var launch proxyGatewayWorkerLaunchConfig
	if json.Unmarshal(data, &launch) != nil || launch.ControlPort <= 0 || strings.TrimSpace(launch.Token) == "" {
		return nil
	}
	return newProxyGatewayClient(launch.ControlPort, launch.Token, launchPath)
}

func (a *App) startProxyGatewayWorker(launchPath string) (*proxyGatewayClient, error) {
	if err := os.MkdirAll(filepath.Dir(launchPath), 0o700); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		port, err := nextAvailablePort()
		if err != nil {
			return nil, err
		}
		token, err := newProxyGatewayToken()
		if err != nil {
			return nil, err
		}
		launch := proxyGatewayWorkerLaunchConfig{AppRoot: a.appRoot, ControlPort: port, Token: token}
		data, err := json.MarshalIndent(launch, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(launchPath, data, 0o600); err != nil {
			return nil, err
		}
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cmd := exec.Command(executable, proxyGatewayWorkerArg, launchPath)
		configureProxyGatewayWorkerProcess(cmd)
		logPath := filepath.Join(filepath.Dir(launchPath), "worker.log")
		logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if logFile != nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
		if err := cmd.Start(); err != nil {
			if logFile != nil {
				_ = logFile.Close()
			}
			continue
		}
		if logFile != nil {
			_ = logFile.Close()
		}
		_ = os.WriteFile(filepath.Join(filepath.Dir(launchPath), "worker.pid"), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o600)
		client := newProxyGatewayClient(port, token, launchPath)
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if client.health() == nil {
				go func() { _ = cmd.Wait() }()
				return client, nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return nil, fmt.Errorf("proxy gateway worker failed to start")
}

func newProxyGatewayClient(port int, token string, launchPath string) *proxyGatewayClient {
	return &proxyGatewayClient{
		baseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		token:      token,
		launchPath: launchPath,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func newProxyGatewayToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func (c *proxyGatewayClient) health() error {
	_, err := c.healthStatus()
	return err
}

func (c *proxyGatewayClient) healthStatus() ([]gateway.Status, error) {
	var response proxyGatewayStatusResponse
	err := c.call(http.MethodGet, "/v1/health", nil, &response)
	return response.Profiles, err
}

func (c *proxyGatewayClient) prepare(input proxyGatewayProfileRequest) (gateway.Status, error) {
	var response proxyGatewayProfileResponse
	err := c.call(http.MethodPost, "/v1/profiles/prepare", input, &response)
	return response.Status, err
}

func (c *proxyGatewayClient) updateRouting(input proxyGatewayProfileRequest) (gateway.Status, error) {
	var response proxyGatewayProfileResponse
	err := c.call(http.MethodPost, "/v1/profiles/routing", input, &response)
	return response.Status, err
}

func (c *proxyGatewayClient) attach(profileID string, pid int, debugPort int) error {
	var response proxyGatewayProfileResponse
	return c.call(http.MethodPost, "/v1/profiles/attach", proxyGatewayProfileRequest{ProfileID: profileID, BrowserPID: pid, BrowserDebugPort: debugPort}, &response)
}

func (c *proxyGatewayClient) stopProfile(profileID string) error {
	var response proxyGatewayProfileResponse
	return c.call(http.MethodPost, "/v1/profiles/stop", proxyGatewayProfileRequest{ProfileID: profileID}, &response)
}

func (c *proxyGatewayClient) status(profileID string) (gateway.Status, error) {
	var response proxyGatewayProfileResponse
	err := c.call(http.MethodGet, "/v1/profiles/status?profileId="+profileID, nil, &response)
	return response.Status, err
}

func (c *proxyGatewayClient) shutdown() error {
	return c.call(http.MethodPost, "/v1/shutdown", map[string]bool{"shutdown": true}, nil)
}

func (c *proxyGatewayClient) call(method string, path string, input interface{}, output interface{}) error {
	if c == nil {
		return fmt.Errorf("proxy gateway client is unavailable")
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("%s", message)
	}
	if output == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func (a *App) stopProfileGateway(profileID string) {
	a.proxyGatewayMu.Lock()
	client := a.proxyGateway
	if client == nil {
		client = loadProxyGatewayClient(a.resolveAppPath(filepath.Join("data", "_gateway", "worker.json")))
	}
	a.proxyGatewayMu.Unlock()
	if client != nil {
		_ = client.stopProfile(profileID)
	}
}

func (a *App) existingProxyGatewayStatuses() []gateway.Status {
	if a == nil {
		return nil
	}
	a.proxyGatewayMu.Lock()
	client := a.proxyGateway
	if client == nil {
		client = loadProxyGatewayClient(a.resolveAppPath(filepath.Join("data", "_gateway", "worker.json")))
		if client != nil {
			a.proxyGateway = client
		}
	}
	a.proxyGatewayMu.Unlock()
	if client == nil {
		return nil
	}
	statuses, err := client.healthStatus()
	if err != nil {
		return nil
	}
	return statuses
}

func (a *App) shutdownProxyGatewayWorker() {
	a.proxyGatewayMu.Lock()
	client := a.proxyGateway
	if client == nil {
		client = loadProxyGatewayClient(a.resolveAppPath(filepath.Join("data", "_gateway", "worker.json")))
	}
	a.proxyGateway = nil
	a.proxyGatewayMu.Unlock()
	if client != nil {
		_ = client.shutdown()
	}
}
