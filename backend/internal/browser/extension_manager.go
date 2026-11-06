package browser

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	extensionDownloadTimeout     = 90 * time.Second
	extensionMaxPackageBytes     = 128 << 20
	extensionsRootDir            = "extensions"
	defaultChromeDownloadVersion = "148.0.0.0"
)

// DefaultChromeDownloadVersion 返回下载 CRX 时的默认 Chrome 内核版本
func DefaultChromeDownloadVersion() string {
	return defaultChromeDownloadVersion
}

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
	return BuildChromeExtensionDownloadURLWithVersion(extensionID, defaultChromeDownloadVersion)
}

func BuildChromeExtensionDownloadURLWithVersion(extensionID string, chromeVersion string) string {
	normalizedID := NormalizeExtensionID(extensionID)
	if normalizedID == "" {
		return ""
	}
	version := normalizeChromeVersion(chromeVersion)
	if version == "" {
		version = defaultChromeDownloadVersion
	}
	return "https://clients2.google.com/service/update2/crx?response=redirect&prodversion=" + version + "&acceptformat=crx2,crx3&x=id%3D" + normalizedID + "%26installsource%3Dondemand%26uc"
}

func normalizeChromeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(strings.ToLower(trimmed), "chrome/"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[idx+len("chrome/"):])
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	validParts := make([]string, 0, 4)
	for i, part := range parts {
		if i >= 4 {
			break
		}
		cleanPart := ""
		for _, ch := range part {
			if ch >= '0' && ch <= '9' {
				cleanPart += string(ch)
			} else {
				break
			}
		}
		if cleanPart == "" {
			cleanPart = "0"
		}
		validParts = append(validParts, cleanPart)
	}
	for len(validParts) < 4 {
		validParts = append(validParts, "0")
	}
	return strings.Join(validParts, ".")
}

func parseChromeMajorVersion(ver string) int {
	parts := strings.Split(strings.TrimSpace(ver), ".")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[0])
	return n
}

func compareChromeMajorVersion(v1, v2 string) int {
	return parseChromeMajorVersion(v1) - parseChromeMajorVersion(v2)
}

// ResolveDownloadChromeVersion 根据配置的内核动态解析适用于扩展下载的 Chrome 版本号
func (m *Manager) ResolveDownloadChromeVersion() string {
	if m != nil {
		if defaultCore, found := m.GetDefaultCore(); found && strings.TrimSpace(defaultCore.CorePath) != "" {
			if ver := m.GetChromeVersion(defaultCore.CorePath); strings.TrimSpace(ver) != "" {
				if norm := normalizeChromeVersion(ver); norm != "" {
					if compareChromeMajorVersion(norm, defaultChromeDownloadVersion) >= 0 {
						return norm
					}
				}
			}
		}
		for _, core := range m.ListCores() {
			if strings.TrimSpace(core.CorePath) == "" {
				continue
			}
			if ver := m.GetChromeVersion(core.CorePath); strings.TrimSpace(ver) != "" {
				if norm := normalizeChromeVersion(ver); norm != "" {
					if compareChromeMajorVersion(norm, defaultChromeDownloadVersion) >= 0 {
						return norm
					}
				}
			}
		}
	}
	return defaultChromeDownloadVersion
}

func (m *Manager) downloadChromeExtensionCRX(ctx context.Context, extensionID string, client *http.Client) ([]byte, error) {
	version := m.ResolveDownloadChromeVersion()
	return downloadChromeExtensionCRXWithVersion(ctx, extensionID, version, client)
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
	data, err := m.downloadChromeExtensionCRX(context.Background(), extensionID, client)
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
	return m.InstallExtensionFromWebStoreWithHTTPClient(ctx, query, nil)
}

func (m *Manager) InstallExtensionFromWebStoreWithHTTPClient(ctx context.Context, query string, client *http.Client) (Extension, error) {
	extensionID := NormalizeExtensionID(query)
	if extensionID == "" {
		return Extension{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	data, err := m.downloadChromeExtensionCRX(ctx, extensionID, client)
	if err != nil {
		return Extension{}, err
	}
	return m.InstallExtensionPackageBytes(extensionID, BuildChromeWebStoreURL(extensionID), data)
}

func (m *Manager) InstallExtensionPackageBytes(extensionID string, sourceURL string, data []byte) (Extension, error) {
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

	resolvedID := NormalizeExtensionID(extensionID)
	if resolvedID == "" {
		resolvedID = extensionIDFromCRX(data)
		if resolvedID == "" {
			resolvedID = extensionIDFromManifest(manifestData)
		}
	}
	if resolvedID == "" {
		return Extension{}, fmt.Errorf("无法识别插件 ID")
	}

	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), resolvedID)
	if err := replaceExtensionDirFromZip(zipData, installDir); err != nil {
		return Extension{}, err
	}
	installMode := ExtensionInstallModePersistent
	packagePath := ""
	packageHash := ""
	if isCRXExtensionPackage(data) {
		packagePath, packageHash, err = m.storeExtensionPackage(resolvedID, data)
		if err != nil {
			return Extension{}, err
		}
		installMode = ExtensionInstallModePersistent
	}

	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	manifestJSON := string(manifestData)
	extension := Extension{
		ExtensionID:  resolvedID,
		Name:         resolveExtensionMessage(resolveExtensionName(manifest, resolvedID), localeMessages),
		Version:      strings.TrimSpace(manifest.Version),
		Description:  resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:  readExtensionIconDataURLFromZip(zipData, manifest),
		ManifestJSON: manifestJSON,
		SourceURL:    strings.TrimSpace(sourceURL),
		InstallDir:   installDir,
		InstallMode:  installMode,
		PackagePath:  packagePath,
		PackageHash:  packageHash,
		Enabled:      true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		stored, err := m.ExtensionDAO.Get(resolvedID)
		if err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

func (m *Manager) InstallExtensionPackageFile(path string) (Extension, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return Extension{}, fmt.Errorf("插件文件路径不能为空")
	}
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return Extension{}, fmt.Errorf("读取插件文件失败: %w", err)
	}
	return m.InstallExtensionPackageBytes("", normalizedPath, data)
}

func (m *Manager) InstallExtensionDirectory(sourceDir string) (Extension, error) {
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
	extensionID := extensionIDFromManifest(manifestData)
	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), extensionID)
	if err := copyExtensionDirectory(normalizedDir, installDir); err != nil {
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
		InstallDir:   installDir,
		InstallMode:  ExtensionInstallModePersistent,
		Enabled:      true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		if stored, err := m.ExtensionDAO.Get(extensionID); err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

func (m *Manager) EnabledExtensionDirs() []string {
	return nil
}

func (m *Manager) EnabledExtensionDirsForProfile(profileID string) []string {
	return nil
}
