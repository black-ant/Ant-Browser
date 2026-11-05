package automation

import (
	"fmt"
	"strings"
)

func (s *ScriptStore) validateRecord(record ScriptRecord) error {
	if err := validateScriptPublicAPIConfig(record.PublicAPI); err != nil {
		return err
	}
	return s.validatePublicAPIUniqueness(record)
}

func validateScriptPublicAPIConfig(config ScriptPublicAPIConfig) error {
	for _, variable := range config.Variables {
		if !isScriptPublicAPIVariableName(variable.Name) {
			return fmt.Errorf("public api variable name %q is invalid", variable.Name)
		}
	}

	if !config.Enabled && strings.TrimSpace(config.Path) == "" {
		return nil
	}

	if method := normalizeScriptPublicAPIMethod(config.Method); method != "POST" {
		return fmt.Errorf("public api method %q is not supported", method)
	}
	if normalizeScriptPublicAPIRequestMode(config.RequestMode) == "" {
		return fmt.Errorf("public api request mode is invalid")
	}
	if normalizeScriptPublicAPIResponseMode(config.ResponseMode) == "" {
		return fmt.Errorf("public api response mode is invalid")
	}

	pathValue := normalizeScriptPublicAPIPath(config.Path)
	if config.Enabled && pathValue == "" {
		return fmt.Errorf("public api path is required when enabled")
	}
	if pathValue == "" {
		return nil
	}

	for _, part := range strings.Split(pathValue, "/") {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("public api path is invalid")
		}
	}
	return nil
}

func isScriptPublicAPIVariableName(value string) bool {
	name := strings.TrimSpace(value)
	if name == "" {
		return false
	}
	for index, ch := range name {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch == '_':
		case index > 0 && ch >= '0' && ch <= '9':
		default:
			return false
		}
	}
	return true
}

func (s *ScriptStore) validatePublicAPIUniqueness(record ScriptRecord) error {
	targetPath := normalizeScriptPublicAPIPath(record.PublicAPI.Path)
	if targetPath == "" {
		return nil
	}

	items, err := s.List()
	if err != nil {
		return err
	}

	for _, item := range items {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(record.ID) {
			continue
		}
		if normalizeScriptPublicAPIPath(item.PublicAPI.Path) != targetPath {
			continue
		}

		targetName := strings.TrimSpace(item.Name)
		if targetName == "" {
			targetName = strings.TrimSpace(item.ID)
		}
		return fmt.Errorf("public api path %q is already used by script %s", scriptPublicAPIRoute(targetPath), targetName)
	}
	return nil
}
