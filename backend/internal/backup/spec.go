package backup

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	// PackageFormat 标识导出包格式类型。
	PackageFormat = "ant-chrome-full-backup"
	// ManifestVersion 标识 manifest.json 的结构版本。
	ManifestVersion = 1
)

type Category string

const (
	CategorySystemConfig Category = "system_config"
	CategoryAppData      Category = "app_data"
	CategoryBrowserData  Category = "browser_data"
	CategoryCoreData     Category = "core_data"
	CategoryLogs         Category = "logs"
)

type EntryType string

const (
	EntryTypeFile EntryType = "file"
	EntryTypeDir  EntryType = "dir"
)

// ScopeEntry 描述一个需要进入备份包的源条目。
type ScopeEntry struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	EntryType   EntryType `json:"entryType"`
	Required    bool      `json:"required"`
	SourcePath  string    `json:"sourcePath"`
	ArchivePath string    `json:"archivePath"`
	Exists      bool      `json:"exists"`
	Description string    `json:"description,omitempty"`
}

// Scope 为导出范围定义。
type Scope struct {
	Format          string       `json:"format"`
	ManifestVersion int          `json:"manifestVersion"`
	AppRoot         string       `json:"appRoot"`
	Entries         []ScopeEntry `json:"entries"`
}

// Manifest 用于写入 zip 根目录下的 manifest.json。
type Manifest struct {
	Format          string          `json:"format"`
	ManifestVersion int             `json:"manifestVersion"`
	CreatedAt       string          `json:"createdAt"`
	App             ManifestAppInfo `json:"app"`
	Entries         []ManifestEntry `json:"entries"`
}

type ManifestAppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ManifestEntry 为写入 manifest 的条目（不包含本机绝对路径）。
type ManifestEntry struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	EntryType   EntryType `json:"entryType"`
	Required    bool      `json:"required"`
	ArchivePath string    `json:"archivePath"`
	Description string    `json:"description,omitempty"`
}

type BuildOptions struct {
	AppRoot string
	Config  *config.Config
}

// BuildScope 构建第一阶段的导出范围定义（不执行实际导出）。
func BuildScope(opts BuildOptions) (Scope, error) {
	appRoot := strings.TrimSpace(opts.AppRoot)
	if appRoot == "" {
		return Scope{}, fmt.Errorf("app root 不能为空")
	}
	appRootAbs, err := filepath.Abs(appRoot)
	if err != nil {
		return Scope{}, fmt.Errorf("解析 app root 失败: %w", err)
	}

	cfg := opts.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	builder := newScopeBuilder(appRootAbs)

	builder.add(ScopeEntry{
		ID:          "system_config_main",
		Category:    CategorySystemConfig,
		EntryType:   EntryTypeFile,
		Required:    true,
		SourcePath:  resolvePath(appRootAbs, "config.yaml"),
		ArchivePath: "payload/system/config.yaml",
		Description: "主配置文件",
	})

	builder.add(ScopeEntry{
		ID:          "system_config_proxies",
		Category:    CategorySystemConfig,
		EntryType:   EntryTypeFile,
		Required:    false,
		SourcePath:  resolvePath(appRootAbs, "proxies.yaml"),
		ArchivePath: "payload/system/proxies.yaml",
		Description: "代理配置文件（存在时导出）",
	})

	appDataRoot := resolvePath(appRootAbs, "data")
	builder.add(ScopeEntry{
		ID:          "app_data_root",
		Category:    CategoryAppData,
		EntryType:   EntryTypeDir,
		Required:    true,
		SourcePath:  appDataRoot,
		ArchivePath: "payload/app/data/",
		Description: "应用数据目录（含数据库、快照及默认浏览器数据）",
	})

	userDataRootSetting := strings.TrimSpace(cfg.Browser.UserDataRoot)
	if userDataRootSetting == "" {
		userDataRootSetting = "data"
	}
	userDataRoot := resolvePath(appRootAbs, userDataRootSetting)
	builder.add(ScopeEntry{
		ID:          "browser_user_data_root",
		Category:    CategoryBrowserData,
		EntryType:   EntryTypeDir,
		Required:    true,
		SourcePath:  userDataRoot,
		ArchivePath: "payload/browser/user-data/",
		Description: "浏览器用户数据根目录（若与 data 重合则自动去重）",
	})

	chromeRoot := resolvePath(appRootAbs, "chrome")
	builder.add(ScopeEntry{
		ID:          "browser_core_root",
		Category:    CategoryCoreData,
		EntryType:   EntryTypeDir,
		Required:    false,
		SourcePath:  chromeRoot,
		ArchivePath: "payload/browser/cores/chrome/",
		Description: "默认内核目录",
	})

	corePaths := collectExtraCorePaths(cfg.Browser.Cores, appRootAbs, chromeRoot)
	for idx, corePath := range corePaths {
		coreID := fmt.Sprintf("external-%02d", idx+1)
		builder.add(ScopeEntry{
			ID:          "browser_core_external_" + coreID,
			Category:    CategoryCoreData,
			EntryType:   EntryTypeDir,
			Required:    false,
			SourcePath:  corePath,
			ArchivePath: "payload/browser/cores/external/" + coreID + "/",
			Description: "额外内核目录（来自配置 cores）",
		})
	}

	dbType := strings.TrimSpace(cfg.Database.Type)
	if dbType == "" || strings.EqualFold(dbType, "sqlite") {
		dbPath := strings.TrimSpace(cfg.Database.SQLite.Path)
		if dbPath == "" {
			dbPath = "data/app.db"
		}
		dbAbs := resolvePath(appRootAbs, dbPath)
		builder.add(ScopeEntry{
			ID:          "database_sqlite_main",
			Category:    CategoryAppData,
			EntryType:   EntryTypeFile,
			Required:    true,
			SourcePath:  dbAbs,
			ArchivePath: "payload/app/database/app.db",
			Description: "SQLite 主数据库（若已被 data 覆盖则自动去重）",
		})
		builder.add(ScopeEntry{
			ID:          "database_sqlite_wal",
			Category:    CategoryAppData,
			EntryType:   EntryTypeFile,
			Required:    false,
			SourcePath:  dbAbs + "-wal",
			ArchivePath: "payload/app/database/app.db-wal",
			Description: "SQLite WAL 文件（存在时导出）",
		})
		builder.add(ScopeEntry{
			ID:          "database_sqlite_shm",
			Category:    CategoryAppData,
			EntryType:   EntryTypeFile,
			Required:    false,
			SourcePath:  dbAbs + "-shm",
			ArchivePath: "payload/app/database/app.db-shm",
			Description: "SQLite SHM 文件（存在时导出）",
		})
	}

	logDir := detectLogDir(appRootAbs, strings.TrimSpace(cfg.Logging.FilePath))
	if logDir != "" {
		builder.add(ScopeEntry{
			ID:          "logs_root",
			Category:    CategoryLogs,
			EntryType:   EntryTypeDir,
			Required:    false,
			SourcePath:  logDir,
			ArchivePath: "payload/app/logs/",
			Description: "日志目录（存在时导出）",
		})
	}

	scope := Scope{
		Format:          PackageFormat,
		ManifestVersion: ManifestVersion,
		AppRoot:         appRootAbs,
		Entries:         builder.entries,
	}
	return scope, nil
}

// BuildManifest 根据 Scope 生成 manifest 结构体。
func BuildManifest(scope Scope, appName, appVersion string, createdAt time.Time) Manifest {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	name := strings.TrimSpace(appName)
	if name == "" {
		name = "Maka Browser"
	}
	version := strings.TrimSpace(appVersion)
	if version == "" {
		version = "unknown"
	}

	entries := make([]ManifestEntry, 0, len(scope.Entries))
	for _, item := range scope.Entries {
		entries = append(entries, ManifestEntry{
			ID:          item.ID,
			Category:    item.Category,
			EntryType:   item.EntryType,
			Required:    item.Required,
			ArchivePath: item.ArchivePath,
			Description: item.Description,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return Manifest{
		Format:          PackageFormat,
		ManifestVersion: ManifestVersion,
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		App: ManifestAppInfo{
			Name:    name,
			Version: version,
		},
		Entries: entries,
	}
}

func collectExtraCorePaths(cores []config.BrowserCore, appRootAbs, defaultChromeRoot string) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, core := range cores {
		corePath := strings.TrimSpace(core.CorePath)
		if corePath == "" {
			continue
		}
		coreAbs := resolvePath(appRootAbs, corePath)
		if isPathWithin(coreAbs, defaultChromeRoot) {
			continue
		}
		key := normalizeForCompare(coreAbs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, coreAbs)
	}
	sort.Strings(result)
	return result
}

func detectLogDir(appRootAbs, logPath string) string {
	if logPath == "" {
		return ""
	}
	resolved := resolvePath(appRootAbs, logPath)
	dir := filepath.Dir(resolved)
	if strings.TrimSpace(dir) == "" || dir == "." {
		return ""
	}
	return filepath.Clean(dir)
}

type scopeBuilder struct {
	entries []ScopeEntry
}

func newScopeBuilder(_ string) *scopeBuilder {
	return &scopeBuilder{
		entries: make([]ScopeEntry, 0, 12),
	}
}

func (b *scopeBuilder) add(entry ScopeEntry) {
	if strings.TrimSpace(entry.SourcePath) == "" {
		return
	}
	entry.SourcePath = filepath.Clean(entry.SourcePath)
	entry.ArchivePath = filepath.ToSlash(strings.TrimSpace(entry.ArchivePath))
	if entry.ArchivePath == "" {
		return
	}

	// 已有目录覆盖时，直接跳过，避免重复导出同一文件。
	if b.isCoveredByExisting(entry.SourcePath) {
		return
	}

	for i, existing := range b.entries {
		if samePath(existing.SourcePath, entry.SourcePath) {
			if entry.Required && !existing.Required {
				b.entries[i].Required = true
			}
			return
		}
	}

	entry.Exists = pathExists(entry.SourcePath)
	b.entries = append(b.entries, entry)
	sort.SliceStable(b.entries, func(i, j int) bool {
		return b.entries[i].ID < b.entries[j].ID
	})
}

func (b *scopeBuilder) isCoveredByExisting(candidate string) bool {
	for _, existing := range b.entries {
		switch existing.EntryType {
		case EntryTypeDir:
			if isPathWithin(candidate, existing.SourcePath) {
				return true
			}
		case EntryTypeFile:
			if samePath(candidate, existing.SourcePath) {
				return true
			}
		}
	}
	return false
}

func resolvePath(appRoot, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return filepath.Clean(appRoot)
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(appRoot, p))
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func samePath(a, b string) bool {
	return normalizeForCompare(a) == normalizeForCompare(b)
}

func isPathWithin(path, dir string) bool {
	p := normalizeForCompare(path)
	d := normalizeForCompare(dir)
	if p == d {
		return true
	}
	if d == "" || p == "" {
		return false
	}
	if !strings.HasSuffix(d, string(filepath.Separator)) {
		d += string(filepath.Separator)
	}
	return strings.HasPrefix(p, d)
}

func normalizeForCompare(p string) string {
	normalized := filepath.Clean(strings.TrimSpace(p))
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}
