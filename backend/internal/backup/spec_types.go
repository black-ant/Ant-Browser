package backup

import "ant-chrome/backend/internal/config"

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
	FileCount   int       `json:"fileCount,omitempty"`
	ByteSize    int64     `json:"byteSize,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	Description string    `json:"description,omitempty"`
}

type BuildOptions struct {
	AppRoot string
	Config  *config.Config
}

type scopeBuilder struct {
	entries []ScopeEntry
}
