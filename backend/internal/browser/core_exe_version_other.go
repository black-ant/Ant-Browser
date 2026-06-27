//go:build !windows

package browser

// readExecutableProductVersion 在非 Windows 平台无 PE 版本资源，返回空串，
// 由调用方回退到 manifest.json / *.manifest 文件名取版本。
func readExecutableProductVersion(string) string {
	return ""
}
