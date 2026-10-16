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
	SelectedProxy      Proxy
	HasSelectedProxy   bool
	FallbackToDirect   bool
	UsedConfigFallback bool
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
		if proxyConfig != "" {
			return resolvedProfileProxyInput{
				ProxyMode:          ProxyModeFixed,
				ProxyId:            "",
				ProxyConfig:        proxyConfig,
				UsedConfigFallback: true,
			}, nil
		}
		return resolvedProfileProxyInput{}, fmt.Errorf("代理ID不存在（proxy id not found: %s），且未提供 proxyConfig", proxyId)
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
