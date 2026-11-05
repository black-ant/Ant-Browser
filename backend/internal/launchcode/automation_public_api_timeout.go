package launchcode

import (
	"net/http"
	"strconv"
	"strings"
)

func resolveAutomationPublicHookTimeout(r *http.Request, requestTimeout int, fallback int) int {
	if requestTimeout > 0 {
		return requestTimeout
	}

	if r != nil {
		if raw := strings.TrimSpace(r.URL.Query().Get("timeoutMs")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				return parsed
			}
		}
	}

	return fallback
}
