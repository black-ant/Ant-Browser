package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ExtensionDAO interface {
	List() ([]Extension, error)
	ListEnabled() ([]Extension, error)
	ListDefaultInstall() ([]Extension, error)
	ListByIDs(extensionIDs []string) ([]Extension, error)
	Get(extensionID string) (Extension, error)
	Upsert(extension Extension) error
	SetEnabled(extensionID string, enabled bool) error
	SetDefaultInstall(extensionID string, enabled bool) error
	Delete(extensionID string) error
	GetProfileSettings(profileID string) (ProfileExtensionSettings, error)
	SetProfileSettings(profileID string, extensionIDs []string, configured bool) (ProfileExtensionSettings, error)
	DeleteProfileSettings(profileID string) error
	ListProfileExtensionRuntime(profileID string) ([]ProfileExtensionRuntime, error)
	GetProfileExtensionRuntime(profileID string, extensionID string) (ProfileExtensionRuntime, error)
	UpsertProfileExtensionRuntime(runtime ProfileExtensionRuntime) error
	DeleteProfileExtensionRuntime(profileID string, extensionID string) error
	DeleteProfileExtensionRuntimeForProfile(profileID string) error
}

type SQLiteExtensionDAO struct {
	db *sql.DB
}

func NewSQLiteExtensionDAO(db *sql.DB) *SQLiteExtensionDAO {
	return &SQLiteExtensionDAO{db: db}
}

func (d *SQLiteExtensionDAO) List() ([]Extension, error) {
	return d.listWhere("", nil)
}

func (d *SQLiteExtensionDAO) ListEnabled() ([]Extension, error) {
	return d.listWhere("WHERE enabled = ?", []any{1})
}

func (d *SQLiteExtensionDAO) ListDefaultInstall() ([]Extension, error) {
	return d.listWhere("WHERE enabled = 1 AND default_install = 1", nil)
}

func (d *SQLiteExtensionDAO) ListByIDs(extensionIDs []string) ([]Extension, error) {
	ids := normalizeExtensionIDs(extensionIDs)
	if len(ids) == 0 {
		return []Extension{}, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	return d.listWhere("WHERE enabled = 1 AND extension_id IN ("+strings.Join(placeholders, ",")+")", args)
}

func (d *SQLiteExtensionDAO) Get(extensionID string) (Extension, error) {
	row := d.db.QueryRow(`
		SELECT extension_id, name, version, description, icon_data_url, manifest_json, source_url, install_dir, install_mode, package_path, package_hash, enabled, default_install, installed_at, updated_at
		FROM browser_extensions WHERE extension_id = ?`, strings.TrimSpace(extensionID))
	return scanExtension(row)
}

func (d *SQLiteExtensionDAO) Upsert(extension Extension) error {
	now := time.Now().Format(time.RFC3339)
	if strings.TrimSpace(extension.InstalledAt) == "" {
		extension.InstalledAt = now
	}
	extension.UpdatedAt = now
	_, err := d.db.Exec(`
		INSERT INTO browser_extensions (extension_id, name, version, description, icon_data_url, manifest_json, source_url, install_dir, install_mode, package_path, package_hash, enabled, default_install, installed_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id) DO UPDATE SET
		  name = excluded.name,
		  version = excluded.version,
		  description = excluded.description,
		  icon_data_url = excluded.icon_data_url,
		  manifest_json = excluded.manifest_json,
		  source_url = excluded.source_url,
		  install_dir = excluded.install_dir,
		  install_mode = excluded.install_mode,
		  package_path = excluded.package_path,
		  package_hash = excluded.package_hash,
		  enabled = excluded.enabled,
		  updated_at = excluded.updated_at`,
		extension.ExtensionID,
		extension.Name,
		extension.Version,
		extension.Description,
		extension.IconDataURL,
		extension.ManifestJSON,
		extension.SourceURL,
		extension.InstallDir,
		normalizeExtensionInstallMode(extension.InstallMode),
		extension.PackagePath,
		extension.PackageHash,
		boolToInt(extension.Enabled),
		boolToInt(extension.DefaultInstall),
		extension.InstalledAt,
		extension.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存插件失败: %w", err)
	}
	return nil
}

func (d *SQLiteExtensionDAO) SetEnabled(extensionID string, enabled bool) error {
	result, err := d.db.Exec(
		`UPDATE browser_extensions SET enabled = ?, updated_at = ? WHERE extension_id = ?`,
		boolToInt(enabled), time.Now().Format(time.RFC3339), strings.TrimSpace(extensionID),
	)
	if err != nil {
		return fmt.Errorf("更新插件状态失败: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *SQLiteExtensionDAO) SetDefaultInstall(extensionID string, enabled bool) error {
	result, err := d.db.Exec(
		`UPDATE browser_extensions SET default_install = ?, updated_at = ? WHERE extension_id = ?`,
		boolToInt(enabled), time.Now().Format(time.RFC3339), strings.TrimSpace(extensionID),
	)
	if err != nil {
		return fmt.Errorf("更新插件默认安装状态失败: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *SQLiteExtensionDAO) Delete(extensionID string) error {
	_, err := d.db.Exec(`DELETE FROM browser_extensions WHERE extension_id = ?`, strings.TrimSpace(extensionID))
	if err != nil {
		return fmt.Errorf("删除插件失败: %w", err)
	}
	_, _ = d.db.Exec(`DELETE FROM browser_profile_extensions WHERE extension_id = ?`, strings.TrimSpace(extensionID))
	_, _ = d.db.Exec(`DELETE FROM browser_profile_extension_runtime WHERE extension_id = ?`, strings.TrimSpace(extensionID))
	return nil
}

func (d *SQLiteExtensionDAO) GetProfileSettings(profileID string) (ProfileExtensionSettings, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ProfileExtensionSettings{}, fmt.Errorf("实例 ID 不能为空")
	}
	settings := ProfileExtensionSettings{ProfileID: profileID}
	var configured int
	row := d.db.QueryRow(`SELECT configured, updated_at FROM browser_profile_extension_settings WHERE profile_id = ?`, profileID)
	if err := row.Scan(&configured, &settings.UpdatedAt); err != nil && err != sql.ErrNoRows {
		return ProfileExtensionSettings{}, err
	} else if err == nil {
		settings.Configured = configured != 0
	}

	rows, err := d.db.Query(`SELECT extension_id FROM browser_profile_extensions WHERE profile_id = ? AND enabled = 1 ORDER BY created_at ASC`, profileID)
	if err != nil {
		return ProfileExtensionSettings{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var extensionID string
		if err := rows.Scan(&extensionID); err != nil {
			return ProfileExtensionSettings{}, err
		}
		settings.ExtensionIDs = append(settings.ExtensionIDs, extensionID)
	}
	return settings, rows.Err()
}

func (d *SQLiteExtensionDAO) SetProfileSettings(profileID string, extensionIDs []string, configured bool) (ProfileExtensionSettings, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ProfileExtensionSettings{}, fmt.Errorf("实例 ID 不能为空")
	}
	ids := []string{}
	if configured {
		ids = normalizeExtensionIDs(extensionIDs)
	}
	now := time.Now().Format(time.RFC3339)
	tx, err := d.db.Begin()
	if err != nil {
		return ProfileExtensionSettings{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO browser_profile_extension_settings (profile_id, configured, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET configured = excluded.configured, updated_at = excluded.updated_at`, profileID, boolToInt(configured), now); err != nil {
		return ProfileExtensionSettings{}, err
	}
	if _, err := tx.Exec(`DELETE FROM browser_profile_extensions WHERE profile_id = ?`, profileID); err != nil {
		return ProfileExtensionSettings{}, err
	}
	for _, extensionID := range ids {
		if _, err := tx.Exec(`INSERT INTO browser_profile_extensions (profile_id, extension_id, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`, profileID, extensionID, now, now); err != nil {
			return ProfileExtensionSettings{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ProfileExtensionSettings{}, err
	}
	return d.GetProfileSettings(profileID)
}

func (d *SQLiteExtensionDAO) DeleteProfileSettings(profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM browser_profile_extensions WHERE profile_id = ?`, profileID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM browser_profile_extension_settings WHERE profile_id = ?`, profileID); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *SQLiteExtensionDAO) ListProfileExtensionRuntime(profileID string) ([]ProfileExtensionRuntime, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return []ProfileExtensionRuntime{}, nil
	}
	rows, err := d.db.Query(`
		SELECT profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status, backup_path, last_verified_at, last_error, created_at, updated_at
		FROM browser_profile_extension_runtime WHERE profile_id = ? ORDER BY extension_id ASC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("查询实例插件运行态失败: %w", err)
	}
	defer rows.Close()
	items := []ProfileExtensionRuntime{}
	for rows.Next() {
		item, err := scanProfileExtensionRuntime(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *SQLiteExtensionDAO) GetProfileExtensionRuntime(profileID string, extensionID string) (ProfileExtensionRuntime, error) {
	row := d.db.QueryRow(`
		SELECT profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status, backup_path, last_verified_at, last_error, created_at, updated_at
		FROM browser_profile_extension_runtime WHERE profile_id = ? AND extension_id = ?`, strings.TrimSpace(profileID), strings.TrimSpace(extensionID))
	return scanProfileExtensionRuntime(row)
}

func (d *SQLiteExtensionDAO) UpsertProfileExtensionRuntime(runtimeState ProfileExtensionRuntime) error {
	profileID := strings.TrimSpace(runtimeState.ProfileID)
	extensionID := strings.TrimSpace(runtimeState.ExtensionID)
	if profileID == "" || extensionID == "" {
		return fmt.Errorf("实例插件运行态缺少实例 ID 或插件 ID")
	}
	now := time.Now().Format(time.RFC3339)
	if strings.TrimSpace(runtimeState.CreatedAt) == "" {
		runtimeState.CreatedAt = now
	}
	runtimeState.UpdatedAt = now
	_, err := d.db.Exec(`
		INSERT INTO browser_profile_extension_runtime (profile_id, extension_id, runtime_extension_id, install_mode, installed_version, package_hash, status, backup_path, last_verified_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		profileID,
		extensionID,
		runtimeState.RuntimeExtensionID,
		normalizeExtensionInstallMode(runtimeState.InstallMode),
		runtimeState.InstalledVersion,
		runtimeState.PackageHash,
		runtimeState.Status,
		runtimeState.BackupPath,
		runtimeState.LastVerifiedAt,
		runtimeState.LastError,
		runtimeState.CreatedAt,
		runtimeState.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存实例插件运行态失败: %w", err)
	}
	return nil
}

func (d *SQLiteExtensionDAO) DeleteProfileExtensionRuntime(profileID string, extensionID string) error {
	_, err := d.db.Exec(`DELETE FROM browser_profile_extension_runtime WHERE profile_id = ? AND extension_id = ?`, strings.TrimSpace(profileID), strings.TrimSpace(extensionID))
	return err
}

func (d *SQLiteExtensionDAO) DeleteProfileExtensionRuntimeForProfile(profileID string) error {
	_, err := d.db.Exec(`DELETE FROM browser_profile_extension_runtime WHERE profile_id = ?`, strings.TrimSpace(profileID))
	return err
}

func (d *SQLiteExtensionDAO) listWhere(where string, args []any) ([]Extension, error) {
	query := `
		SELECT extension_id, name, version, description, icon_data_url, manifest_json, source_url, install_dir, install_mode, package_path, package_hash, enabled, default_install, installed_at, updated_at
		FROM browser_extensions ` + where + ` ORDER BY installed_at DESC, name ASC`
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询插件列表失败: %w", err)
	}
	defer rows.Close()

	items := []Extension{}
	for rows.Next() {
		extension, err := scanExtension(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, extension)
	}
	return items, rows.Err()
}

type extensionScanner interface {
	Scan(dest ...any) error
}

func scanExtension(scanner extensionScanner) (Extension, error) {
	var extension Extension
	var enabled int
	var defaultInstall int
	if err := scanner.Scan(
		&extension.ExtensionID,
		&extension.Name,
		&extension.Version,
		&extension.Description,
		&extension.IconDataURL,
		&extension.ManifestJSON,
		&extension.SourceURL,
		&extension.InstallDir,
		&extension.InstallMode,
		&extension.PackagePath,
		&extension.PackageHash,
		&enabled,
		&defaultInstall,
		&extension.InstalledAt,
		&extension.UpdatedAt,
	); err != nil {
		return Extension{}, err
	}
	extension.Enabled = enabled != 0
	extension.DefaultInstall = defaultInstall != 0
	extension.InstallMode = normalizeExtensionInstallMode(extension.InstallMode)
	return extension, nil
}

func scanProfileExtensionRuntime(scanner extensionScanner) (ProfileExtensionRuntime, error) {
	var runtimeState ProfileExtensionRuntime
	if err := scanner.Scan(
		&runtimeState.ProfileID,
		&runtimeState.ExtensionID,
		&runtimeState.RuntimeExtensionID,
		&runtimeState.InstallMode,
		&runtimeState.InstalledVersion,
		&runtimeState.PackageHash,
		&runtimeState.Status,
		&runtimeState.BackupPath,
		&runtimeState.LastVerifiedAt,
		&runtimeState.LastError,
		&runtimeState.CreatedAt,
		&runtimeState.UpdatedAt,
	); err != nil {
		return ProfileExtensionRuntime{}, err
	}
	runtimeState.InstallMode = normalizeExtensionInstallMode(runtimeState.InstallMode)
	return runtimeState, nil
}

func normalizeExtensionInstallMode(value string) string {
	_ = value
	return ExtensionInstallModePersistent
}

func normalizeExtensionIDs(extensionIDs []string) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(extensionIDs))
	for _, extensionID := range extensionIDs {
		id := strings.TrimSpace(extensionID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
