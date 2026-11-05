package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func extractExtensionIDFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.ToLower(strings.TrimSpace(parts[i]))
		if extensionIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func downloadChromeExtensionCRX(ctx context.Context, extensionID string, client *http.Client) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: extensionDownloadTimeout}
	}
	downloadURL := BuildChromeExtensionDownloadURL(extensionID)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		data, err := downloadChromeExtensionCRXOnce(ctx, client, downloadURL)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !isRetryableExtensionDownloadError(err) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
	return nil, formatExtensionDownloadError(lastErr)
}

func downloadChromeExtensionCRXOnce(ctx context.Context, client *http.Client, downloadURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	request.Header.Set("Accept", "*/*")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载插件失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载插件失败: HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, extensionMaxPackageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("读取插件包失败: %w", err)
	}
	if len(data) > extensionMaxPackageBytes {
		return nil, fmt.Errorf("插件包超过限制")
	}
	return data, nil
}

func isRetryableExtensionDownloadError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "connection refused") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "temporarily unavailable")
}

func formatExtensionDownloadError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "eof") {
		return fmt.Errorf("下载插件失败: 连接在下载过程中提前关闭（EOF），已重试 3 次仍失败；请换一个下载代理节点或稍后重试")
	}
	if strings.Contains(message, "connectex") || strings.Contains(message, "dial tcp") || strings.Contains(message, "i/o timeout") {
		return fmt.Errorf("下载插件失败: 无法连接 Chrome 插件下载服务，请确认网络或下载代理可访问 clients2.google.com: %w", err)
	}
	return err
}

func normalizeExtensionArchiveData(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return data, nil
	}
	zipOffset := bytes.Index(data, []byte("PK\x03\x04"))
	if zipOffset < 0 {
		return nil, fmt.Errorf("插件包不是有效的 CRX/ZIP 文件")
	}
	return data[zipOffset:], nil
}

func readExtensionManifestFromZip(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开插件包失败: %w", err)
	}
	for _, file := range reader.File {
		if normalizeZipEntryPath(file.Name) == "manifest.json" {
			return readZipFile(file, 2<<20)
		}
	}
	return nil, fmt.Errorf("插件包缺少 manifest.json")
}

func parseExtensionManifest(data []byte) (extensionManifest, error) {
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return manifest, fmt.Errorf("manifest.json 缺少 version")
	}
	return manifest, nil
}

func readExtensionLocaleMessagesFromZip(data []byte, manifest extensionManifest) map[string]string {
	locale := resolveExtensionLocale(manifest)
	if locale == "" {
		return nil
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(normalizeZipEntryPath(file.Name), "_locales/"+locale+"/messages.json") {
			content, err := readZipFile(file, 1<<20)
			if err != nil {
				return nil
			}
			return parseExtensionLocaleMessages(content)
		}
	}
	return nil
}

func readExtensionLocaleMessagesFromDir(sourceDir string, manifest extensionManifest) map[string]string {
	locale := resolveExtensionLocale(manifest)
	if locale == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(sourceDir, "_locales", locale, "messages.json"))
	if err != nil {
		return nil
	}
	return parseExtensionLocaleMessages(data)
}

func parseExtensionLocaleMessages(data []byte) map[string]string {
	var raw map[string]struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	messages := make(map[string]string, len(raw))
	for key, value := range raw {
		if key = strings.TrimSpace(key); key != "" {
			messages[key] = strings.TrimSpace(value.Message)
		}
	}
	return messages
}

func resolveExtensionLocale(manifest extensionManifest) string {
	locale := strings.TrimSpace(manifest.DefaultLocale)
	locale = strings.Trim(locale, "/\\. ")
	if locale == "" || strings.Contains(locale, "/") || strings.Contains(locale, "\\") {
		return ""
	}
	return locale
}

func resolveExtensionDescription(manifest extensionManifest, messages map[string]string) string {
	return resolveExtensionMessage(strings.TrimSpace(manifest.Description), messages)
}

func resolveExtensionMessage(value string, messages map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "__MSG_") || !strings.HasSuffix(trimmed, "__") {
		return trimmed
	}
	key := strings.TrimSuffix(strings.TrimPrefix(trimmed, "__MSG_"), "__")
	if message := strings.TrimSpace(messages[key]); message != "" {
		return message
	}
	return trimmed
}

func readExtensionIconDataURLFromZip(data []byte, manifest extensionManifest) string {
	iconPath := resolveExtensionIconPath(manifest)
	if iconPath == "" {
		return ""
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(normalizeZipEntryPath(file.Name), iconPath) {
			content, err := readZipFile(file, 1<<20)
			if err != nil {
				return ""
			}
			return extensionIconDataURL(iconPath, content)
		}
	}
	return ""
}

func readExtensionIconDataURLFromDir(sourceDir string, manifest extensionManifest) string {
	iconPath := resolveExtensionIconPath(manifest)
	if iconPath == "" {
		return ""
	}
	fullPath := filepath.Join(sourceDir, filepath.FromSlash(iconPath))
	content, err := os.ReadFile(fullPath)
	if err != nil || len(content) > 1<<20 {
		return ""
	}
	return extensionIconDataURL(iconPath, content)
}

func resolveExtensionIconPath(manifest extensionManifest) string {
	for _, candidate := range []map[string]any{manifest.Action, manifest.BrowserAction} {
		if path := mapStringValue(candidate, "default_icon"); path != "" {
			return normalizeExtensionAssetPath(path)
		}
	}
	bestSize := -1
	bestPath := ""
	for size, path := range manifest.Icons {
		if normalizedPath := normalizeExtensionAssetPath(path); normalizedPath != "" {
			parsedSize := parseExtensionIconSize(size)
			if parsedSize > bestSize {
				bestSize = parsedSize
				bestPath = normalizedPath
			}
		}
	}
	return bestPath
}

func mapStringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	if nested, ok := values[key].(map[string]any); ok {
		bestSize := -1
		bestPath := ""
		for size, rawPath := range nested {
			path, ok := rawPath.(string)
			if !ok {
				continue
			}
			parsedSize := parseExtensionIconSize(size)
			if parsedSize > bestSize {
				bestSize = parsedSize
				bestPath = path
			}
		}
		return bestPath
	}
	return ""
}

func normalizeExtensionAssetPath(value string) string {
	path := strings.TrimSpace(filepath.ToSlash(value))
	path = strings.TrimLeft(path, "/")
	if path == "" || strings.Contains(path, "..") || filepath.IsAbs(path) {
		return ""
	}
	return path
}

func parseExtensionIconSize(value string) int {
	var size int
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &size)
	return size
}

func extensionIconDataURL(path string, data []byte) string {
	if len(data) == 0 || len(data) > 1<<20 {
		return ""
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return ""
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func replaceExtensionDirFromZip(data []byte, installDir string) error {
	tmpDir := installDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("创建插件目录失败: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("打开插件包失败: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		relativePath := normalizeZipEntryPath(file.Name)
		if relativePath == "" {
			continue
		}
		targetPath := filepath.Join(tmpDir, filepath.FromSlash(relativePath))
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			return fmt.Errorf("插件包包含非法路径: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("创建插件文件目录失败: %w", err)
		}
		content, err := readZipFile(file, extensionMaxPackageBytes)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return fmt.Errorf("写入插件文件失败: %w", err)
		}
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("清理旧插件失败: %w", err)
	}
	if err := os.Rename(tmpDir, installDir); err != nil {
		return fmt.Errorf("安装插件失败: %w", err)
	}
	success = true
	return nil
}

func copyExtensionDirectory(sourceDir string, installDir string) error {
	tmpDir := installDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("创建插件目录失败: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	sourceClean, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	if err := filepath.WalkDir(sourceClean, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件目录包含符号链接: %s", path)
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > extensionMaxPackageBytes {
			return fmt.Errorf("插件文件过大: %s", path)
		}
		relativePath, err := filepath.Rel(sourceClean, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(tmpDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	}); err != nil {
		return fmt.Errorf("复制插件目录失败: %w", err)
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("清理旧插件失败: %w", err)
	}
	if err := os.Rename(tmpDir, installDir); err != nil {
		return fmt.Errorf("安装插件失败: %w", err)
	}
	success = true
	return nil
}

func normalizeZipEntryPath(value string) string {
	path := strings.TrimSpace(filepath.ToSlash(value))
	path = strings.TrimLeft(path, "/")
	if path == "" || strings.Contains(path, "..") || filepath.IsAbs(path) {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) > 1 && parts[0] != "" && parts[1] == "manifest.json" {
		return strings.Join(parts[1:], "/")
	}
	return path
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("读取插件文件失败: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("读取插件文件失败: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("插件文件过大: %s", file.Name)
	}
	return data, nil
}

func extensionIDFromManifest(manifestData []byte) string {
	sum := sha256.Sum256(manifestData)
	hexValue := hex.EncodeToString(sum[:16])
	var builder strings.Builder
	for _, char := range hexValue {
		if char >= '0' && char <= '9' {
			builder.WriteByte(byte('a' + char - '0'))
			continue
		}
		builder.WriteByte(byte('k' + char - 'a'))
	}
	return builder.String()
}

func resolveExtensionName(manifest extensionManifest, fallback string) string {
	for _, value := range []string{manifest.Name, manifest.ShortName, fallback} {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return "Chrome 插件"
}
