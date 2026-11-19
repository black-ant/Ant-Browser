package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"ant-chrome/backend/internal/config"
)

// resolveBridgeEngineBinary 在仓库 bin/ 下查找随包分发的引擎二进制。
// 找不到时返回空串，由调用方跳过校验（CI 未带二进制时不阻塞单测）。
func resolveBridgeEngineBinary(name string) string {
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	root := filepath.Join("..", "..", "..", "bin")
	candidates := []string{
		filepath.Join(root, name+exe),
		filepath.Join(root, runtime.GOOS+"-"+runtime.GOARCH, name+exe),
		filepath.Join(root, runtime.GOOS+"-"+runtime.GOARCH, name, name+exe),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// TestGeneratedBridgeConfigsPassEngineCheck 用仓库自带引擎校验生成的桥接配置。
// 订阅 DNS 段字段写错只会在真正启动节点时暴露，这里提前用官方 check 拦住。
func TestGeneratedBridgeConfigsPassEngineCheck(t *testing.T) {
	root := t.TempDir()

	t.Run("sing-box", func(t *testing.T) {
		binary := resolveBridgeEngineBinary("sing-box")
		if binary == "" {
			t.Skip("bin/ 下没有 sing-box，跳过引擎校验")
		}
		manager := &SingBoxManager{Config: &config.Config{}, AppRoot: root}
		manager.Config.Browser.UserDataRoot = filepath.Join(root, "singbox")
		outbound := map[string]interface{}{
			"type":        "anytls",
			"tag":         "proxy-out",
			"server":      "node.example.com",
			"server_port": 443,
			"password":    "pw",
			"tls":         map[string]interface{}{"enabled": true, "server_name": "node.example.com"},
		}
		key := ComputeBridgeKey("anytls://pw@node.example.com:443", issue85DNSYAML, ProxyKernelSingBox)
		cfgPath, err := manager.buildConfig(key, outbound, 17890, issue85DNSYAML)
		if err != nil {
			t.Fatalf("buildConfig returned error: %v", err)
		}
		output, err := exec.Command(binary, "check", "-c", cfgPath).CombinedOutput()
		if err != nil {
			t.Fatalf("sing-box check 拒绝生成的配置: %v\n%s", err, output)
		}
	})

	t.Run("mihomo", func(t *testing.T) {
		binary := resolveBridgeEngineBinary("mihomo")
		if binary == "" {
			t.Skip("bin/ 下没有 mihomo，跳过引擎校验")
		}
		manager := NewClashManager(&config.Config{}, root)
		manager.Config.Browser.UserDataRoot = filepath.Join(root, "mihomo")
		node := map[string]interface{}{
			"name":     "node",
			"type":     "anytls",
			"server":   "node.example.com",
			"port":     443,
			"password": "pw",
		}
		key := ComputeBridgeKey("anytls://pw@node.example.com:443", issue85DNSYAML, ProxyKernelMihomo)
		cfgPath, err := manager.buildMihomoNodeConfig(key, node, 17890, 17891, issue85DNSYAML)
		if err != nil {
			t.Fatalf("buildMihomoNodeConfig returned error: %v", err)
		}
		output, err := exec.Command(binary, "-t", "-d", filepath.Dir(cfgPath), "-f", cfgPath).CombinedOutput()
		if err != nil {
			t.Fatalf("mihomo 配置校验拒绝生成的配置: %v\n%s", err, output)
		}
	})
}
