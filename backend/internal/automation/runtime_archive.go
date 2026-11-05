package automation

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

func writeRuntimeManifest(path, nodeVersion, playwrightVersion, runtimeVersion, nodeSource, nodePath string) error {
	payload := map[string]string{
		"runtimeVersion":    strings.TrimSpace(runtimeVersion),
		"nodeVersion":       strings.TrimSpace(nodeVersion),
		"playwrightVersion": strings.TrimSpace(playwrightVersion),
		"nodeSource":        strings.TrimSpace(nodeSource),
		"nodePath":          strings.TrimSpace(nodePath),
		"installedAt":       time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeRunnerScript(path string) error {
	runnerDir := filepath.Dir(path)
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return err
	}
	for name, content := range runnerAssetFiles {
		mode := os.FileMode(0o644)
		if name == runnerScriptFileName {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(runnerDir, name), content, mode); err != nil {
			return err
		}
	}
	return nil
}

func syncRunnerScript(path string) error {
	runnerDir := filepath.Dir(path)
	for name, content := range runnerAssetFiles {
		current, err := os.ReadFile(filepath.Join(runnerDir, name))
		if err != nil {
			if os.IsNotExist(err) {
				return writeRunnerScript(path)
			}
			return err
		}
		if string(current) != string(content) {
			return writeRunnerScript(path)
		}
	}
	return nil
}

func extractArchive(archivePath, destDir, format, stripPrefix string) error {
	switch strings.TrimSpace(format) {
	case "zip":
		return extractZip(archivePath, destDir, stripPrefix)
	case "tar.gz":
		return extractTarGz(archivePath, destDir, stripPrefix)
	case "tar.xz":
		return extractTarXz(archivePath, destDir, stripPrefix)
	default:
		return fmt.Errorf("unsupported archive format: %s", format)
	}
}

func extractZip(archivePath, destDir, stripPrefix string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath, skip, err := sanitizedArchivePath(destDir, file.Name, stripPrefix)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		srcCloseErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if srcCloseErr != nil {
			return srcCloseErr
		}
	}
	return nil
}

func extractTarGz(archivePath, destDir, stripPrefix string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	return extractTarReader(tar.NewReader(gzReader), destDir, stripPrefix)
}

func extractTarXz(archivePath, destDir, stripPrefix string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return err
	}
	return extractTarReader(tar.NewReader(xzReader), destDir, stripPrefix)
}

func extractTarReader(reader *tar.Reader, destDir, stripPrefix string) error {
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		targetPath, skip, err := sanitizedArchivePath(destDir, header.Name, stripPrefix)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode)
			if mode == 0 {
				mode = 0o644
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, reader); err != nil {
				file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
}

func sanitizedArchivePath(destDir, rawName, stripPrefix string) (string, bool, error) {
	name := filepath.ToSlash(strings.TrimSpace(rawName))
	if name == "" {
		return "", true, nil
	}
	if stripPrefix != "" {
		prefix := filepath.ToSlash(strings.TrimSpace(stripPrefix))
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if name == strings.TrimSuffix(prefix, "/") {
			return "", true, nil
		}
		if !strings.HasPrefix(name, prefix) {
			return "", true, nil
		}
		name = strings.TrimPrefix(name, prefix)
	}
	name = strings.TrimPrefix(name, "/")
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == "" {
		return "", true, nil
	}
	if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", false, fmt.Errorf("illegal archive path: %s", rawName)
	}
	targetPath := filepath.Join(destDir, cleanName)
	rel, err := filepath.Rel(destDir, targetPath)
	if err != nil {
		return "", false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, fmt.Errorf("illegal archive path: %s", rawName)
	}
	return targetPath, false, nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sha1File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func readPackageVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Version)
}
