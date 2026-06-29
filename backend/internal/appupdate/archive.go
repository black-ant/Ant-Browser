package appupdate

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractFullPayload(zipPath, destination string) error {
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("payload destination is required")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target, err := safeZipEntryPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			if _, err := safeZipSymlinkTarget(destination, target, file); err != nil {
				return err
			}
		}
	}
	requiredDirs, err := requiredZipDirs(destination, reader.File)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}

	for _, file := range reader.File {
		target, err := safeZipEntryPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := mkdirAllSafe(destination, target, dirMode(file.Mode())); err != nil {
				return err
			}
			continue
		}
		if requiredDirs[filepath.Clean(target)] {
			continue
		}
		if err := mkdirAllSafe(destination, filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			if err := extractZipSymlink(destination, file, target); err != nil {
				return err
			}
			continue
		}
		if err := extractZipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func ValidateStagedPayload(target, stagedRoot string) error {
	if err := requireStagedRootDirectory(stagedRoot); err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(target)) {
	case "windows-amd64":
		required := []string{
			"ant-chrome.exe",
			filepath.Join("publish", "runtime-manifest.json"),
		}
		for _, rel := range required {
			if err := requireRegularFileNoSymlink(stagedRoot, rel, rel); err != nil {
				return err
			}
		}
		return rejectMutableUserData(stagedRoot)
	case "darwin-amd64", "darwin-arm64":
		return validateDarwinStagedPayload(stagedRoot)
	default:
		return fmt.Errorf("unsupported app update target: %s", target)
	}
}

func validateDarwinStagedPayload(stagedRoot string) error {
	if err := requireDirectoryNoSymlink(stagedRoot, "Ant Browser.app", "Ant Browser.app"); err != nil {
		return err
	}

	required := []string{
		filepath.Join("Ant Browser.app", "Contents", "Info.plist"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "publish", "runtime-manifest.json"),
	}
	for _, rel := range required {
		if err := requireRegularFileNoSymlink(stagedRoot, rel, rel); err != nil {
			return err
		}
	}

	for _, rel := range []string{
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "ant-chrome"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "bin", "xray"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "bin", "sing-box"),
	} {
		if err := requireExecutableNoSymlink(stagedRoot, rel, rel); err != nil {
			return err
		}
	}
	return rejectMutableUserData(stagedRoot)
}

func requireStagedRootDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("staged payload missing directory: staged payload root")
		}
		return fmt.Errorf("staged payload directory is not readable: staged payload root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staged payload directory must not be a symlink: staged payload root")
	}
	if !info.IsDir() {
		return fmt.Errorf("staged payload required path is not a directory: staged payload root")
	}
	return nil
}

func requireNoSymlinkPath(root, rel string) (string, os.FileInfo, error) {
	if strings.TrimSpace(rel) == "" {
		return "", nil, fmt.Errorf("staged payload relative path is required")
	}
	if filepath.IsAbs(rel) {
		return "", nil, fmt.Errorf("staged payload path must be relative: %s", rel)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	cleanRel := filepath.Clean(rel)
	target := filepath.Join(rootAbs, cleanRel)
	checkedRel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return "", nil, err
	}
	if checkedRel == "." || checkedRel == ".." || strings.HasPrefix(checkedRel, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("staged payload path escapes root: %s", rel)
	}

	current := rootAbs
	var info os.FileInfo
	for _, part := range strings.Split(checkedRel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil {
			return current, nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return current, nil, fmt.Errorf("staged payload path component must not be a symlink: %s", current)
		}
	}
	if info == nil {
		return "", nil, fmt.Errorf("staged payload relative path is required")
	}
	return current, info, nil
}

func requireDirectoryNoSymlink(root, rel, label string) error {
	_, info, err := requireNoSymlinkPath(root, rel)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("staged payload missing directory: %s", label)
		}
		return fmt.Errorf("staged payload directory is not readable: %s: %w", label, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("staged payload required path is not a directory: %s", label)
	}
	return nil
}

func requireRegularFileNoSymlink(root, rel, label string) error {
	_, info, err := requireNoSymlinkPath(root, rel)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("staged payload missing required file: %s", label)
		}
		return fmt.Errorf("staged payload required file is not readable: %s: %w", label, err)
	}
	if info.IsDir() {
		return fmt.Errorf("staged payload required path is a directory: %s", label)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("staged payload required path is not a regular file: %s", label)
	}
	return nil
}

func requireExecutableNoSymlink(root, rel, label string) error {
	_, info, err := requireNoSymlinkPath(root, rel)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("staged payload missing executable: %s", label)
		}
		return fmt.Errorf("staged payload executable is not readable: %s: %w", label, err)
	}
	if info.IsDir() {
		return fmt.Errorf("staged payload executable path is a directory: %s", label)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("staged payload executable path is not a regular file: %s", label)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("staged payload executable bit is not set: %s", label)
	}
	return nil
}

func rejectMutableUserData(stagedRoot string) error {
	return filepath.WalkDir(stagedRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stagedRoot, path)
		if err != nil {
			return err
		}
		rel = strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
		if rel == "." {
			return nil
		}
		if rel == "data" || strings.HasPrefix(rel, "data/") {
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		}
		if hasPathSegment(rel, "user data") {
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		}
		for _, appRoot := range []string{"ant browser.app", "maka browser.app"} {
			darwinData := appRoot + "/contents/macos/data"
			if rel == darwinData || strings.HasPrefix(rel, darwinData+"/") {
				return fmt.Errorf("staged payload contains mutable user data: %s", path)
			}
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".db", ".sqlite", ".sqlite3":
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		default:
			return nil
		}
	})
}

func hasPathSegment(path, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func safeZipEntryPath(destination, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("zip entry name is empty")
	}

	entryPath := filepath.FromSlash(name)
	if filepath.IsAbs(entryPath) {
		return "", fmt.Errorf("zip entry uses absolute path: %s", name)
	}

	target := filepath.Join(destination, entryPath)
	rel, err := filepath.Rel(destination, target)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("zip entry escapes destination: %s", name)
	}
	return target, nil
}

func safeZipSymlinkTarget(root, linkPath string, file *zip.File) (string, error) {
	linkTarget, err := readZipSymlinkTarget(file)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(linkTarget) == "" {
		return "", fmt.Errorf("zip symlink target is empty: %s", file.Name)
	}
	if filepath.IsAbs(linkTarget) {
		return "", fmt.Errorf("zip symlink target uses absolute path: %s -> %s", file.Name, linkTarget)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(linkTarget)))
	rel, err := filepath.Rel(rootAbs, resolved)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("zip symlink target escapes destination: %s -> %s", file.Name, linkTarget)
	}
	return linkTarget, nil
}

func readZipSymlinkTarget(file *zip.File) (string, error) {
	in, err := file.Open()
	if err != nil {
		return "", err
	}
	defer in.Close()

	data, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func requiredZipDirs(destination string, files []*zip.File) (map[string]bool, error) {
	dirs := map[string]bool{
		filepath.Clean(destination): true,
	}
	for _, file := range files {
		target, err := safeZipEntryPath(destination, file.Name)
		if err != nil {
			return nil, err
		}
		if file.FileInfo().IsDir() {
			dirs[filepath.Clean(target)] = true
			continue
		}
		dir := filepath.Clean(filepath.Dir(target))
		for {
			dirs[dir] = true
			parent := filepath.Dir(dir)
			if parent == dir || dir == filepath.Clean(destination) {
				break
			}
			dir = parent
		}
	}
	return dirs, nil
}

func mkdirAllSafe(root, path string, mode os.FileMode) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return os.MkdirAll(root, mode)
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("directory escapes destination: %s", path)
	}

	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.IsDir() {
				continue
			}
			if err := os.RemoveAll(current); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, mode); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func extractZipSymlink(root string, file *zip.File, target string) error {
	linkTarget, err := safeZipSymlinkTarget(root, target, file)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Symlink(linkTarget, target)
}

func extractZipFile(file *zip.File, target string) error {
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode(file.Mode()))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func dirMode(mode os.FileMode) os.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return 0o700
}

func fileMode(mode os.FileMode) os.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return 0o600
}
