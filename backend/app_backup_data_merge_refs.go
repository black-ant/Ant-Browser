package backend

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const (
	backupImportGroupMapTable     = "backup_import_group_map"
	backupImportCoreMapTable      = "backup_import_core_map"
	backupImportProxyMapTable     = "backup_import_proxy_map"
	backupImportProfileMapTable   = "backup_import_profile_map"
	backupImportExtensionMapTable = "backup_import_extension_map"
)

type backupImportIDMapping struct {
	TargetID string
	Imported bool
}

type backupImportReferenceMappings struct {
	Groups     map[string]backupImportIDMapping
	Cores      map[string]backupImportIDMapping
	Proxies    map[string]backupImportIDMapping
	Profiles   map[string]backupImportIDMapping
	Extensions map[string]backupImportIDMapping
}

type backupImportGroupRecord struct {
	ID        string
	Name      string
	ParentID  string
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

type backupImportCoreRecord struct {
	ID        string
	Name      string
	Path      string
	IsDefault int
	SortOrder int
	CreatedAt string
}

type backupImportProxyRecord struct {
	ID     string
	Config string
}

type backupImportProfileRecord struct {
	ID       string
	UserData string
}

func newBackupImportReferenceMappings() *backupImportReferenceMappings {
	return &backupImportReferenceMappings{
		Groups:     make(map[string]backupImportIDMapping),
		Cores:      make(map[string]backupImportIDMapping),
		Proxies:    make(map[string]backupImportIDMapping),
		Profiles:   make(map[string]backupImportIDMapping),
		Extensions: make(map[string]backupImportIDMapping),
	}
}

func backupImportIDKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func backupImportTextKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func backupImportPathKey(a *App, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	path := filepath.FromSlash(strings.ReplaceAll(value, "\\", "/"))
	if !filepath.IsAbs(path) && a != nil {
		path = a.resolveAppPath(path)
	}
	return backupNormalizePath(path)
}

func backupImportMappedID(values map[string]backupImportIDMapping, sourceID string) string {
	if item, ok := values[backupImportIDKey(sourceID)]; ok {
		return strings.TrimSpace(item.TargetID)
	}
	return ""
}

func backupImportSetMapping(values map[string]backupImportIDMapping, sourceID, targetID string, imported bool) {
	sourceKey := backupImportIDKey(sourceID)
	targetID = strings.TrimSpace(targetID)
	if sourceKey == "" || targetID == "" {
		return
	}
	values[sourceKey] = backupImportIDMapping{TargetID: targetID, Imported: imported}
}

func backupImportGeneratedID(used map[string]struct{}) string {
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

func (a *App) backupBuildImportReferenceMappings(tx *sql.Tx, incomingCfg *config.Config, stats *backupMergeStats) (*backupImportReferenceMappings, error) {
	mappings := newBackupImportReferenceMappings()
	if err := a.backupMapImportGroups(tx, mappings, stats); err != nil {
		return nil, err
	}
	if err := a.backupMapImportCores(tx, incomingCfg, mappings, stats); err != nil {
		return nil, err
	}
	if err := a.backupMapImportProxies(tx, incomingCfg, mappings, stats); err != nil {
		return nil, err
	}
	if err := a.backupMapImportProfiles(tx, incomingCfg, mappings, stats); err != nil {
		return nil, err
	}
	if err := a.backupMapImportExtensions(tx, mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

func backupIncomingCorePaths(cfg *config.Config) map[string]string {
	result := make(map[string]string)
	if cfg == nil {
		return result
	}
	for _, item := range cfg.Browser.Cores {
		if key := backupImportIDKey(item.CoreId); key != "" {
			result[key] = strings.TrimSpace(item.CorePath)
		}
	}
	return result
}

func backupIncomingProxyConfigs(cfg *config.Config) map[string]string {
	result := make(map[string]string)
	if cfg == nil {
		return result
	}
	for _, item := range cfg.Browser.Proxies {
		if key := backupImportIDKey(item.ProxyId); key != "" {
			result[key] = strings.TrimSpace(item.ProxyConfig)
		}
	}
	return result
}

func backupIncomingProfilePaths(cfg *config.Config) map[string]string {
	result := make(map[string]string)
	if cfg == nil {
		return result
	}
	for _, item := range cfg.Browser.Profiles {
		if key := backupImportIDKey(item.ProfileId); key != "" {
			result[key] = strings.TrimSpace(item.UserDataDir)
		}
	}
	return result
}

func backupSortedMappingKeys(values map[string]backupImportIDMapping) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func backupRecordImportConflict(stats *backupMergeStats) {
	if stats != nil {
		stats.Conflicts++
	}
}

func (a *App) backupMapImportGroups(tx *sql.Tx, mappings *backupImportReferenceMappings, stats *backupMergeStats) error {
	targetRows, err := backupQueryImportGroups(tx, "browser_groups")
	if err != nil {
		return fmt.Errorf("读取目标分组失败: %w", err)
	}
	targetByID := make(map[string]backupImportGroupRecord, len(targetRows))
	targetByName := make(map[string]string, len(targetRows))
	usedTargetIDs := make(map[string]struct{}, len(targetRows))
	for _, row := range targetRows {
		key := backupImportIDKey(row.ID)
		if key == "" {
			continue
		}
		targetByID[key] = row
		usedTargetIDs[key] = struct{}{}
		backupImportSetMapping(mappings.Groups, row.ID, row.ID, false)
		targetByName[backupImportTextKey(row.ParentID)+"\x00"+backupImportTextKey(row.Name)] = row.ID
	}

	sourceExists, err := backupSrcTableExists(tx, "browser_groups")
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	sourceRows, err := backupQueryImportGroups(tx, "src.browser_groups")
	if err != nil {
		return fmt.Errorf("读取备份分组失败: %w", err)
	}
	sourceByID := make(map[string]backupImportGroupRecord, len(sourceRows))
	for _, row := range sourceRows {
		if key := backupImportIDKey(row.ID); key != "" {
			sourceByID[key] = row
		}
	}
	states := make(map[string]uint8, len(sourceByID))
	var resolve func(string) error
	resolve = func(sourceID string) error {
		key := backupImportIDKey(sourceID)
		if key == "" || states[key] == 2 {
			return nil
		}
		if states[key] == 1 {
			return nil
		}
		row, ok := sourceByID[key]
		if !ok {
			return nil
		}
		states[key] = 1
		parentTargetID := ""
		parentKey := backupImportIDKey(row.ParentID)
		if parentKey != "" {
			if _, sourceParentExists := sourceByID[parentKey]; sourceParentExists {
				if err := resolve(row.ParentID); err != nil {
					return err
				}
				parentTargetID = backupImportMappedID(mappings.Groups, row.ParentID)
			} else if parent, exists := targetByID[parentKey]; exists {
				parentTargetID = parent.ID
			}
		}
		if err := a.backupMapImportGroupRow(row, parentTargetID, targetByID, targetByName, usedTargetIDs, mappings, stats); err != nil {
			return err
		}
		states[key] = 2
		return nil
	}

	for _, row := range sourceRows {
		if backupImportIDKey(row.ID) == "" {
			continue
		}
		if err := resolve(row.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) backupMapImportGroupRow(row backupImportGroupRecord, parentTargetID string, targetByID map[string]backupImportGroupRecord, targetByName map[string]string, usedTargetIDs map[string]struct{}, mappings *backupImportReferenceMappings, stats *backupMergeStats) error {
	sourceKey := backupImportIDKey(row.ID)
	parentKey := backupImportTextKey(parentTargetID)
	nameKey := backupImportTextKey(row.Name)
	if existing, ok := targetByID[sourceKey]; ok {
		if backupImportTextKey(existing.Name) == nameKey && backupImportTextKey(existing.ParentID) == parentKey {
			backupImportSetMapping(mappings.Groups, row.ID, existing.ID, false)
			return nil
		}
		backupRecordImportConflict(stats)
	}
	if existingID, ok := targetByName[parentKey+"\x00"+nameKey]; ok && !strings.EqualFold(existingID, row.ID) {
		backupImportSetMapping(mappings.Groups, row.ID, existingID, false)
		return nil
	}

	targetID := strings.TrimSpace(row.ID)
	if _, exists := usedTargetIDs[backupImportIDKey(targetID)]; exists {
		targetID = backupImportGeneratedID(usedTargetIDs)
	} else {
		usedTargetIDs[backupImportIDKey(targetID)] = struct{}{}
	}
	inserted := row
	inserted.ID = targetID
	inserted.ParentID = parentTargetID
	targetByID[backupImportIDKey(targetID)] = inserted
	targetByName[parentKey+"\x00"+nameKey] = targetID
	backupImportSetMapping(mappings.Groups, row.ID, targetID, true)
	return nil
}

func (a *App) backupMapImportCores(tx *sql.Tx, incomingCfg *config.Config, mappings *backupImportReferenceMappings, stats *backupMergeStats) error {
	targetRows, err := backupQueryImportCores(tx, "browser_cores")
	if err != nil {
		return fmt.Errorf("读取目标内核失败: %w", err)
	}
	targetByID := make(map[string]backupImportCoreRecord, len(targetRows))
	targetByPath := make(map[string]string, len(targetRows))
	usedTargetIDs := make(map[string]struct{}, len(targetRows))
	for _, row := range targetRows {
		key := backupImportIDKey(row.ID)
		if key == "" {
			continue
		}
		targetByID[key] = row
		usedTargetIDs[key] = struct{}{}
		backupImportSetMapping(mappings.Cores, row.ID, row.ID, false)
		if pathKey := backupImportPathKey(a, row.Path); pathKey != "" {
			targetByPath[pathKey] = row.ID
		}
	}

	sourceExists, err := backupSrcTableExists(tx, "browser_cores")
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	sourceRows, err := backupQueryImportCores(tx, "src.browser_cores")
	if err != nil {
		return fmt.Errorf("读取备份内核失败: %w", err)
	}
	incomingPaths := backupIncomingCorePaths(incomingCfg)
	for _, row := range sourceRows {
		sourceKey := backupImportIDKey(row.ID)
		if sourceKey == "" {
			continue
		}
		desiredPath := strings.TrimSpace(incomingPaths[sourceKey])
		if desiredPath == "" {
			desiredPath = strings.TrimSpace(row.Path)
		}
		desiredPathKey := backupImportPathKey(a, desiredPath)
		if existing, ok := targetByID[sourceKey]; ok {
			if desiredPathKey == "" || desiredPathKey == backupImportPathKey(a, existing.Path) {
				backupImportSetMapping(mappings.Cores, row.ID, existing.ID, false)
				continue
			}
			backupRecordImportConflict(stats)
		}
		if desiredPathKey != "" {
			if existingID, ok := targetByPath[desiredPathKey]; ok {
				backupImportSetMapping(mappings.Cores, row.ID, existingID, false)
				continue
			}
		}

		targetID := strings.TrimSpace(row.ID)
		if _, exists := usedTargetIDs[backupImportIDKey(targetID)]; exists {
			targetID = backupImportGeneratedID(usedTargetIDs)
		} else {
			usedTargetIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		inserted := row
		inserted.ID = targetID
		targetByID[backupImportIDKey(targetID)] = inserted
		if pathKey := backupImportPathKey(a, row.Path); pathKey != "" {
			targetByPath[pathKey] = targetID
		}
		if desiredPathKey != "" {
			targetByPath[desiredPathKey] = targetID
		}
		backupImportSetMapping(mappings.Cores, row.ID, targetID, true)
	}
	return nil
}

func (a *App) backupMapImportProxies(tx *sql.Tx, incomingCfg *config.Config, mappings *backupImportReferenceMappings, stats *backupMergeStats) error {
	targetRows, err := backupQueryImportProxies(tx, "browser_proxies")
	if err != nil {
		return fmt.Errorf("读取目标代理失败: %w", err)
	}
	targetByID := make(map[string]backupImportProxyRecord, len(targetRows))
	targetByConfig := make(map[string]string, len(targetRows))
	usedTargetIDs := make(map[string]struct{}, len(targetRows))
	for _, row := range targetRows {
		key := backupImportIDKey(row.ID)
		if key == "" {
			continue
		}
		targetByID[key] = row
		usedTargetIDs[key] = struct{}{}
		backupImportSetMapping(mappings.Proxies, row.ID, row.ID, false)
		if configKey := backupImportTextKey(row.Config); configKey != "" {
			targetByConfig[configKey] = row.ID
		}
	}

	sourceExists, err := backupSrcTableExists(tx, "browser_proxies")
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	sourceRows, err := backupQueryImportProxies(tx, "src.browser_proxies")
	if err != nil {
		return fmt.Errorf("读取备份代理失败: %w", err)
	}
	incomingConfigs := backupIncomingProxyConfigs(incomingCfg)
	for _, row := range sourceRows {
		sourceKey := backupImportIDKey(row.ID)
		if sourceKey == "" {
			continue
		}
		desiredConfig := strings.TrimSpace(row.Config)
		if desiredConfig == "" {
			desiredConfig = strings.TrimSpace(incomingConfigs[sourceKey])
		}
		desiredConfigKey := backupImportTextKey(desiredConfig)
		if existing, ok := targetByID[sourceKey]; ok {
			if desiredConfigKey == "" || desiredConfigKey == backupImportTextKey(existing.Config) {
				backupImportSetMapping(mappings.Proxies, row.ID, existing.ID, false)
				continue
			}
			backupRecordImportConflict(stats)
		}
		if desiredConfigKey != "" {
			if existingID, ok := targetByConfig[desiredConfigKey]; ok {
				backupImportSetMapping(mappings.Proxies, row.ID, existingID, false)
				continue
			}
		}

		targetID := strings.TrimSpace(row.ID)
		if _, exists := usedTargetIDs[backupImportIDKey(targetID)]; exists {
			targetID = backupImportGeneratedID(usedTargetIDs)
		} else {
			usedTargetIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		targetByID[backupImportIDKey(targetID)] = backupImportProxyRecord{ID: targetID, Config: row.Config}
		if desiredConfigKey != "" {
			targetByConfig[desiredConfigKey] = targetID
		}
		backupImportSetMapping(mappings.Proxies, row.ID, targetID, true)
	}
	return nil
}

func (a *App) backupMapImportProfiles(tx *sql.Tx, incomingCfg *config.Config, mappings *backupImportReferenceMappings, stats *backupMergeStats) error {
	targetRows, err := backupQueryImportProfiles(tx, "browser_profiles")
	if err != nil {
		return fmt.Errorf("读取目标实例失败: %w", err)
	}
	targetByID := make(map[string]backupImportProfileRecord, len(targetRows))
	targetByPath := make(map[string]string, len(targetRows))
	usedTargetIDs := make(map[string]struct{}, len(targetRows))
	for _, row := range targetRows {
		key := backupImportIDKey(row.ID)
		if key == "" {
			continue
		}
		targetByID[key] = row
		usedTargetIDs[key] = struct{}{}
		backupImportSetMapping(mappings.Profiles, row.ID, row.ID, false)
		if pathKey := backupImportPathKey(a, row.UserData); pathKey != "" {
			targetByPath[pathKey] = row.ID
		}
	}

	sourceExists, err := backupSrcTableExists(tx, "browser_profiles")
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	sourceRows, err := backupQueryImportProfiles(tx, "src.browser_profiles")
	if err != nil {
		return fmt.Errorf("读取备份实例失败: %w", err)
	}
	incomingPaths := backupIncomingProfilePaths(incomingCfg)
	for _, row := range sourceRows {
		sourceKey := backupImportIDKey(row.ID)
		if sourceKey == "" {
			continue
		}
		desiredPath := strings.TrimSpace(incomingPaths[sourceKey])
		if desiredPath == "" {
			desiredPath = strings.TrimSpace(row.UserData)
		}
		desiredPathKey := backupImportPathKey(a, desiredPath)
		if existing, ok := targetByID[sourceKey]; ok {
			if desiredPathKey == "" || desiredPathKey == backupImportPathKey(a, existing.UserData) {
				backupImportSetMapping(mappings.Profiles, row.ID, existing.ID, false)
				continue
			}
			backupRecordImportConflict(stats)
		}
		if desiredPathKey != "" {
			if existingID, ok := targetByPath[desiredPathKey]; ok {
				backupImportSetMapping(mappings.Profiles, row.ID, existingID, false)
				continue
			}
		}

		targetID := strings.TrimSpace(row.ID)
		if _, exists := usedTargetIDs[backupImportIDKey(targetID)]; exists {
			targetID = backupImportGeneratedID(usedTargetIDs)
		} else {
			usedTargetIDs[backupImportIDKey(targetID)] = struct{}{}
		}
		inserted := row
		inserted.ID = targetID
		targetByID[backupImportIDKey(targetID)] = inserted
		if desiredPathKey != "" {
			targetByPath[desiredPathKey] = targetID
		}
		if sourcePathKey := backupImportPathKey(a, row.UserData); sourcePathKey != "" {
			targetByPath[sourcePathKey] = targetID
		}
		backupImportSetMapping(mappings.Profiles, row.ID, targetID, true)
	}
	return nil
}

func (a *App) backupMapImportExtensions(tx *sql.Tx, mappings *backupImportReferenceMappings) error {
	targetIDs, err := backupQueryImportIDs(tx, "browser_extensions", "extension_id")
	if err != nil {
		return fmt.Errorf("读取目标插件失败: %w", err)
	}
	targetIDSet := make(map[string]struct{}, len(targetIDs))
	for _, targetID := range targetIDs {
		targetIDSet[backupImportIDKey(targetID)] = struct{}{}
		backupImportSetMapping(mappings.Extensions, targetID, targetID, false)
	}
	sourceExists, err := backupSrcTableExists(tx, "browser_extensions")
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	sourceIDs, err := backupQueryImportIDs(tx, "src.browser_extensions", "extension_id")
	if err != nil {
		return fmt.Errorf("读取备份插件失败: %w", err)
	}
	for _, sourceID := range sourceIDs {
		if backupImportIDKey(sourceID) == "" {
			continue
		}
		imported := true
		if _, exists := targetIDSet[backupImportIDKey(sourceID)]; exists {
			imported = false
		}
		backupImportSetMapping(mappings.Extensions, sourceID, sourceID, imported)
	}
	return nil
}

func backupQueryImportGroups(tx *sql.Tx, relation string) ([]backupImportGroupRecord, error) {
	rows, err := tx.Query(`SELECT group_id, COALESCE(group_name,''), COALESCE(parent_id,''), COALESCE(sort_order,0), COALESCE(created_at,''), COALESCE(updated_at,'') FROM ` + relation + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]backupImportGroupRecord, 0)
	for rows.Next() {
		var row backupImportGroupRecord
		if err := rows.Scan(&row.ID, &row.Name, &row.ParentID, &row.SortOrder, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backupQueryImportCores(tx *sql.Tx, relation string) ([]backupImportCoreRecord, error) {
	rows, err := tx.Query(`SELECT core_id, COALESCE(core_name,''), COALESCE(core_path,''), COALESCE(is_default,0), COALESCE(sort_order,0), COALESCE(created_at,'') FROM ` + relation + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]backupImportCoreRecord, 0)
	for rows.Next() {
		var row backupImportCoreRecord
		if err := rows.Scan(&row.ID, &row.Name, &row.Path, &row.IsDefault, &row.SortOrder, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backupQueryImportProxies(tx *sql.Tx, relation string) ([]backupImportProxyRecord, error) {
	rows, err := tx.Query(`SELECT proxy_id, COALESCE(proxy_config,'') FROM ` + relation + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]backupImportProxyRecord, 0)
	for rows.Next() {
		var row backupImportProxyRecord
		if err := rows.Scan(&row.ID, &row.Config); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backupQueryImportProfiles(tx *sql.Tx, relation string) ([]backupImportProfileRecord, error) {
	rows, err := tx.Query(`SELECT profile_id, COALESCE(user_data_dir,'') FROM ` + relation + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]backupImportProfileRecord, 0)
	for rows.Next() {
		var row backupImportProfileRecord
		if err := rows.Scan(&row.ID, &row.UserData); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backupQueryImportIDs(tx *sql.Tx, relation, column string) ([]string, error) {
	rows, err := tx.Query(`SELECT ` + column + ` FROM ` + relation + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

var backupImportReferenceMapTables = []string{
	backupImportGroupMapTable,
	backupImportCoreMapTable,
	backupImportProxyMapTable,
	backupImportProfileMapTable,
	backupImportExtensionMapTable,
}

func backupDropImportReferenceMapTables(tx *sql.Tx) {
	if tx == nil {
		return
	}
	for _, table := range backupImportReferenceMapTables {
		_, _ = tx.Exec(`DROP TABLE IF EXISTS temp.` + table)
	}
}

func backupImportMappingsForTable(mappings *backupImportReferenceMappings, table string) map[string]backupImportIDMapping {
	if mappings == nil {
		return nil
	}
	switch table {
	case backupImportGroupMapTable:
		return mappings.Groups
	case backupImportCoreMapTable:
		return mappings.Cores
	case backupImportProxyMapTable:
		return mappings.Proxies
	case backupImportProfileMapTable:
		return mappings.Profiles
	case backupImportExtensionMapTable:
		return mappings.Extensions
	default:
		return nil
	}
}

func backupInstallImportReferenceMapTables(tx *sql.Tx, mappings *backupImportReferenceMappings) error {
	backupDropImportReferenceMapTables(tx)
	definitions := map[string]string{
		backupImportGroupMapTable:     `CREATE TEMP TABLE backup_import_group_map (source_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, imported INTEGER NOT NULL)`,
		backupImportCoreMapTable:      `CREATE TEMP TABLE backup_import_core_map (source_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, imported INTEGER NOT NULL)`,
		backupImportProxyMapTable:     `CREATE TEMP TABLE backup_import_proxy_map (source_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, imported INTEGER NOT NULL)`,
		backupImportProfileMapTable:   `CREATE TEMP TABLE backup_import_profile_map (source_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, imported INTEGER NOT NULL)`,
		backupImportExtensionMapTable: `CREATE TEMP TABLE backup_import_extension_map (source_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, imported INTEGER NOT NULL)`,
	}
	for _, table := range backupImportReferenceMapTables {
		if _, err := tx.Exec(definitions[table]); err != nil {
			return fmt.Errorf("创建导入引用映射表失败(%s): %w", table, err)
		}
		values := backupImportMappingsForTable(mappings, table)
		for _, sourceID := range backupSortedMappingKeys(values) {
			mapping := values[sourceID]
			imported := 0
			if mapping.Imported {
				imported = 1
			}
			if _, err := tx.Exec(`INSERT INTO temp.`+table+` (source_id, target_id, imported) VALUES (?, ?, ?)`, sourceID, mapping.TargetID, imported); err != nil {
				return fmt.Errorf("写入导入引用映射失败(%s): %w", table, err)
			}
		}
	}
	return nil
}

func backupImportMappedExpression(baseExpression, mapTable string) string {
	return fmt.Sprintf("COALESCE((SELECT target_id FROM temp.%s WHERE source_id = lower(trim(%s))), %s)", mapTable, baseExpression, baseExpression)
}

func backupImportMapExistsExpression(baseExpression, mapTable string, importedOnly bool) string {
	condition := "1=1"
	if importedOnly {
		condition = "m.imported = 1"
	}
	return fmt.Sprintf("EXISTS (SELECT 1 FROM temp.%s m WHERE m.source_id = lower(trim(%s)) AND %s)", mapTable, baseExpression, condition)
}

func backupBuildMappedTableMergeSQL(tx *sql.Tx, table string, specs []backupSourceColumnSpec, mappingColumns map[string]string, safeWhere string) (string, error) {
	columns := make([]string, 0, len(specs))
	expressions := make([]string, 0, len(specs))
	for _, spec := range specs {
		columns = append(columns, spec.name)
		expression, err := backupSourceColumnExpression(tx, table, "s", spec.name, spec.fallback)
		if err != nil {
			return "", err
		}
		if mapTable := mappingColumns[spec.name]; mapTable != "" {
			expression = backupImportMappedExpression(expression, mapTable)
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
