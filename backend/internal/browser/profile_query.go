package browser

import (
	"ant-chrome/backend/internal/logger"
	"sort"
	"strings"
)

// List 获取配置列表
func (m *Manager) List() []Profile {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	list := make([]Profile, 0, len(m.Profiles))
	for _, profile := range m.Profiles {
		p := *profile
		if m.CodeProvider != nil {
			if code, err := m.CodeProvider.EnsureCode(p.ProfileId); err == nil {
				p.LaunchCode = code
			}
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ProfileId < list[j].ProfileId
	})
	log.Debug("浏览器配置列表查询", logger.F("count", len(list)))
	return list
}

// ListByTag 按标签筛选配置列表
func (m *Manager) ListByTag(tag string) []Profile {
	tag = strings.TrimSpace(tag)
	all := m.List()
	if tag == "" {
		return all
	}
	result := make([]Profile, 0)
	for _, p := range all {
		for _, t := range p.Tags {
			if strings.EqualFold(t, tag) {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// GetAllTags 获取所有已使用的标签（去重排序）
func (m *Manager) GetAllTags() []string {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	seen := make(map[string]struct{})
	for _, p := range m.Profiles {
		for _, t := range p.Tags {
			t = strings.TrimSpace(t)
			if t != "" {
				seen[t] = struct{}{}
			}
		}
	}
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}
