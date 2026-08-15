package main

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
		appRoot        = flag.String("app-root", ".", "ZwBrowser app root, for example E:\\software\\ZwBrowser")
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

	if err := os.MkdirAll(userDataRoot, 0o755); err != nil {
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
