package backend

import (
	"fmt"
	"sort"
	"strings"

	"ant-chrome/backend/internal/browser"
)

func filterAutomationProfiles(items []browser.Profile, keep func(browser.Profile) bool) []browser.Profile {
	filtered := make([]browser.Profile, 0, len(items))
	for _, item := range items {
		if keep(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func automationProfileHasAllTags(profile browser.Profile, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(profile.Tags) == 0 {
		return false
	}

	for _, want := range required {
		found := false
		for _, tag := range profile.Tags {
			if strings.EqualFold(strings.TrimSpace(tag), want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func automationProfileMatchesAllKeywordQueries(profile browser.Profile, queries []string) bool {
	if len(queries) == 0 {
		return true
	}
	if len(profile.Keywords) == 0 {
		return false
	}

	for _, query := range queries {
		queryLower := strings.ToLower(strings.TrimSpace(query))
		found := false
		for _, keyword := range profile.Keywords {
			if strings.Contains(strings.ToLower(strings.TrimSpace(keyword)), queryLower) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sortAutomationProfilesForTarget(items []browser.Profile) {
	sort.Slice(items, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(items[i].ProfileName))
		rightName := strings.ToLower(strings.TrimSpace(items[j].ProfileName))
		if leftName != rightName {
			return leftName < rightName
		}
		return items[i].ProfileId < items[j].ProfileId
	})
}

func buildAutomationTargetAmbiguousError(items []browser.Profile) string {
	const maxPreview = 5
	parts := make([]string, 0, minAutomationInt(len(items), maxPreview))
	for i := 0; i < len(items) && i < maxPreview; i++ {
		parts = append(parts, automationProfileLabel(items[i]))
	}
	suffix := ""
	if len(items) > maxPreview {
		suffix = fmt.Sprintf(" 等 %d 个实例", len(items))
	}
	return fmt.Sprintf("命中了多个实例：%s%s。请改用 code/profileId，或继续加分组、标签、关键字缩小范围", strings.Join(parts, "，"), suffix)
}

func automationProfileLabel(profile browser.Profile) string {
	label := strings.TrimSpace(profile.ProfileName)
	if label == "" {
		label = strings.TrimSpace(profile.ProfileId)
	}
	if code := strings.TrimSpace(profile.LaunchCode); code != "" {
		return fmt.Sprintf("%s[id=%s, code=%s]", label, profile.ProfileId, code)
	}
	return fmt.Sprintf("%s[id=%s]", label, profile.ProfileId)
}
