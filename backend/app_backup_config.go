package backend

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
	"fmt"
	"strings"
)

func (a *App) backupResolveDBPath(cfg *config.Config) string {
	if cfg == nil {
		return a.resolveAppPath("data/app.db")
	}
	path := strings.TrimSpace(cfg.Database.SQLite.Path)
	if path == "" {
		path = "data/app.db"
	}
	return a.resolveAppPath(path)
}

func (a *App) backupResolveUserDataRoot(cfg *config.Config) string {
	if cfg == nil {
		return a.resolveAppPath("data")
	}
	root := strings.TrimSpace(cfg.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	return a.resolveAppPath(root)
}

func (a *App) backupClearBusinessTables() error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"launch_codes", "browser_profiles", "browser_proxies", "browser_cores", "browser_bookmarks", "browser_groups", "browser_extensions", "browser_profile_extension_settings", "browser_profile_extensions", "browser_profile_extension_runtime"}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil && !backupIsNoSuchTableError(err) {
			return fmt.Errorf("清空数据表失败(%s): %w", table, err)
		}
	}
	_, _ = tx.Exec(`DELETE FROM sqlite_sequence WHERE name IN ('browser_bookmarks')`)
	return tx.Commit()
}

func (a *App) backupApplyIncomingConfig(incoming *config.Config, resetFirst bool) error {
	if incoming == nil {
		return nil
	}
	current := a.config
	if current == nil {
		current = config.DefaultConfig()
	}

	var target *config.Config
	if resetFirst {
		cloned := *incoming
		target = &cloned
	} else {
		target = backupMergeConfig(current, incoming)
	}
	target.Database = current.Database

	if err := target.Save(a.resolveAppPath("config.yaml")); err != nil {
		return fmt.Errorf("保存导入配置失败: %w", err)
	}
	a.config = target
	a.applyRuntimeConfig(target.Runtime)
	return nil
}

func backupMergeConfig(current, incoming *config.Config) *config.Config {
	if current == nil {
		cp := *incoming
		return &cp
	}
	if incoming == nil {
		cp := *current
		return &cp
	}
	merged := *current
	if strings.TrimSpace(merged.App.Name) == "" {
		merged.App.Name = incoming.App.Name
	}
	merged.Browser.DefaultBookmarks = backupMergeBookmarks(merged.Browser.DefaultBookmarks, incoming.Browser.DefaultBookmarks)
	merged.Browser.Cores = backupMergeCores(merged.Browser.Cores, incoming.Browser.Cores)
	merged.Browser.Proxies = backupMergeProxies(merged.Browser.Proxies, incoming.Browser.Proxies)
	merged.Browser.Profiles = backupMergeProfiles(merged.Browser.Profiles, incoming.Browser.Profiles)
	return &merged
}

func backupUnionStrings(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, item := range append(append([]string{}, a...), b...) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func backupMergeBookmarks(a, b []config.BrowserBookmark) []config.BrowserBookmark {
	seen := map[string]struct{}{}
	out := make([]config.BrowserBookmark, 0, len(a)+len(b))
	appendOne := func(item config.BrowserBookmark) {
		urlKey := strings.ToLower(strings.TrimSpace(item.URL))
		if urlKey == "" {
			return
		}
		if _, ok := seen[urlKey]; ok {
			return
		}
		seen[urlKey] = struct{}{}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeCores(a, b []config.BrowserCore) []config.BrowserCore {
	seenID := map[string]struct{}{}
	seenPath := map[string]struct{}{}
	out := make([]config.BrowserCore, 0, len(a)+len(b))
	appendOne := func(item config.BrowserCore) {
		idKey := strings.ToLower(strings.TrimSpace(item.CoreId))
		pathKey := strings.ToLower(strings.TrimSpace(item.CorePath))
		if idKey == "" && pathKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if pathKey != "" {
			if _, ok := seenPath[pathKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if pathKey != "" {
			seenPath[pathKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeProxies(a, b []config.BrowserProxy) []config.BrowserProxy {
	seenID := map[string]struct{}{}
	seenCfg := map[string]struct{}{}
	out := make([]config.BrowserProxy, 0, len(a)+len(b))
	appendOne := func(item config.BrowserProxy) {
		idKey := strings.ToLower(strings.TrimSpace(item.ProxyId))
		cfgKey := strings.ToLower(strings.TrimSpace(item.ProxyConfig))
		if idKey == "" && cfgKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if cfgKey != "" {
			if _, ok := seenCfg[cfgKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if cfgKey != "" {
			seenCfg[cfgKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeProfiles(a, b []config.BrowserProfileConfig) []config.BrowserProfileConfig {
	seenID := map[string]struct{}{}
	seenDir := map[string]struct{}{}
	out := make([]config.BrowserProfileConfig, 0, len(a)+len(b))
	appendOne := func(item config.BrowserProfileConfig) {
		idKey := strings.ToLower(strings.TrimSpace(item.ProfileId))
		dirKey := strings.ToLower(strings.TrimSpace(item.UserDataDir))
		if idKey == "" && dirKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if dirKey != "" {
			if _, ok := seenDir[dirKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if dirKey != "" {
			seenDir[dirKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupSrcTableExists(tx *sql.Tx, table string) (bool, error) {
	var cnt int
	err := tx.QueryRow(`SELECT COUNT(1) FROM src.sqlite_master WHERE type='table' AND name=?`, table).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func backupSrcColumnExists(tx *sql.Tx, table string, column string) (bool, error) {
	rows, err := tx.Query("PRAGMA src.table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func backupCountRows(tx *sql.Tx, tableName string) (int, error) {
	var cnt int
	row := tx.QueryRow("SELECT COUNT(1) FROM " + tableName)
	if err := row.Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}
