package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ScriptRunRecord struct {
	ID         string `json:"id"`
	ScriptID   string `json:"scriptId"`
	ScriptName string `json:"scriptName"`
	ScriptType string `json:"scriptType"`
	Status     string `json:"status"`
	Summary    string `json:"summary"`
	Error      string `json:"error"`
	ResultText string `json:"resultText"`
	LogText    string `json:"logText"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	DurationMs int64  `json:"durationMs"`
}

type ScriptRunRequest struct {
	ScriptID          string `json:"scriptId"`
	SelectorText      string `json:"selectorText"`
	TargetMode        string `json:"targetMode,omitempty"`
	TargetInput       any    `json:"targetInput,omitempty"`
	ParamsText        string `json:"paramsText"`
	UseScriptSelector bool   `json:"useScriptSelector"`
	UseScriptParams   bool   `json:"useScriptParams"`
	TimeoutMs         int    `json:"timeoutMs,omitempty"`
}

type ScriptRunStore struct {
	rootDir string
}

func NewScriptRunStore(rootDir string) *ScriptRunStore {
	return &ScriptRunStore{
		rootDir: filepath.Clean(strings.TrimSpace(rootDir)),
	}
}

func (s *ScriptRunStore) Save(input ScriptRunRecord) (ScriptRunRecord, error) {
	record := normalizeScriptRunRecord(input)
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return ScriptRunRecord{}, fmt.Errorf("create automation run dir failed: %w", err)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ScriptRunRecord{}, fmt.Errorf("marshal automation run record failed: %w", err)
	}

	if err := writeFileAtomic(filepath.Join(s.rootDir, record.ID+".json"), data, 0o644); err != nil {
		return ScriptRunRecord{}, fmt.Errorf("write automation run record failed: %w", err)
	}
	return record, nil
}

func (s *ScriptRunStore) List(limit int) ([]ScriptRunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create automation run dir failed: %w", err)
	}

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptRunRecord{}, nil
		}
		return nil, fmt.Errorf("read automation run dir failed: %w", err)
	}

	items := make([]ScriptRunRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.rootDir, entry.Name()))
		if err != nil {
			continue
		}
		var record ScriptRunRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		items = append(items, normalizeScriptRunRecord(record))
	}

	sort.Slice(items, func(i, j int) bool {
		return parseRFC3339OrZero(items[i].StartedAt).After(parseRFC3339OrZero(items[j].StartedAt))
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func normalizeScriptRunRecord(input ScriptRunRecord) ScriptRunRecord {
	now := time.Now().Format(time.RFC3339)

	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}

	status := strings.TrimSpace(input.Status)
	switch status {
	case "success", "failed", "running":
	default:
		status = "failed"
	}

	startedAt := firstNonEmpty(input.StartedAt, now)
	finishedAt := firstNonEmpty(input.FinishedAt, startedAt)
	durationMs := input.DurationMs
	if durationMs < 0 {
		durationMs = 0
	}

	return ScriptRunRecord{
		ID:         id,
		ScriptID:   strings.TrimSpace(input.ScriptID),
		ScriptName: strings.TrimSpace(input.ScriptName),
		ScriptType: strings.TrimSpace(input.ScriptType),
		Status:     status,
		Summary:    strings.TrimSpace(input.Summary),
		Error:      strings.TrimSpace(input.Error),
		ResultText: strings.TrimSpace(input.ResultText),
		LogText:    strings.TrimSpace(input.LogText),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMs: durationMs,
	}
}
