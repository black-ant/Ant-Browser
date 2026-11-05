package automation

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	DualInstanceRuntimeScriptID = "dual-instance-runtime-switch"
	NewsQueryTXTScriptID        = "news-query-txt"
	ProtonMailFirstMessageID    = "proton-mail-first-message"
	WebImageGenerateScriptID    = "web-image-generate-download"
	LianjiaWHHomeStep1ScriptID  = "lianjia-wh-home-step1"
	LianjiaWHCookiePrepareID    = "lianjia-wh-cookie-prepare"
	BeikeHousePriceExtractID    = "beike-house-price-extract"
	builtinScriptLibraryRoot    = "demo-library"
)

var builtinScriptLibraryPackageDirs = map[string]string{
	DualInstanceRuntimeScriptID: builtinScriptLibraryRoot + "/" + DualInstanceRuntimeScriptID,
	NewsQueryTXTScriptID:        builtinScriptLibraryRoot + "/" + NewsQueryTXTScriptID,
	ProtonMailFirstMessageID:    builtinScriptLibraryRoot + "/" + ProtonMailFirstMessageID,
	WebImageGenerateScriptID:    builtinScriptLibraryRoot + "/" + WebImageGenerateScriptID,
	LianjiaWHHomeStep1ScriptID:  builtinScriptLibraryRoot + "/" + LianjiaWHHomeStep1ScriptID,
	LianjiaWHCookiePrepareID:    builtinScriptLibraryRoot + "/" + LianjiaWHCookiePrepareID,
	BeikeHousePriceExtractID:    builtinScriptLibraryRoot + "/" + BeikeHousePriceExtractID,
}

var builtinScriptLibraryDefaultOrder = []string{
	DualInstanceRuntimeScriptID,
	NewsQueryTXTScriptID,
	ProtonMailFirstMessageID,
	WebImageGenerateScriptID,
	LianjiaWHHomeStep1ScriptID,
	LianjiaWHCookiePrepareID,
	BeikeHousePriceExtractID,
}

//go:embed demo-library
var builtinScriptLibraryFS embed.FS

func DefaultScriptBundles() ([]ImportedBundle, error) {
	bundles := make([]ImportedBundle, 0, len(builtinScriptLibraryDefaultOrder))
	for _, scriptID := range builtinScriptLibraryDefaultOrder {
		dir, ok := builtinScriptLibraryPackageDirs[scriptID]
		if !ok {
			return nil, fmt.Errorf("内置脚本 %q 不存在", scriptID)
		}

		bundle, err := importBuiltinScriptLibraryBundle(dir)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

func ImportBuiltinBundleFromSource(source ScriptSource) (ImportedBundle, error) {
	dir, err := resolveBuiltinScriptLibraryDir(source)
	if err != nil {
		return ImportedBundle{}, err
	}

	bundle, err := importBuiltinScriptLibraryBundle(dir)
	if err != nil {
		return ImportedBundle{}, err
	}
	bundle.Record.Source.ImportedAt = time.Now().Format(time.RFC3339)
	return bundle, nil
}

func resolveBuiltinScriptLibraryDir(source ScriptSource) (string, error) {
	candidates := []string{
		strings.TrimSpace(source.Path),
		path.Base(strings.TrimSpace(strings.TrimPrefix(source.URI, "repo://"))),
		path.Base(strings.TrimSpace(source.URI)),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if dir, ok := builtinScriptLibraryPackageDirs[candidate]; ok {
			return dir, nil
		}
	}

	switch {
	case strings.TrimSpace(source.Path) != "":
		return "", fmt.Errorf("内置脚本 %q 不存在", strings.TrimSpace(source.Path))
	case strings.TrimSpace(source.URI) != "":
		return "", fmt.Errorf("内置脚本来源 %q 不存在", strings.TrimSpace(source.URI))
	default:
		return "", fmt.Errorf("内置脚本来源缺失")
	}
}

func importBuiltinScriptLibraryBundle(packageDir string) (ImportedBundle, error) {
	manifestPath := path.Join(packageDir, scriptPackageManifestName)
	manifestData, err := fs.ReadFile(builtinScriptLibraryFS, manifestPath)
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("read built-in script manifest failed: %w", err)
	}

	descriptor, err := parseImportManifest(manifestData)
	if err != nil {
		return ImportedBundle{}, err
	}

	entryFile := normalizeScriptEntryFile(mapStringValueAny(descriptor, "entryFile"))
	if entryFile == "" {
		return ImportedBundle{}, fmt.Errorf("built-in script manifest is missing entryFile")
	}

	entryData, err := fs.ReadFile(builtinScriptLibraryFS, path.Join(packageDir, entryFile))
	if err != nil {
		return ImportedBundle{}, fmt.Errorf("read built-in script entry failed: %w", err)
	}

	files, err := collectImportedBundleFilesFromFS(builtinScriptLibraryFS, packageDir)
	if err != nil {
		return ImportedBundle{}, err
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
		EntryFile:       entryFile,
		Tags:            mapStringSliceValue(descriptor, "tags"),
		Selector:        descriptor["selector"],
		SelectorText:    descriptor["selectorText"],
		Params:          descriptor["params"],
		ParamsText:      descriptor["paramsText"],
		ScriptText:      string(entryData),
		Notes:           mapStringValueAny(descriptor, "notes"),
		TargetConfig:    mapObjectValue(descriptor, "targetConfig"),
		PublicAPI:       mapObjectValue(descriptor, "publicAPI"),
		Source:          mapObjectValue(descriptor, "source"),
	}, path.Base(packageDir), "")
	if err != nil {
		return ImportedBundle{}, err
	}
	if manifestID := strings.TrimSpace(mapStringValueAny(descriptor, "id")); manifestID != "" {
		record.ID = manifestID
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

func collectImportedBundleFilesFromFS(fsys fs.FS, root string) ([]ImportedBundleFile, error) {
	files := make([]ImportedBundleFile, 0, 8)
	totalSize := 0

	err := fs.WalkDir(fsys, root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}

		relativePath := strings.TrimPrefix(current, root)
		relativePath = strings.TrimPrefix(relativePath, "/")
		if relativePath == "." {
			return nil
		}

		if entry.IsDir() {
			name := strings.ToLower(strings.TrimSpace(entry.Name()))
			if name == ".git" {
				return fs.SkipDir
			}
			if name == "node_modules" {
				return fmt.Errorf("script bundle must not include node_modules")
			}
			return nil
		}

		normalizedPath, err := normalizeBundleFilePath(relativePath)
		if err != nil {
			return err
		}
		if isImportManifestPath(normalizedPath) {
			return nil
		}

		content, err := fs.ReadFile(fsys, current)
		if err != nil {
			return err
		}

		totalSize += len(content)
		if totalSize > maxImportedBundleBytes {
			return fmt.Errorf("script bundle is too large")
		}
		if len(files) >= maxImportedBundleFiles {
			return fmt.Errorf("script bundle contains too many files")
		}

		files = append(files, ImportedBundleFile{
			Path:    normalizedPath,
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect script bundle files failed: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}
