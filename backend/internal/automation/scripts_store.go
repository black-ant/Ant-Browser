package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultScriptEntryFile          = "index.cjs"
	defaultScriptPackageFormat      = "ant-automation-script"
	defaultScriptManifestVersion    = 1
	defaultScriptCreateNameTemplate = "${templateName}-${timestamp}"
	defaultScriptPublicAPITimeoutMs = 300000
	scriptStoreConfigFileName       = "config"
	scriptStoreLegacyConfigName     = "manifest.json"
	scriptPublicAPIBasePath         = "/api/automation/hooks"
)

type ScriptSource struct {
	Type       string `json:"type"`
	URI        string `json:"uri"`
	Ref        string `json:"ref"`
	Path       string `json:"path"`
	ImportedAt string `json:"importedAt"`
}

type ScriptTargetSelector struct {
	Code        string   `json:"code"`
	ProfileID   string   `json:"profileId"`
	ProfileName string   `json:"profileName"`
	GroupID     string   `json:"groupId"`
	Keywords    []string `json:"keywords"`
	Tags        []string `json:"tags"`
}

type ScriptTargetConfig struct {
	Mode               string               `json:"mode"`
	Selector           ScriptTargetSelector `json:"selector"`
	TemplateSelector   ScriptTargetSelector `json:"templateSelector"`
	CreateNameTemplate string               `json:"createNameTemplate"`
}

type ScriptPublicAPIConfig struct {
	Enabled          bool                      `json:"enabled"`
	Method           string                    `json:"method"`
	Path             string                    `json:"path"`
	RequestMode      string                    `json:"requestMode"`
	ResponseMode     string                    `json:"responseMode"`
	TimeoutMs        int                       `json:"timeoutMs"`
	RequestBodyText  string                    `json:"requestBodyText"`
	ResponseBodyText string                    `json:"responseBodyText"`
	Variables        []ScriptPublicAPIVariable `json:"variables"`
}

type ScriptPublicAPIVariable struct {
	Name         string `json:"name"`
	DefaultValue string `json:"defaultValue"`
	Description  string `json:"description"`
	Required     bool   `json:"required"`
}

type ScriptRecord struct {
	PackageFormat   string                `json:"packageFormat"`
	ManifestVersion int                   `json:"manifestVersion"`
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	Type            string                `json:"type"`
	Status          string                `json:"status"`
	EntryFile       string                `json:"entryFile"`
	Tags            []string              `json:"tags"`
	SelectorText    string                `json:"selectorText"`
	ParamsText      string                `json:"paramsText"`
	ScriptText      string                `json:"scriptText"`
	Notes           string                `json:"notes"`
	TargetConfig    ScriptTargetConfig    `json:"targetConfig"`
	PublicAPI       ScriptPublicAPIConfig `json:"publicAPI"`
	Source          ScriptSource          `json:"source"`
	CreatedAt       string                `json:"createdAt"`
	UpdatedAt       string                `json:"updatedAt"`
}

type ImportedBundleFile struct {
	Path    string
	Content []byte
}

type ImportedBundle struct {
	Record ScriptRecord
	Files  []ImportedBundleFile
}

type scriptStoreConfig struct {
	PackageFormat   string                `json:"packageFormat"`
	ManifestVersion int                   `json:"manifestVersion"`
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	Type            string                `json:"type"`
	Status          string                `json:"status"`
	EntryFile       string                `json:"entryFile"`
	Tags            []string              `json:"tags"`
	SelectorText    string                `json:"selectorText"`
	ParamsText      string                `json:"paramsText"`
	Notes           string                `json:"notes"`
	TargetConfig    ScriptTargetConfig    `json:"targetConfig"`
	PublicAPI       ScriptPublicAPIConfig `json:"publicAPI"`
	Source          ScriptSource          `json:"source"`
	CreatedAt       string                `json:"createdAt"`
	UpdatedAt       string                `json:"updatedAt"`
}

type ScriptStore struct {
	rootDir string
}

func NewScriptStore(rootDir string) *ScriptStore {
	return &ScriptStore{
		rootDir: filepath.Clean(strings.TrimSpace(rootDir)),
	}
}

func (s *ScriptStore) List() ([]ScriptRecord, error) {
	if err := s.ensureRoot(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptRecord{}, nil
		}
		return nil, fmt.Errorf("read automation scripts dir failed: %w", err)
	}

	items := make([]ScriptRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		record, err := s.readScriptDir(filepath.Join(s.rootDir, entry.Name()))
		if err != nil {
			continue
		}
		items = append(items, record)
	}

	sort.Slice(items, func(i, j int) bool {
		return parseRFC3339OrZero(items[i].UpdatedAt).After(parseRFC3339OrZero(items[j].UpdatedAt))
	})

	return items, nil
}

func (s *ScriptStore) Save(input ScriptRecord) (ScriptRecord, error) {
	if err := s.ensureRoot(); err != nil {
		return ScriptRecord{}, err
	}

	normalizedInput, err := normalizeScriptRecord(input, ScriptRecord{})
	if err != nil {
		return ScriptRecord{}, err
	}
	input.ID = normalizedInput.ID

	dir, err := s.scriptDir(input.ID)
	if err != nil {
		return ScriptRecord{}, err
	}

	existing, _ := s.readScriptDir(dir)
	record, err := normalizeScriptRecord(input, existing)
	if err != nil {
		return ScriptRecord{}, err
	}
	if err := s.validateRecord(record); err != nil {
		return ScriptRecord{}, err
	}

	return s.writeRecord(dir, record, existing, nil)
}

func (s *ScriptStore) Get(scriptID string) (ScriptRecord, error) {
	dir, err := s.scriptDir(scriptID)
	if err != nil {
		return ScriptRecord{}, err
	}
	return s.readScriptDir(dir)
}

func (s *ScriptStore) ExportBundle(scriptID string) (ImportedBundle, error) {
	dir, err := s.scriptDir(scriptID)
	if err != nil {
		return ImportedBundle{}, err
	}

	record, err := s.readScriptDir(dir)
	if err != nil {
		return ImportedBundle{}, err
	}

	files, err := collectScriptStoreBundleFiles(dir)
	if err != nil {
		return ImportedBundle{}, err
	}

	return ImportedBundle{
		Record: record,
		Files:  files,
	}, nil
}

func (s *ScriptStore) Dir(scriptID string) (string, error) {
	return s.scriptDir(scriptID)
}

func (s *ScriptStore) ImportBundle(bundle ImportedBundle) (ScriptRecord, error) {
	if err := s.ensureRoot(); err != nil {
		return ScriptRecord{}, err
	}

	record, err := normalizeScriptRecord(bundle.Record, ScriptRecord{})
	if err != nil {
		return ScriptRecord{}, err
	}
	record.ID = firstNonEmpty(record.ID, bundle.Record.ID)

	dir, err := s.scriptDir(record.ID)
	if err != nil {
		return ScriptRecord{}, err
	}

	existing, _ := s.readScriptDir(dir)
	if err := s.validateRecord(record); err != nil {
		return ScriptRecord{}, err
	}
	return s.writeRecord(dir, record, existing, bundle.Files)
}

func (s *ScriptStore) Delete(scriptID string) error {
	dir, err := s.scriptDir(scriptID)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete automation script failed: %w", err)
	}
	return nil
}
