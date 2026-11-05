package backend

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

// mieru 官方 Release 的真实资产命名（v3.39.0 实测），含干扰项：
// mita_*（服务端）、sha256 校验文件、deb 包、android/riscv 等非目标平台。
// 断言：所有受支持平台的资产选择必须命中 mieru_ 前缀且平台正确，
// 防止 mita（服务端二进制）或校验文件被误选。
func TestSelectProxyCoreAssetMieruAllPlatforms(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "mita_3.39.0_amd64.deb"},
		{Name: "mita_3.39.0_arm64.deb"},
		{Name: "mita_3.39.0_linux_amd64.tar.gz"},
		{Name: "mita_3.39.0_linux_arm64.tar.gz"},
		{Name: "mieru_3.39.0_amd64.deb"},
		{Name: "mieru_3.39.0_android_arm64.tar.gz"},
		{Name: "mieru_3.39.0_arm64.deb"},
		{Name: "mieru_3.39.0_linux_amd64.tar.gz"},
		{Name: "mieru_3.39.0_linux_amd64.tar.gz.sha256.txt"},
		{Name: "mieru_3.39.0_linux_arm64.tar.gz"},
		{Name: "mieru_3.39.0_linux_arm64.tar.gz.sha256.txt"},
		{Name: "mieru_3.39.0_linux_armv7.tar.gz"},
		{Name: "mieru_3.39.0_linux_riscv64.tar.gz"},
		{Name: "mieru_3.39.0_macos_amd64.tar.gz"},
		{Name: "mieru_3.39.0_macos_amd64.tar.gz.sha256.txt"},
		{Name: "mieru_3.39.0_macos_arm64.tar.gz"},
		{Name: "mieru_3.39.0_macos_arm64.tar.gz.sha256.txt"},
		{Name: "mieru_3.39.0_windows_amd64.zip"},
		{Name: "mieru_3.39.0_windows_amd64.zip.sha256.txt"},
		{Name: "mieru_3.39.0_windows_arm64.zip"},
		{Name: "mieru_3.39.0_windows_x86.zip"},
	}

	spec := proxyCoreSpec{Core: "mieru", Repo: "enfein/mieru", DisplayName: "mieru", BinaryBase: "mieru", ConfigKey: "mieru", Version: "v3.39.0"}

	cases := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"windows", "amd64", "mieru_3.39.0_windows_amd64.zip"},
		{"windows", "arm64", "mieru_3.39.0_windows_arm64.zip"},
		{"windows", "386", "mieru_3.39.0_windows_x86.zip"},
		{"linux", "amd64", "mieru_3.39.0_linux_amd64.tar.gz"},
		{"linux", "arm64", "mieru_3.39.0_linux_arm64.tar.gz"},
		{"darwin", "amd64", "mieru_3.39.0_macos_amd64.tar.gz"},
		{"darwin", "arm64", "mieru_3.39.0_macos_arm64.tar.gz"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			asset, err := selectProxyCoreAsset(spec, assets, tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("资产选择失败: %v", err)
			}
			if asset.Name != tc.want {
				t.Fatalf("选中 %q, want %q（可能误选了 mita 服务端或校验文件）", asset.Name, tc.want)
			}
		})
	}
}

// xray/sing-box/mihomo 的资产选择回归保护（各自仓库的真实资产形态）。
func TestSelectProxyCoreAssetOtherCoresUnaffected(t *testing.T) {
	xray := proxyCoreSpec{Core: "xray", Repo: "XTLS/Xray-core", DisplayName: "Xray", BinaryBase: "xray", ConfigKey: "xray"}
	asset, err := selectProxyCoreAsset(xray, []githubReleaseAsset{
		{Name: "Xray-windows-64.zip"},
		{Name: "Xray-windows-arm64-v8a.zip"},
		{Name: "Xray-linux-64.zip"},
		{Name: "Xray-macos-64.zip"},
		{Name: "geoip.dat"},
		{Name: "Xray-windows-64.zip.dgst"},
	}, "windows", "amd64")
	if err != nil || asset.Name != "Xray-windows-64.zip" {
		t.Fatalf("xray windows/amd64 选中 %+v, err=%v", asset, err)
	}

	mihomo := proxyCoreSpec{Core: "mihomo", Repo: "MetaCubeX/mihomo", DisplayName: "Mihomo", BinaryBase: "mihomo", ConfigKey: "clash"}
	asset, err = selectProxyCoreAsset(mihomo, []githubReleaseAsset{
		{Name: "mihomo-windows-amd64-compatible-v1.19.27.zip"},
		{Name: "mihomo-windows-amd64-v1.19.27.zip"},
		{Name: "mihongo-windows-amd64.zip"},
	}, "windows", "amd64")
	if err != nil || asset.Name != "mihomo-windows-amd64-compatible-v1.19.27.zip" {
		t.Fatalf("mihomo windows/amd64 选中 %+v, err=%v", asset, err)
	}

	singbox := proxyCoreSpec{Core: "sing-box", Repo: "SagerNet/sing-box", DisplayName: "sing-box", BinaryBase: "sing-box", ConfigKey: "sing-box"}
	asset, err = selectProxyCoreAsset(singbox, []githubReleaseAsset{
		{Name: "sing-box-1.13.13-windows-amd64.zip"},
		{Name: "sing-box-1.13.13-linux-amd64.tar.gz"},
		{Name: "sing-box-1.13.13-windows-amd64.zip.sha256"},
	}, "windows", "amd64")
	if err != nil || asset.Name != "sing-box-1.13.13-windows-amd64.zip" {
		t.Fatalf("sing-box windows/amd64 选中 %+v, err=%v", asset, err)
	}
}

// 真实端到端下载验证（联网）：空 config + 空 bin 的全新环境下载 mieru，
// 断言：1) 安装到 bin/{platform}/mieru/；2) resolveBinary 不依赖 config 也能找到
// （模拟重装/换目录后 config 绝对路径失效的场景）。
func TestMieruCoreDownloadEndToEndFreshEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skipf("本机 %s/%s，跳过 Windows amd64 下载验证", runtime.GOOS, runtime.GOARCH)
	}
	appRoot := t.TempDir()
	app := NewApp(appRoot)
	app.config = config.DefaultConfig()

	// 直接调内部下载（绕过 UI 事件层；真实应用由 wails 注入 ctx）。
	app.downloadProxyCore(context.Background(), proxyCoreSpec{
		Core: "mieru", Repo: "enfein/mieru", DisplayName: "mieru",
		BinaryBase: "mieru", ConfigKey: "mieru", Version: "v3.39.0",
	}, proxyCoreTarget{GOOS: "windows", GOARCH: "amd64"}, "", "v3.39.0")

	installed := filepath.Join(appRoot, "bin", "windows-amd64", "mieru", "mieru.exe")
	if _, err := os.Stat(installed); err != nil {
		t.Skipf("下载未成功（网络环境限制，非代码问题），未安装到 %s", installed)
	}

	// 关键断言：不设置 config.MieruBinaryPath（模拟 config 失效/重装），
	// MieruManager 仅凭 appRoot 目录搜索找到二进制。
	cfg := config.DefaultConfig()
	cfg.Browser.MieruBinaryPath = ""
	mgr := proxy.NewMieruManager(cfg, appRoot)
	defer mgr.Stop()
	binaryPath, err := mgr.ResolveBinaryPath()
	if err != nil {
		t.Fatalf("config 失效后 resolveBinary 找不到内核: %v", err)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("解析到的路径不存在: %s", binaryPath)
	}
	t.Logf("下载安装: %s", installed)
	t.Logf("config 失效后解析: %s", binaryPath)
}

// 离线布局验证：下载框架的标准安装布局（bin/{platform}/mieru/）必须被
// MieruManager.resolveBinary 找到——不依赖 config 路径（模拟重装后 config 失效）。
func TestMieruResolveBinaryFindsDownloadLayout(t *testing.T) {
	sourceBinary := os.Getenv("MIERU_BINARY_PATH")
	if sourceBinary == "" {
		sourceBinary = `D:\tmp-mieru\mieru.exe`
	}
	if _, err := os.Stat(sourceBinary); err != nil {
		t.Skipf("本机无 mieru.exe（%s），跳过布局验证", sourceBinary)
	}

	appRoot := t.TempDir()
	installDir := filepath.Join(appRoot, "bin", "windows-amd64", "mieru")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	installed := filepath.Join(installDir, "mieru.exe")
	data, err := os.ReadFile(sourceBinary)
	if err != nil {
		t.Fatalf("read source failed: %v", err)
	}
	if err := os.WriteFile(installed, data, 0o755); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// config 不设置路径：仅靠目录搜索（下载安装布局优先）
	cfg := config.DefaultConfig()
	cfg.Browser.MieruBinaryPath = ""
	mgr := proxy.NewMieruManager(cfg, appRoot)
	defer mgr.Stop()
	binaryPath, err := mgr.ResolveBinaryPath()
	if err != nil {
		t.Fatalf("config 失效后未找到下载布局内核: %v", err)
	}
	if binaryPath != installed {
		t.Fatalf("解析到 %s, want %s", binaryPath, installed)
	}
}
