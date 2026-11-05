package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) AutomationScriptExport(scriptID string) (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	bundle, err := a.automationScriptStore().ExportBundle(strings.TrimSpace(scriptID))
	if err != nil {
		return nil, err
	}

	payload, err := automation.MarshalScriptTemplate(bundle)
	if err != nil {
		return nil, err
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出脚本模板",
		DefaultFilename: buildAutomationScriptTemplateFilename(bundle.Record.Name),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON 模板 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		return map[string]any{
			"cancelled": true,
			"message":   "已取消导出",
		}, nil
	}

	savePath = ensureAutomationScriptTemplateJSONSuffix(savePath)
	if err := os.WriteFile(savePath, payload, 0o644); err != nil {
		return nil, fmt.Errorf("写入脚本模板失败: %w", err)
	}

	return map[string]any{
		"cancelled": false,
		"format":    "json",
		"path":      savePath,
		"fileCount": len(bundle.Files),
		"message":   "模板已导出",
	}, nil
}

func (a *App) AutomationScriptExportZip(scriptID string) (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	bundle, err := a.automationScriptStore().ExportBundle(strings.TrimSpace(scriptID))
	if err != nil {
		return nil, err
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出脚本 ZIP",
		DefaultFilename: buildAutomationScriptPackageZipFilename(bundle.Record.Name),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 脚本包 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		return map[string]any{
			"cancelled": true,
			"message":   "已取消导出",
		}, nil
	}

	savePath = ensureAutomationScriptZipSuffix(savePath)
	if err := automation.WriteScriptPackageZip(savePath, bundle); err != nil {
		return nil, err
	}

	return map[string]any{
		"cancelled": false,
		"format":    "zip",
		"path":      savePath,
		"fileCount": len(bundle.Files),
		"message":   "脚本包已导出",
	}, nil
}

func (a *App) AutomationScriptExportBatchZip(scriptIDs []string) (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	normalizedIDs := make([]string, 0, len(scriptIDs))
	seen := map[string]bool{}
	for _, scriptID := range scriptIDs {
		normalizedID := strings.TrimSpace(scriptID)
		if normalizedID == "" || seen[normalizedID] {
			continue
		}
		seen[normalizedID] = true
		normalizedIDs = append(normalizedIDs, normalizedID)
	}
	if len(normalizedIDs) == 0 {
		return nil, fmt.Errorf("请先勾选要导出的脚本")
	}

	store := a.automationScriptStore()
	bundles := make([]automation.ImportedBundle, 0, len(normalizedIDs))
	fileCount := 0
	for _, scriptID := range normalizedIDs {
		bundle, err := store.ExportBundle(scriptID)
		if err != nil {
			return nil, err
		}
		fileCount += len(bundle.Files)
		bundles = append(bundles, bundle)
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出脚本 ZIP",
		DefaultFilename: buildAutomationScriptBatchZipFilename(len(bundles)),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 脚本包 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		return map[string]any{
			"cancelled": true,
			"message":   "已取消导出",
		}, nil
	}

	savePath = ensureAutomationScriptZipSuffix(savePath)
	if err := automation.WriteScriptPackagesZip(savePath, bundles); err != nil {
		return nil, err
	}

	return map[string]any{
		"cancelled":   false,
		"format":      "zip",
		"path":        savePath,
		"fileCount":   fileCount,
		"scriptCount": len(bundles),
		"message":     "脚本包已导出",
	}, nil
}

func (a *App) AutomationScriptExportDirectory(scriptID string) (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	bundle, err := a.automationScriptStore().ExportBundle(strings.TrimSpace(scriptID))
	if err != nil {
		return nil, err
	}

	baseDir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择导出目录",
	})
	if err != nil {
		return nil, fmt.Errorf("打开目录对话框失败: %w", err)
	}
	if strings.TrimSpace(baseDir) == "" {
		return map[string]any{
			"cancelled": true,
			"message":   "已取消导出",
		}, nil
	}

	targetDir := filepath.Join(
		strings.TrimSpace(baseDir),
		buildAutomationScriptPackageDirectoryName(bundle.Record.Name),
	)
	if err := automation.WriteScriptPackageDirectory(targetDir, bundle); err != nil {
		return nil, err
	}

	return map[string]any{
		"cancelled": false,
		"format":    "directory",
		"path":      targetDir,
		"fileCount": len(bundle.Files),
		"message":   "脚本目录已导出",
	}, nil
}

func buildAutomationScriptTemplateFilename(scriptName string) string {
	name := sanitizeAutomationScriptTemplateFilename(strings.TrimSpace(scriptName))
	if name == "" {
		name = "automation-script"
	}
	return fmt.Sprintf("%s-template-%s.json", name, time.Now().Format("20060102-150405"))
}

func ensureAutomationScriptTemplateJSONSuffix(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasSuffix(strings.ToLower(trimmed), ".json") {
		return trimmed
	}
	return trimmed + ".json"
}

func buildAutomationScriptPackageZipFilename(scriptName string) string {
	name := sanitizeAutomationScriptTemplateFilename(strings.TrimSpace(scriptName))
	if name == "" {
		name = "automation-script"
	}
	return fmt.Sprintf("%s-package-%s.zip", name, time.Now().Format("20060102-150405"))
}

func buildAutomationScriptBatchZipFilename(scriptCount int) string {
	if scriptCount <= 1 {
		return buildAutomationScriptPackageZipFilename("automation-script")
	}
	return fmt.Sprintf("automation-scripts-%d-package-%s.zip", scriptCount, time.Now().Format("20060102-150405"))
}

func ensureAutomationScriptZipSuffix(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasSuffix(strings.ToLower(trimmed), ".zip") {
		return trimmed
	}
	return trimmed + ".zip"
}

func buildAutomationScriptPackageDirectoryName(scriptName string) string {
	name := sanitizeAutomationScriptTemplateFilename(strings.TrimSpace(scriptName))
	if name == "" {
		name = "automation-script"
	}
	return fmt.Sprintf("%s-package-%s", name, time.Now().Format("20060102-150405"))
}

func sanitizeAutomationScriptTemplateFilename(value string) string {
	replacer := strings.NewReplacer(
		"\\", "-",
		"/", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	cleaned := strings.TrimSpace(replacer.Replace(value))
	cleaned = strings.Trim(cleaned, ". ")
	if cleaned == "" {
		return ""
	}
	return cleaned
}
