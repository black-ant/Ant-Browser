//go:build darwin

package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// extractDmgAndStripRoot 在 macOS 上解包 .dmg 内核镜像:
// 挂载镜像 → 把其中的 .app 应用包拷贝到 dest → 卸载镜像。
// 解包后 dest 直接包含 <App>.app,FindCoreExecutable 即可定位 Contents/MacOS/ 下的可执行文件,
// 与 zip/tar 解包后“dest 直接是内核根目录”的约定保持一致。
func extractDmgAndStripRoot(dmgPath, dest string, progressCb func(int, string)) error {
	if progressCb == nil {
		progressCb = func(int, string) {}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	mountDir, err := os.MkdirTemp("", "fpc-dmg-mount-*")
	if err != nil {
		return fmt.Errorf("创建挂载点失败: %w", err)
	}
	defer os.RemoveAll(mountDir)

	progressCb(10, "正在挂载 DMG 镜像...")
	attach := exec.Command("hdiutil", "attach", dmgPath,
		"-nobrowse", "-readonly", "-noverify", "-noautoopen",
		"-mountpoint", mountDir)
	if out, aErr := attach.CombinedOutput(); aErr != nil {
		return fmt.Errorf("挂载 DMG 失败: %v (%s)", aErr, strings.TrimSpace(string(out)))
	}
	defer func() {
		// 强制卸载,避免残留挂载点;失败忽略(RemoveAll 兜底清理挂载点目录本身)。
		_ = exec.Command("hdiutil", "detach", mountDir, "-force").Run()
	}()

	progressCb(40, "正在从镜像中查找应用包...")
	appPath, err := findAppBundleInDir(mountDir)
	if err != nil {
		return err
	}

	progressCb(60, "正在拷贝内核应用包...")
	targetApp := filepath.Join(dest, filepath.Base(appPath))
	// ditto 能正确保留 .app bundle 的符号链接与资源结构(cp -R 会破坏 Framework 里的版本软链)。
	if out, cErr := exec.Command("ditto", appPath, targetApp).CombinedOutput(); cErr != nil {
		return fmt.Errorf("拷贝应用包失败: %v (%s)", cErr, strings.TrimSpace(string(out)))
	}

	// 去除隔离属性,避免 Gatekeeper 拦截内核子进程启动
	// (本进程下载的文件通常本就无隔离属性,这里作兜底)。
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", targetApp).Run()

	progressCb(100, "DMG 解压完成！")
	return nil
}

// findAppBundleInDir 在挂载目录(含向下一层子目录)中查找第一个 .app 应用包,
// 跳过指向 /Applications 的软链等非应用条目。
func findAppBundleInDir(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	// 先在根层查找。
	for _, e := range entries {
		if isAppBundleEntry(root, e) {
			return filepath.Join(root, e.Name()), nil
		}
	}
	// 再向下找一层(部分镜像会把 .app 放进子目录)。
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(strings.ToLower(e.Name()), ".app") {
			continue
		}
		sub := filepath.Join(root, e.Name())
		subEntries, sErr := os.ReadDir(sub)
		if sErr != nil {
			continue
		}
		for _, se := range subEntries {
			if isAppBundleEntry(sub, se) {
				return filepath.Join(sub, se.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("DMG 镜像内未找到 .app 应用包")
}

// isAppBundleEntry 判断某目录项是否为真正的 .app 应用包(名字以 .app 结尾且是目录)。
// os.Stat 会跟随软链,因此 “Applications -> /Applications” 这类软链(名字不以 .app 结尾)会被排除。
func isAppBundleEntry(parent string, e os.DirEntry) bool {
	if !strings.HasSuffix(strings.ToLower(e.Name()), ".app") {
		return false
	}
	info, err := os.Stat(filepath.Join(parent, e.Name()))
	if err != nil {
		return false
	}
	return info.IsDir()
}
