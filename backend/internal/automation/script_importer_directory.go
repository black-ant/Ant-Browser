package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ImportBundleFromDirectory(rootDir string, targetPath string, sourceLabel string) (ImportedBundle, error) {
	return ImportBundleFromDirectoryWithOptions(rootDir, targetPath, sourceLabel, ImportOptions{})
}

func ImportBundleFromDirectoryWithOptions(rootDir string, targetPath string, sourceLabel string, options ImportOptions) (ImportedBundle, error) {
	baseDir := filepath.Clean(strings.TrimSpace(rootDir))
	if baseDir == "" || baseDir == "." {
		return ImportedBundle{}, fmt.Errorf("script directory is required")
	}

	resolvedPath, err := resolvePathUnderRoot(baseDir, targetPath)
	if err != nil {
		return ImportedBundle{}, err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("stat script path failed: %w", err)
	}

	if !info.IsDir() {
		for _, candidate := range importManifestCandidates {
			if strings.EqualFold(filepath.Base(resolvedPath), candidate) {
				return ImportBundleFromDirectoryWithOptions(filepath.Dir(resolvedPath), "", sourceLabel, options)
			}
		}
		return ImportBundleFromFileWithOptions(resolvedPath, sourceLabel, options)
	}

	manifestPath, err := resolveImportManifest(resolvedPath)
	if err != nil {
		return ImportedBundle{}, err
	}

	if manifestPath == "" {
		entryCandidates := []string{"index.cjs", "index.js", "index.mjs"}
		if options.AllowTypeScriptBuild {
			entryCandidates = append(entryCandidates, "index.ts", "index.cts", "index.mts")
		}
		for _, candidate := range entryCandidates {
			entryPath := filepath.Join(resolvedPath, candidate)
			if _, statErr := os.Stat(entryPath); statErr == nil {
				if isTypeScriptSourceFile(candidate) {
					return importTypeScriptDirectoryBundle(resolvedPath, entryPath, map[string]any{
						"name":      strings.TrimSpace(filepath.Base(resolvedPath)),
						"type":      "playwright-cdp",
						"entryFile": candidate,
					}, sourceLabel)
				}
				return importDirectoryBundle(resolvedPath, entryPath, map[string]any{
					"name":      strings.TrimSpace(filepath.Base(resolvedPath)),
					"type":      "playwright-cdp",
					"entryFile": candidate,
				}, sourceLabel)
			}
		}
		return ImportedBundle{}, fmt.Errorf("no supported script manifest or entry file found in %s", resolvedPath)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("read script manifest failed: %w", err)
	}

	descriptor, err := parseImportManifest(manifestData)
	if err != nil {
		return ImportedBundle{}, err
	}

	entryFile := normalizeScriptEntryFile(mapStringValueAny(descriptor, "entryFile"))
	if isTypeScriptSourceFile(entryFile) {
		if !options.AllowTypeScriptBuild {
			return ImportedBundle{}, fmt.Errorf("当前环境未开启 TypeScript 脚本构建支持，暂不支持 .ts / .mts / .cts 入口")
		}
		entryPath := filepath.Join(resolvedPath, filepath.FromSlash(entryFile))
		if _, err := os.Stat(entryPath); err != nil {
			return ImportedBundle{}, fmt.Errorf("entry file %s not found", entryFile)
		}
		return importTypeScriptDirectoryBundle(resolvedPath, entryPath, descriptor, sourceLabel)
	}

	entryPath := filepath.Join(resolvedPath, filepath.FromSlash(entryFile))
	if _, err := os.Stat(entryPath); err != nil {
		return ImportedBundle{}, fmt.Errorf("entry file %s not found", entryFile)
	}

	return importDirectoryBundle(resolvedPath, entryPath, descriptor, sourceLabel)
}

func importDirectoryBundle(packageRoot string, entryPath string, descriptor map[string]any, sourceLabel string) (ImportedBundle, error) {
	files, err := collectImportedBundleFiles(packageRoot)
	if err != nil {
		return ImportedBundle{}, err
	}

	entryPath = filepath.Clean(entryPath)
	entryRelPath, err := filepath.Rel(packageRoot, entryPath)
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("resolve entry file failed: %w", err)
	}
	entryRelPath = filepath.ToSlash(entryRelPath)

	entryData, err := os.ReadFile(entryPath)
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("read entry file failed: %w", err)
	}

	record, err := buildImportedRecord(scriptImportEnvelope{
		Format:          mapStringValueAny(descriptor, "format"),
		PackageFormat:   mapStringValueAny(descriptor, "packageFormat"),
		ManifestVersion: mapIntValueAny(descriptor, "manifestVersion"),
		ID:              mapStringValueAny(descriptor, "id"),
		Name:            mapStringValueAny(descriptor, "name"),
		Description:     mapStringValueAny(descriptor, "description"),
		Type:            mapStringValueAny(descriptor, "type"),
		Status:          mapStringValueAny(descriptor, "status"),
		EntryFile:       entryRelPath,
		Tags:            mapStringSliceValue(descriptor, "tags"),
		Selector:        descriptor["selector"],
		SelectorText:    descriptor["selectorText"],
		Params:          descriptor["params"],
		ParamsText:      descriptor["paramsText"],
		ScriptText:      string(entryData),
		Notes:           appendImportSourceNote(mapStringValueAny(descriptor, "notes"), sourceLabel),
		TargetConfig:    mapObjectValue(descriptor, "targetConfig"),
		PublicAPI:       mapObjectValue(descriptor, "publicAPI"),
		Source:          mapObjectValue(descriptor, "source"),
	}, filepath.Base(packageRoot), sourceLabel)
	if err != nil {
		return ImportedBundle{}, err
	}

	bundle := ImportedBundle{
		Record: record,
		Files:  files,
	}
	if err := validateImportedBundle(bundle.Record, bundle.Files); err != nil {
		return ImportedBundle{}, err
	}
	return bundle, nil
}
