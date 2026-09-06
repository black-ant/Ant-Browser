package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
)

const (
	profilePackageVersion         = 2
	profilePackageDatabasePath    = "database.json"
	profilePackageDatabaseFormat  = "ant-chrome-profile-database"
	profilePackageDatabaseVersion = 1
)

// ProfilePackageDatabase 保存实例及其数据库关联数据，不包含运行时 launch code。
type ProfilePackageDatabase struct {
	Format                   string                                    `json:"format"`
	Version                  int                                       `json:"version"`
	Profiles                 []browser.Profile                         `json:"profiles"`
	Groups                   []ProfilePackageDatabaseGroup             `json:"groups"`
	Cores                    []ProfilePackageDatabaseCore              `json:"cores"`
	Proxies                  []ProfilePackageDatabaseProxy             `json:"proxies"`
	Extensions               []browser.Extension                       `json:"extensions"`
	ProfileExtensionSettings []ProfilePackageDatabaseExtensionSettings `json:"profileExtensionSettings"`
	ProfileExtensions        []ProfilePackageDatabaseExtensionBinding  `json:"profileExtensions"`
	ProfileExtensionRuntime  []browser.ProfileExtensionRuntime         `json:"profileExtensionRuntime"`
	Warnings                 []string                                  `json:"warnings,omitempty"`
}

type ProfilePackageDatabaseGroup struct {
	GroupId   string `json:"groupId"`
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ProfilePackageDatabaseCore struct {
	CoreId    string `json:"coreId"`
	CoreName  string `json:"coreName"`
	CorePath  string `json:"corePath"`
	IsDefault bool   `json:"isDefault"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

type ProfilePackageDatabaseProxy struct {
	browser.Proxy
	CreatedAt string `json:"createdAt,omitempty"`
}

type ProfilePackageDatabaseExtensionSettings struct {
	ProfileID  string `json:"profileId"`
	Configured bool   `json:"configured"`
	UpdatedAt  string `json:"updatedAt"`
}

type ProfilePackageDatabaseExtensionBinding struct {
	ProfileID   string `json:"profileId"`
	ExtensionID string `json:"extensionId"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func (a *App) collectProfilePackageDatabase(profiles []browser.Profile) (ProfilePackageDatabase, error) {
	snapshot := ProfilePackageDatabase{
		Format:                   profilePackageDatabaseFormat,
		Version:                  profilePackageDatabaseVersion,
		Profiles:                 make([]browser.Profile, 0, len(profiles)),
		Groups:                   []ProfilePackageDatabaseGroup{},
		Cores:                    []ProfilePackageDatabaseCore{},
		Proxies:                  []ProfilePackageDatabaseProxy{},
		Extensions:               []browser.Extension{},
		ProfileExtensionSettings: []ProfilePackageDatabaseExtensionSettings{},
		ProfileExtensions:        []ProfilePackageDatabaseExtensionBinding{},
		ProfileExtensionRuntime:  []browser.ProfileExtensionRuntime{},
	}
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return ProfilePackageDatabase{}, fmt.Errorf("数据库未初始化，无法导出实例关联数据")
	}

	profileIDs := make([]string, 0, len(profiles))
	seenProfiles := make(map[string]struct{}, len(profiles))
	for _, source := range profiles {
		profile := source
		profile.LaunchCode = ""
		profile.Running = false
		profile.DebugPort = 0
		profile.DebugReady = false
		profile.Pid = 0
		profile.RuntimeWarning = ""
		profile.LastError = ""
		profile.WindowMarkerCode = ""
		profile.LastStartAt = ""
		profile.LastStopAt = ""
		snapshot.Profiles = append(snapshot.Profiles, profile)
		id := strings.TrimSpace(profile.ProfileId)
		if id == "" {
			return ProfilePackageDatabase{}, fmt.Errorf("实例缺少 profileId")
		}
		key := backupImportIDKey(id)
		if _, exists := seenProfiles[key]; exists {
			continue
		}
		seenProfiles[key] = struct{}{}
		profileIDs = append(profileIDs, id)
	}

	conn := a.db.GetConn()
	groupIDs := make([]string, 0)
	coreIDs := make([]string, 0)
	proxyIDs := make([]string, 0)
	seenGroups := map[string]struct{}{}
	seenCores := map[string]struct{}{}
	seenProxies := map[string]struct{}{}
	for _, profile := range snapshot.Profiles {
		if id := strings.TrimSpace(profile.GroupId); id != "" {
			key := backupImportIDKey(id)
			if _, exists := seenGroups[key]; !exists {
				seenGroups[key] = struct{}{}
				groupIDs = append(groupIDs, id)
			}
		}
		if id := strings.TrimSpace(profile.CoreId); id != "" {
			key := backupImportIDKey(id)
			if _, exists := seenCores[key]; !exists {
				seenCores[key] = struct{}{}
				coreIDs = append(coreIDs, id)
			}
		}
		if id := strings.TrimSpace(profile.ProxyId); id != "" {
			key := backupImportIDKey(id)
			if _, exists := seenProxies[key]; !exists {
				seenProxies[key] = struct{}{}
				proxyIDs = append(proxyIDs, id)
			}
		}
	}

	groups, warnings, err := queryProfilePackageGroups(conn, groupIDs)
	if err != nil {
		return ProfilePackageDatabase{}, err
	}
	snapshot.Groups = groups
	snapshot.Warnings = append(snapshot.Warnings, warnings...)

	cores, warnings, err := queryProfilePackageCores(conn, coreIDs)
	if err != nil {
		return ProfilePackageDatabase{}, err
	}
	snapshot.Cores = cores
	snapshot.Warnings = append(snapshot.Warnings, warnings...)

	proxies, warnings, err := queryProfilePackageProxies(conn, proxyIDs)
	if err != nil {
		return ProfilePackageDatabase{}, err
	}
	snapshot.Proxies = proxies
	snapshot.Warnings = append(snapshot.Warnings, warnings...)

	extensionIDs := make(map[string]struct{})
	for _, profileID := range profileIDs {
		var configured int
		var updatedAt string
		err := conn.QueryRow(`SELECT configured, updated_at FROM browser_profile_extension_settings WHERE profile_id = ?`, profileID).Scan(&configured, &updatedAt)
		if err == nil {
			snapshot.ProfileExtensionSettings = append(snapshot.ProfileExtensionSettings, ProfilePackageDatabaseExtensionSettings{
				ProfileID:  profileID,
				Configured: configured != 0,
				UpdatedAt:  updatedAt,
			})
		} else if !errors.Is(err, sql.ErrNoRows) {
			return ProfilePackageDatabase{}, fmt.Errorf("读取实例插件设置失败(%s): %w", profileID, err)
		}

		bindings, err := queryProfilePackageExtensionBindings(conn, profileID)
		if err != nil {
			return ProfilePackageDatabase{}, err
		}
		for _, binding := range bindings {
			snapshot.ProfileExtensions = append(snapshot.ProfileExtensions, binding)
			if id := strings.TrimSpace(binding.ExtensionID); id != "" {
				extensionIDs[backupImportIDKey(id)] = struct{}{}
			}
		}

		runtimeStates, err := queryProfilePackageExtensionRuntime(conn, profileID)
		if err != nil {
			return ProfilePackageDatabase{}, err
		}
		for _, runtimeState := range runtimeStates {
			snapshot.ProfileExtensionRuntime = append(snapshot.ProfileExtensionRuntime, runtimeState)
			if id := strings.TrimSpace(runtimeState.ExtensionID); id != "" {
				extensionIDs[backupImportIDKey(id)] = struct{}{}
			}
		}
	}

	extensionKeys := make([]string, 0, len(extensionIDs))
	for key := range extensionIDs {
		extensionKeys = append(extensionKeys, key)
	}
	sort.Strings(extensionKeys)
	extensionDAO := browser.NewSQLiteExtensionDAO(conn)
	for _, key := range extensionKeys {
		extension, err := extensionDAO.Get(key)
		if err == nil {
			snapshot.Extensions = append(snapshot.Extensions, extension)
			continue
		}
		if errors.Is(err, sql.ErrNoRows) {
			snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("插件「%s」记录不存在，未导出插件定义", key))
			continue
		}
		return ProfilePackageDatabase{}, fmt.Errorf("读取插件定义失败(%s): %w", key, err)
	}

	sort.Slice(snapshot.Groups, func(i, j int) bool {
		return backupImportIDKey(snapshot.Groups[i].GroupId) < backupImportIDKey(snapshot.Groups[j].GroupId)
	})
	sort.Slice(snapshot.Cores, func(i, j int) bool {
		return backupImportIDKey(snapshot.Cores[i].CoreId) < backupImportIDKey(snapshot.Cores[j].CoreId)
	})
	sort.Slice(snapshot.Proxies, func(i, j int) bool {
		return backupImportIDKey(snapshot.Proxies[i].ProxyId) < backupImportIDKey(snapshot.Proxies[j].ProxyId)
	})
	sort.Slice(snapshot.Extensions, func(i, j int) bool {
		return backupImportIDKey(snapshot.Extensions[i].ExtensionID) < backupImportIDKey(snapshot.Extensions[j].ExtensionID)
	})
	sort.Slice(snapshot.ProfileExtensionSettings, func(i, j int) bool {
		return backupImportIDKey(snapshot.ProfileExtensionSettings[i].ProfileID) < backupImportIDKey(snapshot.ProfileExtensionSettings[j].ProfileID)
	})
	sort.Slice(snapshot.ProfileExtensions, func(i, j int) bool {
		left := backupImportIDKey(snapshot.ProfileExtensions[i].ProfileID) + "\x00" + backupImportIDKey(snapshot.ProfileExtensions[i].ExtensionID)
		right := backupImportIDKey(snapshot.ProfileExtensions[j].ProfileID) + "\x00" + backupImportIDKey(snapshot.ProfileExtensions[j].ExtensionID)
		return left < right
	})
	sort.Slice(snapshot.ProfileExtensionRuntime, func(i, j int) bool {
		left := backupImportIDKey(snapshot.ProfileExtensionRuntime[i].ProfileID) + "\x00" + backupImportIDKey(snapshot.ProfileExtensionRuntime[i].ExtensionID)
		right := backupImportIDKey(snapshot.ProfileExtensionRuntime[j].ProfileID) + "\x00" + backupImportIDKey(snapshot.ProfileExtensionRuntime[j].ExtensionID)
		return left < right
	})
	return snapshot, nil
}

func queryProfilePackageGroups(conn *sql.DB, groupIDs []string) ([]ProfilePackageDatabaseGroup, []string, error) {
	result := make([]ProfilePackageDatabaseGroup, 0)
	warnings := make([]string, 0)
	seen := make(map[string]struct{}, len(groupIDs))
	pending := append([]string{}, groupIDs...)
	for index := 0; index < len(pending); index++ {
		groupID := strings.TrimSpace(pending[index])
		key := backupImportIDKey(groupID)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		var group ProfilePackageDatabaseGroup
		err := conn.QueryRow(`
			SELECT group_id, group_name, COALESCE(parent_id, ''), sort_order, created_at, updated_at
			FROM browser_groups WHERE group_id = ?`, groupID).Scan(
			&group.GroupId, &group.GroupName, &group.ParentId, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			warnings = append(warnings, fmt.Sprintf("实例关联的分组「%s」不存在，未导出该分组", groupID))
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("读取分组失败(%s): %w", groupID, err)
		}
		result = append(result, group)
		if parentID := strings.TrimSpace(group.ParentId); parentID != "" {
			pending = append(pending, parentID)
		}
	}
	return result, warnings, nil
}

func queryProfilePackageCores(conn *sql.DB, coreIDs []string) ([]ProfilePackageDatabaseCore, []string, error) {
	result := make([]ProfilePackageDatabaseCore, 0, len(coreIDs))
	warnings := make([]string, 0)
	for _, coreID := range coreIDs {
		var core ProfilePackageDatabaseCore
		var isDefault int
		err := conn.QueryRow(`
			SELECT core_id, core_name, core_path, is_default, sort_order, created_at
			FROM browser_cores WHERE core_id = ?`, coreID).Scan(
			&core.CoreId, &core.CoreName, &core.CorePath, &isDefault, &core.SortOrder, &core.CreatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			warnings = append(warnings, fmt.Sprintf("实例关联的内核「%s」不存在，未导出该内核", coreID))
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("读取内核失败(%s): %w", coreID, err)
		}
		core.IsDefault = isDefault != 0
		result = append(result, core)
	}
	return result, warnings, nil
}

func queryProfilePackageProxies(conn *sql.DB, proxyIDs []string) ([]ProfilePackageDatabaseProxy, []string, error) {
	result := make([]ProfilePackageDatabaseProxy, 0, len(proxyIDs))
	warnings := make([]string, 0)
	for _, proxyID := range proxyIDs {
		var proxy ProfilePackageDatabaseProxy
		var autoRefresh, testOK int
		err := conn.QueryRow(`
			SELECT proxy_id, proxy_name, proxy_config, COALESCE(preferred_kernel, ''), dns_servers, COALESCE(group_name, ''),
			       COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''),
			       COALESCE(source_auto_refresh, 0), COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
			       COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''),
			       COALESCE(last_ip_health_json, ''), sort_order, created_at
			FROM browser_proxies WHERE proxy_id = ?`, proxyID).Scan(
			&proxy.ProxyId, &proxy.ProxyName, &proxy.ProxyConfig, &proxy.PreferredKernel, &proxy.DnsServers, &proxy.GroupName,
			&proxy.SourceID, &proxy.SourceURL, &proxy.SourceNamePrefix, &autoRefresh, &proxy.SourceRefreshIntervalM, &proxy.SourceLastRefreshAt,
			&proxy.LastLatencyMs, &testOK, &proxy.LastTestedAt, &proxy.LastIPHealthJSON, &proxy.SortOrder, &proxy.CreatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			warnings = append(warnings, fmt.Sprintf("实例关联的代理「%s」不存在，未导出该代理", proxyID))
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("读取代理失败(%s): %w", proxyID, err)
		}
		proxy.SourceAutoRefresh = autoRefresh != 0
		proxy.LastTestOk = testOK != 0
		result = append(result, proxy)
	}
	return result, warnings, nil
}

func queryProfilePackageExtensionBindings(conn *sql.DB, profileID string) ([]ProfilePackageDatabaseExtensionBinding, error) {
	rows, err := conn.Query(`
		SELECT profile_id, extension_id, enabled, created_at, updated_at
		FROM browser_profile_extensions WHERE profile_id = ? ORDER BY extension_id ASC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("读取实例插件绑定失败(%s): %w", profileID, err)
	}
	defer rows.Close()
	result := make([]ProfilePackageDatabaseExtensionBinding, 0)
	for rows.Next() {
		var item ProfilePackageDatabaseExtensionBinding
		var enabled int
		if err := rows.Scan(&item.ProfileID, &item.ExtensionID, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		result = append(result, item)
	}
	return result, rows.Err()
}

func queryProfilePackageExtensionRuntime(conn *sql.DB, profileID string) ([]browser.ProfileExtensionRuntime, error) {
	rows, err := conn.Query(`
		SELECT profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status,
		       backup_path, last_verified_at, last_error, created_at, updated_at
		FROM browser_profile_extension_runtime WHERE profile_id = ? ORDER BY extension_id ASC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("读取实例插件运行态失败(%s): %w", profileID, err)
	}
	defer rows.Close()
	result := make([]browser.ProfileExtensionRuntime, 0)
	for rows.Next() {
		var item browser.ProfileExtensionRuntime
		if err := rows.Scan(
			&item.ProfileID, &item.ExtensionID, &item.RuntimeExtensionID, &item.InstallMode, &item.InstalledVersion,
			&item.PackageHash, &item.Status, &item.BackupPath, &item.LastVerifiedAt, &item.LastError,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (a *App) restoreProfilePackageDatabase(snapshot ProfilePackageDatabase, prepared []preparedProfilePackageImport, warnings *[]string) error {
	if snapshot.Format != profilePackageDatabaseFormat || snapshot.Version != profilePackageDatabaseVersion {
		return fmt.Errorf("不支持的实例数据库快照格式")
	}
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化，无法恢复实例关联数据")
	}
	if len(snapshot.Profiles) != len(prepared) {
		return fmt.Errorf("实例数据库快照与实例配置数量不一致")
	}

	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return fmt.Errorf("开启实例数据库恢复事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	profileMappings := make(map[string]string, len(prepared))
	for index, item := range prepared {
		oldID := strings.TrimSpace(item.OldProfileID)
		if oldID == "" {
			return fmt.Errorf("实例数据库快照包含空 profileId")
		}
		if !strings.EqualFold(oldID, strings.TrimSpace(snapshot.Profiles[index].ProfileId)) {
			return fmt.Errorf("实例数据库快照与实例配置顺序不一致")
		}
		profileMappings[backupImportIDKey(oldID)] = item.Profile.ProfileId
	}
	for _, item := range prepared {
		if !item.Overwrite {
			continue
		}
		for _, statement := range []string{
			`DELETE FROM browser_profile_extension_runtime WHERE profile_id = ?`,
			`DELETE FROM browser_profile_extensions WHERE profile_id = ?`,
			`DELETE FROM browser_profile_extension_settings WHERE profile_id = ?`,
		} {
			if _, err := tx.Exec(statement, item.Profile.ProfileId); err != nil {
				return fmt.Errorf("清理被覆盖实例关联数据失败(%s): %w", item.Profile.ProfileId, err)
			}
		}
	}

	groupMappings, err := a.restoreProfilePackageGroups(tx, snapshot.Groups, warnings)
	if err != nil {
		return err
	}
	coreMappings, err := a.restoreProfilePackageCores(tx, snapshot.Cores, warnings)
	if err != nil {
		return err
	}
	proxyMappings, proxyValues, err := a.restoreProfilePackageProxies(tx, snapshot.Proxies, warnings)
	if err != nil {
		return err
	}
	extensionMappings, err := restoreProfilePackageExtensions(tx, snapshot.Extensions, warnings)
	if err != nil {
		return err
	}

	for index := range prepared {
		profile := &prepared[index].Profile
		if originalGroupID := strings.TrimSpace(profile.GroupId); originalGroupID != "" {
			if mapped := groupMappings[backupImportIDKey(originalGroupID)]; mapped != "" {
				profile.GroupId = mapped
			} else {
				appendProfilePackageWarning(warnings, fmt.Sprintf("实例「%s」的分组未找到，已清空分组", profile.ProfileName))
				profile.GroupId = ""
			}
		}
		if originalCoreID := strings.TrimSpace(profile.CoreId); originalCoreID != "" {
			if mapped := coreMappings[backupImportIDKey(originalCoreID)]; mapped != "" {
				profile.CoreId = mapped
			} else {
				appendProfilePackageWarning(warnings, fmt.Sprintf("实例「%s」的内核记录未找到，保留原内核 ID", profile.ProfileName))
			}
		}
		if originalProxyID := strings.TrimSpace(profile.ProxyId); originalProxyID != "" {
			if mapped := proxyMappings[backupImportIDKey(originalProxyID)]; mapped != "" {
				profile.ProxyId = mapped
				if value, exists := proxyValues[backupImportIDKey(originalProxyID)]; exists {
					if strings.TrimSpace(profile.ProxyConfig) == "" {
						profile.ProxyConfig = value.ProxyConfig
					}
					if strings.TrimSpace(profile.ProxyBindName) == "" {
						profile.ProxyBindName = value.ProxyName
					}
				}
			} else {
				appendProfilePackageWarning(warnings, fmt.Sprintf("实例「%s」的代理记录未找到，已清空代理绑定", profile.ProfileName))
				profile.ProxyId = ""
				profile.ProxyConfig = ""
			}
		}
		if err := insertProfilePackageProfile(tx, *profile); err != nil {
			return err
		}
	}

	for _, setting := range snapshot.ProfileExtensionSettings {
		targetProfileID := profileMappings[backupImportIDKey(setting.ProfileID)]
		if targetProfileID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_profile_extension_settings (profile_id, configured, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT(profile_id) DO UPDATE SET configured = excluded.configured, updated_at = excluded.updated_at`,
			targetProfileID, profilePackageBoolToInt(setting.Configured), setting.UpdatedAt,
		); err != nil {
			return fmt.Errorf("恢复实例插件设置失败(%s): %w", targetProfileID, err)
		}
	}

	for _, binding := range snapshot.ProfileExtensions {
		targetProfileID := profileMappings[backupImportIDKey(binding.ProfileID)]
		targetExtensionID := extensionMappings[backupImportIDKey(binding.ExtensionID)]
		if targetProfileID == "" || targetExtensionID == "" {
			appendProfilePackageWarning(warnings, fmt.Sprintf("实例插件绑定缺少目标引用（实例=%s，插件=%s）", binding.ProfileID, binding.ExtensionID))
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(profile_id, extension_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
			targetProfileID, targetExtensionID, profilePackageBoolToInt(binding.Enabled), binding.CreatedAt, binding.UpdatedAt,
		); err != nil {
			return fmt.Errorf("恢复实例插件绑定失败(%s,%s): %w", targetProfileID, targetExtensionID, err)
		}
	}

	for _, runtimeState := range snapshot.ProfileExtensionRuntime {
		targetProfileID := profileMappings[backupImportIDKey(runtimeState.ProfileID)]
		targetExtensionID := extensionMappings[backupImportIDKey(runtimeState.ExtensionID)]
		if targetProfileID == "" || targetExtensionID == "" {
			appendProfilePackageWarning(warnings, fmt.Sprintf("实例插件运行态缺少目标引用（实例=%s，插件=%s）", runtimeState.ProfileID, runtimeState.ExtensionID))
			continue
		}
		backupPath := ""
		if strings.TrimSpace(runtimeState.BackupPath) != "" {
			appendProfilePackageWarning(warnings, fmt.Sprintf("实例「%s」插件运行态的备份路径未恢复", runtimeState.ProfileID))
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_profile_extension_runtime (
				profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status,
				backup_path, last_verified_at, last_error, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(profile_id, extension_id) DO UPDATE SET
				runtime_extension_id = excluded.runtime_extension_id,
				install_mode = excluded.install_mode,
				installed_version = excluded.installed_version,
				package_hash = excluded.package_hash,
				status = excluded.status,
				backup_path = excluded.backup_path,
				last_verified_at = excluded.last_verified_at,
				last_error = excluded.last_error,
				updated_at = excluded.updated_at`,
			targetProfileID, targetExtensionID, runtimeState.RuntimeExtensionID, runtimeState.InstallMode,
			runtimeState.InstalledVersion, runtimeState.PackageHash, runtimeState.Status, backupPath,
			runtimeState.LastVerifiedAt, runtimeState.LastError, runtimeState.CreatedAt, runtimeState.UpdatedAt,
		); err != nil {
			return fmt.Errorf("恢复实例插件运行态失败(%s,%s): %w", targetProfileID, targetExtensionID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交实例数据库恢复事务失败: %w", err)
	}
	committed = true
	return nil
}

func (a *App) restoreLegacyProfilePackageDatabase(prepared []preparedProfilePackageImport) error {
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化，无法恢复实例配置")
	}

	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return fmt.Errorf("开启实例配置恢复事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, item := range prepared {
		if item.Overwrite {
			for _, statement := range []string{
				`DELETE FROM browser_profile_extension_runtime WHERE profile_id = ?`,
				`DELETE FROM browser_profile_extensions WHERE profile_id = ?`,
				`DELETE FROM browser_profile_extension_settings WHERE profile_id = ?`,
			} {
				if _, err := tx.Exec(statement, item.Profile.ProfileId); err != nil {
					return fmt.Errorf("清理被覆盖实例关联数据失败(%s): %w", item.Profile.ProfileId, err)
				}
			}
		}
		if err := insertProfilePackageProfile(tx, item.Profile); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交实例配置恢复事务失败: %w", err)
	}
	committed = true
	return nil
}

func appendProfilePackageWarning(warnings *[]string, message string) {
	if warnings != nil && strings.TrimSpace(message) != "" {
		*warnings = append(*warnings, message)
	}
}

func profilePackageGeneratedID(used map[string]struct{}) string {
	for {
		candidate := generateUUID()
		key := backupImportIDKey(candidate)
		if _, exists := used[key]; exists {
			continue
		}
		used[key] = struct{}{}
		return candidate
	}
}

type profilePackageTargetGroup struct {
	ID        string
	Name      string
	ParentID  string
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

func (a *App) restoreProfilePackageGroups(tx *sql.Tx, source []ProfilePackageDatabaseGroup, warnings *[]string) (map[string]string, error) {
	targetByID := make(map[string]profilePackageTargetGroup)
	targetByName := make(map[string]string)
	usedIDs := make(map[string]struct{})
	rows, err := tx.Query(`SELECT group_id, group_name, COALESCE(parent_id, ''), sort_order, created_at, updated_at FROM browser_groups`)
	if err != nil {
		return nil, fmt.Errorf("读取现有分组失败: %w", err)
	}
	for rows.Next() {
		var item profilePackageTargetGroup
		if err := rows.Scan(&item.ID, &item.Name, &item.ParentID, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		key := backupImportIDKey(item.ID)
		if key == "" {
			continue
		}
		targetByID[key] = item
		usedIDs[key] = struct{}{}
		targetByName[profilePackageGroupNameKey(item.ParentID, item.Name)] = item.ID
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	sourceByID := make(map[string]ProfilePackageDatabaseGroup, len(source))
	for _, item := range source {
		if key := backupImportIDKey(item.GroupId); key != "" {
			sourceByID[key] = item
		}
	}
	mappings := make(map[string]string, len(sourceByID))
	states := make(map[string]uint8, len(sourceByID))
	var resolve func(string) (string, error)
	resolve = func(sourceID string) (string, error) {
		key := backupImportIDKey(sourceID)
		if key == "" {
			return "", nil
		}
		if mapped, exists := mappings[key]; exists {
			return mapped, nil
		}
		if states[key] == 1 {
			appendProfilePackageWarning(warnings, fmt.Sprintf("分组「%s」存在循环父级，已断开父级关系", sourceID))
			mappings[key] = ""
			return "", nil
		}
		row, exists := sourceByID[key]
		if !exists {
			if target, targetExists := targetByID[key]; targetExists {
				mappings[key] = target.ID
				return target.ID, nil
			}
			return "", nil
		}
		states[key] = 1
		parentTargetID := ""
		parentSourceID := strings.TrimSpace(row.ParentId)
		parentResolved := parentSourceID == ""
		if parentSourceID != "" {
			if _, sourceParentExists := sourceByID[backupImportIDKey(parentSourceID)]; sourceParentExists {
				parentTargetID, err = resolve(parentSourceID)
				if err != nil {
					return "", err
				}
				parentResolved = parentTargetID != ""
			} else if target, targetExists := targetByID[backupImportIDKey(parentSourceID)]; targetExists {
				parentTargetID = target.ID
				parentResolved = true
			}
		}

		nameKey := strings.TrimSpace(row.GroupName)
		if existing, targetExists := targetByID[key]; targetExists && strings.EqualFold(strings.TrimSpace(existing.Name), nameKey) && strings.EqualFold(strings.TrimSpace(existing.ParentID), strings.TrimSpace(parentTargetID)) {
			mappings[key] = existing.ID
			states[key] = 2
			return existing.ID, nil
		}
		if parentResolved {
			if existingID, targetExists := targetByName[profilePackageGroupNameKey(parentTargetID, nameKey)]; targetExists {
				mappings[key] = existingID
				states[key] = 2
				return existingID, nil
			}
		} else if parentSourceID != "" {
			appendProfilePackageWarning(warnings, fmt.Sprintf("分组「%s」的父分组不存在，已导入为根分组", row.GroupName))
		}

		targetID := strings.TrimSpace(row.GroupId)
		if _, exists := usedIDs[backupImportIDKey(targetID)]; exists || targetID == "" {
			targetID = profilePackageGeneratedID(usedIDs)
		} else {
			usedIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		createdAt := row.CreatedAt
		if strings.TrimSpace(createdAt) == "" {
			createdAt = time.Now().Format(time.RFC3339)
		}
		updatedAt := row.UpdatedAt
		if strings.TrimSpace(updatedAt) == "" {
			updatedAt = createdAt
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, targetID, row.GroupName, parentTargetID, row.SortOrder, createdAt, updatedAt); err != nil {
			return "", fmt.Errorf("恢复分组失败(%s): %w", row.GroupName, err)
		}
		targetByID[backupImportIDKey(targetID)] = profilePackageTargetGroup{ID: targetID, Name: row.GroupName, ParentID: parentTargetID, SortOrder: row.SortOrder, CreatedAt: createdAt, UpdatedAt: updatedAt}
		targetByName[profilePackageGroupNameKey(parentTargetID, nameKey)] = targetID
		mappings[key] = targetID
		states[key] = 2
		return targetID, nil
	}

	for _, row := range source {
		if _, err := resolve(row.GroupId); err != nil {
			return nil, err
		}
	}
	return mappings, nil
}

func profilePackageGroupNameKey(parentID, name string) string {
	return strings.ToLower(strings.TrimSpace(parentID)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}

type profilePackageTargetCore struct {
	ID   string
	Path string
}

func (a *App) restoreProfilePackageCores(tx *sql.Tx, source []ProfilePackageDatabaseCore, warnings *[]string) (map[string]string, error) {
	targetByID := make(map[string]profilePackageTargetCore)
	targetByPath := make(map[string]string)
	usedIDs := make(map[string]struct{})
	rows, err := tx.Query(`SELECT core_id, core_path FROM browser_cores`)
	if err != nil {
		return nil, fmt.Errorf("读取现有内核失败: %w", err)
	}
	for rows.Next() {
		var item profilePackageTargetCore
		if err := rows.Scan(&item.ID, &item.Path); err != nil {
			rows.Close()
			return nil, err
		}
		key := backupImportIDKey(item.ID)
		if key == "" {
			continue
		}
		targetByID[key] = item
		usedIDs[key] = struct{}{}
		if pathKey := backupImportPathKey(a, item.Path); pathKey != "" {
			targetByPath[pathKey] = item.ID
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	mappings := make(map[string]string, len(source))
	for _, row := range source {
		key := backupImportIDKey(row.CoreId)
		if key == "" {
			continue
		}
		pathKey := backupImportPathKey(a, row.CorePath)
		if existing, exists := targetByID[key]; exists && (pathKey == "" || pathKey == backupImportPathKey(a, existing.Path)) {
			mappings[key] = existing.ID
			continue
		}
		if pathKey != "" {
			if existingID, exists := targetByPath[pathKey]; exists {
				mappings[key] = existingID
				continue
			}
		}
		if _, exists := targetByID[key]; exists {
			appendProfilePackageWarning(warnings, fmt.Sprintf("内核「%s」ID 冲突，已生成新 ID", row.CoreName))
		}
		targetID := strings.TrimSpace(row.CoreId)
		if _, exists := usedIDs[backupImportIDKey(targetID)]; exists || targetID == "" {
			targetID = profilePackageGeneratedID(usedIDs)
		} else {
			usedIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		createdAt := row.CreatedAt
		if strings.TrimSpace(createdAt) == "" {
			createdAt = time.Now().Format(time.RFC3339)
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`, targetID, row.CoreName, row.CorePath, 0, row.SortOrder, createdAt); err != nil {
			return nil, fmt.Errorf("恢复内核失败(%s): %w", row.CoreName, err)
		}
		targetByID[backupImportIDKey(targetID)] = profilePackageTargetCore{ID: targetID, Path: row.CorePath}
		if pathKey != "" {
			targetByPath[pathKey] = targetID
		}
		mappings[key] = targetID
		if filepath.IsAbs(strings.TrimSpace(row.CorePath)) {
			appendProfilePackageWarning(warnings, fmt.Sprintf("内核「%s」的绝对路径未随实例包复制", row.CoreName))
		}
	}
	return mappings, nil
}

type profilePackageTargetProxy struct {
	ID     string
	Config string
}

func (a *App) restoreProfilePackageProxies(tx *sql.Tx, source []ProfilePackageDatabaseProxy, warnings *[]string) (map[string]string, map[string]browser.Proxy, error) {
	targetByID := make(map[string]profilePackageTargetProxy)
	targetByConfig := make(map[string]string)
	usedIDs := make(map[string]struct{})
	rows, err := tx.Query(`SELECT proxy_id, proxy_config FROM browser_proxies`)
	if err != nil {
		return nil, nil, fmt.Errorf("读取现有代理失败: %w", err)
	}
	for rows.Next() {
		var item profilePackageTargetProxy
		if err := rows.Scan(&item.ID, &item.Config); err != nil {
			rows.Close()
			return nil, nil, err
		}
		key := backupImportIDKey(item.ID)
		if key == "" {
			continue
		}
		targetByID[key] = item
		usedIDs[key] = struct{}{}
		if configKey := backupImportTextKey(item.Config); configKey != "" {
			targetByConfig[configKey] = item.ID
		}
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	mappings := make(map[string]string, len(source))
	values := make(map[string]browser.Proxy, len(source))
	for _, item := range source {
		proxy := item.Proxy
		key := backupImportIDKey(proxy.ProxyId)
		if key == "" {
			continue
		}
		configKey := backupImportTextKey(proxy.ProxyConfig)
		if existing, exists := targetByID[key]; exists && (configKey == "" || configKey == backupImportTextKey(existing.Config)) {
			mappings[key] = existing.ID
			values[key] = proxy
			continue
		}
		if configKey != "" {
			if existingID, exists := targetByConfig[configKey]; exists {
				mappings[key] = existingID
				values[key] = proxy
				continue
			}
		}
		if _, exists := targetByID[key]; exists {
			appendProfilePackageWarning(warnings, fmt.Sprintf("代理「%s」ID 冲突，已生成新 ID", proxy.ProxyName))
		}
		targetID := strings.TrimSpace(proxy.ProxyId)
		if _, exists := usedIDs[backupImportIDKey(targetID)]; exists || targetID == "" {
			targetID = profilePackageGeneratedID(usedIDs)
		} else {
			usedIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		createdAt := item.CreatedAt
		if strings.TrimSpace(createdAt) == "" {
			createdAt = time.Now().Format(time.RFC3339)
		}
		testOK := profilePackageBoolToInt(proxy.LastTestOk)
		autoRefresh := profilePackageBoolToInt(proxy.SourceAutoRefresh)
		if _, err := tx.Exec(`
			INSERT INTO browser_proxies (
				proxy_id, proxy_name, proxy_config, preferred_kernel, dns_servers, group_name,
				source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at,
				last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			targetID, proxy.ProxyName, proxy.ProxyConfig, proxy.PreferredKernel, proxy.DnsServers, proxy.GroupName,
			proxy.SourceID, proxy.SourceURL, proxy.SourceNamePrefix, autoRefresh, proxy.SourceRefreshIntervalM, proxy.SourceLastRefreshAt,
			proxy.LastLatencyMs, testOK, proxy.LastTestedAt, proxy.LastIPHealthJSON, proxy.SortOrder, createdAt,
		); err != nil {
			return nil, nil, fmt.Errorf("恢复代理失败(%s): %w", proxy.ProxyName, err)
		}
		targetByID[backupImportIDKey(targetID)] = profilePackageTargetProxy{ID: targetID, Config: proxy.ProxyConfig}
		if configKey != "" {
			targetByConfig[configKey] = targetID
		}
		mappings[key] = targetID
		values[key] = proxy
	}
	return mappings, values, nil
}

func restoreProfilePackageExtensions(tx *sql.Tx, source []browser.Extension, warnings *[]string) (map[string]string, error) {
	targetIDs := make(map[string]struct{})
	rows, err := tx.Query(`SELECT extension_id FROM browser_extensions`)
	if err != nil {
		return nil, fmt.Errorf("读取现有插件失败: %w", err)
	}
	for rows.Next() {
		var extensionID string
		if err := rows.Scan(&extensionID); err != nil {
			rows.Close()
			return nil, err
		}
		targetIDs[backupImportIDKey(extensionID)] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	mappings := make(map[string]string, len(source))
	for _, extension := range source {
		key := backupImportIDKey(extension.ExtensionID)
		if key == "" {
			continue
		}
		if _, exists := targetIDs[key]; exists {
			mappings[key] = extension.ExtensionID
			continue
		}
		installedAt := extension.InstalledAt
		if strings.TrimSpace(installedAt) == "" {
			installedAt = time.Now().Format(time.RFC3339)
		}
		updatedAt := extension.UpdatedAt
		if strings.TrimSpace(updatedAt) == "" {
			updatedAt = installedAt
		}
		installMode := strings.TrimSpace(extension.InstallMode)
		if installMode == "" {
			installMode = browser.ExtensionInstallModePersistent
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_extensions (
				extension_id, name, version, description, icon_data_url, manifest_json, source_url, install_dir,
				install_mode, package_path, package_hash, enabled, default_install, installed_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			extension.ExtensionID, extension.Name, extension.Version, extension.Description, extension.IconDataURL,
			extension.ManifestJSON, extension.SourceURL, extension.InstallDir, installMode, extension.PackagePath,
			extension.PackageHash, profilePackageBoolToInt(extension.Enabled), profilePackageBoolToInt(extension.DefaultInstall), installedAt, updatedAt,
		); err != nil {
			return nil, fmt.Errorf("恢复插件失败(%s): %w", extension.Name, err)
		}
		targetIDs[key] = struct{}{}
		mappings[key] = extension.ExtensionID
		if filepath.IsAbs(strings.TrimSpace(extension.InstallDir)) || filepath.IsAbs(strings.TrimSpace(extension.PackagePath)) {
			appendProfilePackageWarning(warnings, fmt.Sprintf("插件「%s」的外部文件路径未随实例包复制", extension.Name))
		}
	}
	return mappings, nil
}

func insertProfilePackageProfile(tx *sql.Tx, profile browser.Profile) error {
	fingerprintArgs, err := json.Marshal(profile.FingerprintArgs)
	if err != nil {
		return fmt.Errorf("序列化实例指纹参数失败: %w", err)
	}
	launchArgs, err := json.Marshal(profile.LaunchArgs)
	if err != nil {
		return fmt.Errorf("序列化实例启动参数失败: %w", err)
	}
	tags, err := json.Marshal(profile.Tags)
	if err != nil {
		return fmt.Errorf("序列化实例标签失败: %w", err)
	}
	keywords, err := json.Marshal(profile.Keywords)
	if err != nil {
		return fmt.Errorf("序列化实例关键词失败: %w", err)
	}
	createdAt := profile.CreatedAt
	if strings.TrimSpace(createdAt) == "" {
		createdAt = time.Now().Format(time.RFC3339)
	}
	updatedAt := profile.UpdatedAt
	if strings.TrimSpace(updatedAt) == "" {
		updatedAt = createdAt
	}
	_, err = tx.Exec(`
		INSERT INTO browser_profiles (
			profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config,
			proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at, memory_limit_mb,
			launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			profile_name = excluded.profile_name,
			user_data_dir = excluded.user_data_dir,
			core_id = excluded.core_id,
			fingerprint_args = excluded.fingerprint_args,
			proxy_id = excluded.proxy_id,
			proxy_config = excluded.proxy_config,
			proxy_bind_source_id = excluded.proxy_bind_source_id,
			proxy_bind_source_url = excluded.proxy_bind_source_url,
			proxy_bind_name = excluded.proxy_bind_name,
			proxy_bind_updated_at = excluded.proxy_bind_updated_at,
			memory_limit_mb = excluded.memory_limit_mb,
			launch_args = excluded.launch_args,
			tags = excluded.tags,
			keywords = excluded.keywords,
			group_id = excluded.group_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			restore_last_session = excluded.restore_last_session,
			deleted_at = excluded.deleted_at`,
		profile.ProfileId, profile.ProfileName, profile.UserDataDir, profile.CoreId, string(fingerprintArgs), profile.ProxyId,
		profile.ProxyConfig, profile.ProxyBindSourceID, profile.ProxyBindSourceURL, profile.ProxyBindName,
		profile.ProxyBindUpdatedAt, profile.MemoryLimitMB, string(launchArgs), string(tags), string(keywords), profile.GroupId,
		createdAt, updatedAt, profile.RestoreLastSession, profile.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("恢复实例配置失败(%s): %w", profile.ProfileName, err)
	}
	return nil
}

func profilePackageBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
