package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os"
	"testing"
	"time"
)

func TestReconcileProfileRuntimeFromExistingGateway(t *testing.T) {
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	worker, err := newProxyGatewayWorker(t.TempDir(), port, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- worker.run() }()
	t.Cleanup(func() {
		worker.close()
		select {
		case err := <-runDone:
			if err != nil {
				t.Errorf("worker shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop")
		}
	})

	client := newProxyGatewayClient(port, worker.token, "")
	_, err = client.prepare(proxyGatewayProfileRequest{
		ProfileID:   "profile-recovered",
		ProxyID:     "direct-test",
		ProxyConfig: "direct://",
		Proxies: []config.BrowserProxy{{
			ProxyId:     "direct-test",
			ProxyName:   "Direct",
			ProxyConfig: "direct://",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.attach("profile-recovered", os.Getpid(), 0); err != nil {
		t.Fatal(err)
	}

	manager := browser.NewManager(config.DefaultConfig(), t.TempDir())
	manager.InitData()
	manager.Profiles["profile-recovered"] = &BrowserProfile{
		ProfileId:   "profile-recovered",
		ProfileName: "Recovered",
	}
	app := &App{browserMgr: manager, proxyGateway: client}
	app.reconcileProfileRuntimeFromGateway()

	profile := manager.Profiles["profile-recovered"]
	if !profile.Running {
		t.Fatal("profile runtime was not recovered from the existing gateway")
	}
	if profile.Pid != os.Getpid() {
		t.Fatalf("recovered pid = %d, want %d", profile.Pid, os.Getpid())
	}
	statuses := app.existingProxyGatewayStatuses()
	if len(statuses) != 1 || statuses[0].BrowserPID != os.Getpid() {
		t.Fatalf("gateway status did not expose attached browser pid: %+v", statuses)
	}
}
