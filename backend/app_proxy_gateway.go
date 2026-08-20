package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/gateway"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (a *App) prepareProfileProxyGateway(input browserStartInput, profile *BrowserProfile) (gateway.Status, error) {
	proxyID, proxyConfig, proxies, err := a.resolveGatewayProxySelection(input, profile)
	if err != nil {
		return gateway.Status{}, err
	}
	client, err := a.ensureProxyGatewayClient()
	if err != nil {
		return gateway.Status{}, fmt.Errorf("实例启动失败：本地代理网关启动失败：%w", err)
	}
	connectorType := config.BrowserConnectorXray
	if a.config != nil {
		connectorType = config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
	}
	status, err := client.prepare(proxyGatewayProfileRequest{
		ProfileID:     input.ProfileID,
		ProxyID:       proxyID,
		ProxyConfig:   proxyConfig,
		Proxies:       proxies,
		ConnectorType: connectorType,
		Routing:       a.proxyRoutingConfig(input.ProfileID),
	})
	if err != nil {
		return gateway.Status{}, fmt.Errorf("实例启动失败：代理网关准备失败：%w", err)
	}
	if strings.TrimSpace(status.ProxyURL) == "" {
		a.stopProfileGateway(input.ProfileID)
		return gateway.Status{}, fmt.Errorf("实例启动失败：代理网关未返回本地端口")
	}
	return status, nil
}

func (a *App) resolveGatewayProxySelection(input browserStartInput, profile *BrowserProfile) (string, string, []BrowserProxy, error) {
	proxies := a.getLatestProxies()
	if input.ForceDirectProxy {
		return temporaryDirectProxyID, "direct://", proxies, nil
	}
	proxyID := strings.TrimSpace(profile.ProxyId)
	proxyConfig := strings.TrimSpace(profile.ProxyConfig)
	if input.hasTemporaryProxy() {
		resolvedID, resolvedConfig, err := resolveTemporaryBrowserStartProxy(input.TemporaryProxyID, input.TemporaryProxyConfig, proxies)
		return resolvedID, resolvedConfig, proxies, err
	}
	if proxyID != "" {
		for _, item := range proxies {
			if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyID) {
				return strings.TrimSpace(item.ProxyId), strings.TrimSpace(item.ProxyConfig), proxies, nil
			}
		}
	}
	return proxyID, proxyConfig, proxies, nil
}

func (a *App) proxyRoutingConfig(profileID string) gateway.RoutingConfig {
	fallback := gateway.RoutingConfig{Mode: gateway.ModeProxy}
	if a == nil || a.db == nil || strings.TrimSpace(profileID) == "" {
		return fallback
	}
	var mode string
	var rulesJSON string
	err := a.db.GetConn().QueryRow(`SELECT mode, rules_json FROM browser_profile_proxy_routing WHERE profile_id = ?`, strings.TrimSpace(profileID)).Scan(&mode, &rulesJSON)
	if err == sql.ErrNoRows || err != nil {
		return fallback
	}
	var rules []gateway.Rule
	if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
		return fallback
	}
	return gateway.NormalizeRoutingConfig(gateway.RoutingConfig{Mode: mode, Rules: rules})
}

func (a *App) saveProxyRoutingConfig(profileID string, input gateway.RoutingConfig) (gateway.RoutingConfig, error) {
	routing := gateway.NormalizeRoutingConfig(input)
	if a == nil || a.db == nil {
		return routing, fmt.Errorf("数据库未初始化")
	}
	rulesJSON, err := json.Marshal(routing.Rules)
	if err != nil {
		return routing, err
	}
	_, err = a.db.GetConn().Exec(`
		INSERT INTO browser_profile_proxy_routing (profile_id, mode, rules_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			mode = excluded.mode,
			rules_json = excluded.rules_json,
			updated_at = excluded.updated_at
	`, strings.TrimSpace(profileID), routing.Mode, string(rulesJSON), time.Now().Format(time.RFC3339))
	return routing, err
}

func (a *App) deleteProxyRoutingConfig(profileID string) {
	if a == nil || a.db == nil {
		return
	}
	_, _ = a.db.GetConn().Exec(`DELETE FROM browser_profile_proxy_routing WHERE profile_id = ?`, strings.TrimSpace(profileID))
}

func (a *App) attachProfileGateway(profileID string, pid int, debugPort int) {
	a.proxyGatewayMu.Lock()
	client := a.proxyGateway
	a.proxyGatewayMu.Unlock()
	if client != nil {
		_ = client.attach(profileID, pid, debugPort)
	}
}

func (a *App) switchProfileGateway(profileID string, proxyID string, proxyConfig string, force bool) (gateway.Status, error) {
	profileID = strings.TrimSpace(profileID)
	proxies := a.getLatestProxies()
	resolvedID, resolvedConfig, err := resolveTemporaryBrowserStartProxy(proxyID, proxyConfig, proxies)
	if err != nil {
		return gateway.Status{}, err
	}
	client, err := a.ensureProxyGatewayClient()
	if err != nil {
		return gateway.Status{}, err
	}
	connectorType := config.BrowserConnectorXray
	if a.config != nil {
		connectorType = config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
	}
	return client.prepare(proxyGatewayProfileRequest{
		ProfileID:     profileID,
		ProxyID:       resolvedID,
		ProxyConfig:   resolvedConfig,
		Proxies:       proxies,
		ConnectorType: connectorType,
		Routing:       a.proxyRoutingConfig(profileID),
		Force:         force,
	})
}

func (a *App) profileForGatewayUpdate(profileID string) *BrowserProfile {
	if a == nil || a.browserMgr == nil {
		return nil
	}
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	return copyBrowserProfileSnapshot(a.browserMgr.Profiles[strings.TrimSpace(profileID)])
}

func browserProfileInputWithProxy(profile *BrowserProfile, proxyID string, proxyConfig string) BrowserProfileInput {
	return BrowserProfileInput{
		ProfileName:        profile.ProfileName,
		UserDataDir:        profile.UserDataDir,
		CoreId:             profile.CoreId,
		RestoreLastSession: profile.RestoreLastSession,
		FingerprintArgs:    append([]string{}, profile.FingerprintArgs...),
		ProxyId:            proxyID,
		ProxyConfig:        proxyConfig,
		MemoryLimitMB:      profile.MemoryLimitMB,
		LaunchArgs:         append([]string{}, profile.LaunchArgs...),
		Tags:               append([]string{}, profile.Tags...),
		Keywords:           append([]string{}, profile.Keywords...),
		GroupId:            profile.GroupId,
	}
}
