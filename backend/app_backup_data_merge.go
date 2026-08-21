package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) backupMergeProxiesFile(payloadRoot string, resetFirst bool, stats *backupMergeStats) error {
	srcPath := filepath.Join(payloadRoot, "system", "proxies.yaml")
	dstPath := a.resolveAppPath("proxies.yaml")

	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			if resetFirst {
				_ = os.Remove(dstPath)
			}
			return nil
		}
		return err
	}

	if resetFirst {
		return backupCopyFile(srcPath, dstPath)
	}

	incoming, err := config.LoadProxies(srcPath)
	if err != nil {
		return err
	}
	current, err := config.LoadProxies(dstPath)
	if err != nil {
		return err
	}

	merged := append([]config.BrowserProxy{}, current...)
	existingID := make(map[string]struct{}, len(current))
	existingCfg := make(map[string]struct{}, len(current))
	for _, p := range current {
		existingID[strings.ToLower(strings.TrimSpace(p.ProxyId))] = struct{}{}
		existingCfg[strings.ToLower(strings.TrimSpace(p.ProxyConfig))] = struct{}{}
	}
	for _, p := range incoming {
		idKey := strings.ToLower(strings.TrimSpace(p.ProxyId))
		cfgKey := strings.ToLower(strings.TrimSpace(p.ProxyConfig))
		if _, ok := existingID[idKey]; ok {
			stats.Skipped++
			continue
		}
		if cfgKey != "" {
			if _, ok := existingCfg[cfgKey]; ok {
				stats.Skipped++
				continue
			}
		}
		merged = append(merged, p)
		existingID[idKey] = struct{}{}
		if cfgKey != "" {
			existingCfg[cfgKey] = struct{}{}
		}
		stats.Imported++
	}

	return config.SaveProxies(dstPath, merged)
}

func backupFindDatabaseFile(payloadRoot string) string {
	candidates := []string{
		filepath.Join(payloadRoot, "app", "database", "app.db"),
		filepath.Join(payloadRoot, "app", "data", "app.db"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (a *App) backupMergeDatabaseFromSource(srcDBPath string, resetFirst bool, stats *backupMergeStats) error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ATTACH DATABASE ? AS src`, srcDBPath); err != nil {
		return fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	defer tx.Exec(`DETACH DATABASE src`)

	mergeTables := []struct {
		name       string
		insertAll  string
		insertSafe string
	}{
		{
			name: "browser_groups",
			insertAll: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at FROM src.browser_groups`,
			insertSafe: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT s.group_id, s.group_name, s.parent_id, s.sort_order, s.created_at, s.updated_at
FROM src.browser_groups s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_groups t
  WHERE t.group_id = s.group_id OR (t.parent_id = s.parent_id AND lower(t.group_name) = lower(s.group_name))
)`,
		},
		{
			name: "browser_cores",
			insertAll: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT core_id, core_name, core_path, is_default, sort_order, created_at FROM src.browser_cores`,
			insertSafe: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT s.core_id, s.core_name, s.core_path, s.is_default, s.sort_order, s.created_at
FROM src.browser_cores s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_cores t
  WHERE t.core_id = s.core_id OR lower(t.core_path) = lower(s.core_path)
)`,
		},
		{
			name: "browser_proxies",
			insertAll: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name,''), COALESCE(source_id,''), COALESCE(source_url,''), COALESCE(source_name_prefix,''), COALESCE(source_auto_refresh,0), COALESCE(source_refresh_interval_m,0), COALESCE(source_last_refresh_at,''), COALESCE(last_latency_ms,-1), COALESCE(last_test_ok,0), COALESCE(last_tested_at,''), COALESCE(last_ip_health_json,''), sort_order, created_at
FROM src.browser_proxies`,
			insertSafe: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT s.proxy_id, s.proxy_name, s.proxy_config, s.dns_servers, COALESCE(s.group_name,''), COALESCE(s.source_id,''), COALESCE(s.source_url,''), COALESCE(s.source_name_prefix,''), COALESCE(s.source_auto_refresh,0), COALESCE(s.source_refresh_interval_m,0), COALESCE(s.source_last_refresh_at,''), COALESCE(s.last_latency_ms,-1), COALESCE(s.last_test_ok,0), COALESCE(s.last_tested_at,''), COALESCE(s.last_ip_health_json,''), s.sort_order, s.created_at
FROM src.browser_proxies s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_proxies t
  WHERE t.proxy_id = s.proxy_id OR lower(t.proxy_config) = lower(s.proxy_config)
)`,
		},
		{
			name: "browser_profiles",
			insertAll: `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at)
SELECT profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, COALESCE(group_id,''), created_at, updated_at
FROM src.browser_profiles`,
			insertSafe: `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at)
SELECT s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at
FROM src.browser_profiles s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`,
		},
		{
			name: "browser_bookmarks",
			insertAll: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT name, url, sort_order FROM src.browser_bookmarks`,
			insertSafe: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT s.name, s.url, s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`,
		},
		{
			name: "browser_extensions",
			insertAll: `INSERT INTO browser_extensions (extension_id, name, version, description, manifest_json, source_url, install_dir, enabled, installed_at, updated_at)
SELECT extension_id, name, version, description, manifest_json, source_url, install_dir, enabled, installed_at, updated_at FROM src.browser_extensions`,
			insertSafe: `INSERT INTO browser_extensions (extension_id, name, version, description, manifest_json, source_url, install_dir, enabled, installed_at, updated_at)
SELECT s.extension_id, s.name, s.version, s.description, s.manifest_json, s.source_url, s.install_dir, s.enabled, s.installed_at, s.updated_at
FROM src.browser_extensions s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_extensions t WHERE t.extension_id = s.extension_id
)`,
		},
		{
			name: "browser_profile_extension_settings",
			insertAll: `INSERT INTO browser_profile_extension_settings (profile_id, configured, updated_at)
SELECT profile_id, configured, updated_at FROM src.browser_profile_extension_settings`,
			insertSafe: `INSERT INTO browser_profile_extension_settings (profile_id, configured, updated_at)
SELECT s.profile_id, s.configured, s.updated_at
FROM src.browser_profile_extension_settings s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profile_extension_settings t WHERE t.profile_id = s.profile_id
)`,
		},
		{
			name: "browser_profile_extensions",
			insertAll: `INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at)
SELECT profile_id, extension_id, enabled, created_at, updated_at FROM src.browser_profile_extensions`,
			insertSafe: `INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at)
SELECT s.profile_id, s.extension_id, s.enabled, s.created_at, s.updated_at
FROM src.browser_profile_extensions s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profile_extensions t WHERE t.profile_id = s.profile_id AND t.extension_id = s.extension_id
)`,
		},
		{
			name: "launch_codes",
			insertAll: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT profile_id, code, created_at, updated_at FROM src.launch_codes`,
			insertSafe: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT s.profile_id, s.code, s.created_at, s.updated_at
FROM src.launch_codes s
WHERE NOT EXISTS (
  SELECT 1 FROM launch_codes t
  WHERE t.profile_id = s.profile_id OR t.code = s.code
)`,
		},
	}

	for _, item := range mergeTables {
		exists, err := backupSrcTableExists(tx, item.name)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		total, err := backupCountRows(tx, "src."+item.name)
		if err != nil {
			return err
		}
		if total == 0 {
			continue
		}

		sqlText := item.insertAll
		if !resetFirst {
			sqlText = item.insertSafe
		}
		if item.name == "browser_bookmarks" {
			hasOpenOnStart, err := backupSrcColumnExists(tx, item.name, "open_on_start")
			if err != nil {
				return err
			}
			if hasOpenOnStart {
				if resetFirst {
					sqlText = `INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order)
SELECT name, url, COALESCE(open_on_start,0), sort_order FROM src.browser_bookmarks`
				} else {
					sqlText = `INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order)
SELECT s.name, s.url, COALESCE(s.open_on_start,0), s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`
				}
			}
		}
		if item.name == "browser_cores" {
			// core_backend / core_env 是同一次迁移新增的，旧备份没有这两列时沿用基础语句。
			hasCoreBackend, err := backupSrcColumnExists(tx, item.name, "core_backend")
			if err != nil {
				return err
			}
			hasCoreEnv, err := backupSrcColumnExists(tx, item.name, "core_env")
			if err != nil {
				return err
			}
			if hasCoreBackend && hasCoreEnv {
				if resetFirst {
					sqlText = `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, core_backend, core_env, sort_order, created_at)
SELECT core_id, core_name, core_path, is_default, COALESCE(core_backend,''), COALESCE(core_env,'[]'), sort_order, created_at FROM src.browser_cores`
				} else {
					sqlText = `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, core_backend, core_env, sort_order, created_at)
SELECT s.core_id, s.core_name, s.core_path, s.is_default, COALESCE(s.core_backend,''), COALESCE(s.core_env,'[]'), s.sort_order, s.created_at
FROM src.browser_cores s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_cores t
  WHERE t.core_id = s.core_id OR lower(t.core_path) = lower(s.core_path)
)`
				}
			}
		}
		if item.name == "browser_profiles" {
			hasRestoreLastSession, err := backupSrcColumnExists(tx, item.name, "restore_last_session")
			if err != nil {
				return err
			}
			if hasRestoreLastSession {
				if resetFirst {
					sqlText = `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session)
SELECT profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, COALESCE(group_id,''), created_at, updated_at, COALESCE(restore_last_session,'')
FROM src.browser_profiles`
				} else {
					sqlText = `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session)
SELECT s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at, COALESCE(s.restore_last_session,'')
FROM src.browser_profiles s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`
				}
			}
		}
		res, err := tx.Exec(sqlText)
		if err != nil {
			return fmt.Errorf("导入数据表失败(%s): %w", item.name, err)
		}
		affected, _ := res.RowsAffected()
		inserted := int(affected)
		if inserted < 0 {
			inserted = total
		}
		stats.Imported += inserted
		if !resetFirst && total > inserted {
			stats.Skipped += total - inserted
		}
	}

	return tx.Commit()
}
