package automation

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func normalizeScriptRecord(input ScriptRecord, existing ScriptRecord) (ScriptRecord, error) {
	now := time.Now().Format(time.RFC3339)

	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	if !isSafeScriptID(id) {
		return ScriptRecord{}, fmt.Errorf("script id is invalid")
	}

	entryFile := normalizeScriptEntryFile(input.EntryFile)
	recordType := normalizeScriptType(input.Type)
	recordStatus := normalizeScriptStatus(input.Status)
	packageFormat := normalizeScriptPackageFormat(firstNonEmpty(strings.TrimSpace(input.PackageFormat), strings.TrimSpace(existing.PackageFormat)))
	manifestVersion := normalizeScriptManifestVersion(input.ManifestVersion, existing.ManifestVersion)
	createdAt := firstNonEmpty(strings.TrimSpace(existing.CreatedAt), strings.TrimSpace(input.CreatedAt), now)
	updatedAt := firstNonEmpty(strings.TrimSpace(input.UpdatedAt), now)

	if strings.TrimSpace(input.Name) == "" {
		return ScriptRecord{}, fmt.Errorf("script name is required")
	}

	return ScriptRecord{
		PackageFormat:   packageFormat,
		ManifestVersion: manifestVersion,
		ID:              id,
		Name:            strings.TrimSpace(input.Name),
		Description:     strings.TrimSpace(input.Description),
		Type:            recordType,
		Status:          recordStatus,
		EntryFile:       entryFile,
		Tags:            normalizeScriptTags(input.Tags),
		SelectorText:    normalizeScriptJSONText(input.SelectorText),
		ParamsText:      normalizeScriptJSONText(input.ParamsText),
		ScriptText:      normalizeScriptText(input.ScriptText),
		Notes:           strings.TrimSpace(input.Notes),
		TargetConfig:    normalizeScriptTargetConfig(input.TargetConfig),
		PublicAPI:       normalizeScriptPublicAPIConfig(input.PublicAPI),
		Source:          normalizeScriptSource(input.Source, existing.Source),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

func normalizeScriptPackageFormat(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultScriptPackageFormat
	}
	return normalized
}

func normalizeScriptManifestVersion(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return defaultScriptManifestVersion
}

func normalizeScriptType(value string) string {
	switch strings.TrimSpace(value) {
	case "launch-api":
		return "launch-api"
	default:
		return "playwright-cdp"
	}
}

func normalizeScriptStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "ready":
		return "ready"
	case "disabled":
		return "disabled"
	default:
		return "draft"
	}
}

func normalizeScriptEntryFile(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultScriptEntryFile
	}
	normalized = filepath.ToSlash(filepath.Clean(normalized))
	if normalized == "." || normalized == "/" || normalized == scriptStoreConfigFileName {
		return defaultScriptEntryFile
	}
	if strings.HasPrefix(normalized, "../") || normalized == ".." || filepath.IsAbs(normalized) {
		return defaultScriptEntryFile
	}
	return normalized
}

func normalizeBundleFilePath(value string) (string, error) {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if normalized == "." || normalized == "/" || normalized == "" {
		return "", fmt.Errorf("bundle file path is invalid")
	}
	if strings.HasPrefix(normalized, "../") || normalized == ".." || filepath.IsAbs(normalized) {
		return "", fmt.Errorf("bundle file path is invalid")
	}
	return normalized, nil
}

func normalizeScriptTags(tags []string) []string {
	deduped := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		if _, exists := deduped[normalized]; exists {
			continue
		}
		deduped[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeScriptJSONText(value string) string {
	return strings.TrimSpace(value)
}

func normalizeScriptText(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func normalizeScriptSource(input ScriptSource, existing ScriptSource) ScriptSource {
	source := ScriptSource{
		Type:       firstNonEmpty(strings.TrimSpace(input.Type), strings.TrimSpace(existing.Type)),
		URI:        firstNonEmpty(strings.TrimSpace(input.URI), strings.TrimSpace(existing.URI)),
		Ref:        firstNonEmpty(strings.TrimSpace(input.Ref), strings.TrimSpace(existing.Ref)),
		Path:       firstNonEmpty(strings.TrimSpace(input.Path), strings.TrimSpace(existing.Path)),
		ImportedAt: firstNonEmpty(strings.TrimSpace(input.ImportedAt), strings.TrimSpace(existing.ImportedAt)),
	}
	if source.Type == "" && (source.URI != "" || source.Ref != "" || source.Path != "" || source.ImportedAt != "") {
		source.Type = "manual"
	}
	return source
}

func normalizeScriptTargetConfig(input ScriptTargetConfig) ScriptTargetConfig {
	mode := normalizeScriptTargetMode(input.Mode)
	createNameTemplate := strings.TrimSpace(input.CreateNameTemplate)
	if createNameTemplate == "" && mode == "create" {
		createNameTemplate = defaultScriptCreateNameTemplate
	}

	return ScriptTargetConfig{
		Mode:               mode,
		Selector:           normalizeScriptTargetSelector(input.Selector),
		TemplateSelector:   normalizeScriptTargetSelector(input.TemplateSelector),
		CreateNameTemplate: createNameTemplate,
	}
}

func normalizeScriptTargetMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "existing":
		return "existing"
	case "create":
		return "create"
	case "rotate":
		return "rotate"
	default:
		return "manual"
	}
}

func normalizeScriptTargetSelector(input ScriptTargetSelector) ScriptTargetSelector {
	return ScriptTargetSelector{
		Code:        strings.ToUpper(strings.TrimSpace(input.Code)),
		ProfileID:   strings.TrimSpace(input.ProfileID),
		ProfileName: strings.TrimSpace(input.ProfileName),
		GroupID:     strings.TrimSpace(input.GroupID),
		Keywords:    normalizeScriptTags(input.Keywords),
		Tags:        normalizeScriptTags(input.Tags),
	}
}

func normalizeScriptPublicAPIConfig(input ScriptPublicAPIConfig) ScriptPublicAPIConfig {
	return ScriptPublicAPIConfig{
		Enabled:          input.Enabled,
		Method:           normalizeScriptPublicAPIMethod(input.Method),
		Path:             normalizeScriptPublicAPIPath(input.Path),
		RequestMode:      normalizeScriptPublicAPIRequestMode(input.RequestMode),
		ResponseMode:     normalizeScriptPublicAPIResponseMode(input.ResponseMode),
		TimeoutMs:        normalizeScriptPublicAPITimeout(input.TimeoutMs),
		RequestBodyText:  normalizeScriptJSONText(input.RequestBodyText),
		ResponseBodyText: normalizeScriptJSONText(input.ResponseBodyText),
		Variables:        normalizeScriptPublicAPIVariables(input.Variables),
	}
}

func normalizeScriptPublicAPIVariables(variables []ScriptPublicAPIVariable) []ScriptPublicAPIVariable {
	seen := make(map[string]struct{}, len(variables))
	result := make([]ScriptPublicAPIVariable, 0, len(variables))
	for _, variable := range variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, ScriptPublicAPIVariable{
			Name:         name,
			DefaultValue: strings.TrimSpace(variable.DefaultValue),
			Description:  strings.TrimSpace(variable.Description),
			Required:     variable.Required,
		})
	}
	return result
}

func normalizeScriptPublicAPIMethod(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "POST":
		return "POST"
	default:
		return "POST"
	}
}

func normalizeScriptPublicAPIRequestMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "params-only":
		return "params-only"
	default:
		return "standard"
	}
}

func normalizeScriptPublicAPIResponseMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "result-only":
		return "result-only"
	default:
		return "envelope"
	}
}

func normalizeScriptPublicAPITimeout(value int) int {
	if value <= 0 {
		return defaultScriptPublicAPITimeoutMs
	}
	if value < 1000 {
		return 1000
	}
	if value > 30*60*1000 {
		return 30 * 60 * 1000
	}
	return value
}

func normalizeScriptPublicAPIPath(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}

	normalized = strings.ReplaceAll(normalized, "\\", "/")
	lower := strings.ToLower(normalized)
	if strings.HasPrefix(lower, scriptPublicAPIBasePath+"/") {
		normalized = normalized[len(scriptPublicAPIBasePath)+1:]
	} else if strings.HasPrefix(lower, strings.TrimPrefix(scriptPublicAPIBasePath, "/")+"/") {
		normalized = normalized[len(strings.TrimPrefix(scriptPublicAPIBasePath, "/"))+1:]
	}

	cleaned := path.Clean("/" + normalized)
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return ""
	}

	parts := strings.Split(cleaned, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalizedPart := normalizeScriptPublicAPIPathSegment(part); normalizedPart != "" {
			result = append(result, normalizedPart)
		}
	}
	return strings.Join(result, "/")
}

func normalizeScriptPublicAPIPathSegment(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, ch := range strings.TrimSpace(value) {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
			lastDash = false
		case ch >= 'A' && ch <= 'Z':
			builder.WriteRune(ch + 32)
			lastDash = false
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastDash = false
		case ch == '-', ch == '_', ch == '.':
			builder.WriteRune(ch)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

func scriptPublicAPIRoute(pathValue string) string {
	pathValue = normalizeScriptPublicAPIPath(pathValue)
	if pathValue == "" {
		return scriptPublicAPIBasePath
	}
	return scriptPublicAPIBasePath + "/" + pathValue
}

func isSafeScriptID(value string) bool {
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '-', ch == '_', ch == '.':
		default:
			return false
		}
	}
	return true
}

func parseRFC3339OrZero(value string) time.Time {
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return ts
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
