package browser

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type coreDAOStub struct {
	list []Core
	err  error
}

func (s *coreDAOStub) List() ([]Core, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Core{}, s.list...), nil
}

func (s *coreDAOStub) Upsert(Core) error       { return nil }
func (s *coreDAOStub) Delete(string) error     { return nil }
func (s *coreDAOStub) SetDefault(string) error { return nil }

func TestResolveChromeBinaryUsesDefaultCoreFromDAO(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	coreDir := filepath.Join(root, "chrome142")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}

	exePath := filepath.Join(coreDir, filepath.FromSlash(CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建可执行文件目录失败: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("写入可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Cores = nil // 模拟 ReloadConfig 后被 config.yaml 清空的场景

	mgr := NewManager(cfg, root)
	mgr.CoreDAO = &coreDAOStub{
		list: []Core{
			{
				CoreId:    "core-142",
				CoreName:  "Chrome 142",
				CorePath:  "chrome142",
				IsDefault: true,
			},
		},
	}

	got, err := mgr.ResolveChromeBinary(&Profile{CoreId: ""})
	if err != nil {
		t.Fatalf("ResolveChromeBinary 返回错误: %v", err)
	}
	if got != exePath {
		t.Fatalf("ResolveChromeBinary 路径错误: got=%q want=%q", got, exePath)
	}
}

func TestResolveChromeBinaryNormalizesWindowsStyleRelativeCorePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	coreDir := filepath.Join(root, "chrome", "Chrom-144")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}

	exePath := filepath.Join(coreDir, filepath.FromSlash(CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建可执行文件目录失败: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("写入可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, root)
	mgr.CoreDAO = &coreDAOStub{
		list: []Core{
			{
				CoreId:    "core-144",
				CoreName:  "Chrome 144",
				CorePath:  `chrome\Chrom-144`,
				IsDefault: true,
			},
		},
	}

	got, err := mgr.ResolveChromeBinary(&Profile{})
	if err != nil {
		t.Fatalf("ResolveChromeBinary 返回错误: %v", err)
	}
	if got != exePath {
		t.Fatalf("ResolveChromeBinary 路径错误: got=%q want=%q", got, exePath)
	}
}

func TestResolveChromeBinaryAcceptsDirectExecutablePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	coreDir := filepath.Join(root, "chrome-direct")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}

	exePath := filepath.Join(coreDir, filepath.FromSlash(CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建可执行文件目录失败: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("写入可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, root)
	mgr.CoreDAO = &coreDAOStub{
		list: []Core{
			{
				CoreId:    "core-direct",
				CoreName:  "Chrome Direct",
				CorePath:  exePath,
				IsDefault: true,
			},
		},
	}

	got, err := mgr.ResolveChromeBinary(&Profile{})
	if err != nil {
		t.Fatalf("ResolveChromeBinary 返回错误: %v", err)
	}
	if got != exePath {
		t.Fatalf("ResolveChromeBinary 路径错误: got=%q want=%q", got, exePath)
	}
}

func TestResolveChromeBinaryAcceptsDarwinAppBundlePath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("仅验证 darwin .app 路径解析")
	}

	root := t.TempDir()
	appDir := filepath.Join(root, "Chromium.app")
	exePath := filepath.Join(appDir, "Contents", "MacOS", "Chromium")
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建可执行文件目录失败: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("写入可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, root)
	mgr.CoreDAO = &coreDAOStub{
		list: []Core{
			{
				CoreId:    "core-app",
				CoreName:  "Chromium App",
				CorePath:  appDir,
				IsDefault: true,
			},
		},
	}

	got, err := mgr.ResolveChromeBinary(&Profile{})
	if err != nil {
		t.Fatalf("ResolveChromeBinary 返回错误: %v", err)
	}
	if got != exePath {
		t.Fatalf("ResolveChromeBinary 路径错误: got=%q want=%q", got, exePath)
	}
}

func TestCountInstancesByCoreTreatsLegacyDefaultReferenceAsDefault(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Browser.Profiles = []config.BrowserProfileConfig{
		{ProfileId: "p-empty", CoreId: ""},
		{ProfileId: "p-legacy", CoreId: "default"},
		{ProfileId: "p-explicit", CoreId: "core-142"},
	}

	mgr := NewManager(cfg, "")
	mgr.CoreDAO = &coreDAOStub{
		list: []Core{
			{
				CoreId:    "core-142",
				CoreName:  "Chrome 142",
				CorePath:  "chrome142",
				IsDefault: true,
			},
		},
	}

	if got := mgr.CountInstancesByCore("core-142"); got != 3 {
		t.Fatalf("默认内核窗口计数错误: got=%d want=3", got)
	}
}

// TestGetChromeVersionFallsBackToManifest 验证：无法从真实 exe 版本资源取版本时
// （非 Windows、stub exe、或无 exe），GetChromeVersion 回退到 manifest。
// 真实 exe 版本读取路径依赖 Windows 版本资源，归手动/集成验证。
func TestGetChromeVersionFallsBackToManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mgr := NewManager(config.DefaultConfig(), root)

	// 1) *.manifest 文件名回退（无 exe、无 manifest.json）。
	dirName := filepath.Join(root, "core-manifest-name")
	if err := os.MkdirAll(dirName, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "144.0.7559.96.manifest"), []byte(""), 0o644); err != nil {
		t.Fatalf("写入 *.manifest 失败: %v", err)
	}
	if got := mgr.GetChromeVersion("core-manifest-name"); got != "144.0.7559.96" {
		t.Fatalf("应回退 *.manifest 文件名版本: got=%q want=144.0.7559.96", got)
	}

	// 2) manifest.json version 字段回退。
	dirJSON := filepath.Join(root, "core-manifest-json")
	if err := os.MkdirAll(dirJSON, 0o755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirJSON, "manifest.json"), []byte(`{"version":"143.0.7000.50"}`), 0o644); err != nil {
		t.Fatalf("写入 manifest.json 失败: %v", err)
	}
	if got := mgr.GetChromeVersion("core-manifest-json"); got != "143.0.7000.50" {
		t.Fatalf("应读取 manifest.json version 字段: got=%q want=143.0.7000.50", got)
	}
}
