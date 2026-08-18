package browser

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/identity"
)

// 内核版本选择模式常量。批量/单个新建时由前端传入,决定本批实例在多内核间的分配方式。
const (
	// KernelSelectAuto:按 config.KernelDistribution 加权自动分配(默认,148 为主、144 少数)。
	KernelSelectAuto = "auto"
	// KernelSelectAll148:全部用大版本为 148 的内核(若内置则强制 148)。
	KernelSelectAll148 = "all148"
	// KernelSelectAll144:全部用大版本为 144 的内核。
	KernelSelectAll144 = "all144"
)

// resolveProfileCoreForKernelSelect 按"内核版本选择模式"解析本实例应使用的内核 CoreId。
//   - auto:按 config.Browser.KernelDistribution 加权,在已注册内核里挑一个(major→coreId 映射);
//     序号 index 用于确定性分配(一批内可复现、可测 ~70/30),同一内核多实例分布稳定。
//   - all148 / all144:强制选对应大版本内核;若该版本内核未注册,回退到默认内核并返回错误,
//     调用方按"指定却失败"语义硬失败(与指定平台但池空一致),不回退成宿主默认伪造。
//   - 空串视为 auto。
//
// 返回 (coreId, error)。coreId 为空表示走默认内核(无可注册内核时)。
func (m *Manager) resolveProfileCoreForKernelSelect(kernelSelect string, index int) (string, error) {
	kernelSelect = strings.ToLower(strings.TrimSpace(kernelSelect))
	if kernelSelect == "" {
		kernelSelect = KernelSelectAuto
	}

	majorToCore := m.coreMajorToCoreIDMap()

	switch kernelSelect {
	case KernelSelectAuto:
		// 确定性加权:按 index 落到累计权重区间。
		dist := m.distributionOrDefault()
		if len(majorToCore) == 0 || len(dist) == 0 {
			return "", nil // 无内核或无分布:交给默认内核路径
		}
		total := 0
		// 只对"已注册内核"的权重求和,避免配置里有但内核未注册导致永远选不到。
		for major, w := range dist {
			if _, ok := majorToCore[major]; ok && w > 0 {
				total += w
			}
		}
		if total <= 0 {
			return "", nil
		}
		pick := index % total // 确定性:同一 index 总落同一点 → 一批内 ~按权重比例分布
		acc := 0
		// 按 major 降序遍历(148 优先)以保证确定性顺序。
		majors := sortedMajorKeysDesc(dist)
		for _, major := range majors {
			w := dist[major]
			coreId, ok := majorToCore[major]
			if !ok || w <= 0 {
				continue
			}
			acc += w
			if pick < acc {
				return coreId, nil
			}
		}
		return "", nil

	case KernelSelectAll148, KernelSelectAll144:
		want := 148
		if kernelSelect == KernelSelectAll144 {
			want = 144
		}
		if coreId, ok := majorToCore[fmt.Sprintf("%d", want)]; ok {
			return coreId, nil
		}
		return "", fmt.Errorf("内置内核中没有大版本 %d 的内核,无法按 %s 分配", want, kernelSelect)

	default:
		return "", fmt.Errorf("不支持的内核选择模式: %q(支持 auto/all148/all144)", kernelSelect)
	}
}

// coreMajorToCoreIDMap 返回"内核大版本字符串 → CoreId"映射(按已注册内核解析版本)。
// 多个内核同版本时取首个;无法解析版本的内核不计入。
func (m *Manager) coreMajorToCoreIDMap() map[string]string {
	out := make(map[string]string)
	for _, core := range m.ListCores() {
		major := identity.MajorFromVersion(m.GetChromeVersion(core.CorePath))
		if major <= 0 {
			continue
		}
		key := fmt.Sprintf("%d", major)
		if _, exists := out[key]; !exists {
			out[key] = core.CoreId
		}
	}
	return out
}

func (m *Manager) distributionOrDefault() map[string]int {
	if m.Config != nil && len(m.Config.Browser.KernelDistribution) > 0 {
		return m.Config.Browser.KernelDistribution
	}
	return map[string]int{"148": 70, "144": 30}
}

// sortedMajorKeysDesc 返回权重表的 key(数字串)按数值降序排列,保证遍历确定性。
func sortedMajorKeysDesc(dist map[string]int) []string {
	type mk struct {
		k string
		v int
	}
	all := make([]mk, 0, len(dist))
	for k := range dist {
		all = append(all, mk{k: k})
	}
	// 简单插入排序(map 元素少,避免额外依赖)。
	for i := 1; i < len(all); i++ {
		for j := i; j > 0; j-- {
			if identity.MajorFromVersion(all[j].k) > identity.MajorFromVersion(all[j-1].k) {
				all[j], all[j-1] = all[j-1], all[j]
			}
		}
	}
	out := make([]string, len(all))
	for i, a := range all {
		out[i] = a.k
	}
	return out
}
