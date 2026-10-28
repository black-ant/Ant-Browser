package browser

import (
	"fmt"
	"strings"
)

type resolvedProfileProxyInput struct {
	ProxyMode          string
	ProxyGroupName     string
	ProxyId            string
	ProxyConfig        string
	SelectedProxy    Proxy
	HasSelectedProxy bool
	FallbackToDirect bool
}

func (m *Manager) resolveProfileProxyInput(input ProfileInput) (resolvedProfileProxyInput, error) {
	proxyId := strings.TrimSpace(input.ProxyId)
	proxyConfig := strings.TrimSpace(input.ProxyConfig)
	mode := strings.ToLower(strings.TrimSpace(input.ProxyMode))
	if mode == "" {
		if proxyId == "" && proxyConfig != "" {
			mode = ProxyModeLocal
		} else {
			mode = ProxyModeFixed
		}
	}
	mode = NormalizeProxyMode(mode)
	groupName := NormalizeProxyGroupName(input.ProxyGroupName)

	if mode == ProxyModeDynamic {
		return resolvedProfileProxyInput{
			ProxyMode:      mode,
			ProxyGroupName: groupName,
		}, nil
	}

	if mode == ProxyModeLocal {
		if proxyConfig == "" {
			return resolvedProfileProxyInput{}, fmt.Errorf("本地代理地址不能为空")
		}
		return resolvedProfileProxyInput{
			ProxyMode:   mode,
			ProxyConfig: proxyConfig,
		}, nil
	}

	if proxyId != "" {
		if proxyItem, ok := m.GetProxyByID(proxyId); ok {
			return resolvedProfileProxyInput{
				ProxyMode:        ProxyModeFixed,
				ProxyId:          strings.TrimSpace(proxyItem.ProxyId),
				ProxyConfig:      strings.TrimSpace(proxyItem.ProxyConfig),
				SelectedProxy:    proxyItem,
				HasSelectedProxy: true,
			}, nil
		}
		// 虚拟分组绑定：保留分组 ID，代理配置在实例启动时按成员策略解析。
		if group, ok := m.getProxyVirtualGroupByIDOrName(proxyId); ok {
			return resolvedProfileProxyInput{
				ProxyMode: ProxyModeFixed,
				ProxyId:   strings.TrimSpace(group.GroupId),
			}, nil
		}
		// 代理 ID 找不到时不允许丢弃 ID 改用配置：那会把「池节点绑定」静默降级为裸配置，
		// 之后订阅刷新也无法再通过绑定快照找回节点。
		return resolvedProfileProxyInput{}, fmt.Errorf("代理ID不存在（proxy id not found: %s）。请重新选择代理后再保存（不会自动降级为代理配置或直连）", proxyId)
	}

	if proxyConfig != "" {
		return resolvedProfileProxyInput{
			ProxyMode:   ProxyModeFixed,
			ProxyId:     "",
			ProxyConfig: proxyConfig,
		}, nil
	}

	return resolvedProfileProxyInput{
		ProxyMode:        ProxyModeFixed,
		ProxyId:          "",
		ProxyConfig:      "",
		FallbackToDirect: true,
	}, nil
}

// getProxyVirtualGroupByIDOrName 按分组 ID（或名称）识别虚拟分组绑定。
func (m *Manager) getProxyVirtualGroupByIDOrName(key string) (ProxyVirtualGroup, bool) {
	key = strings.TrimSpace(key)
	if key == "" || m == nil || m.ProxyVirtualGroupDAO == nil {
		return ProxyVirtualGroup{}, false
	}
	groups, err := m.ProxyVirtualGroupDAO.List()
	if err != nil {
		return ProxyVirtualGroup{}, false
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.GroupId), key) {
			return group, true
		}
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.GroupName), key) {
			return group, true
		}
	}
	return ProxyVirtualGroup{}, false
}
