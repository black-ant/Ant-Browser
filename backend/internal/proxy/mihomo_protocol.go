package proxy

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func IsMihomoOnlyProtocol(proxyConfig string) bool {
	return mihomoOnlyProtocolType(proxyConfig) != ""
}

func mihomoOnlyProtocolType(proxyConfig string) string {
	nodeType := clashNodeType(proxyConfig)
	switch nodeType {
	case "mieru", "wireguard":
		return nodeType
	case "ss", "shadowsocks":
		if clashNodeHasPlugin(proxyConfig) {
			return nodeType
		}
	}
	return ""
}

func validateMihomoOnlyProtocol(proxyConfig string) error {
	src := strings.TrimSpace(proxyConfig)
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		return fmt.Errorf("YAML 解析失败: %w", err)
	}
	node := pickClashNode(payload)
	if node == nil {
		return fmt.Errorf("mihomo 节点解析失败")
	}
	nodeType := strings.ToLower(strings.TrimSpace(getMapString(node, "type")))
	if getMapString(node, "server") == "" {
		return fmt.Errorf("%s 节点缺少 server", nodeType)
	}
	if getMapInt(node, "port") == 0 {
		return fmt.Errorf("%s 节点缺少 port", nodeType)
	}
	return nil
}

func clashNodeHasPlugin(proxyConfig string) bool {
	src := strings.TrimSpace(proxyConfig)
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		return false
	}
	node := pickClashNode(payload)
	if node == nil {
		return false
	}
	return getMapString(node, "plugin") != ""
}

func clashNodeType(proxyConfig string) string {
	src := strings.TrimSpace(proxyConfig)
	if src == "" || (!strings.Contains(strings.ToLower(src), "type:") && !strings.Contains(strings.ToLower(src), "proxies:")) {
		return ""
	}
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		// 历史损坏数据（缩进错位）做一次规范化修复重试，保证协议可识别。
		if repaired, ok := repairClashNodeYAML(src); ok {
			if retryErr := yaml.Unmarshal([]byte(repaired), &payload); retryErr != nil {
				return ""
			}
		} else {
			return ""
		}
	}
	node := pickClashNode(payload)
	if node == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(getMapString(node, "type")))
}
