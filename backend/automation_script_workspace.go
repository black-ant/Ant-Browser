package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
)

func (a *App) preparePlaywrightScriptWorkspace(runtimeDir string, script automation.ScriptRecord) (string, string, func(), error) {
	execRoot := filepath.Join(runtimeDir, "tmp", "script-run", fmt.Sprintf("%s-%d", strings.TrimSpace(script.ID), time.Now().UnixNano()))
	scriptPath := filepath.Join(execRoot, filepath.FromSlash(script.EntryFile))
	artifactDir := filepath.Join(a.automationArtifactsRootDir(), strings.TrimSpace(script.ID), time.Now().Format("20060102-150405"))

	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", "", nil, fmt.Errorf("create script artifact dir failed: %w", err)
	}

	scriptDir, err := a.automationScriptStore().Dir(script.ID)
	if err == nil {
		if _, statErr := os.Stat(scriptDir); statErr == nil {
			if copyErr := copyAutomationScriptDir(scriptDir, execRoot); copyErr != nil {
				return "", "", nil, copyErr
			}
		} else if !os.IsNotExist(statErr) {
			return "", "", nil, fmt.Errorf("stat script workspace failed: %w", statErr)
		}
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return "", "", nil, fmt.Errorf("create script workspace failed: %w", err)
	}
	if err := os.WriteFile(scriptPath, []byte(script.ScriptText), 0o644); err != nil {
		return "", "", nil, fmt.Errorf("write script workspace failed: %w", err)
	}
	if err := writePlaywrightCompatModule(execRoot, runtimeDir); err != nil {
		return "", "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(execRoot)
	}
	return scriptPath, artifactDir, cleanup, nil
}

func (a *App) automationArtifactsRootDir() string {
	configured := ""
	if a.config != nil {
		configured = strings.TrimSpace(a.config.Automation.ArtifactsDir)
	}
	if configured == "" {
		configured = filepath.ToSlash(filepath.Join("data", "automation", "artifacts"))
	}
	return a.resolveAppPath(configured)
}

func copyAutomationScriptDir(srcDir string, dstDir string) error {
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return os.MkdirAll(dstDir, 0o755)
		}

		targetPath := filepath.Join(dstDir, relativePath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	}); err != nil {
		return fmt.Errorf("copy script workspace failed: %w", err)
	}
	return nil
}

func writePlaywrightCompatModule(execRoot string, runtimeDir string) error {
	target := filepath.Join(runtimeDir, "node_modules", "playwright-core")
	for _, packageName := range []string{"playwright", "playwright-core"} {
		compatDir := filepath.Join(execRoot, "node_modules", packageName)
		if err := os.MkdirAll(compatDir, 0o755); err != nil {
			return fmt.Errorf("create %s compatibility module failed: %w", packageName, err)
		}

		content := fmt.Sprintf("module.exports = require(%q)\n", target)
		if err := os.WriteFile(filepath.Join(compatDir, "index.js"), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s compatibility module failed: %w", packageName, err)
		}
		packageJSON := fmt.Sprintf("{\"name\":%q,\"main\":\"index.js\"}\n", packageName)
		if err := os.WriteFile(filepath.Join(compatDir, "package.json"), []byte(packageJSON), 0o644); err != nil {
			return fmt.Errorf("write %s compatibility package.json failed: %w", packageName, err)
		}
	}
	return nil
}
