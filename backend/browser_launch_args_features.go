package backend

import "strings"

// mergeFeatureSwitches 把重复的 --disable-features/--enable-features 合并成各一条。
//
// Chromium 对同名开关只取最后一条。指纹层、省内存层、直播性能层、用户自定义 LaunchArgs
// 若各带一条 --disable-features=,前面的会被静默覆盖 —— 不合并则叠加层不可靠。
func mergeFeatureSwitches(args []string) []string {
	return mergeValueSwitch(mergeValueSwitch(args, "--disable-features="), "--enable-features=")
}

// mergeValueSwitch 把所有以 prefix 开头的逗号分隔值开关合并成一条,
// 保留第一条出现的位置,值去重且保序。
func mergeValueSwitch(args []string, prefix string) []string {
	values := make([]string, 0, 8)
	seen := map[string]struct{}{}
	firstIdx := -1
	out := make([]string, 0, len(args))

	for _, arg := range args {
		if !strings.HasPrefix(arg, prefix) {
			out = append(out, arg)
			continue
		}
		if firstIdx == -1 {
			firstIdx = len(out)
			out = append(out, "") // 占位,最后回填合并结果
		}
		for _, v := range strings.Split(strings.TrimPrefix(arg, prefix), ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			values = append(values, v)
		}
	}

	if firstIdx == -1 {
		return args
	}
	if len(values) == 0 {
		// 只有空值开关(如 --disable-features=),整条丢弃
		return append(out[:firstIdx], out[firstIdx+1:]...)
	}
	out[firstIdx] = prefix + strings.Join(values, ",")
	return out
}
