package launchcode

import (
	"testing"

	"ant-chrome/backend/internal/browser"
)

func TestProfileProxySelectionSurvivesLaunchCodeInputNormalization(t *testing.T) {
	input := normalizeProfileInput(browser.ProfileInput{
		ProxyMode:      browser.ProxyModeDynamic,
		ProxyGroupName: " mobile ",
	})
	if input.ProxyMode != browser.ProxyModeDynamic || input.ProxyGroupName != "mobile" {
		t.Fatalf("normalized proxy selection = (%q, %q), want (%q, %q)", input.ProxyMode, input.ProxyGroupName, browser.ProxyModeDynamic, "mobile")
	}

	profile := &browser.Profile{
		ProxyMode:      browser.ProxyModeDynamic,
		ProxyGroupName: " mobile ",
	}
	rollbackInput := profileToInput(profile)
	if rollbackInput.ProxyMode != browser.ProxyModeDynamic || rollbackInput.ProxyGroupName != "mobile" {
		t.Fatalf("rollback proxy selection = (%q, %q), want (%q, %q)", rollbackInput.ProxyMode, rollbackInput.ProxyGroupName, browser.ProxyModeDynamic, "mobile")
	}

	legacyInput := normalizeProfileInput(browser.ProfileInput{ProxyConfig: "http://127.0.0.1:8080"})
	if legacyInput.ProxyMode != "" {
		t.Fatalf("legacy proxy mode = %q, want empty so profile input can infer it", legacyInput.ProxyMode)
	}
}
