package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func stringifyImportJSONValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return string(bytes.TrimSpace(encoded))
}

func mergeDescriptorValue(target map[string]any, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, exists := target[key]; exists {
		return
	}
	target[key] = strings.TrimSpace(value)
}

func mapStringSliceValue(payload map[string]any, key string) []string {
	raw, exists := payload[key]
	if !exists || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		if stringsValue, ok := raw.([]string); ok {
			return normalizeScriptTags(stringsValue)
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" {
			result = append(result, value)
		}
	}
	return normalizeScriptTags(result)
}

func mapStringValueAny(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, exists := payload[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mapIntValueAny(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, exists := payload[key]
	if !exists || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func mapObjectValue(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, exists := payload[key]
	if !exists || value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return nil
}

func inferImportSource(sourceLabel string) ScriptSource {
	now := time.Now().Format(time.RFC3339)
	trimmed := strings.TrimSpace(sourceLabel)
	source := ScriptSource{
		ImportedAt: now,
	}
	switch {
	case strings.HasPrefix(trimmed, "本地文件 "):
		source.Type = "local-file"
		source.URI = strings.TrimSpace(strings.TrimPrefix(trimmed, "本地文件 "))
	case strings.HasPrefix(trimmed, "本地目录 "):
		source.Type = "local-dir"
		source.URI = strings.TrimSpace(strings.TrimPrefix(trimmed, "本地目录 "))
	case strings.HasPrefix(trimmed, "远程地址 "):
		source.Type = "remote-url"
		source.URI = strings.TrimSpace(strings.TrimPrefix(trimmed, "远程地址 "))
	case strings.HasPrefix(trimmed, "Git "):
		source.Type = "git"
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "Git "))
		repo := rest
		if index := strings.Index(rest, " : "); index >= 0 {
			source.Path = strings.TrimSpace(rest[index+3:])
			repo = strings.TrimSpace(rest[:index])
		}
		if index := strings.Index(repo, " @ "); index >= 0 {
			source.Ref = strings.TrimSpace(repo[index+3:])
			repo = strings.TrimSpace(repo[:index])
		}
		source.URI = repo
	default:
		if trimmed != "" {
			source.Type = "manual"
			source.URI = trimmed
		}
	}
	return source
}

func mergeImportedSource(base ScriptSource, override map[string]any) ScriptSource {
	if override == nil {
		return base
	}
	next := base
	if value := mapStringValueAny(override, "type"); value != "" {
		next.Type = value
	}
	if value := firstNonEmpty(mapStringValueAny(override, "uri"), mapStringValueAny(override, "url")); value != "" {
		next.URI = value
	}
	if value := mapStringValueAny(override, "ref"); value != "" {
		next.Ref = value
	}
	if value := mapStringValueAny(override, "path"); value != "" {
		next.Path = value
	}
	if value := mapStringValueAny(override, "importedAt"); value != "" {
		next.ImportedAt = value
	}
	return next
}

func mapScriptTargetConfigValue(value any) ScriptTargetConfig {
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return ScriptTargetConfig{}
	}

	return ScriptTargetConfig{
		Mode:               mapStringValueAny(object, "mode"),
		Selector:           mapScriptTargetSelectorValue(object["selector"]),
		TemplateSelector:   mapScriptTargetSelectorValue(object["templateSelector"]),
		CreateNameTemplate: mapStringValueAny(object, "createNameTemplate"),
	}
}

func mapScriptPublicAPIValue(value any) ScriptPublicAPIConfig {
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return ScriptPublicAPIConfig{}
	}

	return ScriptPublicAPIConfig{
		Enabled:          mapBoolValueAny(object, "enabled"),
		Method:           mapStringValueAny(object, "method"),
		Path:             firstNonEmpty(mapStringValueAny(object, "path"), mapStringValueAny(object, "route")),
		RequestMode:      mapStringValueAny(object, "requestMode"),
		ResponseMode:     mapStringValueAny(object, "responseMode"),
		TimeoutMs:        mapIntValueAny(object, "timeoutMs"),
		RequestBodyText:  stringifyImportJSONValue(firstNonNil(object["requestBodyText"], object["requestBody"], object["requestExample"])),
		ResponseBodyText: stringifyImportJSONValue(firstNonNil(object["responseBodyText"], object["responseBody"], object["responseExample"])),
		Variables:        mapScriptPublicAPIVariablesValue(object["variables"]),
	}
}

func mapScriptPublicAPIVariablesValue(value any) []ScriptPublicAPIVariable {
	switch typed := value.(type) {
	case []ScriptPublicAPIVariable:
		return normalizeScriptPublicAPIVariables(typed)
	case []any:
		variables := make([]ScriptPublicAPIVariable, 0, len(typed))
		for _, item := range typed {
			object, ok := item.(map[string]any)
			if !ok || object == nil {
				continue
			}
			variables = append(variables, ScriptPublicAPIVariable{
				Name:         firstNonEmpty(mapStringValueAny(object, "name"), mapStringValueAny(object, "key")),
				DefaultValue: firstNonEmpty(mapStringValueAny(object, "defaultValue"), mapStringValueAny(object, "default"), mapStringValueAny(object, "value")),
				Description:  firstNonEmpty(mapStringValueAny(object, "description"), mapStringValueAny(object, "label"), mapStringValueAny(object, "note")),
				Required:     mapBoolValueAny(object, "required"),
			})
		}
		return normalizeScriptPublicAPIVariables(variables)
	case map[string]any:
		variables := make([]ScriptPublicAPIVariable, 0, len(typed))
		for key, rawValue := range typed {
			variable := ScriptPublicAPIVariable{Name: key}
			if object, ok := rawValue.(map[string]any); ok && object != nil {
				variable.DefaultValue = firstNonEmpty(mapStringValueAny(object, "defaultValue"), mapStringValueAny(object, "default"), mapStringValueAny(object, "value"))
				variable.Description = firstNonEmpty(mapStringValueAny(object, "description"), mapStringValueAny(object, "label"), mapStringValueAny(object, "note"))
				variable.Required = mapBoolValueAny(object, "required")
			} else if rawValue != nil {
				variable.DefaultValue = strings.TrimSpace(fmt.Sprint(rawValue))
			}
			variables = append(variables, variable)
		}
		return normalizeScriptPublicAPIVariables(variables)
	default:
		return nil
	}
}

func mapScriptTargetSelectorValue(value any) ScriptTargetSelector {
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return ScriptTargetSelector{}
	}

	return ScriptTargetSelector{
		Code:        firstNonEmpty(mapStringValueAny(object, "code"), mapStringValueAny(object, "launchCode")),
		ProfileID:   mapStringValueAny(object, "profileId"),
		ProfileName: mapStringValueAny(object, "profileName"),
		GroupID:     mapStringValueAny(object, "groupId"),
		Keywords:    mapStringSliceValue(object, "keywords"),
		Tags:        mapStringSliceValue(object, "tags"),
	}
}

func mapBoolValueAny(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, exists := payload[key]
	if !exists || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "enabled", "on":
			return true
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	}
	return false
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
