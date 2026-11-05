package automation

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	scriptPackageManifestName = "automation.script.json"
	maxImportedZipFiles       = maxImportedBundleFiles + 8
	maxImportedZipBytes       = maxImportedBundleBytes + (256 << 10)
)

func WriteScriptPackageZip(zipPath string, bundle ImportedBundle) error {
	normalizedPath := strings.TrimSpace(zipPath)
	if normalizedPath == "" {
		return fmt.Errorf("script zip path is required")
	}

	record, files, err := collectScriptPackageExportFiles(bundle)
	if err != nil {
		return err
	}

	manifestData, err := MarshalScriptPackageManifest(record)
	if err != nil {
		return fmt.Errorf("marshal script package manifest failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(normalizedPath), 0o755); err != nil {
		return fmt.Errorf("create script zip dir failed: %w", err)
	}

	tmpPath := normalizedPath + ".tmp"
	_ = os.Remove(tmpPath)

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create script zip failed: %w", err)
	}

	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	writer := zip.NewWriter(file)
	if err := writeScriptZipEntry(writer, scriptPackageManifestName, manifestData); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write script package manifest failed: %w", err)
	}
	for _, bundleFile := range files {
		if err := writeScriptZipEntry(writer, bundleFile.Path, bundleFile.Content); err != nil {
			_ = writer.Close()
			return fmt.Errorf("write script package file %s failed: %w", bundleFile.Path, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize script zip failed: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close script zip failed: %w", err)
	}
	if err := replaceFile(tmpPath, normalizedPath); err != nil {
		return fmt.Errorf("move script zip failed: %w", err)
	}

	success = true
	return nil
}

func WriteScriptPackagesZip(zipPath string, bundles []ImportedBundle) error {
	normalizedPath := strings.TrimSpace(zipPath)
	if normalizedPath == "" {
		return fmt.Errorf("script zip path is required")
	}
	if len(bundles) == 0 {
		return fmt.Errorf("script bundles are required")
	}

	type packageFiles struct {
		Root         string
		Record       ScriptRecord
		ManifestData []byte
		Files        []ImportedBundleFile
	}

	packages := make([]packageFiles, 0, len(bundles))
	usedRoots := map[string]int{}
	for _, bundle := range bundles {
		record, files, err := collectScriptPackageExportFiles(bundle)
		if err != nil {
			return err
		}

		manifestData, err := MarshalScriptPackageManifest(record)
		if err != nil {
			return fmt.Errorf("marshal script package manifest failed: %w", err)
		}

		root := buildScriptPackageZipRoot(record)
		usedRoots[root]++
		if usedRoots[root] > 1 {
			root = fmt.Sprintf("%s-%d", root, usedRoots[root])
		}

		packages = append(packages, packageFiles{
			Root:         root,
			Record:       record,
			ManifestData: manifestData,
			Files:        files,
		})
	}

	if err := os.MkdirAll(filepath.Dir(normalizedPath), 0o755); err != nil {
		return fmt.Errorf("create script zip dir failed: %w", err)
	}

	tmpPath := normalizedPath + ".tmp"
	_ = os.Remove(tmpPath)

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create script zip failed: %w", err)
	}

	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	writer := zip.NewWriter(file)
	for _, item := range packages {
		manifestPath := filepath.ToSlash(filepath.Join(item.Root, scriptPackageManifestName))
		if err := writeScriptZipEntry(writer, manifestPath, item.ManifestData); err != nil {
			_ = writer.Close()
			return fmt.Errorf("write script package manifest failed: %w", err)
		}
		for _, bundleFile := range item.Files {
			archivePath := filepath.ToSlash(filepath.Join(item.Root, bundleFile.Path))
			if err := writeScriptZipEntry(writer, archivePath, bundleFile.Content); err != nil {
				_ = writer.Close()
				return fmt.Errorf("write script package file %s failed: %w", bundleFile.Path, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize script zip failed: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close script zip failed: %w", err)
	}
	if err := replaceFile(tmpPath, normalizedPath); err != nil {
		return fmt.Errorf("move script zip failed: %w", err)
	}

	success = true
	return nil
}

func collectScriptPackageExportFiles(bundle ImportedBundle) (ScriptRecord, []ImportedBundleFile, error) {
	record, err := normalizeScriptRecord(bundle.Record, ScriptRecord{})
	if err != nil {
		return ScriptRecord{}, nil, err
	}
	if err := validateImportedBundle(record, bundle.Files); err != nil {
		return ScriptRecord{}, nil, err
	}

	fileIndex, err := buildImportedBundleFileIndex(record, bundle.Files)
	if err != nil {
		return ScriptRecord{}, nil, err
	}

	paths := make([]string, 0, len(fileIndex))
	for relativePath := range fileIndex {
		if isImportManifestPath(relativePath) {
			continue
		}
		paths = append(paths, relativePath)
	}
	sort.Strings(paths)

	files := make([]ImportedBundleFile, 0, len(paths))
	for _, relativePath := range paths {
		files = append(files, ImportedBundleFile{
			Path:    relativePath,
			Content: fileIndex[relativePath],
		})
	}
	return record, files, nil
}

func writeScriptZipEntry(writer *zip.Writer, archivePath string, content []byte) error {
	normalizedPath, err := normalizeBundleFilePath(archivePath)
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   normalizedPath,
		Method: zip.Deflate,
	}
	header.SetMode(0o644)

	entryWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entryWriter.Write(content)
	return err
}

func buildScriptPackageZipRoot(record ScriptRecord) string {
	name := strings.TrimSpace(record.Name)
	if name == "" {
		name = strings.TrimSpace(record.ID)
	}
	if name == "" {
		return "automation-script"
	}

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
	cleaned := strings.Trim(replacer.Replace(name), ". ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "automation-script"
	}
	return cleaned
}

func replaceFile(sourcePath string, targetPath string) error {
	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	}
	if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	return os.Rename(sourcePath, targetPath)
}
