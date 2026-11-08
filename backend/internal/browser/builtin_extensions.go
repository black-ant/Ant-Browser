package browser

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
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

// ErrBuiltinExtensionConflict 用户拒绝替换内置插件时返回。
var ErrBuiltinExtensionConflict = errors.New("内置插件版本冲突")

// BuiltinConflictHandler 安装遇到同名同版本内置插件时的回调：
// 返回 true 表示用户确认用本次安装替换内置版本；nil 或返回 false 都会取消安装。
type BuiltinConflictHandler func(conflict Extension) bool

// isBuiltinExtensionSource 判断插件记录是否来自内置登记。
func isBuiltinExtensionSource(sourceURL string) bool {
	return strings.HasPrefix(strings.TrimSpace(sourceURL), "builtin://")
}

// findConflictingBuiltinExtension 查找与给定名称和版本完全相同的内置插件记录。
func (m *Manager) findConflictingBuiltinExtension(name string, version string) (Extension, bool) {
	if m == nil || m.ExtensionDAO == nil {
		return Extension{}, false
	}
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" || version == "" {
		return Extension{}, false
	}
	all, err := m.ExtensionDAO.List()
	if err != nil {
		return Extension{}, false
	}
	for _, item := range all {
		if !isBuiltinExtensionSource(item.SourceURL) {
			continue
		}
		if strings.TrimSpace(item.Name) == name && strings.TrimSpace(item.Version) == version {
			return item, true
		}
	}
	return Extension{}, false
}

// prepareBuiltinReplacement 处理安装前的"同名同版本内置插件"冲突：
// 用户确认替换后先确认内置插件没有被运行中的实例加载（删目录会打断实例），
// 返回待结算的内置插件记录；绑定迁移与记录删除留到 successor 落库之后。
func (m *Manager) prepareBuiltinReplacement(extension *Extension, onBuiltinConflict BuiltinConflictHandler) (*Extension, error) {
	if m == nil || extension == nil {
		return nil, nil
	}
	conflict, has := m.findConflictingBuiltinExtension(extension.Name, extension.Version)
	if !has || conflict.ExtensionID == extension.ExtensionID {
		return nil, nil
	}
	replaced := false
	if onBuiltinConflict != nil {
		replaced = onBuiltinConflict(conflict)
	}
	if !replaced {
		return nil, fmt.Errorf("%w：已存在内置插件「%s」v%s，本次安装未执行", ErrBuiltinExtensionConflict, conflict.Name, conflict.Version)
	}
	// 运行中的实例仍通过 --load-extension 使用内置目录，替换会把正在使用的文件删掉。
	if err := m.EnsureExtensionUnusedByRunningProfiles(conflict.ExtensionID); err != nil {
		return nil, fmt.Errorf("替换内置插件「%s」被拒绝：%w", conflict.Name, err)
	}
	return &conflict, nil
}

// settleBuiltinReplacement 在 successor 记录落库后结算替换：
// 写取代标记（防止下次启动重新登记内置）→ 迁移实例绑定 → 删除内置记录与目录。
// 标记与绑定迁移失败会返回错误（此时内置记录保留、实例绑定仍有效）；
// 删除记录与清理目录属于收尾，失败只记日志，不影响本次安装结果。
func (m *Manager) settleBuiltinReplacement(pending *Extension, successorID string) error {
	if pending == nil {
		return nil
	}
	if m == nil || m.ExtensionDAO == nil {
		return fmt.Errorf("插件管理器未初始化")
	}
	builtinID := strings.TrimSpace(pending.ExtensionID)
	successorID = strings.TrimSpace(successorID)
	if builtinID == "" || successorID == "" {
		return nil
	}
	log := logger.New("Extension")
	// 先写标记：后续步骤即便部分失败，重启同步也不会把内置版本重新登记回来。
	if err := m.ExtensionDAO.SetBuiltinSuperseded(builtinID, successorID); err != nil {
		return fmt.Errorf("记录内置插件取代标记失败: %w", err)
	}
	// 绑定迁移必须成功，否则删除内置记录会让实例绑定指向不存在的插件。
	if err := m.ExtensionDAO.MigrateExtensionReferences(builtinID, successorID); err != nil {
		return fmt.Errorf("迁移实例插件绑定失败: %w", err)
	}
	if err := m.ExtensionDAO.Delete(builtinID); err != nil {
		log.Warn("删除被取代的内置插件记录失败，下次启动将跳过其登记",
			logger.F("extension_id", builtinID), logger.F("error", err.Error()))
	}
	builtinDir := filepath.Clean(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir, builtinID)))
	if err := os.RemoveAll(builtinDir); err != nil {
		// 文件残留不影响正确性：记录已删且同步会跳过，等取代者被删除后内置会重新解包覆盖。
		log.Warn("清理被取代内置插件目录失败", logger.F("dir", builtinDir), logger.F("error", err.Error()))
	}
	log.Info("内置插件已被手动安装的同名同版本插件取代",
		logger.F("builtin_id", builtinID),
		logger.F("successor_id", successorID),
	)
	return nil
}

// SyncBuiltinExtensions 把内置插件同步到 data/extensions 并登记到数据库。
// 已存在时不覆盖用户设置（启用状态、默认安装由用户决定），仅在代码缺失或版本变化时刷新文件。
// 被手动安装的同名同版本插件取代过的内置插件会跳过登记，直到取代者被删除。
func (m *Manager) SyncBuiltinExtensions() error {
	if m == nil || m.ExtensionDAO == nil {
		return nil
	}
	log := logger.New("Extension")

	for _, spec := range builtinExtensionSpecs {
		if successor, err := m.ExtensionDAO.GetBuiltinSupersededBy(spec.ID); err != nil {
			log.Warn("读取内置插件取代标记失败", logger.F("extension_id", spec.ID), logger.F("error", err.Error()))
		} else if successor != "" {
			if _, getErr := m.ExtensionDAO.Get(successor); getErr == nil {
				log.Info("内置插件已被手动安装的同名同版本插件取代，跳过登记",
					logger.F("extension_id", spec.ID),
					logger.F("successor_id", successor),
				)
				continue
			}
			// 取代者已被删除：清除标记，让内置插件重新可用。
			_ = m.ExtensionDAO.ClearBuiltinSuperseded(spec.ID)
		}

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
