package backend

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) backupMergeProxiesFile(payloadRoot string, stats *backupMergeStats) error {
	srcPath := filepath.Join(payloadRoot, "system", "proxies.yaml")
	dstPath := a.resolveAppPath("proxies.yaml")

	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
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

func (a *App) backupMergeDatabaseFromSource(srcDBPath string, incomingCfg *config.Config, stats *backupMergeStats) error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	dbConn := a.db.GetConn()
	tx, err := dbConn.Begin()
	if err != nil {
		return err
	}
	committed := false
	sourceAttached := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
		if sourceAttached {
			_, _ = dbConn.Exec(`DETACH DATABASE src`)
		}
	}()

	if _, err := tx.Exec(`ATTACH DATABASE ? AS src`, srcDBPath); err != nil {
		return fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	sourceAttached = true

	existingCoreIDs, err := backupListTargetIDs(tx, "browser_cores", "core_id")
	if err != nil {
		return err
	}
	existingProfileIDs, err := backupListTargetIDs(tx, "browser_profiles", "profile_id")
	if err != nil {
		return err
	}

	mergeTables := []struct {
		name       string
		insertSafe string
	}{
		{
			name: "browser_groups",
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
			insertSafe: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT s.name, s.url, s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`,
		},
		{
			name: "browser_extensions",
			insertSafe: `INSERT INTO browser_extensions (extension_id, name, version, description, manifest_json, source_url, install_dir, enabled, default_install, installed_at, updated_at)
SELECT s.extension_id, s.name, s.version, s.description, s.manifest_json, s.source_url, s.install_dir, s.enabled, COALESCE(s.default_install, 0), s.installed_at, s.updated_at
FROM src.browser_extensions s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_extensions t WHERE t.extension_id = s.extension_id
)`,
		},
		{
			name: "browser_profile_extension_settings",
			insertSafe: `INSERT INTO browser_profile_extension_settings (profile_id, configured, updated_at)
SELECT s.profile_id, s.configured, s.updated_at
FROM src.browser_profile_extension_settings s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profile_extension_settings t WHERE t.profile_id = s.profile_id
)`,
		},
		{
			name: "browser_profile_extensions",
			insertSafe: `INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at)
SELECT s.profile_id, s.extension_id, s.enabled, s.created_at, s.updated_at
FROM src.browser_profile_extensions s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profile_extensions t WHERE t.profile_id = s.profile_id AND t.extension_id = s.extension_id
				)`,
		},
		{
			name: "browser_profile_extension_runtime",
			insertSafe: `INSERT INTO browser_profile_extension_runtime (profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status, backup_path, last_verified_at, last_error, created_at, updated_at)
SELECT s.profile_id, s.extension_id, s.runtime_extension_id, s.install_mode, s.installed_version, s.package_hash, s.status, s.backup_path, s.last_verified_at, s.last_error, s.created_at, s.updated_at
FROM src.browser_profile_extension_runtime s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profile_extension_runtime t WHERE t.profile_id = s.profile_id AND t.extension_id = s.extension_id
)`,
		},
		{
			name: "launch_codes",
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

		sqlText := item.insertSafe
		if item.name == "browser_bookmarks" {
			hasOpenOnStart, err := backupSrcColumnExists(tx, item.name, "open_on_start")
			if err != nil {
				return err
			}
			if hasOpenOnStart {
				sqlText = `INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order)
SELECT s.name, s.url, COALESCE(s.open_on_start,0), s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`
			}
		}
		if item.name == "browser_profiles" {
			hasRestoreLastSession, err := backupSrcColumnExists(tx, item.name, "restore_last_session")
			if err != nil {
				return err
			}
			if hasRestoreLastSession {
				sqlText = `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session)
SELECT s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at, COALESCE(s.restore_last_session,'')
FROM src.browser_profiles s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`
			}
		}
		if item.name == "browser_proxies" {
			sqlText, err = backupBuildProxyMergeSQL(tx)
			if err != nil {
				return err
			}
		}
		if item.name == "browser_profiles" {
			sqlText, err = backupBuildProfileMergeSQL(tx)
			if err != nil {
				return err
			}
		}
		if item.name == "browser_extensions" {
			var hasIconDataURL bool
			var hasInstallMode bool
			var hasPackagePath bool
			var hasPackageHash bool
			var hasDefaultInstall bool
			hasIconDataURL, err = backupSrcColumnExists(tx, item.name, "icon_data_url")
			if err != nil {
				return err
			}
			hasInstallMode, err = backupSrcColumnExists(tx, item.name, "install_mode")
			if err != nil {
				return err
			}
			hasPackagePath, err = backupSrcColumnExists(tx, item.name, "package_path")
			if err != nil {
				return err
			}
			hasPackageHash, err = backupSrcColumnExists(tx, item.name, "package_hash")
			if err != nil {
				return err
			}
			hasDefaultInstall, err = backupSrcColumnExists(tx, item.name, "default_install")
			if err != nil {
				return err
			}
			iconExpression := `''`
			if hasIconDataURL {
				iconExpression = `COALESCE(icon_data_url,'')`
			}
			installModeExpression := `'persistent'`
			if hasInstallMode {
				installModeExpression = `COALESCE(install_mode,'persistent')`
			}
			packagePathExpression := `''`
			if hasPackagePath {
				packagePathExpression = `COALESCE(package_path,'')`
			}
			packageHashExpression := `''`
			if hasPackageHash {
				packageHashExpression = `COALESCE(package_hash,'')`
			}
			defaultInstallExpression := `0`
			if hasDefaultInstall {
				defaultInstallExpression = `COALESCE(default_install, 0)`
			}
			sqlText = fmt.Sprintf(`INSERT INTO browser_extensions (extension_id, name, version, description, icon_data_url, manifest_json, source_url, install_dir, install_mode, package_path, package_hash, enabled, default_install, installed_at, updated_at)
SELECT s.extension_id, s.name, s.version, s.description, %s, s.manifest_json, s.source_url, s.install_dir, %s, %s, %s, s.enabled, %s, s.installed_at, s.updated_at
FROM src.browser_extensions s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_extensions t WHERE t.extension_id = s.extension_id
)`, qualifyBackupExpression(iconExpression, "s"), qualifyBackupExpression(installModeExpression, "s"), qualifyBackupExpression(packagePathExpression, "s"), qualifyBackupExpression(packageHashExpression, "s"), qualifyBackupExpression(defaultInstallExpression, "s"))
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
		if total > inserted {
			stats.Skipped += total - inserted
		}
	}

	if err := a.backupNormalizeImportedDatabasePaths(tx, incomingCfg, existingCoreIDs, existingProfileIDs); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	if _, err := dbConn.Exec(`DETACH DATABASE src`); err != nil {
		return fmt.Errorf("卸载备份数据库失败: %w", err)
	}
	sourceAttached = false
	return nil
}

type backupSourceColumnSpec struct {
	name     string
	fallback string
}

func backupBuildProxyMergeSQL(tx *sql.Tx) (string, error) {
	return backupBuildTableMergeSQL(
		tx,
		"browser_proxies",
		[]backupSourceColumnSpec{
			{name: "proxy_id", fallback: "''"},
			{name: "proxy_name", fallback: "''"},
			{name: "proxy_config", fallback: "''"},
			{name: "preferred_kernel", fallback: "''"},
			{name: "dns_servers", fallback: "''"},
			{name: "group_name", fallback: "''"},
			{name: "source_id", fallback: "''"},
			{name: "source_url", fallback: "''"},
			{name: "source_name_prefix", fallback: "''"},
			{name: "source_auto_refresh", fallback: "0"},
			{name: "source_refresh_interval_m", fallback: "0"},
			{name: "source_last_refresh_at", fallback: "''"},
			{name: "last_latency_ms", fallback: "-1"},
			{name: "last_test_ok", fallback: "0"},
			{name: "last_tested_at", fallback: "''"},
			{name: "last_ip_health_json", fallback: "''"},
			{name: "sort_order", fallback: "0"},
			{name: "created_at", fallback: "CURRENT_TIMESTAMP"},
		},
		`NOT EXISTS (
  SELECT 1 FROM browser_proxies t
  WHERE t.proxy_id = s.proxy_id OR lower(t.proxy_config) = lower(s.proxy_config)
)`,
	)
}

func backupBuildProfileMergeSQL(tx *sql.Tx) (string, error) {
	return backupBuildTableMergeSQL(
		tx,
		"browser_profiles",
		[]backupSourceColumnSpec{
			{name: "profile_id", fallback: "''"},
			{name: "profile_name", fallback: "''"},
			{name: "user_data_dir", fallback: "''"},
			{name: "core_id", fallback: "''"},
			{name: "fingerprint_args", fallback: "'[]'"},
			{name: "proxy_id", fallback: "''"},
			{name: "proxy_config", fallback: "''"},
			{name: "proxy_bind_source_id", fallback: "''"},
			{name: "proxy_bind_source_url", fallback: "''"},
			{name: "proxy_bind_name", fallback: "''"},
			{name: "proxy_bind_updated_at", fallback: "''"},
			{name: "memory_limit_mb", fallback: "0"},
			{name: "launch_args", fallback: "'[]'"},
			{name: "tags", fallback: "'[]'"},
			{name: "keywords", fallback: "'[]'"},
			{name: "group_id", fallback: "''"},
			{name: "created_at", fallback: "CURRENT_TIMESTAMP"},
			{name: "updated_at", fallback: "CURRENT_TIMESTAMP"},
			{name: "restore_last_session", fallback: "''"},
			{name: "deleted_at", fallback: "''"},
		},
		`NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`,
	)
}

func backupBuildTableMergeSQL(tx *sql.Tx, table string, specs []backupSourceColumnSpec, safeWhere string) (string, error) {
	columns := make([]string, 0, len(specs))
	expressions := make([]string, 0, len(specs))
	for _, spec := range specs {
		columns = append(columns, spec.name)
		expression, err := backupSourceColumnExpression(tx, table, "s", spec.name, spec.fallback)
		if err != nil {
			return "", err
		}
		expressions = append(expressions, expression)
	}

	insertPrefix := fmt.Sprintf(
		"INSERT INTO %s (%s)\nSELECT %s\nFROM src.%s s",
		table,
		strings.Join(columns, ", "),
		strings.Join(expressions, ", "),
		table,
	)
	return insertPrefix + "\nWHERE " + safeWhere, nil
}

func backupSourceColumnExpression(tx *sql.Tx, table, alias, column, fallback string) (string, error) {
	exists, err := backupSrcColumnExists(tx, table, column)
	if err != nil {
		return "", err
	}
	if !exists {
		return fallback, nil
	}
	return fmt.Sprintf("COALESCE(%s.%s,%s)", alias, column, fallback), nil
}

func backupListTargetIDs(tx *sql.Tx, table, column string) (map[string]struct{}, error) {
	rows, err := tx.Query(fmt.Sprintf("SELECT %s FROM %s", column, table))
	if err != nil {
		return nil, fmt.Errorf("读取现有数据标识失败(%s): %w", table, err)
	}
	defer rows.Close()

	result := make(map[string]struct{})
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("读取现有数据标识失败(%s): %w", table, err)
		}
		if key := strings.ToLower(strings.TrimSpace(value)); key != "" {
			result[key] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取现有数据标识失败(%s): %w", table, err)
	}
	return result, nil
}

func (a *App) backupNormalizeImportedDatabasePaths(tx *sql.Tx, incomingCfg *config.Config, existingCoreIDs, existingProfileIDs map[string]struct{}) error {
	if incomingCfg == nil {
		return nil
	}

	corePaths := make(map[string]string)
	for _, core := range incomingCfg.Browser.Cores {
		id := strings.TrimSpace(core.CoreId)
		path := strings.TrimSpace(core.CorePath)
		if id != "" && path != "" {
			corePaths[strings.ToLower(id)] = path
		}
	}
	if len(corePaths) > 0 {
		exists, err := backupSrcTableExists(tx, "browser_cores")
		if err != nil {
			return err
		}
		if exists {
			for id, path := range corePaths {
				if _, alreadyExists := existingCoreIDs[id]; alreadyExists {
					continue
				}
				if _, err := tx.Exec(`UPDATE browser_cores
SET core_path = ?
WHERE lower(core_id) = ?
  AND EXISTS (SELECT 1 FROM src.browser_cores s WHERE lower(s.core_id) = ?)`, path, id, id); err != nil {
					return fmt.Errorf("归一化内核路径失败(%s): %w", id, err)
				}
			}
		}
	}

	profilePaths := make(map[string]string)
	for _, profile := range incomingCfg.Browser.Profiles {
		id := strings.TrimSpace(profile.ProfileId)
		path := strings.TrimSpace(profile.UserDataDir)
		if id != "" && path != "" {
			profilePaths[strings.ToLower(id)] = path
		}
	}
	if len(profilePaths) > 0 {
		exists, err := backupSrcTableExists(tx, "browser_profiles")
		if err != nil {
			return err
		}
		if exists {
			for id, path := range profilePaths {
				if _, alreadyExists := existingProfileIDs[id]; alreadyExists {
					continue
				}
				if _, err := tx.Exec(`UPDATE browser_profiles
SET user_data_dir = ?
WHERE lower(profile_id) = ?
  AND EXISTS (SELECT 1 FROM src.browser_profiles s WHERE lower(s.profile_id) = ?)`, path, id, id); err != nil {
					return fmt.Errorf("归一化实例数据路径失败(%s): %w", id, err)
				}
			}
		}
	}
	return nil
}

func qualifyBackupExpression(expression string, tableAlias string) string {
	if strings.TrimSpace(expression) == `''` {
		return expression
	}
	for _, columnName := range []string{"icon_data_url", "install_mode", "package_path", "package_hash", "default_install"} {
		expression = strings.ReplaceAll(expression, columnName, tableAlias+"."+columnName)
	}
	return expression
}
