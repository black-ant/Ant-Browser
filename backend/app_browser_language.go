package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	browserLangArg         = "--lang"
	browserAcceptLangArg   = "--accept-lang"
	browserDefaultEncoding = "UTF-8"
)

func normalizeBrowserLanguageArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	normalized := append([]string{}, args...)
	lang := browserArgValue(normalized, browserLangArg)
	acceptLang := browserArgValue(normalized, browserAcceptLangArg)

	if lang != "" && acceptLang == "" {
		normalized = append(normalized, fmt.Sprintf("%s=%s", browserAcceptLangArg, buildAcceptLanguage(lang)))
	}
	if lang == "" && acceptLang != "" {
		if inferredLang := browserLangFromAcceptLanguage(acceptLang); inferredLang != "" {
			normalized = append(normalized, fmt.Sprintf("%s=%s", browserLangArg, inferredLang))
		}
	}
	return normalized
}

func writeBrowserLanguagePreferences(userDataDir string, args []string) error {
	lang := browserArgValue(args, browserLangArg)
	acceptLang := browserArgValue(args, browserAcceptLangArg)
	if acceptLang == "" && lang != "" {
		acceptLang = buildAcceptLanguage(lang)
	}
	if lang == "" && acceptLang != "" {
		lang = browserLangFromAcceptLanguage(acceptLang)
	}
	if lang == "" && acceptLang == "" {
		return nil
	}

	profileDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}

	prefsPath := filepath.Join(profileDir, "Preferences")
	prefs := map[string]interface{}{}
	if data, err := os.ReadFile(prefsPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &prefs); err != nil {
			return fmt.Errorf("解析浏览器语言偏好失败: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	intl, _ := prefs["intl"].(map[string]interface{})
	if intl == nil {
		intl = map[string]interface{}{}
	}
	if acceptLang != "" {
		intl["accept_languages"] = acceptLang
	}
	prefs["intl"] = intl

	webkit, _ := prefs["webkit"].(map[string]interface{})
	if webkit == nil {
		webkit = map[string]interface{}{}
	}
	webprefs, _ := webkit["webprefs"].(map[string]interface{})
	if webprefs == nil {
		webprefs = map[string]interface{}{}
	}
	webprefs["default_encoding"] = browserDefaultEncoding
	webkit["webprefs"] = webprefs
	prefs["webkit"] = webkit

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(prefsPath, append(data, '\n'), 0o644)
}

func browserArgValue(args []string, key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	prefix := key + "="
	for i := len(args) - 1; i >= 0; i-- {
		arg := strings.TrimSpace(args[i])
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(arg[len(prefix):])
		}
	}
	return ""
}

func browserArgsHaveFingerprintSeed(args []string) bool {
	return browserArgValue(args, "--fingerprint") != ""
}

func browserFingerprintSeedForProfileID(profileID string) int {
	seed := 0
	for _, char := range profileID {
		seed = (seed << 5) - seed + int(char)
	}
	if seed < 0 {
		seed = -seed
	}
	return seed
}

func buildAcceptLanguage(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}
	base := strings.FieldsFunc(lang, func(r rune) bool { return r == '-' || r == '_' })[0]
	if base == "" || strings.EqualFold(base, lang) {
		return lang
	}
	return lang + "," + base
}

func browserLangFromAcceptLanguage(acceptLang string) string {
	for _, part := range strings.Split(acceptLang, ",") {
		value := strings.TrimSpace(strings.Split(part, ";")[0])
		if value != "" {
			return value
		}
	}
	return ""
}
