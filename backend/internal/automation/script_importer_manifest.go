package automation

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func parseImportManifest(data []byte) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("script manifest is not valid JSON")
	}

	if manifest, ok := parsed["manifest"].(map[string]any); ok {
		for key, value := range parsed {
			if key == "manifest" {
				continue
			}
			if _, exists := manifest[key]; !exists {
				manifest[key] = value
			}
		}
		if mapStringValueAny(manifest, "script") == "" && mapStringValueAny(manifest, "scriptText") == "" && parsed["script"] == nil && parsed["scriptText"] == nil {
			return manifest, nil
		}
	}

	if mapStringValueAny(parsed, "script") != "" || mapStringValueAny(parsed, "scriptText") != "" {
		return parsed, nil
	}

	if mapStringValueAny(parsed, "entryFile") == "" {
		return nil, fmt.Errorf("script manifest is missing entryFile")
	}
	return parsed, nil
}

func buildImportedRecord(envelope scriptImportEnvelope, defaultName string, sourceLabel string) (ScriptRecord, error) {
	descriptor := map[string]any{}
	if envelope.Manifest != nil {
		for key, value := range envelope.Manifest {
			descriptor[key] = value
		}
	}
	mergeDescriptorValue(descriptor, "format", envelope.Format)
	mergeDescriptorValue(descriptor, "packageFormat", envelope.PackageFormat)
	mergeDescriptorValue(descriptor, "id", envelope.ID)
	if envelope.ManifestVersion > 0 {
		descriptor["manifestVersion"] = envelope.ManifestVersion
	}
	mergeDescriptorValue(descriptor, "name", envelope.Name)
	mergeDescriptorValue(descriptor, "description", envelope.Description)
	mergeDescriptorValue(descriptor, "type", envelope.Type)
	mergeDescriptorValue(descriptor, "status", envelope.Status)
	mergeDescriptorValue(descriptor, "entryFile", envelope.EntryFile)
	if len(envelope.Tags) > 0 {
		descriptor["tags"] = envelope.Tags
	}
	mergeDescriptorValue(descriptor, "notes", envelope.Notes)
	if envelope.TargetConfig != nil {
		descriptor["targetConfig"] = envelope.TargetConfig
	}
	if envelope.PublicAPI != nil {
		descriptor["publicAPI"] = envelope.PublicAPI
	}
	if envelope.Selector != nil {
		descriptor["selector"] = envelope.Selector
	}
	if envelope.SelectorText != nil {
		descriptor["selectorText"] = envelope.SelectorText
	}
	if envelope.Params != nil {
		descriptor["params"] = envelope.Params
	}
	if envelope.ParamsText != nil {
		descriptor["paramsText"] = envelope.ParamsText
	}
	if envelope.Source != nil {
		descriptor["source"] = envelope.Source
	}

	scriptText := firstNonEmpty(strings.TrimSpace(envelope.Script), strings.TrimSpace(envelope.ScriptText))
	if scriptText == "" {
		if raw, exists := descriptor["script"]; exists {
			scriptText = strings.TrimSpace(fmt.Sprint(raw))
		}
	}
	if scriptText == "" {
		if raw, exists := descriptor["scriptText"]; exists {
			scriptText = strings.TrimSpace(fmt.Sprint(raw))
		}
	}
	if scriptText == "" {
		return ScriptRecord{}, fmt.Errorf("inline script content is missing")
	}

	now := time.Now().Format(time.RFC3339)
	packageFormat := normalizeScriptPackageFormat(firstNonEmpty(mapStringValueAny(descriptor, "packageFormat"), mapStringValueAny(descriptor, "format")))
	recordID := uuid.NewString()
	recordStatus := "draft"
	if packageFormat == defaultScriptPackageFormat {
		if manifestID := strings.TrimSpace(mapStringValueAny(descriptor, "id")); manifestID != "" {
			recordID = manifestID
		}
		recordStatus = mapStringValueAny(descriptor, "status")
	}
	source := inferImportSource(sourceLabel)
	if explicitSource, ok := descriptor["source"].(map[string]any); ok {
		source = mergeImportedSource(source, explicitSource)
	}
	return normalizeScriptRecord(ScriptRecord{
		PackageFormat:   packageFormat,
		ManifestVersion: normalizeScriptManifestVersion(mapIntValueAny(descriptor, "manifestVersion"), 0),
		ID:              recordID,
		Name:            firstNonEmpty(mapStringValueAny(descriptor, "name"), defaultName, "导入脚本"),
		Description:     mapStringValueAny(descriptor, "description"),
		Type:            mapStringValueAny(descriptor, "type"),
		Status:          recordStatus,
		EntryFile:       normalizeScriptEntryFile(firstNonEmpty(mapStringValueAny(descriptor, "entryFile"), defaultEntryFileForName(defaultName))),
		Tags:            mapStringSliceValue(descriptor, "tags"),
		SelectorText:    stringifyImportJSONValue(firstNonNil(descriptor["selectorText"], descriptor["selector"])),
		ParamsText:      stringifyImportJSONValue(firstNonNil(descriptor["paramsText"], descriptor["params"])),
		ScriptText:      scriptText,
		Notes:           mapStringValueAny(descriptor, "notes"),
		TargetConfig:    mapScriptTargetConfigValue(descriptor["targetConfig"]),
		PublicAPI:       mapScriptPublicAPIValue(descriptor["publicAPI"]),
		Source:          source,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, ScriptRecord{})
}

func appendImportSourceNote(notes string, sourceLabel string) string {
	sourceLabel = strings.TrimSpace(sourceLabel)
	if sourceLabel == "" {
		return strings.TrimSpace(notes)
	}

	line := "来源: " + sourceLabel
	if strings.TrimSpace(notes) == "" {
		return line
	}
	if strings.Contains(notes, line) {
		return strings.TrimSpace(notes)
	}
	return strings.TrimSpace(notes) + "\n" + line
}

func defaultEntryFileForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".js", ".cjs", ".mjs":
		return filepath.Base(name)
	default:
		return defaultScriptEntryFile
	}
}

func trimExtension(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "导入脚本"
	}
	base := filepath.Base(trimmed)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	nameWithoutExt := strings.TrimSpace(strings.TrimSuffix(base, ext))
	if nameWithoutExt == "" {
		return base
	}
	return nameWithoutExt
}
