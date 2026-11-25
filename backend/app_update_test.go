package backend

import (
	"ant-chrome/backend/internal/config"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCheckForUpdatesDetectsNewRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v1.9.0","html_url":"https://example.com/release"}`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if !result.OK {
		t.Fatalf("checkForUpdates failed: %+v", result)
	}
	if !result.UpdateAvailable {
		t.Fatalf("expected update available: %+v", result)
	}
	if result.LatestVersion != "1.9.0" {
		t.Fatalf("latest version = %q, want 1.9.0", result.LatestVersion)
	}
}

func TestCheckForUpdatesReportsNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v1.8.0","name":"1.8.0"}`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if !result.OK {
		t.Fatalf("checkForUpdates failed: %+v", result)
	}
	if result.UpdateAvailable {
		t.Fatalf("expected no update: %+v", result)
	}
}

func TestCheckForUpdatesReportsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if result.OK {
		t.Fatalf("expected HTTP error: %+v", result)
	}
	if result.Error != "更新服务返回 HTTP 503" {
		t.Fatalf("error = %q, want HTTP 503 error", result.Error)
	}
}

func TestCheckForUpdatesReportsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if result.OK {
		t.Fatalf("expected invalid response error: %+v", result)
	}
	if result.Error == "" {
		t.Fatal("expected invalid response error")
	}
}

func TestCheckForUpdatesDoesNotRequestWithUnknownCurrentVersion(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "unknown", server.URL, server.Client())
	if result.OK || result.Error != "当前应用版本未知" {
		t.Fatalf("expected unknown version error: %+v", result)
	}
	if requested {
		t.Fatal("update check requested release data with unknown current version")
	}
}

func TestFetchAppUpdateInfoParsesManifestAssets(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/manifest") {
			_, _ = writer.Write([]byte(`{"version":"1.9.6","notesUrl":"https://example.com/notes","force":true,"assets":{"windows-amd64":{"name":"setup.exe","sha256":"abc123"}}}`))
			return
		}
		base := strings.TrimSuffix(server.URL, "/")
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`[{"tag_name":"v1.9.6","html_url":"https://example.com/release","prerelease":false,"assets":[{"name":"setup.exe","browser_download_url":"%s/setup.exe"},{"name":"update-manifest.json","browser_download_url":"%s/manifest"}]}]`,
			base, base,
		)))
	}))
	defer server.Close()

	info := fetchAppUpdateInfo(context.Background(), "1.9.5", server.URL, server.Client())
	if !info.OK || !info.UpdateAvailable {
		t.Fatalf("update info = %+v", info)
	}
	asset, ok := info.Assets["windows-amd64"]
	if !ok || asset.Name != "setup.exe" || asset.SHA256 != "abc123" {
		t.Fatalf("windows asset = %+v, want setup.exe/abc123", asset)
	}
	if !strings.HasSuffix(asset.URL, "/setup.exe") {
		t.Fatalf("asset url = %q", asset.URL)
	}
	if !info.Force || info.NotesURL != "https://example.com/notes" {
		t.Fatalf("manifest flags = force:%v notes:%q", info.Force, info.NotesURL)
	}
}

// 预览通道：从 Release 列表里挑带 prerelease 标记的版本，并同样读取更新清单。
func TestFetchAppUpdateInfoPreviewChannelPicksPrerelease(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		base := strings.TrimSuffix(server.URL, "/")
		switch {
		case strings.HasSuffix(request.URL.Path, "/manifest"):
			_, _ = writer.Write([]byte(`{"version":"1.9.8-beta.1","assets":{"windows-amd64":{"name":"setup.exe","sha256":"abc123"}}}`))
		case strings.HasSuffix(request.URL.Path, "/list"):
			_, _ = writer.Write([]byte(fmt.Sprintf(
				`[{"tag_name":"v1.9.8-beta.1","html_url":"https://example.com/beta","prerelease":true,"assets":[{"name":"setup.exe","browser_download_url":"%s/setup.exe"},{"name":"update-manifest.json","browser_download_url":"%s/manifest"}]},{"tag_name":"v1.9.7","html_url":"https://example.com/stable","prerelease":false,"assets":[]}]`,
				base, base,
			)))
		default:
			_, _ = writer.Write([]byte(`{"tag_name":"v1.9.7","html_url":"https://example.com/stable","prerelease":false,"assets":[]}`))
		}
	}))
	defer server.Close()

	info := fetchAppUpdateInfoForChannel(context.Background(), "1.9.7", UpdateChannelPreview, "", server.URL+"/list", server.Client())
	if !info.OK || !info.UpdateAvailable {
		t.Fatalf("预览通道结果异常: %+v", info)
	}
	if info.LatestVersion != "1.9.8-beta.1" {
		t.Fatalf("latest = %q, want 1.9.8-beta.1", info.LatestVersion)
	}
	if _, ok := info.Assets["windows-amd64"]; !ok {
		t.Fatalf("预览版本缺少安装包: %+v", info.Assets)
	}
}

// 预览通道没有任何预发布版本时必须明确提示，不能悄悄退回正式版。
func TestFetchAppUpdateInfoPreviewChannelWithoutPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"tag_name":"v1.9.7","prerelease":false,"assets":[]}]`))
	}))
	defer server.Close()

	info := fetchAppUpdateInfoForChannel(context.Background(), "1.9.6", UpdateChannelPreview, "", server.URL+"/list", server.Client())
	if info.OK || !strings.Contains(info.Error, "预览版本") {
		t.Fatalf("update info = %+v, want 预览版本缺失提示", info)
	}
}

// 正式通道必须拒绝被标记为预发布的 Release。
func TestFetchAppUpdateInfoStableChannelRejectsPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"tag_name":"v1.9.8-beta.1","prerelease":true,"assets":[]}]`))
	}))
	defer server.Close()

	info := fetchAppUpdateInfoForChannel(context.Background(), "1.9.7", UpdateChannelStable, "", server.URL+"/list", server.Client())
	if info.OK || !strings.Contains(info.Error, "没有可用的正式版本") {
		t.Fatalf("update info = %+v, want 拒绝预览版", info)
	}
}

// 指定版本：从列表里精确匹配目标版本；可选版本最多 2 个且都高于当前版本。
func TestFetchAppUpdateInfoSelectsRequestedVersion(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		base := strings.TrimSuffix(server.URL, "/")
		if strings.HasPrefix(request.URL.Path, "/manifest/") {
			version := strings.TrimPrefix(request.URL.Path, "/manifest/")
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"version":%q,"assets":{"windows-amd64":{"name":"setup.exe","sha256":"abc123"}}}`, version)))
			return
		}
		release := func(version string) string {
			return fmt.Sprintf(`{"tag_name":"v%s","html_url":"https://example.com/%s","prerelease":false,"assets":[{"name":"setup.exe","browser_download_url":"%s/setup-%s"},{"name":"update-manifest.json","browser_download_url":"%s/manifest/%s"}]}`,
				version, version, base, version, base, version)
		}
		_, _ = writer.Write([]byte("[" + release("1.9.9") + "," + release("1.9.8") + "," + release("1.9.7") + "]"))
	}))
	defer server.Close()

	info := fetchAppUpdateInfoForChannel(context.Background(), "1.9.6", UpdateChannelStable, "1.9.8", server.URL+"/list", server.Client())
	if !info.OK || !info.UpdateAvailable || info.LatestVersion != "1.9.8" {
		t.Fatalf("指定版本结果异常: %+v", info)
	}
	if len(info.Versions) != 2 || info.Versions[0] != "1.9.9" || info.Versions[1] != "1.9.8" {
		t.Fatalf("可选版本 = %v, want [1.9.9 1.9.8]", info.Versions)
	}

	missing := fetchAppUpdateInfoForChannel(context.Background(), "1.9.6", UpdateChannelStable, "1.9.1", server.URL+"/list", server.Client())
	if missing.OK || !strings.Contains(missing.Error, "没有找到版本") {
		t.Fatalf("缺失版本结果 = %+v, want 没有找到版本", missing)
	}
}

func TestFetchAppUpdateInfoReportsMissingManifest(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		base := strings.TrimSuffix(server.URL, "/")
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`[{"tag_name":"v1.9.6","html_url":"https://example.com/release","prerelease":false,"assets":[{"name":"setup.exe","browser_download_url":"%s/setup.exe"}]}]`,
			base,
		)))
	}))
	defer server.Close()

	info := fetchAppUpdateInfo(context.Background(), "1.9.5", server.URL, server.Client())
	if info.OK || info.UpdateAvailable {
		t.Fatalf("expected missing manifest failure: %+v", info)
	}
	if !strings.Contains(info.Error, "update-manifest.json") {
		t.Fatalf("error = %q, want missing manifest error", info.Error)
	}
}

func TestFetchAppUpdateInfoReportsMissingPlatformAsset(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/manifest") {
			_, _ = writer.Write([]byte(`{"version":"1.9.6","assets":{"other-platform":{"name":"setup.exe","sha256":"abc123"}}}`))
			return
		}
		base := strings.TrimSuffix(server.URL, "/")
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`[{"tag_name":"v1.9.6","html_url":"https://example.com/release","prerelease":false,"assets":[{"name":"setup.exe","browser_download_url":"%s/setup.exe"},{"name":"update-manifest.json","browser_download_url":"%s/manifest"}]}]`,
			base, base,
		)))
	}))
	defer server.Close()

	info := fetchAppUpdateInfo(context.Background(), "1.9.5", server.URL, server.Client())
	if info.OK || info.UpdateAvailable {
		t.Fatalf("expected missing platform asset failure: %+v", info)
	}
	if !strings.Contains(info.Error, updateAssetKey()) {
		t.Fatalf("error = %q, want missing platform error", info.Error)
	}
}

func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256 = %q", sum)
	}
}

func TestUpdateHTTPClientForRejectsBadProxyWithoutDirectFallback(t *testing.T) {
	client, err := (&App{}).updateHTTPClientFor("https://127.0.0.1:443")
	if err == nil {
		t.Fatal("expected bad proxy error")
	}
	if client != nil {
		t.Fatal("expected no direct fallback client")
	}
}

func TestUpdateDownloadClientUsesLongTimeout(t *testing.T) {
	client, err := (&App{}).updateDownloadHTTPClientFor("", "")
	if err != nil {
		t.Fatalf("build download client failed: %v", err)
	}
	if client.Timeout != updateDownloadTimeout {
		t.Fatalf("download timeout = %v, want %v", client.Timeout, updateDownloadTimeout)
	}
	if client.Timeout <= updateCheckTimeout {
		t.Fatal("download timeout must be longer than update check timeout")
	}
}

func TestUpdateHTTPClientRejectsUnknownProxyID(t *testing.T) {
	client, err := (&App{}).updateHTTPClientForSelection(updateCheckTimeout, "", "missing-proxy")
	if err == nil {
		t.Fatal("expected unknown proxy id error")
	}
	if client != nil {
		t.Fatal("expected no direct fallback client")
	}
}

func TestDownloadedUpdateMetadataRoundTrip(t *testing.T) {
	app := &App{appRoot: t.TempDir()}
	want := downloadedUpdateInfo{Version: "1.9.6", AssetName: "setup.exe"}
	if err := app.saveDownloadedUpdate(want); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	got := app.loadDownloadedUpdate()
	if got == nil || *got != want {
		t.Fatalf("downloaded update = %+v, want %+v", got, want)
	}
}

func TestCompareUpdateVersionsHandlesPrerelease(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v1.8.0", right: "1.8.0", want: 0},
		{left: "1.8.0-beta.1", right: "1.8.0", want: -1},
		{left: "1.8.0", right: "1.8.1", want: -1},
		{left: "1.9.0", right: "1.8.1", want: 1},
	}

	for _, item := range cases {
		got, err := compareUpdateVersions(item.left, item.right)
		if err != nil {
			t.Fatalf("compareUpdateVersions(%q, %q) failed: %v", item.left, item.right, err)
		}
		if got != item.want {
			t.Fatalf("compareUpdateVersions(%q, %q) = %d, want %d", item.left, item.right, got, item.want)
		}
	}
}

func newUpdateTestApp(t *testing.T, proxy string) *App {
	t.Helper()
	return &App{
		appRoot: t.TempDir(),
		version: "1.9.6",
		config:  &config.Config{Update: config.UpdateConfig{Proxy: proxy}},
	}
}

// 代理配置错误必须在检查阶段明确失败，不能静默回退直连。
func TestCheckAppUpdateSurfacesProxyConfigError(t *testing.T) {
	app := newUpdateTestApp(t, "https://127.0.0.1:443")
	info := app.CheckAppUpdate()
	if info.OK {
		t.Fatalf("代理配置错误仍返回成功: %+v", info)
	}
	if !strings.Contains(info.Error, "更新代理配置错误") {
		t.Fatalf("error = %q, want 明确代理配置错误", info.Error)
	}
	if info.Assets == nil {
		t.Fatal("assets 不能为 nil，前端按对象渲染")
	}
}

// 检查更新也必须支持临时指定代理：坏代理/未知节点都明确报错，不回退直连。
func TestCheckAppUpdateWithProxySelectionRejectsBadSelection(t *testing.T) {
	app := newUpdateTestApp(t, "")

	badProxy := app.CheckAppUpdateWithProxySelection("https://127.0.0.1:443", "")
	if badProxy.OK || !strings.Contains(badProxy.Error, "更新代理配置错误") {
		t.Fatalf("bad proxy info = %+v", badProxy)
	}
	if badProxy.Assets == nil {
		t.Fatal("assets 不能为 nil，前端按对象渲染")
	}

	unknownNode := app.CheckAppUpdateWithProxySelection("", "missing-proxy")
	if unknownNode.OK || !strings.Contains(unknownNode.Error, "代理节点不存在") {
		t.Fatalf("unknown node info = %+v", unknownNode)
	}
}

// 关闭更新检查时必须给出明确原因，而不是空结果。
func TestCheckAppUpdateReportsDisabled(t *testing.T) {
	disabled := false
	app := newUpdateTestApp(t, "")
	app.config.Update.Enabled = &disabled

	info := app.CheckAppUpdate()
	if info.OK || !strings.Contains(info.Error, "更新检查已关闭") {
		t.Fatalf("update info = %+v, want 更新检查已关闭", info)
	}
	if err := app.ApplyAppUpdate("", ""); err == nil || !strings.Contains(err.Error(), "更新检查已关闭") {
		t.Fatalf("apply error = %v, want 更新检查已关闭", err)
	}
}

// 没有已下载安装包时必须明确提示，不能重新联网下载。
func TestApplyAppUpdateRequiresDownloadedPackage(t *testing.T) {
	app := newUpdateTestApp(t, "")
	err := app.ApplyAppUpdate("", "")
	if err == nil || !strings.Contains(err.Error(), "没有已下载的更新包") {
		t.Fatalf("error = %v, want 没有已下载的更新包", err)
	}
}

// 请求安装的版本与已下载包不一致时必须拒绝。
func TestApplyAppUpdateRejectsVersionMismatch(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.6", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	err := app.ApplyAppUpdate("1.9.5", "")
	if err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("error = %v, want 版本不一致", err)
	}
}

// 已下载记录存在但安装包被删除时，必须明确报错而不是写入 pending 卡住。
func TestApplyAppUpdateReportsMissingInstaller(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.6", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	err := app.ApplyAppUpdate("", "")
	if err == nil || !strings.Contains(err.Error(), "更新包不存在") {
		t.Fatalf("error = %v, want 更新包不存在", err)
	}
	if pending := app.GetPendingAppUpdate(); pending != nil {
		t.Fatalf("安装失败仍写入 pending: %+v", pending)
	}
}

// 同一进程重复安排安装时必须复用已启动的等待脚本，避免安装包被执行两次。
func TestApplyAppUpdateSkipsDuplicateWaiter(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.7", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	installerPath := app.updateDataPath("updates", "1.9.7", "setup.exe")
	if err := os.MkdirAll(filepath.Dir(installerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	createdAt := "2026-01-01T00:00:00Z"
	if err := app.savePendingUpdate(UpdatePendingInfo{
		Version:       "1.9.7",
		InstallerPath: installerPath,
		PID:           os.Getpid(),
		CreatedAt:     createdAt,
	}); err != nil {
		t.Fatalf("save pending update failed: %v", err)
	}

	if err := app.ApplyAppUpdate("", ""); err != nil {
		t.Fatalf("重复安排应保持幂等: %v", err)
	}
	pending := app.GetPendingAppUpdate()
	if pending == nil || pending.CreatedAt != createdAt {
		t.Fatalf("重复安排改写了 pending 记录: %+v", pending)
	}
}

// 重新打开安装时，同一进程已经安排过等待脚本就不能再拉起一次。
func TestApplyPendingAppUpdateSkipsDuplicateWaiter(t *testing.T) {
	app := newUpdateTestApp(t, "")
	installerPath := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	createdAt := "2026-01-01T00:00:00Z"
	if err := app.savePendingUpdate(UpdatePendingInfo{
		Version:       "1.9.7",
		InstallerPath: installerPath,
		PID:           os.Getpid(),
		CreatedAt:     createdAt,
	}); err != nil {
		t.Fatalf("save pending update failed: %v", err)
	}

	if err := app.ApplyPendingAppUpdate(); err != nil {
		t.Fatalf("重复安排应保持幂等: %v", err)
	}
	pending := app.GetPendingAppUpdate()
	if pending == nil || pending.CreatedAt != createdAt {
		t.Fatalf("重复安排改写了 pending 记录: %+v", pending)
	}
}

// 安装包被删除后，pending 必须保留并标记 Missing，前端才能给出明确提示。
func TestPendingUpdateMarksMissingInstaller(t *testing.T) {
	app := newUpdateTestApp(t, "")
	installerPath := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	pending := UpdatePendingInfo{Version: "1.9.7", InstallerPath: installerPath, PID: 1, CreatedAt: "2026-01-01T00:00:00Z"}
	if err := app.savePendingUpdate(pending); err != nil {
		t.Fatalf("save pending update failed: %v", err)
	}
	if got := app.GetPendingAppUpdate(); got == nil || got.Missing {
		t.Fatalf("安装包存在时应返回正常 pending: %+v", got)
	}
	if err := os.Remove(installerPath); err != nil {
		t.Fatal(err)
	}
	got := app.GetPendingAppUpdate()
	if got == nil || !got.Missing {
		t.Fatalf("安装包丢失时必须返回 Missing 状态: %+v", got)
	}
	if err := app.ApplyPendingAppUpdate(); err == nil || !strings.Contains(err.Error(), "待安装更新包不存在") {
		t.Fatalf("error = %v, want 待安装更新包不存在", err)
	}
}

// 版本已经装上时，残留的 pending 标记不能再提示安装。
func TestPendingUpdateIgnoredWhenVersionInstalled(t *testing.T) {
	app := newUpdateTestApp(t, "")
	installerPath := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	pending := UpdatePendingInfo{Version: "1.9.6", InstallerPath: installerPath, PID: 1, CreatedAt: "2026-01-01T00:00:00Z"}
	if err := app.savePendingUpdate(pending); err != nil {
		t.Fatalf("save pending update failed: %v", err)
	}
	if got := app.GetPendingAppUpdate(); got != nil {
		t.Fatalf("同版本 pending 应被忽略: %+v", got)
	}
}

// 重启后必须能发现已下载的安装包，并给出可直接安装的路径。
func TestDownloadedUpdateRecoveredAfterRestart(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.7", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	installerPath := app.updateDataPath("updates", "1.9.7", "setup.exe")
	if err := os.MkdirAll(filepath.Dir(installerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := app.GetDownloadedAppUpdate()
	if got == nil {
		t.Fatal("已下载安装包应可恢复")
	}
	if got.Version != "1.9.7" || got.AssetName != "setup.exe" || got.InstallerPath != installerPath {
		t.Fatalf("downloaded update = %+v", got)
	}
}

// 版本已安装或安装包丢失时，不应再提示已下载。
func TestDownloadedUpdateIgnoredWhenUnusable(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.6", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	installerPath := app.updateDataPath("updates", "1.9.6", "setup.exe")
	if err := os.MkdirAll(filepath.Dir(installerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := app.GetDownloadedAppUpdate(); got != nil {
		t.Fatalf("同版本已下载包应被忽略: %+v", got)
	}

	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.8", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	if got := app.GetDownloadedAppUpdate(); got != nil {
		t.Fatalf("安装包丢失时应忽略已下载记录: %+v", got)
	}
}

// 没有已下载包或安装包丢失时，打开文件位置必须明确报错，不能去调系统文件管理器。
func TestOpenUpdatePackageLocationReportsMissingPackage(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.OpenUpdatePackageLocation(); err == nil || !strings.Contains(err.Error(), "没有已下载的更新包") {
		t.Fatalf("error = %v, want 没有已下载的更新包", err)
	}

	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.9", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	if err := app.OpenUpdatePackageLocation(); err == nil || !strings.Contains(err.Error(), "更新包不存在") {
		t.Fatalf("error = %v, want 更新包不存在", err)
	}
}

// 更新功能关闭时不再提示已下载的更新包，避免给出点了就会失败的安装入口。
func TestDownloadedUpdateIgnoredWhenUpdateDisabled(t *testing.T) {
	app := newUpdateTestApp(t, "")
	disabled := false
	app.config.Update.Enabled = &disabled
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.7", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	installerPath := app.updateDataPath("updates", "1.9.7", "setup.exe")
	if err := os.MkdirAll(filepath.Dir(installerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := app.GetDownloadedAppUpdate(); got != nil {
		t.Fatalf("更新关闭时不应返回已下载包: %+v", got)
	}
}

// 忽略已下载包时要同时清掉记录与安装文件。
func TestClearDownloadedAppUpdateRemovesPackage(t *testing.T) {
	app := newUpdateTestApp(t, "")
	if err := app.saveDownloadedUpdate(downloadedUpdateInfo{Version: "1.9.7", AssetName: "setup.exe"}); err != nil {
		t.Fatalf("save downloaded update failed: %v", err)
	}
	installerPath := app.updateDataPath("updates", "1.9.7", "setup.exe")
	if err := os.MkdirAll(filepath.Dir(installerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerPath, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.ClearDownloadedAppUpdate(); err != nil {
		t.Fatalf("clear downloaded update failed: %v", err)
	}
	if got := app.GetDownloadedAppUpdate(); got != nil {
		t.Fatalf("清除后仍能读回已下载包: %+v", got)
	}
	if _, err := os.Stat(installerPath); !os.IsNotExist(err) {
		t.Fatalf("清除后安装包仍然存在: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(installerPath)); !os.IsNotExist(err) {
		t.Fatalf("清除后版本目录仍然存在: %v", err)
	}
	if err := app.ClearDownloadedAppUpdate(); err != nil {
		t.Fatalf("重复清除应保持幂等: %v", err)
	}
}

func newUpdateAssetServer(t *testing.T, payload []byte, statusCode int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if statusCode != http.StatusOK {
			writer.WriteHeader(statusCode)
			return
		}
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server
}

// 下载链路必须落盘到 updates/<version>/、校验 SHA256 并写入已下载记录。
func TestDownloadUpdatePackageWritesVerifiedInstaller(t *testing.T) {
	payload := bytes.Repeat([]byte("installer-binary"), 512)
	server := newUpdateAssetServer(t, payload, http.StatusOK)
	app := newUpdateTestApp(t, "")
	asset := AppUpdateAsset{Name: "setup.exe", URL: server.URL + "/setup.exe", SHA256: sha256Hex(payload)}

	var lastPhase string
	err := app.downloadUpdatePackage(context.Background(), "1.9.6", asset, server.Client(), func(phase string, progress int, message string) {
		lastPhase = phase
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if lastPhase != "done" {
		t.Fatalf("last phase = %q, want done", lastPhase)
	}

	targetPath := app.updateDataPath("updates", "1.9.6", "setup.exe")
	written, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read downloaded installer failed: %v", err)
	}
	if !bytes.Equal(written, payload) {
		t.Fatalf("downloaded size = %d, want %d", len(written), len(payload))
	}
	if _, err := os.Stat(targetPath + ".part"); !os.IsNotExist(err) {
		t.Fatalf("临时文件未清理: %v", err)
	}
	saved := app.loadDownloadedUpdate()
	if saved == nil || saved.Version != "1.9.6" || saved.AssetName != "setup.exe" {
		t.Fatalf("已下载记录 = %+v", saved)
	}
}

// 校验不通过必须报错、丢弃临时文件且不写入已下载记录。
func TestDownloadUpdatePackageRejectsSHA256Mismatch(t *testing.T) {
	payload := []byte("tampered-installer")
	server := newUpdateAssetServer(t, payload, http.StatusOK)
	app := newUpdateTestApp(t, "")
	asset := AppUpdateAsset{Name: "setup.exe", URL: server.URL + "/setup.exe", SHA256: strings.Repeat("0", 64)}

	err := app.downloadUpdatePackage(context.Background(), "1.9.6", asset, server.Client(), func(string, int, string) {})
	if err == nil || !strings.Contains(err.Error(), "SHA256 不匹配") {
		t.Fatalf("error = %v, want SHA256 不匹配", err)
	}
	targetPath := app.updateDataPath("updates", "1.9.6", "setup.exe")
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("校验失败仍留下安装包: %v", statErr)
	}
	if _, statErr := os.Stat(targetPath + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("校验失败仍留下临时文件: %v", statErr)
	}
	if saved := app.loadDownloadedUpdate(); saved != nil {
		t.Fatalf("校验失败仍写入已下载记录: %+v", saved)
	}
}

// 下载源返回非 200 时必须给出明确错误。
func TestDownloadUpdatePackageReportsHTTPError(t *testing.T) {
	server := newUpdateAssetServer(t, nil, http.StatusInternalServerError)
	app := newUpdateTestApp(t, "")
	asset := AppUpdateAsset{Name: "setup.exe", URL: server.URL + "/setup.exe", SHA256: "abc"}

	err := app.downloadUpdatePackage(context.Background(), "1.9.6", asset, server.Client(), func(string, int, string) {})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error = %v, want HTTP 500", err)
	}
	if _, statErr := os.Stat(app.updateDataPath("updates", "1.9.6", "setup.exe") + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("下载失败仍留下临时文件: %v", statErr)
	}
	if saved := app.loadDownloadedUpdate(); saved != nil {
		t.Fatalf("下载失败仍写入已下载记录: %+v", saved)
	}
}
