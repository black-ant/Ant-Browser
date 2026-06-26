package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/release"
)

func TestGetDesktopEnvironmentStatusCurrentPointerMissing(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimeManifestFixture(t, root)
	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusCurrentPointerInvalid(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":`), 0o600); err != nil {
		t.Fatalf("write invalid active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-INVALID" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusCurrentPointerMissingFields(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":""}`), 0o600); err != nil {
		t.Fatalf("write invalid active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-INVALID" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusPassesWhenPointerAndCoreAreHealthy(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	coreDir := filepath.Join(versionDir, "core")
	writeCoreFixture(t, coreDir)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestGetDesktopEnvironmentStatusPassesWhenBrowserCoreIsMissingAtStartup(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestGetDesktopEnvironmentStatusBlocksWhenWorkspaceHostIsUnavailable(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	coreDir := filepath.Join(versionDir, "core")
	writeCoreFixture(t, coreDir)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed test port: %v", err)
	}
	serverOrigin := "http://" + listener.Addr().String()
	_ = listener.Close()

	app := newRuntimeStatusTestApp(t, root)
	app.config.Workspace.ServerOrigin = serverOrigin

	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateBlocked {
		t.Fatalf("expected blocked state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-WORKSPACE-HOST-APP-CONFIG-UNREACHABLE" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
	if result.Items[0].Repairable {
		t.Fatalf("workspace host unreachable must not be auto repairable: %#v", result.Items)
	}
	if !strings.Contains(result.Items[0].Message, serverOrigin) {
		t.Fatalf("expected workspace host error to mention server origin %s, got %q", serverOrigin, result.Items[0].Message)
	}
	if !strings.Contains(result.Items[0].RecommendedAction, serverOrigin) {
		t.Fatalf("expected recommended action to mention server origin %s, got %q", serverOrigin, result.Items[0].RecommendedAction)
	}
	if !strings.Contains(result.Items[0].RecommendedAction, "config.yaml") {
		t.Fatalf("expected recommended action to mention config source, got %q", result.Items[0].RecommendedAction)
	}
	if got := result.Items[0].Details["source"]; got != "config.yaml" {
		t.Fatalf("expected details.source=config.yaml, got %q", got)
	}
}

func TestGetDesktopEnvironmentStatusBlocksWhenPackagedWorkspaceAgentPayloadIsMissing(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	writeCoreFixture(t, filepath.Join(versionDir, "core"))
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ant-chrome.exe"), []byte("stub"), 0o755); err != nil {
		t.Fatalf("write packaged app stub: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateBlocked {
		t.Fatalf("expected blocked state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-WORKSPACE-AGENT-PAYLOAD-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusClassifiesWorkspaceHostFailureFromEnv(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	writeCoreFixture(t, filepath.Join(versionDir, "core"))
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed test port: %v", err)
	}
	serverOrigin := "http://" + listener.Addr().String()
	_ = listener.Close()

	t.Setenv("DESKTOP_SERVER_BASE_URL", serverOrigin)
	app := newRuntimeStatusTestApp(t, root)
	app.config.Workspace.ServerOrigin = ""

	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateBlocked {
		t.Fatalf("expected blocked state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-WORKSPACE-HOST-ENV-UNREACHABLE" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
	if !strings.Contains(result.Items[0].RecommendedAction, "DESKTOP_SERVER_BASE_URL") {
		t.Fatalf("expected env-specific recommended action, got %q", result.Items[0].RecommendedAction)
	}
	if got := result.Items[0].Details["source"]; got != "env:DESKTOP_SERVER_BASE_URL" {
		t.Fatalf("expected env source detail, got %q", got)
	}
}

func TestGetDesktopEnvironmentStatusClassifiesWorkspaceHostFailureFromRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	writeCoreFixture(t, filepath.Join(versionDir, "core"))
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(nil)
	t.Setenv("ProgramData", filepath.Join(root, "program-data"))
	runtimeDir = resolveWorkspaceRuntimeDirWithConfig(nil)
	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir runtime config: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed test port: %v", err)
	}
	serverOrigin := "http://" + listener.Addr().String()
	_ = listener.Close()

	payload, err := json.Marshal(workspaceServerConnectionConfig{ServerOrigin: serverOrigin})
	if err != nil {
		t.Fatalf("marshal runtime server config: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("write runtime server config: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	app.config.Workspace.RuntimeDir = runtimeDir
	app.config.Workspace.ServerOrigin = ""

	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateBlocked {
		t.Fatalf("expected blocked state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-WORKSPACE-HOST-RUNTIME-CONFIG-UNREACHABLE" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
	if !strings.Contains(result.Items[0].RecommendedAction, "server-connection.json") {
		t.Fatalf("expected runtime-config specific action, got %q", result.Items[0].RecommendedAction)
	}
	if got := result.Items[0].Details["source"]; got != "runtime-config" {
		t.Fatalf("expected runtime-config source detail, got %q", got)
	}
	if got := result.Items[0].Details["configPath"]; filepath.Clean(got) != filepath.Clean(configPath) {
		t.Fatalf("expected configPath detail=%s, got %q", configPath, got)
	}
}

func TestGetDesktopEnvironmentStatusBlocksWhenRuntimeRootIsNotWritable(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.RemoveAll(layout.RuntimeRoot()); err != nil {
		t.Fatalf("remove runtime root: %v", err)
	}
	if err := os.WriteFile(layout.RuntimeRoot(), []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write blocking runtime root file: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateBlocked {
		t.Fatalf("expected blocked state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-ROOT-UNWRITABLE" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
	if strings.TrimSpace(result.Items[0].RecommendedAction) == "" {
		t.Fatalf("expected recommended action, got %#v", result.Items[0])
	}
	if got := result.Items[0].Details["runtimeRoot"]; filepath.Clean(got) != filepath.Clean(layout.RuntimeRoot()) {
		t.Fatalf("expected runtimeRoot detail=%s, got %q", layout.RuntimeRoot(), got)
	}
}

func TestGetDesktopEnvironmentStatusWarnsWhenDiagnosticsRootIsNotWritable(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	coreDir := filepath.Join(versionDir, "core")
	writeCoreFixture(t, coreDir)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}
	if err := os.WriteFile(layout.DiagnosticsRoot(), []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write blocking diagnostics file: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-DIAGNOSTICS-ROOT-UNWRITABLE" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
	if result.Items[0].Severity != "warning" {
		t.Fatalf("expected warning severity, got %#v", result.Items[0])
	}
	if strings.TrimSpace(result.Items[0].RecommendedAction) == "" {
		t.Fatalf("expected recommended action, got %#v", result.Items[0])
	}
	if got := result.Items[0].Details["diagnosticsRoot"]; filepath.Clean(got) != filepath.Clean(layout.DiagnosticsRoot()) {
		t.Fatalf("expected diagnosticsRoot detail=%s, got %q", layout.DiagnosticsRoot(), got)
	}
}

func TestGetDesktopEnvironmentStatusFallsBackToLegacyWorkspaceHealthEndpoint(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	coreDir := filepath.Join(versionDir, "core")
	writeCoreFixture(t, coreDir)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	legacyWorkspaceHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/client/health":
			http.NotFound(w, r)
		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"status": "ok",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer legacyWorkspaceHost.Close()

	app := newRuntimeStatusTestApp(t, root)
	app.config.Workspace.ServerOrigin = legacyWorkspaceHost.URL

	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state with legacy workspace health endpoint, got %s with items %#v", result.State, result.Items)
	}
}

func TestRepairDesktopEnvironmentRepairsMissingPointer(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	writeCoreFixture(t, filepath.Join(versionDir, "core"))

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.RepairDesktopEnvironment()
	if err != nil {
		t.Fatalf("RepairDesktopEnvironment returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state after repair, got %s with items %#v", result.State, result.Items)
	}

	data, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read active runtime pointer: %v", err)
	}
	expected := `{"version":"2026.05.12","resourceVersion":"2026.05.12"}`
	if string(data) != expected {
		t.Fatalf("unexpected active runtime pointer content: %s", string(data))
	}
}

func TestRepairDesktopEnvironmentRepairsMissingPointerByMaterializingRuntimePackage(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixture(t, root)

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.RepairDesktopEnvironment()
	if err != nil {
		t.Fatalf("RepairDesktopEnvironment returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state after repair, got %s with items %#v", result.State, result.Items)
	}

	newVersionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("new version dir: %v", err)
	}
	runtimeFile := filepath.Join(newVersionDir, "bin", "test-runtime")
	if _, err := os.Stat(runtimeFile); err != nil {
		t.Fatalf("expected runtime package to be materialized, stat error: %v", err)
	}

	pointerData, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read repaired active runtime pointer: %v", err)
	}
	expectedPointer := `{"version":"2026.05.12","resourceVersion":"2026.05.12"}`
	if string(pointerData) != expectedPointer {
		t.Fatalf("unexpected active runtime pointer content after repair: %s", string(pointerData))
	}
}

func TestRepairDesktopEnvironmentFallsBackToWorkspaceRuntimeSource(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixtureWithoutPublishSource(t, root)

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.RepairDesktopEnvironment()
	if err != nil {
		t.Fatalf("RepairDesktopEnvironment returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state after repair, got %s with items %#v", result.State, result.Items)
	}

	newVersionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("new version dir: %v", err)
	}
	runtimeFile := filepath.Join(newVersionDir, "bin", "test-runtime")
	if _, err := os.Stat(runtimeFile); err != nil {
		t.Fatalf("expected runtime package to be materialized from workspace source, stat error: %v", err)
	}
}

func TestRepairDesktopEnvironmentRepairsOutdatedRuntimePackage(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixture(t, root)
	oldVersionDir, err := layout.VersionDir("2026.05.01")
	if err != nil {
		t.Fatalf("old version dir: %v", err)
	}
	if err := os.MkdirAll(oldVersionDir, 0o755); err != nil {
		t.Fatalf("mkdir old version dir: %v", err)
	}
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.01","resourceVersion":"2026.05.01"}`), 0o600); err != nil {
		t.Fatalf("write stale active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	before, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus before repair returned error: %v", err)
	}
	if before.State != release.StateRepairable {
		t.Fatalf("expected repairable state before repair, got %s with items %#v", before.State, before.Items)
	}
	if len(before.Items) == 0 || before.Items[0].Code != "PKG-RESOURCE-OUTDATED" {
		t.Fatalf("expected PKG-RESOURCE-OUTDATED before repair, got %#v", before.Items)
	}

	after, err := app.RepairDesktopEnvironment()
	if err != nil {
		t.Fatalf("RepairDesktopEnvironment returned error: %v", err)
	}
	if after.State != release.StatePass {
		t.Fatalf("expected pass state after repair, got %s with items %#v", after.State, after.Items)
	}

	newVersionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("new version dir: %v", err)
	}
	runtimeFile := filepath.Join(newVersionDir, "bin", "test-runtime")
	data, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatalf("read repaired runtime file: %v", err)
	}
	if string(data) != "runtime-binary" {
		t.Fatalf("unexpected repaired runtime file content: %q", string(data))
	}

	pointerData, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read repaired active runtime pointer: %v", err)
	}
	expectedPointer := `{"version":"2026.05.12","resourceVersion":"2026.05.12"}`
	if string(pointerData) != expectedPointer {
		t.Fatalf("unexpected active runtime pointer content after repair: %s", string(pointerData))
	}
}

func TestCheckDesktopReleaseUpdateReturnsRequired(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	app := newRuntimeStatusTestApp(t, root)
	app.releaseManagerFn = func() (*releaseRuntimeManager, error) {
		return &releaseRuntimeManager{
			app: app,
			remoteManifestProvider: func(context.Context) (release.Manifest, error) {
				return release.Manifest{
					SchemaVersion:          2,
					AppVersion:             "1.2.0",
					MinimumResourceVersion: "2026.06.01",
					Packages: []release.RuntimePackage{
						{ID: "runtime-bin", Target: release.DefaultTarget(), Kind: "runtime-binary", Required: true, Version: "1.0.0", Path: "bin/test-runtime"},
					},
				}, nil
			},
		}, nil
	}

	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "required" {
		t.Fatalf("expected required update, got %#v", state)
	}
}

func TestCheckDesktopReleaseUpdateUsesEnvManifestSource(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	remoteManifestPath := writeRemoteUpdateManifestFixture(t, root, "1.1.1", "2026.05.12")
	t.Setenv("DESKTOP_UPDATE_MANIFEST_URL", remoteManifestPath)

	app := newRuntimeStatusTestApp(t, root)
	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "soft" {
		t.Fatalf("expected soft update, got %#v", state)
	}
	if state.ManifestSource != "env:DESKTOP_UPDATE_MANIFEST_URL" {
		t.Fatalf("expected env manifest source, got %#v", state)
	}
	if filepath.Clean(state.ManifestURL) != filepath.Clean(remoteManifestPath) {
		t.Fatalf("expected manifest path %s, got %s", remoteManifestPath, state.ManifestURL)
	}
}

func TestCheckDesktopReleaseUpdateUsesRuntimeConfigManifestSource(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	remoteManifestPath := writeRemoteUpdateManifestFixture(t, root, "1.2.0", "2026.06.01")

	t.Setenv("ProgramData", filepath.Join(root, "program-data"))
	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(nil)
	configPath := filepath.Join(runtimeDir, "config", "release-update.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir runtime config dir: %v", err)
	}
	payload, err := json.Marshal(releaseUpdateConfig{ManifestURL: remoteManifestPath})
	if err != nil {
		t.Fatalf("marshal release update config: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("write release update config: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	app.config.Workspace.RuntimeDir = runtimeDir
	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "required" {
		t.Fatalf("expected required update, got %#v", state)
	}
	if state.ManifestSource != "runtime-config" {
		t.Fatalf("expected runtime-config source, got %#v", state)
	}
	if filepath.Clean(state.ManifestURL) != filepath.Clean(remoteManifestPath) {
		t.Fatalf("expected manifest path %s, got %s", remoteManifestPath, state.ManifestURL)
	}
}

func TestCheckDesktopReleaseUpdateUsesConfigManifestSource(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	remoteManifestPath := writeRemoteUpdateManifestFixture(t, root, "1.1.1", "2026.05.12")

	app := newRuntimeStatusTestApp(t, root)
	app.config.Release.UpdateManifestURL = remoteManifestPath

	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "soft" {
		t.Fatalf("expected soft update, got %#v", state)
	}
	if state.ManifestSource != "config.yaml" {
		t.Fatalf("expected config manifest source, got %#v", state)
	}
	if filepath.Clean(state.ManifestURL) != filepath.Clean(remoteManifestPath) {
		t.Fatalf("expected manifest path %s, got %s", remoteManifestPath, state.ManifestURL)
	}
}

func TestCheckDesktopReleaseUpdateUsesHTTPManifestSource(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"schemaVersion": 2,
			"appVersion": "1.1.1",
			"minimumResourceVersion": "2026.05.12",
			"packages": [
				{"id":"runtime-bin","target":"` + release.DefaultTarget() + `","kind":"runtime-binary","required":true,"version":"1.0.0","path":"bin/test-runtime","sha256":"ignored"}
			]
		}`))
	}))
	defer server.Close()
	t.Setenv("DESKTOP_UPDATE_MANIFEST_URL", server.URL)

	app := newRuntimeStatusTestApp(t, root)
	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "soft" {
		t.Fatalf("expected soft update, got %#v", state)
	}
	if state.ManifestSource != "env:DESKTOP_UPDATE_MANIFEST_URL" {
		t.Fatalf("expected env manifest source, got %#v", state)
	}
	if state.ManifestURL != server.URL {
		t.Fatalf("expected manifest url %s, got %s", server.URL, state.ManifestURL)
	}
}

func TestCheckDesktopReleaseUpdateWrapsManifestLoadError(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	t.Setenv("DESKTOP_UPDATE_MANIFEST_URL", filepath.Join(root, "missing-remote-manifest.json"))

	app := newRuntimeStatusTestApp(t, root)
	_, err := app.CheckDesktopReleaseUpdate()
	if err == nil {
		t.Fatal("expected CheckDesktopReleaseUpdate to fail")
	}
	if !strings.Contains(err.Error(), "load update manifest from env:DESKTOP_UPDATE_MANIFEST_URL") {
		t.Fatalf("expected wrapped manifest load error, got %v", err)
	}
}

func TestCheckDesktopReleaseUpdateReturnsNoneWithoutRemoteSource(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimePackageManifestFixture(t, root)
	app := newRuntimeStatusTestApp(t, root)

	state, err := app.CheckDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("CheckDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "none" {
		t.Fatalf("expected no update, got %#v", state)
	}
	if state.ManifestSource != "" || state.ManifestURL != "" {
		t.Fatalf("expected empty manifest source details, got %#v", state)
	}
}

func TestApplyDesktopReleaseUpdateRollsBackOnProbeFailure(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}
	app := newRuntimeStatusTestApp(t, root)
	app.releaseManagerFn = func() (*releaseRuntimeManager, error) {
		return &releaseRuntimeManager{
			app: app,
			remoteManifestProvider: func(context.Context) (release.Manifest, error) {
				return release.Manifest{
					SchemaVersion:          2,
					AppVersion:             "1.2.0",
					MinimumResourceVersion: "2026.06.01",
					Packages: []release.RuntimePackage{
						{ID: "runtime-bin", Target: release.DefaultTarget(), Kind: "runtime-binary", Required: true, Version: "1.0.0", Path: "bin/test-runtime"},
					},
				}, nil
			},
			activationProbe: func(string) error {
				return fmt.Errorf("probe failed")
			},
		}, nil
	}

	if _, err := app.ApplyDesktopReleaseUpdate(); err == nil {
		t.Fatal("expected ApplyDesktopReleaseUpdate to fail")
	}
	data, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read active runtime pointer: %v", err)
	}
	expected := `{"version":"2026.05.12","resourceVersion":"2026.05.12"}`
	if string(data) != expected {
		t.Fatalf("expected pointer rollback to previous version, got %s", string(data))
	}
}

func TestApplyDesktopReleaseUpdateSwitchesPointerOnSuccess(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}
	newVersionDir, err := layout.VersionDir("2026.06.01")
	if err != nil {
		t.Fatalf("new version dir: %v", err)
	}
	runtimeFile := filepath.Join(newVersionDir, "bin", "test-runtime")
	if err := os.MkdirAll(filepath.Dir(runtimeFile), 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := os.WriteFile(runtimeFile, []byte("runtime-binary"), 0o755); err != nil {
		t.Fatalf("write runtime file: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	app.releaseManagerFn = func() (*releaseRuntimeManager, error) {
		return &releaseRuntimeManager{
			app: app,
			remoteManifestProvider: func(context.Context) (release.Manifest, error) {
				return release.Manifest{
					SchemaVersion:          2,
					AppVersion:             "1.2.0",
					MinimumResourceVersion: "2026.06.01",
					Packages: []release.RuntimePackage{
						{ID: "runtime-bin", Target: release.DefaultTarget(), Kind: "runtime-binary", Required: true, Version: "1.0.0", Path: "bin/test-runtime"},
					},
				}, nil
			},
		}, nil
	}

	state, err := app.ApplyDesktopReleaseUpdate()
	if err != nil {
		t.Fatalf("ApplyDesktopReleaseUpdate returned error: %v", err)
	}
	if state.Kind != "required" {
		t.Fatalf("expected required update state, got %#v", state)
	}
	data, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read active runtime pointer: %v", err)
	}
	expected := `{"version":"2026.06.01","resourceVersion":"2026.06.01"}`
	if string(data) != expected {
		t.Fatalf("expected pointer to switch to new version, got %s", string(data))
	}
}

func TestExportDesktopEnvironmentDiagnostics(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimePackageManifestFixture(t, root)
	logger.Init(context.Background(), "info")
	defer func() {
		logger.GetMemoryWriter().Clear()
		_ = logger.Close()
	}()
	logger.GetMemoryWriter().Clear()

	log := logger.New("Release")
	log.Error("runtime update failed", logger.F("accessToken", "secret-token"), logger.F("proxyPassword", "top-secret"))

	app := newRuntimeStatusTestApp(t, root)
	path, err := app.ExportDesktopEnvironmentDiagnostics()
	if err != nil {
		t.Fatalf("ExportDesktopEnvironmentDiagnostics returned error: %v", err)
	}
	if !strings.HasPrefix(path, layout.DiagnosticsRoot()) {
		t.Fatalf("expected diagnostics path under %s, got %s", layout.DiagnosticsRoot(), path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostics bundle: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "secret-token") || strings.Contains(content, "top-secret") {
		t.Fatal("expected exported diagnostics to redact sensitive values")
	}
	if !strings.Contains(content, "[REDACTED]") {
		t.Fatal("expected exported diagnostics to contain redacted marker")
	}
	if !strings.Contains(content, "ENV-RUNTIME-POINTER-MISSING") {
		t.Fatalf("expected diagnostics to include current environment failure code, got %s", content)
	}
	if !strings.Contains(content, "\"updateManifestSource\": \"\"") {
		t.Fatalf("expected diagnostics to include update manifest source, got %s", content)
	}
}

func newRuntimeStatusTestApp(t *testing.T, root string) *App {
	t.Helper()

	workspaceHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/client/health", "/api/health":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"status": "ok",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(workspaceHost.Close)

	app := NewApp(root)
	app.ctx = context.Background()
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			RuntimeDir:   t.TempDir(),
			ServerOrigin: workspaceHost.URL,
		},
	}
	return app
}

func writeRuntimeManifestFixture(t *testing.T, root string) release.RuntimeLayout {
	t.Helper()
	layout := RuntimeReleaseLayout(root)
	manifestPath := filepath.Clean(filepath.Join(root, "publish", "runtime-manifest.json"))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{
		"schemaVersion": 2,
		"appVersion": "1.0.0",
		"minimumResourceVersion": "2026.05.12",
		"packages": [
			{"id":"desktop-core","target":"`+release.DefaultTarget()+`","kind":"browser-core","required":true,"version":"136.0.0","path":"core"}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(layout.ActivePointerPath()), 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	return layout
}

func writeRuntimePackageManifestFixture(t *testing.T, root string) release.RuntimeLayout {
	t.Helper()
	layout := RuntimeReleaseLayout(root)
	manifestPath := filepath.Clean(filepath.Join(root, "publish", "runtime-manifest.json"))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}

	runtimeContent := []byte("runtime-binary")
	runtimeSHA := fmt.Sprintf("%x", sha256.Sum256(runtimeContent))
	publishRuntimePath := filepath.Join(root, "publish", "bin", "test-runtime")
	if err := os.MkdirAll(filepath.Dir(publishRuntimePath), 0o755); err != nil {
		t.Fatalf("mkdir publish runtime dir: %v", err)
	}
	if err := os.WriteFile(publishRuntimePath, runtimeContent, 0o755); err != nil {
		t.Fatalf("write publish runtime file: %v", err)
	}

	manifest := fmt.Sprintf(`{
		"schemaVersion": 2,
		"appVersion": "1.0.0",
		"minimumResourceVersion": "2026.05.12",
		"packages": [
			{"id":"runtime-bin","target":"%s","kind":"runtime-binary","required":true,"version":"1.0.0","path":"bin/test-runtime","sha256":"%s"}
		]
	}`, release.DefaultTarget(), runtimeSHA)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(layout.ActivePointerPath()), 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	return layout
}

func writeRuntimePackageManifestFixtureWithoutPublishSource(t *testing.T, root string) release.RuntimeLayout {
	t.Helper()
	layout := RuntimeReleaseLayout(root)
	manifestPath := filepath.Clean(filepath.Join(root, "publish", "runtime-manifest.json"))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}

	runtimeContent := []byte("runtime-binary")
	runtimeSHA := fmt.Sprintf("%x", sha256.Sum256(runtimeContent))
	workspaceRuntimePath := filepath.Join(root, "bin", "test-runtime")
	if err := os.MkdirAll(filepath.Dir(workspaceRuntimePath), 0o755); err != nil {
		t.Fatalf("mkdir workspace runtime dir: %v", err)
	}
	if err := os.WriteFile(workspaceRuntimePath, runtimeContent, 0o755); err != nil {
		t.Fatalf("write workspace runtime file: %v", err)
	}

	manifest := fmt.Sprintf(`{
		"schemaVersion": 2,
		"appVersion": "1.0.0",
		"minimumResourceVersion": "2026.05.12",
		"packages": [
			{"id":"runtime-bin","target":"%s","kind":"runtime-binary","required":true,"version":"1.0.0","path":"bin/test-runtime","sha256":"%s"}
		]
	}`, release.DefaultTarget(), runtimeSHA)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(layout.ActivePointerPath()), 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	return layout
}

func writeRemoteUpdateManifestFixture(t *testing.T, root, appVersion, resourceVersion string) string {
	t.Helper()
	manifestPath := filepath.Join(root, "remote-runtime-manifest.json")
	manifest := fmt.Sprintf(`{
		"schemaVersion": 2,
		"appVersion": %q,
		"minimumResourceVersion": %q,
		"packages": [
			{"id":"runtime-bin","target":"%s","kind":"runtime-binary","required":true,"version":"1.0.0","path":"bin/test-runtime","sha256":"ignored"}
		]
	}`, appVersion, resourceVersion, release.DefaultTarget())
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write remote manifest: %v", err)
	}
	return manifestPath
}

func writeCoreFixture(t *testing.T, coreDir string) {
	t.Helper()
	exePath := filepath.Join(coreDir, coreExecutableCandidateForTest())
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func coreExecutableCandidateForTest() string {
	return filepath.FromSlash(browser.CoreExecutableCandidates()[0])
}
