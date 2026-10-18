package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"database/sql"
	"encoding/json"
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

func (a *App) backupReconcileProxyReferencesWithDatabase(sourceProxies []browser.Proxy) error {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return nil
	}
	targetProxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return fmt.Errorf("读取代理数据库失败: %w", err)
	}
	if len(sourceProxies) == 0 {
		return nil
	}

	targetByID := make(map[string]backupImportProxyRecord, len(targetProxies))
	targetByConfig := make(map[string]string, len(targetProxies))
	usedTargetIDs := make(map[string]struct{}, len(targetProxies))
	for _, item := range targetProxies {
		proxyID := strings.TrimSpace(item.ProxyId)
		if proxyID == "" {
			continue
		}
		idKey := backupImportIDKey(proxyID)
		targetByID[idKey] = backupImportProxyRecord{ID: proxyID, Config: strings.TrimSpace(item.ProxyConfig)}
		usedTargetIDs[idKey] = struct{}{}
		if configKey := backupImportTextKey(item.ProxyConfig); configKey != "" {
			targetByConfig[configKey] = proxyID
		}
	}

	mappings := newBackupImportReferenceMappings()
	importedIDs := make([]string, 0)
	needsConfigSave := false
	seenSourceIDs := make(map[string]struct{}, len(sourceProxies))
	for _, sourceProxy := range sourceProxies {
		sourceID := strings.TrimSpace(sourceProxy.ProxyId)
		sourceConfig := strings.TrimSpace(sourceProxy.ProxyConfig)
		if sourceID == "" || sourceConfig == "" {
			continue
		}
		sourceKey := backupImportIDKey(sourceID)
		if _, exists := seenSourceIDs[sourceKey]; exists {
			continue
		}
		seenSourceIDs[sourceKey] = struct{}{}

		targetID := ""
		if existing, exists := targetByID[sourceKey]; exists && strings.EqualFold(existing.Config, sourceConfig) {
			targetID = existing.ID
		}
		if targetID == "" {
			if existingID, exists := targetByConfig[backupImportTextKey(sourceConfig)]; exists {
				targetID = existingID
			}
		}
		if targetID == "" {
			targetID = sourceID
			if _, exists := usedTargetIDs[backupImportIDKey(targetID)]; exists {
				targetID = backupImportGeneratedID(usedTargetIDs)
			} else {
				usedTargetIDs[backupImportIDKey(targetID)] = struct{}{}
			}
			sourceProxy.ProxyId = targetID
			if err := a.browserMgr.ProxyDAO.Upsert(sourceProxy); err != nil {
				for _, importedID := range importedIDs {
					_ = a.browserMgr.ProxyDAO.Delete(importedID)
				}
				return fmt.Errorf("补充代理到数据库失败(%s): %w", sourceProxy.ProxyName, err)
			}
			targetByID[backupImportIDKey(targetID)] = backupImportProxyRecord{ID: targetID, Config: sourceConfig}
			targetByConfig[backupImportTextKey(sourceConfig)] = targetID
			importedIDs = append(importedIDs, targetID)
		}

		backupImportSetMapping(mappings.Proxies, sourceID, targetID, false)
		if !strings.EqualFold(sourceID, targetID) {
			needsConfigSave = true
		}
	}

	if a.config != nil && needsConfigSave {
		backupApplyImportReferenceMappings(a.config, mappings)
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			for _, importedID := range importedIDs {
				_ = a.browserMgr.ProxyDAO.Delete(importedID)
			}
			return fmt.Errorf("保存代理引用映射失败: %w", err)
		}
	}
	return nil
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

func (a *App) backupMergeDatabaseFromSource(srcDBPath string, incomingCfg *config.Config, stats *backupMergeStats) (*backupImportReferenceMappings, error) {
	if a.db == nil || a.db.GetConn() == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	dbConn := a.db.GetConn()
	tx, err := dbConn.Begin()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	sourceAttached = true
	if err := backupMergeBookmarkState(tx, stats, a.config, incomingCfg); err != nil {
		return nil, err
	}

	mappings, err := a.backupBuildImportReferenceMappings(tx, incomingCfg, stats)
	if err != nil {
		return nil, err
	}
	if err := backupInstallImportReferenceMapTables(tx, mappings); err != nil {
		return nil, err
	}

	mergeTables := []struct {
		name       string
		insertSafe string
	}{
		{
			name: "browser_groups",
			insertSafe: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT (SELECT target_id FROM temp.backup_import_group_map WHERE source_id = lower(trim(s.group_id))), s.group_name,
       COALESCE((SELECT target_id FROM temp.backup_import_group_map WHERE source_id = lower(trim(s.parent_id))), COALESCE(s.parent_id,'')),
       s.sort_order, s.created_at, s.updated_at
FROM src.browser_groups s
				WHERE EXISTS (SELECT 1 FROM temp.backup_import_group_map m WHERE m.source_id = lower(trim(s.group_id)) AND m.imported = 1)`,
		},
		{
			name: "browser_cores",
		},
		{
			name: "browser_proxies",
		},
		{
			name: "browser_proxy_virtual_groups",
		},
		{
			name: "browser_profiles",
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
SELECT COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id), s.configured, s.updated_at
FROM src.browser_profile_extension_settings s
WHERE EXISTS (SELECT 1 FROM temp.backup_import_profile_map m WHERE m.source_id = lower(trim(s.profile_id)))
  AND NOT EXISTS (
    SELECT 1 FROM browser_profile_extension_settings t
    WHERE t.profile_id = COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id)
  )`,
		},
		{
			name: "browser_profile_extensions",
			insertSafe: `INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at)
SELECT COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id),
       COALESCE((SELECT target_id FROM temp.backup_import_extension_map WHERE source_id = lower(trim(s.extension_id))), s.extension_id),
       s.enabled, s.created_at, s.updated_at
FROM src.browser_profile_extensions s
WHERE EXISTS (SELECT 1 FROM temp.backup_import_profile_map p WHERE p.source_id = lower(trim(s.profile_id)))
  AND EXISTS (SELECT 1 FROM temp.backup_import_extension_map e WHERE e.source_id = lower(trim(s.extension_id)))
  AND NOT EXISTS (
    SELECT 1 FROM browser_profile_extensions t
    WHERE t.profile_id = COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id)
      AND t.extension_id = COALESCE((SELECT target_id FROM temp.backup_import_extension_map WHERE source_id = lower(trim(s.extension_id))), s.extension_id)
  )`,
		},
		{
			name: "browser_profile_extension_runtime",
			insertSafe: `INSERT INTO browser_profile_extension_runtime (profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status, backup_path, last_verified_at, last_error, created_at, updated_at)
SELECT COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id),
       COALESCE((SELECT target_id FROM temp.backup_import_extension_map WHERE source_id = lower(trim(s.extension_id))), s.extension_id),
       s.runtime_extension_id, s.install_mode, s.installed_version, s.package_hash, s.status, s.backup_path, s.last_verified_at, s.last_error, s.created_at, s.updated_at
FROM src.browser_profile_extension_runtime s
WHERE EXISTS (SELECT 1 FROM temp.backup_import_profile_map p WHERE p.source_id = lower(trim(s.profile_id)))
  AND EXISTS (SELECT 1 FROM temp.backup_import_extension_map e WHERE e.source_id = lower(trim(s.extension_id)))
  AND NOT EXISTS (
    SELECT 1 FROM browser_profile_extension_runtime t
    WHERE t.profile_id = COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id)
      AND t.extension_id = COALESCE((SELECT target_id FROM temp.backup_import_extension_map WHERE source_id = lower(trim(s.extension_id))), s.extension_id)
  )`,
		},
		{
			name: "launch_codes",
			insertSafe: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id), s.code, s.created_at, s.updated_at
FROM src.launch_codes s
WHERE EXISTS (SELECT 1 FROM temp.backup_import_profile_map m WHERE m.source_id = lower(trim(s.profile_id)) AND m.imported = 1)
  AND NOT EXISTS (
    SELECT 1 FROM launch_codes t
    WHERE t.profile_id = COALESCE((SELECT target_id FROM temp.backup_import_profile_map WHERE source_id = lower(trim(s.profile_id))), s.profile_id)
       OR t.code = s.code
  )`,
		},
	}

	for _, item := range mergeTables {
		exists, err := backupSrcTableExists(tx, item.name)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}

		total, err := backupCountRows(tx, "src."+item.name)
		if err != nil {
			return nil, err
		}
		if total == 0 {
			continue
		}

		sqlText := item.insertSafe
		if item.name == "browser_proxy_virtual_groups" {
			imported, skipped, err := backupMergeDatabaseProxyVirtualGroups(tx, mappings, stats)
			if err != nil {
				return nil, err
			}
			stats.Imported += imported
			stats.Skipped += skipped
			continue
		}
		if item.name == "browser_profiles" {
			hasRestoreLastSession, err := backupSrcColumnExists(tx, item.name, "restore_last_session")
			if err != nil {
				return nil, err
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
			sqlText, err = backupBuildMappedProxyMergeSQL(tx)
			if err != nil {
				return nil, err
			}
		}
		if item.name == "browser_profiles" {
			sqlText, err = backupBuildMappedProfileMergeSQL(tx)
			if err != nil {
				return nil, err
			}
		}
		if item.name == "browser_cores" {
			sqlText, err = backupBuildMappedCoreMergeSQL(tx)
			if err != nil {
				return nil, err
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
				return nil, err
			}
			hasInstallMode, err = backupSrcColumnExists(tx, item.name, "install_mode")
			if err != nil {
				return nil, err
			}
			hasPackagePath, err = backupSrcColumnExists(tx, item.name, "package_path")
			if err != nil {
				return nil, err
			}
			hasPackageHash, err = backupSrcColumnExists(tx, item.name, "package_hash")
			if err != nil {
				return nil, err
			}
			hasDefaultInstall, err = backupSrcColumnExists(tx, item.name, "default_install")
			if err != nil {
				return nil, err
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
			return nil, fmt.Errorf("导入数据表失败(%s): %w", item.name, err)
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

	if err := a.backupNormalizeImportedDatabasePaths(tx, incomingCfg, mappings); err != nil {
		return nil, err
	}

	backupDropImportReferenceMapTables(tx)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	if _, err := dbConn.Exec(`DETACH DATABASE src`); err != nil {
		return nil, fmt.Errorf("卸载备份数据库失败: %w", err)
	}
	sourceAttached = false
	backupApplyImportReferenceMappings(incomingCfg, mappings)
	return mappings, nil
}

type backupBookmarkRecord struct {
	name        string
	url         string
	openOnStart int
	sortOrder   int
}

func backupMergeBookmarkState(tx *sql.Tx, stats *backupMergeStats, current, incoming *config.Config) error {
	targetActiveURLs, err := backupReadBookmarkURLKeys(tx, "browser_bookmarks")
	if err != nil {
		return fmt.Errorf("读取当前书签失败: %w", err)
	}
	targetDeletedURLs, err := backupReadBookmarkURLs(tx, "browser_bookmark_deletions")
	if err != nil {
		return fmt.Errorf("读取当前书签删除标记失败: %w", err)
	}

	sourceDeletedURLs := make([]string, 0)
	sourceDeletionExists, err := backupSrcTableExists(tx, "browser_bookmark_deletions")
	if err != nil {
		return err
	}
	if sourceDeletionExists {
		sourceDeletedURLs, err = backupReadBookmarkURLs(tx, "src.browser_bookmark_deletions")
		if err != nil {
			return fmt.Errorf("读取备份书签删除标记失败: %w", err)
		}
	}

	if current != nil {
		for bookmarkURL := range activeBookmarkURLSet(current.Browser.DefaultBookmarks) {
			targetActiveURLs[bookmarkURL] = struct{}{}
		}
	}
	configDeletedURLs := make([]string, 0)
	for _, cfg := range []*config.Config{current, incoming} {
		if cfg != nil {
			configDeletedURLs = append(configDeletedURLs, cfg.Browser.DeletedBookmarkURLs...)
		}
	}
	deletedURLs := mergeBookmarkDeletionURLs(configDeletedURLs, targetDeletedURLs, sourceDeletedURLs)
	deletedURLs = filterBookmarkDeletionsByActiveURLs(deletedURLs, targetActiveURLs)
	deletedURLSet := make(map[string]struct{}, len(deletedURLs))
	for _, bookmarkURL := range deletedURLs {
		deletedURLSet[bookmarkURL] = struct{}{}
	}

	if _, err := tx.Exec(`DELETE FROM browser_bookmark_deletions`); err != nil {
		return fmt.Errorf("清理旧书签删除标记失败: %w", err)
	}
	for _, bookmarkURL := range deletedURLs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO browser_bookmark_deletions (url) VALUES (?)`, bookmarkURL); err != nil {
			return fmt.Errorf("写入书签删除标记失败: %w", err)
		}
	}

	sourceBookmarks, sourceBookmarksExists, err := backupReadSourceBookmarks(tx)
	if err != nil {
		return err
	}
	if !sourceBookmarksExists {
		return nil
	}
	for _, bookmark := range sourceBookmarks {
		bookmarkURL := bookmarkURLKey(bookmark.url)
		if bookmarkURL == "" {
			backupRecordBookmarkSkipped(stats)
			continue
		}
		if _, exists := targetActiveURLs[bookmarkURL]; exists {
			backupRecordBookmarkSkipped(stats)
			continue
		}
		if _, deleted := deletedURLSet[bookmarkURL]; deleted {
			backupRecordBookmarkSkipped(stats)
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order) VALUES (?, ?, ?, ?)`,
			bookmark.name, bookmark.url, bookmark.openOnStart, bookmark.sortOrder,
		); err != nil {
			return fmt.Errorf("导入书签失败: %w", err)
		}
		targetActiveURLs[bookmarkURL] = struct{}{}
		backupRecordBookmarkImported(stats)
	}
	return nil
}

func backupReadBookmarkURLKeys(tx *sql.Tx, relation string) (map[string]struct{}, error) {
	urls, err := backupReadBookmarkURLs(tx, relation)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(urls))
	for _, rawURL := range urls {
		if bookmarkURL := bookmarkURLKey(rawURL); bookmarkURL != "" {
			keys[bookmarkURL] = struct{}{}
		}
	}
	return keys, nil
}

func backupReadBookmarkURLs(tx *sql.Tx, relation string) ([]string, error) {
	rows, err := tx.Query("SELECT url FROM " + relation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := make([]string, 0)
	for rows.Next() {
		var rawURL string
		if err := rows.Scan(&rawURL); err != nil {
			return nil, err
		}
		urls = append(urls, rawURL)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

func backupReadSourceBookmarks(tx *sql.Tx) ([]backupBookmarkRecord, bool, error) {
	exists, err := backupSrcTableExists(tx, "browser_bookmarks")
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	hasOpenOnStart, err := backupSrcColumnExists(tx, "browser_bookmarks", "open_on_start")
	if err != nil {
		return nil, false, err
	}
	query := `SELECT name, url, 0, sort_order FROM src.browser_bookmarks ORDER BY sort_order, rowid`
	if hasOpenOnStart {
		query = `SELECT name, url, COALESCE(open_on_start, 0), sort_order FROM src.browser_bookmarks ORDER BY sort_order, rowid`
	}
	rows, err := tx.Query(query)
	if err != nil {
		return nil, false, fmt.Errorf("读取备份书签失败: %w", err)
	}
	defer rows.Close()

	bookmarks := make([]backupBookmarkRecord, 0)
	for rows.Next() {
		var bookmark backupBookmarkRecord
		if err := rows.Scan(&bookmark.name, &bookmark.url, &bookmark.openOnStart, &bookmark.sortOrder); err != nil {
			return nil, false, err
		}
		bookmarks = append(bookmarks, bookmark)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return bookmarks, true, nil
}

func backupRecordBookmarkImported(stats *backupMergeStats) {
	if stats != nil {
		stats.Imported++
	}
}

func backupRecordBookmarkSkipped(stats *backupMergeStats) {
	if stats != nil {
		stats.Skipped++
	}
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
			{name: "proxy_mode", fallback: "''"},
			{name: "proxy_group_name", fallback: "''"},
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

type backupProxyVirtualGroupSourceRow struct {
	groupID              string
	groupName            string
	strategy             string
	selectedProxyID      string
	memberProxyIDsJSON   string
	memberGroupNamesJSON string
	sortOrder            int
	createdAt            string
	updatedAt            string
}

func backupMergeDatabaseProxyVirtualGroups(tx *sql.Tx, mappings *backupImportReferenceMappings, stats *backupMergeStats) (int, int, error) {
	if mappings == nil {
		return 0, 0, fmt.Errorf("导入虚拟代理分组缺少代理节点映射")
	}

	groupIDExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "group_id", "''")
	if err != nil {
		return 0, 0, err
	}
	groupNameExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "group_name", "''")
	if err != nil {
		return 0, 0, err
	}
	strategyExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "strategy", "'round_robin'")
	if err != nil {
		return 0, 0, err
	}
	selectedProxyIDExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "selected_proxy_id", "''")
	if err != nil {
		return 0, 0, err
	}
	memberProxyIDsExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "member_proxy_ids", "'[]'")
	if err != nil {
		return 0, 0, err
	}
	memberGroupNamesExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "member_group_names", "'[]'")
	if err != nil {
		return 0, 0, err
	}
	sortOrderExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "sort_order", "0")
	if err != nil {
		return 0, 0, err
	}
	createdAtExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "created_at", "''")
	if err != nil {
		return 0, 0, err
	}
	updatedAtExpression, err := backupSourceColumnExpression(tx, "browser_proxy_virtual_groups", "s", "updated_at", "''")
	if err != nil {
		return 0, 0, err
	}
	rows, err := tx.Query(fmt.Sprintf(`
		SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s
		FROM src.browser_proxy_virtual_groups s
		ORDER BY %s ASC, %s COLLATE NOCASE ASC`,
		groupIDExpression, groupNameExpression, strategyExpression, selectedProxyIDExpression,
		memberProxyIDsExpression, memberGroupNamesExpression, sortOrderExpression, createdAtExpression, updatedAtExpression,
		sortOrderExpression, groupNameExpression))
	if err != nil {
		return 0, 0, fmt.Errorf("读取备份虚拟代理分组失败: %w", err)
	}
	sourceRows := make([]backupProxyVirtualGroupSourceRow, 0)
	for rows.Next() {
		var item backupProxyVirtualGroupSourceRow
		if err := rows.Scan(&item.groupID, &item.groupName, &item.strategy, &item.selectedProxyID, &item.memberProxyIDsJSON, &item.memberGroupNamesJSON, &item.sortOrder, &item.createdAt, &item.updatedAt); err != nil {
			rows.Close()
			return 0, 0, err
		}
		sourceRows = append(sourceRows, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}

	sourceProxies, err := backupQuerySourceProxyVirtualGroupMembers(tx)
	if err != nil {
		return 0, 0, err
	}
	usedGroupIDs, err := backupListTargetIDs(tx, "browser_proxy_virtual_groups", "group_id")
	if err != nil {
		return 0, 0, err
	}
	usedGroupNames := make(map[string]struct{})
	rows, err = tx.Query(`SELECT COALESCE(group_name,'') FROM browser_proxy_virtual_groups`)
	if err != nil {
		return 0, 0, fmt.Errorf("读取现有虚拟代理分组名称失败: %w", err)
	}
	for rows.Next() {
		var groupName string
		if err := rows.Scan(&groupName); err != nil {
			rows.Close()
			return 0, 0, err
		}
		if key := backupImportTextKey(groupName); key != "" {
			usedGroupNames[key] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}

	imported := 0
	for _, sourceRow := range sourceRows {
		group := browser.ProxyVirtualGroup{
			GroupId:         sourceRow.groupID,
			GroupName:       sourceRow.groupName,
			Strategy:        sourceRow.strategy,
			SelectedProxyId: sourceRow.selectedProxyID,
			SortOrder:       sourceRow.sortOrder,
			CreatedAt:       sourceRow.createdAt,
			UpdatedAt:       sourceRow.updatedAt,
		}
		if strings.TrimSpace(sourceRow.memberProxyIDsJSON) != "" {
			if err := json.Unmarshal([]byte(sourceRow.memberProxyIDsJSON), &group.MemberProxyIds); err != nil {
				return 0, 0, fmt.Errorf("解析备份虚拟代理分组代理节点失败(%s): %w", group.GroupName, err)
			}
		}
		if strings.TrimSpace(sourceRow.memberGroupNamesJSON) != "" {
			if err := json.Unmarshal([]byte(sourceRow.memberGroupNamesJSON), &group.MemberGroupNames); err != nil {
				return 0, 0, fmt.Errorf("解析备份虚拟代理分组成员失败(%s): %w", group.GroupName, err)
			}
		}
		group = browser.NormalizeProxyVirtualGroup(browser.ResolveProxyVirtualGroup(group, sourceProxies))
		if group.GroupName == "" {
			continue
		}
		if mappedName := backupImportMappedID(mappings.ProxyVirtualGroups, group.GroupName); mappedName != "" {
			group.GroupName = mappedName
		}
		nameKey := backupImportTextKey(group.GroupName)
		if _, exists := usedGroupNames[nameKey]; exists {
			return 0, 0, fmt.Errorf("虚拟代理分组名称映射冲突(%s)，已停止导入以避免覆盖", group.GroupName)
		}

		mappedProxyIDs := make([]string, 0, len(group.MemberProxyIds))
		seenProxyIDs := make(map[string]struct{}, len(group.MemberProxyIds))
		for _, sourceProxyID := range group.MemberProxyIds {
			targetProxyID := backupImportMappedID(mappings.Proxies, sourceProxyID)
			key := backupImportIDKey(targetProxyID)
			if key == "" {
				backupRecordImportConflict(stats)
				continue
			}
			if _, exists := seenProxyIDs[key]; exists {
				continue
			}
			seenProxyIDs[key] = struct{}{}
			mappedProxyIDs = append(mappedProxyIDs, targetProxyID)
		}
		group.MemberProxyIds = mappedProxyIDs
		if selectedProxyID := strings.TrimSpace(group.SelectedProxyId); selectedProxyID != "" {
			if mappedSelectedProxyID := backupImportMappedID(mappings.Proxies, selectedProxyID); mappedSelectedProxyID != "" {
				group.SelectedProxyId = mappedSelectedProxyID
			} else {
				backupRecordImportConflict(stats)
				group.SelectedProxyId = ""
			}
		}

		targetGroupID := strings.TrimSpace(group.GroupId)
		if targetGroupID == "" {
			targetGroupID = backupImportGeneratedID(usedGroupIDs)
		} else if _, exists := usedGroupIDs[backupImportIDKey(targetGroupID)]; exists {
			targetGroupID = backupImportGeneratedID(usedGroupIDs)
		} else {
			usedGroupIDs[backupImportIDKey(targetGroupID)] = struct{}{}
		}
		memberProxyIDsJSON, err := json.Marshal(group.MemberProxyIds)
		if err != nil {
			return 0, 0, fmt.Errorf("序列化虚拟代理分组代理节点失败(%s): %w", group.GroupName, err)
		}
		memberGroupNamesJSON, err := json.Marshal(group.MemberGroupNames)
		if err != nil {
			return 0, 0, fmt.Errorf("序列化虚拟代理分组成员失败(%s): %w", group.GroupName, err)
		}
		if _, err := tx.Exec(`
			INSERT INTO browser_proxy_virtual_groups (
				group_id, group_name, strategy, selected_proxy_id, member_proxy_ids, member_group_names, sort_order, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			targetGroupID, group.GroupName, group.Strategy, group.SelectedProxyId, string(memberProxyIDsJSON), string(memberGroupNamesJSON), group.SortOrder, group.CreatedAt, group.UpdatedAt,
		); err != nil {
			return 0, 0, fmt.Errorf("导入虚拟代理分组失败(%s): %w", group.GroupName, err)
		}
		usedGroupNames[nameKey] = struct{}{}
		imported++
	}

	return imported, len(sourceRows) - imported, nil
}

func backupQuerySourceProxyVirtualGroupMembers(tx *sql.Tx) ([]browser.Proxy, error) {
	exists, err := backupSrcTableExists(tx, "browser_proxies")
	if err != nil {
		return nil, err
	}
	if !exists {
		return []browser.Proxy{}, nil
	}
	hasGroupName, err := backupSrcColumnExists(tx, "browser_proxies", "group_name")
	if err != nil {
		return nil, err
	}
	groupNameExpression := `''`
	if hasGroupName {
		groupNameExpression = `COALESCE(s.group_name,'')`
	}
	rows, err := tx.Query(fmt.Sprintf(`
		SELECT COALESCE(s.proxy_id,''), %s
		FROM src.browser_proxies s
		ORDER BY s.rowid`, groupNameExpression))
	if err != nil {
		return nil, fmt.Errorf("读取备份代理节点分组失败: %w", err)
	}
	defer rows.Close()
	result := make([]browser.Proxy, 0)
	for rows.Next() {
		var item browser.Proxy
		if err := rows.Scan(&item.ProxyId, &item.GroupName); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) backupNormalizeImportedDatabasePaths(tx *sql.Tx, incomingCfg *config.Config, mappings *backupImportReferenceMappings) error {
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
				mapping, ok := mappings.Cores[id]
				if !ok || !mapping.Imported {
					continue
				}
				if _, err := tx.Exec(`UPDATE browser_cores
		SET core_path = ?
		WHERE lower(core_id) = ?`, path, strings.ToLower(strings.TrimSpace(mapping.TargetID))); err != nil {
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
				mapping, ok := mappings.Profiles[id]
				if !ok || !mapping.Imported {
					continue
				}
				if _, err := tx.Exec(`UPDATE browser_profiles
		SET user_data_dir = ?
		WHERE lower(profile_id) = ?`, path, strings.ToLower(strings.TrimSpace(mapping.TargetID))); err != nil {
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
