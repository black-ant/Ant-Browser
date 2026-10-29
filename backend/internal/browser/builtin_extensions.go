package browser

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// 内置插件随二进制发布：编译时嵌入，首次启动解包并登记为默认安装。
//
//go:embed builtinassets
var builtinExtensionFS embed.FS

const builtinExtensionAssetRoot = "builtinassets"

type builtinExtensionSpec struct {
	// ID 只作为本应用的插件标识使用，需为一串 a-p 字符。
	ID  string
	Dir string
}

var builtinExtensionSpecs = []builtinExtensionSpec{
	{ID: "ababababcdcdcdefefefefghghghgh", Dir: "iso-pro"},
}

// SyncBuiltinExtensions 把内置插件同步到 data/extensions 并登记到数据库。
// 已存在时不覆盖用户设置（启用状态、默认安装由用户决定），仅在代码缺失或版本变化时刷新文件。
func (m *Manager) SyncBuiltinExtensions() error {
	if m == nil || m.ExtensionDAO == nil {
		return nil
	}
	log := logger.New("Extension")

	for _, spec := range builtinExtensionSpecs {
		sourceDir := path.Join(builtinExtensionAssetRoot, spec.Dir)
		manifestData, err := builtinExtensionFS.ReadFile(path.Join(sourceDir, "manifest.json"))
		if err != nil {
			log.Error("内置插件清单缺失", logger.F("dir", sourceDir), logger.F("error", err))
			continue
		}
		var manifest struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			log.Error("内置插件清单解析失败", logger.F("dir", sourceDir), logger.F("error", err))
			continue
		}

		targetDir := filepath.Clean(m.ResolveRelativePath(filepath.Join("data", "extensions", spec.ID)))
		if m.builtinExtensionNeedsWrite(targetDir, manifest.Version) {
			if err := writeBuiltinExtensionFiles(sourceDir, targetDir); err != nil {
				log.Error("内置插件解包失败", logger.F("dir", targetDir), logger.F("error", err))
				continue
			}
			log.Info("内置插件已解包", logger.F("dir", targetDir), logger.F("version", manifest.Version))
		}

		existing, err := m.ExtensionDAO.Get(spec.ID)
		if err != nil && err != sql.ErrNoRows {
			log.Warn("读取内置插件记录失败", logger.F("extension_id", spec.ID), logger.F("error", err))
			continue
		}
		now := time.Now().Format(time.RFC3339)
		if err == sql.ErrNoRows {
			if err := m.ExtensionDAO.Upsert(Extension{
				ExtensionID:    spec.ID,
				Name:           manifest.Name,
				Version:        manifest.Version,
				Description:    manifest.Description,
				ManifestJSON:   string(manifestData),
				SourceURL:      "builtin://" + spec.Dir,
				InstallDir:     targetDir,
				InstallMode: ExtensionInstallModePersistent,
				Enabled:     true,
				// 内置插件只保证「装完就有」，不默认安装到实例：
				// 由用户在插件包管理里勾「默认安装」或按实例指定。
				DefaultInstall: false,
				InstalledAt:    now,
				UpdatedAt:      now,
			}); err != nil {
				log.Error("登记内置插件失败", logger.F("extension_id", spec.ID), logger.F("error", err))
			}
			continue
		}

		if existing.Version == manifest.Version && existing.InstallDir == targetDir {
			continue
		}
		existing.Version = manifest.Version
		existing.ManifestJSON = string(manifestData)
		existing.InstallDir = targetDir
		existing.SourceURL = "builtin://" + spec.Dir
		existing.UpdatedAt = now
		if err := m.ExtensionDAO.Upsert(existing); err != nil {
			log.Error("更新内置插件记录失败", logger.F("extension_id", spec.ID), logger.F("error", err))
		}
	}
	return nil
}

func (m *Manager) builtinExtensionNeedsWrite(targetDir string, version string) bool {
	manifestData, err := os.ReadFile(filepath.Join(targetDir, "manifest.json"))
	if err != nil {
		return true
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return true
	}
	return strings.TrimSpace(manifest.Version) != strings.TrimSpace(version)
}

func writeBuiltinExtensionFiles(sourceDir string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("创建内置插件目录失败: %w", err)
	}
	return fs.WalkDir(builtinExtensionFS, sourceDir, func(assetPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(filepath.FromSlash(sourceDir), filepath.FromSlash(assetPath))
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		destination := filepath.Join(targetDir, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := builtinExtensionFS.ReadFile(assetPath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}
