package main

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type options struct {
	appRoot        string
	configPath     string
	apply          bool
	repairStrategy string
	namePrefix     string
	onlyDirs       map[string]struct{}
}

type selectedCore struct {
	CoreID     string `json:"coreId"`
	CoreName   string `json:"coreName"`
	CorePath   string `json:"corePath"`
	BinaryPath string `json:"binaryPath"`
	Source     string `json:"source"`
}

type repairResult struct {
	TargetDirName string `json:"targetDirName"`
	TargetPath    string `json:"targetPath"`
}

type candidateInspection struct {
	LooksLikeBrowserData bool     `json:"looksLikeBrowserData"`
	Markers              []string `json:"markers,omitempty"`
	LastBrowser          string   `json:"lastBrowser,omitempty"`
	LastVersion          string   `json:"lastVersion,omitempty"`
	Risky                bool     `json:"risky"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type reportEntry struct {
	DirName               string              `json:"dirName"`
	ResolvedPath          string              `json:"resolvedPath"`
	Action                string              `json:"action"`
	Reason                string              `json:"reason,omitempty"`
	ExistingProfileID     string              `json:"existingProfileId,omitempty"`
	ExistingProfileName   string              `json:"existingProfileName,omitempty"`
	RestoredProfileID     string              `json:"restoredProfileId,omitempty"`
	RestoredProfileName   string              `json:"restoredProfileName,omitempty"`
	RegisteredUserDataDir string              `json:"registeredUserDataDir,omitempty"`
	Repair                *repairResult       `json:"repair,omitempty"`
	Inspection            candidateInspection `json:"inspection"`
}

type reportSummary struct {
	Scanned      int `json:"scanned"`
	Candidates   int `json:"candidates"`
	Existing     int `json:"existing"`
	Restored     int `json:"restored"`
	RepairCopies int `json:"repairCopies"`
	Skipped      int `json:"skipped"`
	Warnings     int `json:"warnings"`
}

type recoveryReport struct {
	Timestamp      string        `json:"timestamp"`
	AppRoot        string        `json:"appRoot"`
	ConfigPath     string        `json:"configPath"`
	DBPath         string        `json:"dbPath"`
	UserDataRoot   string        `json:"userDataRoot"`
	Apply          bool          `json:"apply"`
	RepairStrategy string        `json:"repairStrategy"`
	NamePrefix     string        `json:"namePrefix"`
	SelectedCore   selectedCore  `json:"selectedCore"`
	BackupDir      string        `json:"backupDir,omitempty"`
	ReportPath     string        `json:"reportPath,omitempty"`
	Warnings       []string      `json:"warnings,omitempty"`
	Summary        reportSummary `json:"summary"`
	Entries        []reportEntry `json:"entries"`
}

type existingProfile struct {
	ProfileID    string
	ProfileName  string
	UserDataDir  string
	ResolvedPath string
}

var volatileDirNames = map[string]struct{}{
	"browsermetrics":         {},
	"deferredbrowsermetrics": {},
	"graphitedawncache":      {},
	"grshadercache":          {},
	"shadercache":            {},
	"component_crx_cache":    {},
	"extensions_crx_cache":   {},
	"cache":                  {},
	"code cache":             {},
	"gpucache":               {},
}

var volatileFileNames = map[string]struct{}{
	"lock":            {},
	"local state.bad": {},
}

func main() {
	opts := parseFlags()

	report, err := run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "profile recovery failed: %v\n", err)
		os.Exit(1)
	}

	printSummary(report)
}

func parseFlags() options {
	var (
		appRoot        = flag.String("app-root", ".", "Ant Browser app root, for example E:\\software\\Ant Browser")
		configPath     = flag.String("config", "", "Optional config.yaml path override")
		apply          = flag.Bool("apply", false, "Write restored profiles into app.db")
		repairStrategy = flag.String("repair-strategy", "none", "Repair strategy for risky directories: none or risky")
		namePrefix     = flag.String("name-prefix", "恢复", "Prefix used for restored profile names")
		only           = flag.String("only", "", "Optional comma-separated directory names to restore")
	)
	flag.Parse()

	filter := make(map[string]struct{})
	for _, item := range strings.Split(strings.TrimSpace(*only), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		filter[strings.ToLower(item)] = struct{}{}
	}

	return options{
		appRoot:        strings.TrimSpace(*appRoot),
		configPath:     strings.TrimSpace(*configPath),
		apply:          *apply,
		repairStrategy: strings.ToLower(strings.TrimSpace(*repairStrategy)),
		namePrefix:     strings.TrimSpace(*namePrefix),
		onlyDirs:       filter,
	}
}

func run(opts options) (*recoveryReport, error) {
	appRoot := normalizeRoot(opts.appRoot)
	configPath := opts.configPath
	if configPath == "" {
		configPath = filepath.Join(appRoot, "config.yaml")
	}
	configPath = normalizePath(configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dbPath := apppath.Resolve(appRoot, cfg.Database.SQLite.Path)
	userDataRoot := apppath.Resolve(appRoot, cfg.Browser.UserDataRoot)
	now := time.Now()

	report := &recoveryReport{
		Timestamp:      now.Format(time.RFC3339),
		AppRoot:        appRoot,
		ConfigPath:     configPath,
		DBPath:         dbPath,
		UserDataRoot:   userDataRoot,
		Apply:          opts.apply,
		RepairStrategy: normalizeRepairStrategy(opts.repairStrategy),
		NamePrefix:     opts.namePrefix,
	}

	if report.RepairStrategy == "" {
		return nil, fmt.Errorf("unsupported repair strategy %q", opts.repairStrategy)
	}

	if err := os.MkdirAll(userDataRoot, 0755); err != nil {
		return nil, fmt.Errorf("ensure user data root: %w", err)
	}

	selectedCore, warnings, err := selectCore(appRoot, cfg, dbPath, opts.apply)
	if err != nil {
		return nil, err
	}
	report.SelectedCore = selectedCore
	report.Warnings = append(report.Warnings, warnings...)
	report.Summary.Warnings = len(report.Warnings)

	existingProfiles, dbConn, dbHandle, err := loadExistingProfiles(dbPath, userDataRoot, opts.apply)
	if err != nil {
		return nil, err
	}
	if dbHandle != nil {
		defer dbHandle.Close()
	}
	if dbConn != nil {
		defer dbConn.Close()
	}

	existingByPath := make(map[string]existingProfile, len(existingProfiles))
	for _, item := range existingProfiles {
		existingByPath[normalizePath(item.ResolvedPath)] = item
	}

	if opts.apply {
		backupDir, backupErr := backupDatabaseFiles(dbPath, now)
		if backupErr != nil {
			return nil, backupErr
		}
		report.BackupDir = backupDir
	}

	entries, err := os.ReadDir(userDataRoot)
	if err != nil {
		return nil, fmt.Errorf("read user data root: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var profileDAO *browser.SQLiteProfileDAO
	if opts.apply {
		if dbHandle == nil {
			return nil, fmt.Errorf("database handle not initialized in apply mode")
		}
		if err := dbHandle.Migrate(); err != nil {
			return nil, fmt.Errorf("migrate database: %w", err)
		}
		profileDAO = browser.NewSQLiteProfileDAO(dbHandle.GetConn())
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		if len(opts.onlyDirs) > 0 {
			if _, ok := opts.onlyDirs[strings.ToLower(dirName)]; !ok {
				continue
			}
		}

		report.Summary.Scanned++

		resolvedPath := filepath.Join(userDataRoot, dirName)
		inspection := inspectUserDataDir(resolvedPath, selectedCore.BinaryPath)
		item := reportEntry{
			DirName:      dirName,
			ResolvedPath: resolvedPath,
			Inspection:   inspection,
		}

		if !inspection.LooksLikeBrowserData {
			item.Action = "skipped"
			item.Reason = "not a browser user data directory"
			report.Summary.Skipped++
			report.Entries = append(report.Entries, item)
			continue
		}
		report.Summary.Candidates++

		if existing, ok := existingByPath[normalizePath(resolvedPath)]; ok {
			item.Action = "existing"
			item.Reason = "already registered in browser_profiles"
			item.ExistingProfileID = existing.ProfileID
			item.ExistingProfileName = existing.ProfileName
			report.Summary.Existing++
			report.Entries = append(report.Entries, item)
			continue
		}

		targetDirName := dirName
		targetPath := resolvedPath
		var repair *repairResult
		action := "would_restore"
		if opts.apply {
			action = "restored"
		}

		if report.RepairStrategy == "risky" && inspection.Risky {
			if opts.apply {
				targetDirName, targetPath, err = createRepairCopy(userDataRoot, dirName, resolvedPath)
				if err != nil {
					item.Action = "error"
					item.Reason = fmt.Sprintf("create repair copy failed: %v", err)
					report.Summary.Skipped++
					report.Entries = append(report.Entries, item)
					report.Warnings = append(report.Warnings, item.Reason)
					report.Summary.Warnings = len(report.Warnings)
					continue
				}
				report.Summary.RepairCopies++
			} else {
				targetDirName = predictedRepairDirName(dirName, now)
				targetPath = filepath.Join(userDataRoot, targetDirName)
			}
			repair = &repairResult{
				TargetDirName: targetDirName,
				TargetPath:    targetPath,
			}
			if opts.apply {
				action = "restored_with_repair_copy"
			} else {
				action = "would_restore_with_repair_copy"
			}
		}

		profileID := uuid.NewString()
		profileName := buildProfileName(opts.namePrefix, targetDirName)
		registeredUserDataDir := targetDirName

		if opts.apply {
			if profileDAO == nil {
				return nil, fmt.Errorf("profile dao not initialized in apply mode")
			}
			p := &browser.Profile{
				ProfileId:       profileID,
				ProfileName:     profileName,
				UserDataDir:     registeredUserDataDir,
				CoreId:          selectedCore.CoreID,
				FingerprintArgs: append([]string{}, cfg.Browser.DefaultFingerprintArgs...),
				ProxyId:         "",
				ProxyConfig:     "",
				LaunchArgs:      append([]string{}, cfg.Browser.DefaultLaunchArgs...),
				Tags:            []string{"恢复"},
				Keywords:        []string{},
				GroupId:         "",
				CreatedAt:       now.Format(time.RFC3339),
				UpdatedAt:       now.Format(time.RFC3339),
			}
			if err := profileDAO.Upsert(p); err != nil {
				item.Action = "error"
				item.Reason = fmt.Sprintf("insert browser_profiles failed: %v", err)
				report.Summary.Skipped++
				report.Entries = append(report.Entries, item)
				report.Warnings = append(report.Warnings, item.Reason)
				report.Summary.Warnings = len(report.Warnings)
				continue
			}
			existingByPath[normalizePath(targetPath)] = existingProfile{
				ProfileID:    profileID,
				ProfileName:  profileName,
				UserDataDir:  registeredUserDataDir,
				ResolvedPath: targetPath,
			}
		}

		item.Action = action
		item.Reason = "directory is present on disk but missing in browser_profiles"
		item.RestoredProfileID = profileID
		item.RestoredProfileName = profileName
		item.RegisteredUserDataDir = registeredUserDataDir
		item.Repair = repair
		report.Summary.Restored++
		report.Entries = append(report.Entries, item)
	}

	reportPath, err := writeReport(report, now)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("write report failed: %v", err))
		report.Summary.Warnings = len(report.Warnings)
	} else {
		report.ReportPath = reportPath
	}

	return report, nil
}

func normalizeRepairStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none":
		return "none"
	case "risky":
		return "risky"
	default:
		return ""
	}
}

func selectCore(appRoot string, cfg *config.Config, dbPath string, apply bool) (selectedCore, []string, error) {
	var warnings []string

	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		db, err := openQueryDB(dbPath)
		if err != nil {
			return selectedCore{}, warnings, fmt.Errorf("open database for core selection: %w", err)
		}
		defer db.Close()

		cores, err := browser.NewSQLiteCoreDAO(db).List()
		if err == nil && len(cores) > 0 {
			picked := pickCoreFromList(appRoot, cores, "database")
			return picked, warnings, nil
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("load cores from database failed, fallback to config: %v", err))
		}
	}

	if len(cfg.Browser.Cores) > 0 {
		picked := pickCoreFromConfig(appRoot, cfg.Browser.Cores, "config")
		return picked, warnings, nil
	}

	if apply {
		warnings = append(warnings, "no browser core was found; restored profiles will be created with empty core_id")
	}

	return selectedCore{
		CoreID:   "",
		CoreName: "",
		CorePath: "",
		Source:   "none",
	}, warnings, nil
}

func pickCoreFromList(appRoot string, cores []browser.Core, source string) selectedCore {
	for _, core := range cores {
		if core.IsDefault {
			return buildSelectedCore(appRoot, core.CoreId, core.CoreName, core.CorePath, source)
		}
	}
	first := cores[0]
	return buildSelectedCore(appRoot, first.CoreId, first.CoreName, first.CorePath, source)
}

func pickCoreFromConfig(appRoot string, cores []config.BrowserCore, source string) selectedCore {
	for _, core := range cores {
		if core.IsDefault {
			return buildSelectedCore(appRoot, core.CoreId, core.CoreName, core.CorePath, source)
		}
	}
	first := cores[0]
	return buildSelectedCore(appRoot, first.CoreId, first.CoreName, first.CorePath, source)
}

func buildSelectedCore(appRoot, coreID, coreName, corePath, source string) selectedCore {
	coreAbsPath := apppath.Resolve(appRoot, corePath)
	return selectedCore{
		CoreID:     strings.TrimSpace(coreID),
		CoreName:   strings.TrimSpace(coreName),
		CorePath:   strings.TrimSpace(corePath),
		BinaryPath: filepath.Join(coreAbsPath, "chrome.exe"),
		Source:     source,
	}
}

func loadExistingProfiles(dbPath string, userDataRoot string, apply bool) ([]existingProfile, *sql.DB, *database.DB, error) {
	dbExists := fileExists(dbPath)
	if !dbExists && !apply {
		return nil, nil, nil, nil
	}

	if apply {
		handle, err := database.NewDB(dbPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open database: %w", err)
		}
		list, err := browser.NewSQLiteProfileDAO(handle.GetConn()).List()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such table") {
				return nil, nil, handle, nil
			}
			_ = handle.Close()
			return nil, nil, nil, fmt.Errorf("load existing profiles: %w", err)
		}
		return toExistingProfiles(list, userDataRoot), nil, handle, nil
	}

	db, err := openQueryDB(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open database: %w", err)
	}
	list, err := browser.NewSQLiteProfileDAO(db).List()
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("load existing profiles: %w", err)
	}
	return toExistingProfiles(list, userDataRoot), db, nil, nil
}

func toExistingProfiles(items []*browser.Profile, userDataRoot string) []existingProfile {
	out := make([]existingProfile, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, existingProfile{
			ProfileID:    item.ProfileId,
			ProfileName:  item.ProfileName,
			UserDataDir:  item.UserDataDir,
			ResolvedPath: resolveUserDataPath(userDataRoot, item.UserDataDir),
		})
	}
	return out
}

func resolveUserDataPath(userDataRoot string, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Join(userDataRoot, raw)
}

func openQueryDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func backupDatabaseFiles(dbPath string, now time.Time) (string, error) {
	dataRoot := filepath.Dir(dbPath)
	backupDir := filepath.Join(dataRoot, "recovery-backups", now.Format("20060102-150405"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	for _, src := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if !fileExists(src) {
			continue
		}
		dst := filepath.Join(backupDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("backup %s: %w", src, err)
		}
	}

	return backupDir, nil
}

func inspectUserDataDir(dirPath string, currentCoreBinaryPath string) candidateInspection {
	inspection := candidateInspection{}

	markers := make([]string, 0, 4)
	for _, marker := range []string{"Local State", "Default", "Last Browser", "Last Version"} {
		if fileExists(filepath.Join(dirPath, marker)) {
			markers = append(markers, marker)
		}
	}
	inspection.Markers = markers
	inspection.LooksLikeBrowserData = len(markers) > 0
	if !inspection.LooksLikeBrowserData {
		return inspection
	}

	if raw, err := os.ReadFile(filepath.Join(dirPath, "Last Browser")); err == nil {
		inspection.LastBrowser = decodePossiblyUTF16(raw)
	}
	if raw, err := os.ReadFile(filepath.Join(dirPath, "Last Version")); err == nil {
		inspection.LastVersion = strings.TrimSpace(string(raw))
	}

	if fileExists(filepath.Join(dirPath, "Local State.bad")) {
		inspection.Risky = true
		inspection.RiskReasons = append(inspection.RiskReasons, "Local State.bad exists")
	}
	if inspection.LastBrowser != "" && currentCoreBinaryPath != "" {
		if normalizePath(inspection.LastBrowser) != normalizePath(currentCoreBinaryPath) {
			inspection.Risky = true
			inspection.RiskReasons = append(inspection.RiskReasons, fmt.Sprintf("Last Browser points to %s", inspection.LastBrowser))
		}
	}

	return inspection
}

func createRepairCopy(userDataRoot, dirName, sourcePath string) (string, string, error) {
	targetDirName := uniqueRepairDirName(userDataRoot, dirName)
	targetPath := filepath.Join(userDataRoot, targetDirName)
	if err := copyDirFiltered(sourcePath, targetPath); err != nil {
		return "", "", err
	}
	return targetDirName, targetPath, nil
}

func predictedRepairDirName(dirName string, now time.Time) string {
	return fmt.Sprintf("%s__repair_%s", dirName, now.Format("20060102-150405"))
}

func uniqueRepairDirName(userDataRoot, dirName string) string {
	base := predictedRepairDirName(dirName, time.Now())
	target := filepath.Join(userDataRoot, base)
	if !fileExists(target) {
		return base
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%02d", base, i)
		if !fileExists(filepath.Join(userDataRoot, candidate)) {
			return candidate
		}
	}
}

func copyDirFiltered(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if shouldSkipRepairPath(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func shouldSkipRepairPath(rel string, isDir bool) bool {
	clean := filepath.ToSlash(strings.TrimSpace(rel))
	base := strings.ToLower(filepath.Base(clean))

	if strings.HasPrefix(base, "singleton") {
		return true
	}
	if strings.HasSuffix(base, ".tmp") {
		return true
	}
	if _, ok := volatileFileNames[base]; ok {
		return true
	}

	if !isDir {
		return false
	}

	if _, ok := volatileDirNames[base]; ok {
		return true
	}

	parent := strings.ToLower(filepath.Base(filepath.Dir(clean)))
	if parent == "default" {
		if _, ok := volatileDirNames[base]; ok {
			return true
		}
	}

	return false
}

func buildProfileName(prefix, dirName string) string {
	name := strings.TrimSpace(dirName)
	if isUUIDLike(name) {
		name = name[:8]
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return name
	}
	return fmt.Sprintf("%s-%s", prefix, name)
}

func isUUIDLike(value string) bool {
	_, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil
}

func writeReport(report *recoveryReport, now time.Time) (string, error) {
	if report == nil {
		return "", fmt.Errorf("report is nil")
	}

	reportDir := filepath.Join(report.UserDataRoot, "recovery-reports")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", err
	}
	reportPath := filepath.Join(reportDir, fmt.Sprintf("profile-recover-%s.json", now.Format("20060102-150405")))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return "", err
	}
	return reportPath, nil
}

func printSummary(report *recoveryReport) {
	fmt.Printf("AppRoot: %s\n", report.AppRoot)
	fmt.Printf("DBPath: %s\n", report.DBPath)
	fmt.Printf("UserDataRoot: %s\n", report.UserDataRoot)
	mode := "preview"
	if report.Apply {
		mode = "apply"
	}
	fmt.Printf("Mode: %s\n", mode)
	if report.SelectedCore.CoreID != "" || report.SelectedCore.CoreName != "" {
		fmt.Printf("SelectedCore: %s (%s)\n", report.SelectedCore.CoreName, report.SelectedCore.CoreID)
	}
	if report.BackupDir != "" {
		fmt.Printf("BackupDir: %s\n", report.BackupDir)
	}
	if report.ReportPath != "" {
		fmt.Printf("Report: %s\n", report.ReportPath)
	}
	fmt.Printf("Scanned=%d Candidates=%d Existing=%d Restored=%d RepairCopies=%d Skipped=%d Warnings=%d\n",
		report.Summary.Scanned,
		report.Summary.Candidates,
		report.Summary.Existing,
		report.Summary.Restored,
		report.Summary.RepairCopies,
		report.Summary.Skipped,
		report.Summary.Warnings,
	)
	for _, entry := range report.Entries {
		fmt.Printf("- [%s] %s", entry.Action, entry.DirName)
		if entry.RestoredProfileName != "" {
			fmt.Printf(" -> %s", entry.RestoredProfileName)
		}
		if entry.ExistingProfileName != "" {
			fmt.Printf(" -> %s", entry.ExistingProfileName)
		}
		if entry.Reason != "" {
			fmt.Printf(" (%s)", entry.Reason)
		}
		fmt.Println()
	}
	if len(report.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, warning := range report.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
}

func decodePossiblyUTF16(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) >= 2 && len(raw)%2 == 0 {
		zeros := 0
		for i := 1; i < len(raw); i += 2 {
			if raw[i] == 0 {
				zeros++
			}
		}
		if zeros >= len(raw)/4 {
			u16 := make([]uint16, 0, len(raw)/2)
			for i := 0; i+1 < len(raw); i += 2 {
				u16 = append(u16, binary.LittleEndian.Uint16(raw[i:i+2]))
			}
			return strings.TrimSpace(string(utf16.Decode(u16)))
		}
	}
	return strings.TrimSpace(string(raw))
}

func normalizeRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return filepath.Clean(abs)
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return strings.ToLower(filepath.Clean(p))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
