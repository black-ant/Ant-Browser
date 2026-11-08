package browser

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	extensionDownloadTimeout = 90 * time.Second
	extensionMaxPackageBytes = 128 << 20
	extensionsRootDir        = "extensions"
)

func ExtensionDownloadTimeout() time.Duration {
	return extensionDownloadTimeout
}

var extensionIDPattern = regexp.MustCompile(`^[a-p]{32}$`)

type extensionManifest struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description"`
	ShortName     string            `json:"short_name"`
	DefaultLocale string            `json:"default_locale"`
	Icons         map[string]string `json:"icons"`
	Action        map[string]any    `json:"action"`
	BrowserAction map[string]any    `json:"browser_action"`
}

func NormalizeExtensionID(value string) string {
	trimmed := strings.TrimSpace(value)
	if parsed := extractExtensionIDFromURL(trimmed); parsed != "" {
		trimmed = parsed
	}
	trimmed = strings.ToLower(strings.Trim(trimmed, "/#?& "))
	if extensionIDPattern.MatchString(trimmed) {
		return trimmed
	}
	return ""
}

func BuildChromeWebStoreURL(extensionID string) string {
	normalizedID := NormalizeExtensionID(extensionID)
	if normalizedID == "" {
		return ""
	}
	return "https://chromewebstore.google.com/detail/" + normalizedID
}

func BuildChromeExtensionDownloadURL(extensionID string) string {
	normalizedID := NormalizeExtensionID(extensionID)
	if normalizedID == "" {
		return ""
	}
	return "https://clients2.google.com/service/update2/crx?response=redirect&prodversion=120.0.0.0&acceptformat=crx2,crx3&x=id%3D" + normalizedID + "%26installsource%3Dondemand%26uc"
}

func (m *Manager) LookupExtension(query string) (ExtensionLookupResult, error) {
	return m.LookupExtensionWithHTTPClient(query, nil)
}

func (m *Manager) LookupExtensionWithHTTPClient(query string, client *http.Client) (ExtensionLookupResult, error) {
	extensionID := NormalizeExtensionID(query)
	if extensionID == "" {
		return ExtensionLookupResult{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	result := ExtensionLookupResult{
		ExtensionID: extensionID,
		Name:        extensionID,
		StoreURL:    BuildChromeWebStoreURL(extensionID),
		Installable: true,
		Message:     "已识别插件 ID，可下载安装",
	}
	data, err := downloadChromeExtensionCRX(context.Background(), extensionID, client)
	if err != nil {
		result.Message = "已识别插件 ID，但暂时无法读取商店元信息: " + err.Error()
		return result, nil
	}
	zipData, err := normalizeExtensionArchiveData(data)
	if err != nil {
		result.Message = "已识别插件 ID，但插件包格式无法解析: " + err.Error()
		return result, nil
	}
	manifestData, err := readExtensionManifestFromZip(zipData)
	if err != nil {
		result.Message = "已识别插件 ID，但 manifest 无法解析: " + err.Error()
		return result, nil
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		result.Message = "已识别插件 ID，但 manifest 无法解析: " + err.Error()
		return result, nil
	}
	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	result.Name = resolveExtensionName(manifest, extensionID)
	result.Name = resolveExtensionMessage(result.Name, localeMessages)
	result.Version = strings.TrimSpace(manifest.Version)
	result.Description = resolveExtensionDescription(manifest, localeMessages)
	result.Message = "已读取插件信息，可下载安装"
	return ExtensionLookupResult{
		ExtensionID: result.ExtensionID,
		Name:        result.Name,
		Version:     result.Version,
		Description: result.Description,
		StoreURL:    result.StoreURL,
		Installable: result.Installable,
		Message:     result.Message,
	}, nil
}

func (m *Manager) InstallExtensionFromWebStore(ctx context.Context, query string) (Extension, error) {
	return m.InstallExtensionFromWebStoreWithHTTPClient(ctx, query, nil, nil)
}

func (m *Manager) InstallExtensionFromWebStoreWithHTTPClient(ctx context.Context, query string, client *http.Client, onBuiltinConflict BuiltinConflictHandler) (Extension, error) {
	extensionID := NormalizeExtensionID(query)
	if extensionID == "" {
		return Extension{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	data, err := downloadChromeExtensionCRX(ctx, extensionID, client)
	if err != nil {
		return Extension{}, err
	}
	return m.InstallExtensionPackageBytes(extensionID, BuildChromeWebStoreURL(extensionID), data, onBuiltinConflict)
}

func (m *Manager) InstallExtensionPackageBytes(extensionID string, sourceURL string, data []byte, onBuiltinConflict BuiltinConflictHandler) (Extension, error) {
	if len(data) == 0 {
		return Extension{}, fmt.Errorf("插件包为空")
	}
	if len(data) > extensionMaxPackageBytes {
		return Extension{}, fmt.Errorf("插件包超过限制")
	}
	zipData, err := normalizeExtensionArchiveData(data)
	if err != nil {
		return Extension{}, err
	}
	manifestData, err := readExtensionManifestFromZip(zipData)
	if err != nil {
		return Extension{}, err
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		return Extension{}, err
	}

	explicitID := NormalizeExtensionID(extensionID)
	if explicitID == "" {
		explicitID = extensionIDFromCRX(data)
	}
	resolvedID := m.resolveExtensionInstallID(explicitID, manifestData)
	if resolvedID == "" {
		return Extension{}, fmt.Errorf("无法识别插件 ID")
	}
	// 运行中的实例正通过 --load-extension 使用这个目录，覆盖安装会把目录整个换掉。
	// 与删除插件保持一致：先拦下来，让用户显式停止实例。
	if err := m.ensureExtensionInstallAllowed(resolvedID); err != nil {
		return Extension{}, err
	}

	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	extension := Extension{
		ExtensionID:  resolvedID,
		Name:         resolveExtensionMessage(resolveExtensionName(manifest, resolvedID), localeMessages),
		Version:      strings.TrimSpace(manifest.Version),
		Description:  resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:  readExtensionIconDataURLFromZip(zipData, manifest),
		ManifestJSON: string(manifestData),
		SourceURL:    strings.TrimSpace(sourceURL),
		InstallMode:  ExtensionInstallModePersistent,
		Enabled:      true,
	}
	pendingBuiltin, err := m.prepareBuiltinReplacement(&extension, onBuiltinConflict)
	if err != nil {
		return Extension{}, err
	}

	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), resolvedID)
	if err := replaceExtensionDirFromZip(zipData, installDir); err != nil {
		return Extension{}, err
	}
	packagePath := ""
	packageHash := ""
	if isCRXExtensionPackage(data) {
		packagePath, packageHash, err = m.storeExtensionPackage(resolvedID, data)
		if err != nil {
			return Extension{}, err
		}
	}

	extension.InstallDir = installDir
	extension.PackagePath = packagePath
	extension.PackageHash = packageHash
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		if err := m.settleBuiltinReplacement(pendingBuiltin, resolvedID); err != nil {
			return Extension{}, fmt.Errorf("插件已安装，但替换内置版本未完成: %w", err)
		}
		stored, err := m.ExtensionDAO.Get(resolvedID)
		if err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

func (m *Manager) InstallExtensionPackageFile(path string, onBuiltinConflict BuiltinConflictHandler) (Extension, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return Extension{}, fmt.Errorf("插件文件路径不能为空")
	}
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return Extension{}, fmt.Errorf("读取插件文件失败: %w", err)
	}
	return m.InstallExtensionPackageBytes("", normalizedPath, data, onBuiltinConflict)
}

func (m *Manager) InstallExtensionDirectory(sourceDir string, onBuiltinConflict BuiltinConflictHandler) (Extension, error) {
	normalizedDir := strings.TrimSpace(sourceDir)
	if normalizedDir == "" {
		return Extension{}, fmt.Errorf("插件目录不能为空")
	}
	manifestPath := filepath.Join(normalizedDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return Extension{}, fmt.Errorf("插件目录缺少 manifest.json: %w", err)
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		return Extension{}, err
	}
	extensionID := m.resolveExtensionInstallID("", manifestData)
	if extensionID == "" {
		return Extension{}, fmt.Errorf("无法识别插件 ID")
	}
	if err := m.ensureExtensionInstallAllowed(extensionID); err != nil {
		return Extension{}, err
	}
	localeMessages := readExtensionLocaleMessagesFromDir(normalizedDir, manifest)
	extension := Extension{
		ExtensionID:  extensionID,
		Name:         resolveExtensionMessage(resolveExtensionName(manifest, extensionID), localeMessages),
		Version:      strings.TrimSpace(manifest.Version),
		Description:  resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:  readExtensionIconDataURLFromDir(normalizedDir, manifest),
		ManifestJSON: string(manifestData),
		SourceURL:    normalizedDir,
		InstallMode:  ExtensionInstallModePersistent,
		Enabled:      true,
	}
	pendingBuiltin, err := m.prepareBuiltinReplacement(&extension, onBuiltinConflict)
	if err != nil {
		return Extension{}, err
	}

	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), extensionID)
	if err := copyExtensionDirectory(normalizedDir, installDir); err != nil {
		return Extension{}, err
	}
	extension.InstallDir = installDir
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		if err := m.settleBuiltinReplacement(pendingBuiltin, extensionID); err != nil {
			return Extension{}, fmt.Errorf("插件已安装，但替换内置版本未完成: %w", err)
		}
		if stored, err := m.ExtensionDAO.Get(extensionID); err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

// resolveExtensionInstallID 解析安装目标的插件 ID。
//
// 优先级：CRX 内嵌 ID > 已安装插件中"同一身份"的 ID > 按 manifest 身份哈希生成。
// 中间这一步是升级覆盖的关键：manifest 去掉 version 后再比对，
// 同一插件的 1.0.0 → 2.0.0 会落到同一个 ID 上，走覆盖安装而不是新增一条记录。
func (m *Manager) resolveExtensionInstallID(explicitID string, manifestData []byte) string {
	explicit := NormalizeExtensionID(explicitID)
	// 同一插件可能从不同渠道装进来：商店（ID 来自 CRX 公钥）与本地 crx/zip
	// （ID 来自 manifest 身份哈希）。先按身份找已装记录，命中就复用它的 ID，
	// 避免同一插件在列表里出现两份、实例里绑的是其中一个。
	if identityID := m.findExtensionIDByIdentity(manifestData); identityID != "" {
		if explicit == "" || !m.extensionExists(explicit) {
			return identityID
		}
		return explicit
	}
	if explicit != "" {
		return explicit
	}
	return extensionIDFromIdentityManifest(manifestData)
}

func (m *Manager) findExtensionIDByIdentity(manifestData []byte) string {
	if m == nil || m.ExtensionDAO == nil {
		return ""
	}
	existing, err := m.ExtensionDAO.List()
	if err != nil {
		return ""
	}
	identity := string(extensionIdentityManifest(manifestData))
	for _, item := range existing {
		if strings.TrimSpace(item.ManifestJSON) == "" {
			continue
		}
		if string(extensionIdentityManifest([]byte(item.ManifestJSON))) == identity {
			return item.ExtensionID
		}
	}
	return ""
}

func (m *Manager) extensionExists(extensionID string) bool {
	if m == nil || m.ExtensionDAO == nil {
		return false
	}
	if _, err := m.ExtensionDAO.Get(extensionID); err != nil {
		return false
	}
	return true
}


// ensureExtensionInstallAllowed 覆盖安装前确认没有运行中的实例正在加载该插件目录。
func (m *Manager) ensureExtensionInstallAllowed(extensionID string) error {
	if m == nil || strings.TrimSpace(extensionID) == "" {
		return nil
	}
	if err := m.EnsureExtensionUnusedByRunningProfiles(extensionID); err != nil {
		return err
	}
	return nil
}

func (m *Manager) EnabledExtensionDirs() []string {
	return nil
}


func (m *Manager) EnabledExtensionDirsForProfile(profileID string) []string {
	return nil
}
