package backend

import "strings"

func appendAutomationRunSummary(summary string, targetSummary string) string {
	summary = strings.TrimSpace(summary)
	targetSummary = strings.TrimSpace(targetSummary)
	if targetSummary == "" {
		return summary
	}
	if summary == "" {
		return targetSummary
	}
	return summary + " · " + targetSummary
}

func minAutomationInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
