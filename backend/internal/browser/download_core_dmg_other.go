//go:build !darwin

package browser

import "fmt"

// extractDmgAndStripRoot 在非 macOS 平台不受支持:.dmg 是 macOS 专有磁盘镜像,
// 依赖 hdiutil 挂载。前端的资产推荐逻辑只会在 darwin 上选择 .dmg,
// 因此其它平台正常情况下不会走到这里;若确实收到 .dmg 则给出明确错误。
func extractDmgAndStripRoot(_, _ string, _ func(int, string)) error {
	return fmt.Errorf(".dmg 镜像仅支持在 macOS 上解压")
}
