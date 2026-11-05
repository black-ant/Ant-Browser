package automation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type scriptTemplateFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
}

func MarshalScriptPackageManifest(record ScriptRecord) ([]byte, error) {
	normalized, err := normalizeScriptRecord(record, ScriptRecord{})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"format":          normalized.PackageFormat,
		"packageFormat":   normalized.PackageFormat,
		"manifestVersion": normalized.ManifestVersion,
		"id":              normalized.ID,
		"name":            normalized.Name,
		"description":     normalized.Description,
		"type":            normalized.Type,
		"status":          normalized.Status,
		"entryFile":       normalized.EntryFile,
		"tags":            append([]string{}, normalized.Tags...),
		"notes":           normalized.Notes,
		"targetConfig":    normalized.TargetConfig,
		"publicAPI":       normalized.PublicAPI,
		"source": map[string]any{
			"type":       normalized.Source.Type,
			"uri":        normalized.Source.URI,
			"ref":        normalized.Source.Ref,
			"path":       normalized.Source.Path,
			"importedAt": normalized.Source.ImportedAt,
		},
		"createdAt": normalized.CreatedAt,
		"updatedAt": normalized.UpdatedAt,
	}

	if selectorValue := parseScriptTemplateJSON(normalized.SelectorText); selectorValue != nil {
		payload["selector"] = selectorValue
	}
	if paramsValue := parseScriptTemplateJSON(normalized.ParamsText); paramsValue != nil {
		payload["params"] = paramsValue
	}

	return json.MarshalIndent(payload, "", "  ")
}

func MarshalScriptTemplate(bundle ImportedBundle) ([]byte, error) {
	record, err := normalizeScriptRecord(bundle.Record, ScriptRecord{})
	if err != nil {
		return nil, err
	}

	files, err := buildTemplateFiles(bundle.Files, record.EntryFile)
	if err != nil {
		return nil, err
	}

	envelope := scriptImportEnvelope{
		Format:          record.PackageFormat,
		PackageFormat:   record.PackageFormat,
		ManifestVersion: record.ManifestVersion,
		Manifest: map[string]any{
			"packageFormat":   record.PackageFormat,
			"manifestVersion": record.ManifestVersion,
			"id":              record.ID,
			"name":            record.Name,
			"description":     record.Description,
			"type":            record.Type,
			"status":          record.Status,
			"entryFile":       record.EntryFile,
			"tags":            append([]string{}, record.Tags...),
			"notes":           record.Notes,
			"targetConfig":    record.TargetConfig,
			"publicAPI":       record.PublicAPI,
			"source":          record.Source,
			"createdAt":       record.CreatedAt,
			"updatedAt":       record.UpdatedAt,
		},
		ScriptText: record.ScriptText,
		Notes:      record.Notes,
		PublicAPI: map[string]any{
			"enabled":          record.PublicAPI.Enabled,
			"method":           record.PublicAPI.Method,
			"path":             record.PublicAPI.Path,
			"requestMode":      record.PublicAPI.RequestMode,
			"responseMode":     record.PublicAPI.ResponseMode,
			"timeoutMs":        record.PublicAPI.TimeoutMs,
			"requestBodyText":  record.PublicAPI.RequestBodyText,
			"responseBodyText": record.PublicAPI.ResponseBodyText,
			"variables":        record.PublicAPI.Variables,
		},
		Source: map[string]any{
			"type":       record.Source.Type,
			"uri":        record.Source.URI,
			"ref":        record.Source.Ref,
			"path":       record.Source.Path,
			"importedAt": record.Source.ImportedAt,
		},
		Files: files,
	}

	if selectorValue := parseScriptTemplateJSON(record.SelectorText); selectorValue != nil {
		envelope.Selector = selectorValue
	}
	if paramsValue := parseScriptTemplateJSON(record.ParamsText); paramsValue != nil {
		envelope.Params = paramsValue
	}

	return json.MarshalIndent(envelope, "", "  ")
}

func decodeScriptTemplateFiles(files []scriptTemplateFile, entryFile string) ([]ImportedBundleFile, error) {
	entryFile = strings.TrimSpace(entryFile)
	result := make([]ImportedBundleFile, 0, len(files))

	for _, file := range files {
		normalizedPath, err := normalizeBundleFilePath(file.Path)
		if err != nil {
			return nil, err
		}
		if normalizedPath == entryFile {
			continue
		}

		content, err := decodeScriptTemplateFileContent(file)
		if err != nil {
			return nil, fmt.Errorf("decode template file %s failed: %w", normalizedPath, err)
		}
		result = append(result, ImportedBundleFile{
			Path:    normalizedPath,
			Content: content,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result, nil
}

func buildTemplateFiles(files []ImportedBundleFile, entryFile string) ([]scriptTemplateFile, error) {
	entryFile = strings.TrimSpace(entryFile)
	items := make([]scriptTemplateFile, 0, len(files))

	for _, file := range files {
		normalizedPath, err := normalizeBundleFilePath(file.Path)
		if err != nil {
			return nil, err
		}
		if normalizedPath == entryFile || normalizedPath == scriptStoreConfigFileName {
			continue
		}

		content, encoding := encodeScriptTemplateFileContent(file.Content)
		items = append(items, scriptTemplateFile{
			Path:     normalizedPath,
			Content:  content,
			Encoding: encoding,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func encodeScriptTemplateFileContent(content []byte) (string, string) {
	if utf8.Valid(content) {
		return string(content), "utf8"
	}
	return base64.StdEncoding.EncodeToString(content), "base64"
}

func decodeScriptTemplateFileContent(file scriptTemplateFile) ([]byte, error) {
	switch strings.TrimSpace(strings.ToLower(file.Encoding)) {
	case "", "utf8", "text":
		return []byte(file.Content), nil
	case "base64":
		return base64.StdEncoding.DecodeString(strings.TrimSpace(file.Content))
	default:
		return nil, fmt.Errorf("unsupported encoding %q", file.Encoding)
	}
}

func parseScriptTemplateJSON(text string) any {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return trimmed
	}
	return decoded
}
