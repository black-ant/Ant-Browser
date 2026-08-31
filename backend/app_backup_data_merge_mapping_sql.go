package backend

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
)

func backupBuildMappedCoreMergeSQL(tx *sql.Tx) (string, error) {
	return backupBuildMappedTableMergeSQL(tx, "browser_cores", []backupSourceColumnSpec{
		{name: "core_id", fallback: "''"}, {name: "core_name", fallback: "''"}, {name: "core_path", fallback: "''"},
		{name: "is_default", fallback: "0"}, {name: "sort_order", fallback: "0"}, {name: "created_at", fallback: "CURRENT_TIMESTAMP"},
	}, map[string]string{"core_id": backupImportCoreMapTable}, backupImportMapExistsExpression("s.core_id", backupImportCoreMapTable, true))
}

func backupBuildMappedProxyMergeSQL(tx *sql.Tx) (string, error) {
	return backupBuildMappedTableMergeSQL(tx, "browser_proxies", []backupSourceColumnSpec{
		{name: "proxy_id", fallback: "''"}, {name: "proxy_name", fallback: "''"}, {name: "proxy_config", fallback: "''"},
		{name: "preferred_kernel", fallback: "''"}, {name: "dns_servers", fallback: "''"}, {name: "group_name", fallback: "''"},
		{name: "source_id", fallback: "''"}, {name: "source_url", fallback: "''"}, {name: "source_name_prefix", fallback: "''"},
		{name: "source_auto_refresh", fallback: "0"}, {name: "source_refresh_interval_m", fallback: "0"}, {name: "source_last_refresh_at", fallback: "''"},
		{name: "last_latency_ms", fallback: "-1"}, {name: "last_test_ok", fallback: "0"}, {name: "last_tested_at", fallback: "''"},
		{name: "last_ip_health_json", fallback: "''"}, {name: "sort_order", fallback: "0"}, {name: "created_at", fallback: "CURRENT_TIMESTAMP"},
	}, map[string]string{"proxy_id": backupImportProxyMapTable}, backupImportMapExistsExpression("s.proxy_id", backupImportProxyMapTable, true))
}

func backupBuildMappedProfileMergeSQL(tx *sql.Tx) (string, error) {
	return backupBuildMappedTableMergeSQL(tx, "browser_profiles", []backupSourceColumnSpec{
		{name: "profile_id", fallback: "''"}, {name: "profile_name", fallback: "''"}, {name: "user_data_dir", fallback: "''"},
		{name: "core_id", fallback: "''"}, {name: "fingerprint_args", fallback: "'[]'"}, {name: "proxy_id", fallback: "''"},
		{name: "proxy_config", fallback: "''"}, {name: "proxy_bind_source_id", fallback: "''"}, {name: "proxy_bind_source_url", fallback: "''"},
		{name: "proxy_bind_name", fallback: "''"}, {name: "proxy_bind_updated_at", fallback: "''"}, {name: "memory_limit_mb", fallback: "0"},
		{name: "launch_args", fallback: "'[]'"}, {name: "tags", fallback: "'[]'"}, {name: "keywords", fallback: "'[]'"},
		{name: "group_id", fallback: "''"}, {name: "created_at", fallback: "CURRENT_TIMESTAMP"}, {name: "updated_at", fallback: "CURRENT_TIMESTAMP"},
		{name: "restore_last_session", fallback: "''"}, {name: "deleted_at", fallback: "''"},
	}, map[string]string{
		"profile_id": backupImportProfileMapTable, "core_id": backupImportCoreMapTable,
		"proxy_id": backupImportProxyMapTable, "group_id": backupImportGroupMapTable,
	}, backupImportMapExistsExpression("s.profile_id", backupImportProfileMapTable, true))
}

func backupImportMappedIDOrOriginal(values map[string]backupImportIDMapping, sourceID string) string {
	if mapped := backupImportMappedID(values, sourceID); mapped != "" {
		return mapped
	}
	return sourceID
}

func backupApplyImportReferenceMappings(incomingCfg *config.Config, mappings *backupImportReferenceMappings) {
	if incomingCfg == nil || mappings == nil {
		return
	}
	for i := range incomingCfg.Browser.Cores {
		incomingCfg.Browser.Cores[i].CoreId = backupImportMappedIDOrOriginal(mappings.Cores, incomingCfg.Browser.Cores[i].CoreId)
	}
	for i := range incomingCfg.Browser.Proxies {
		incomingCfg.Browser.Proxies[i].ProxyId = backupImportMappedIDOrOriginal(mappings.Proxies, incomingCfg.Browser.Proxies[i].ProxyId)
	}
	for i := range incomingCfg.Browser.Profiles {
		profile := &incomingCfg.Browser.Profiles[i]
		profile.ProfileId = backupImportMappedIDOrOriginal(mappings.Profiles, profile.ProfileId)
		profile.CoreId = backupImportMappedIDOrOriginal(mappings.Cores, profile.CoreId)
		profile.ProxyId = backupImportMappedIDOrOriginal(mappings.Proxies, profile.ProxyId)
	}
	incomingCfg.Browser.DefaultCoreId = backupImportMappedIDOrOriginal(mappings.Cores, incomingCfg.Browser.DefaultCoreId)
	for i := range incomingCfg.Browser.Environments {
		incomingCfg.Browser.Environments[i].CoreId = backupImportMappedIDOrOriginal(mappings.Cores, incomingCfg.Browser.Environments[i].CoreId)
	}
}
