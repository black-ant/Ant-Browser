package browser

// BuildLaunchArgs 把默认起始页 URL 追加到启动参数末尾。
// startURLs 为空时不追加任何内容（窗口启动落到浏览器默认新标签页），
// 避免每次启动都自动外联第三方 IP 检测站点而暴露行为特征。
func BuildLaunchArgs(args []string, startURLs []string) []string {
	for _, u := range startURLs {
		if u != "" {
			args = append(args, u)
		}
	}
	return args
}
