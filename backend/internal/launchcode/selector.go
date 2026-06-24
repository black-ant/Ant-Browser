package launchcode

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	launchMatchModeUnique = "unique"
	launchMatchModeFirst  = "first"
	launchMatchModeAll    = "all"
)

// LaunchSelector 定义实例选择条件。
// 推荐在 POST /api/launch 中通过 selector 传入，兼容旧版 top-level code 用法。
type LaunchSelector struct {
	Code        string   `json:"code,omitempty"`
	Key         string   `json:"key,omitempty"`
	ProfileID   string   `json:"profileId,omitempty"`
	ProfileName string   `json:"profileName,omitempty"`
	Keyword     string   `json:"keyword,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Tag         string   `json:"tag,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	GroupID     string   `json:"groupId,omitempty"`
	MatchMode   string   `json:"matchMode,omitempty"`
}

func mergeLaunchSelector(req LaunchRequest) LaunchSelector {
	var nested LaunchSelector
	if req.Selector != nil {
		nested = *req.Selector
	}

	return normalizeLaunchSelector(LaunchSelector{
		Code:        firstNonEmpty(nested.Code, req.Code),
		Key:         firstNonEmpty(nested.Key, req.Key),
		ProfileID:   firstNonEmpty(nested.ProfileID, req.ProfileID),
		ProfileName: firstNonEmpty(nested.ProfileName, req.ProfileName),
		Keywords:    appendSelectorTerms(nil, "", nested.Keywords, nested.Keyword, req.Keyword, req.Keywords),
		Tags:        appendSelectorTerms(nil, nested.Tag, nested.Tags, req.Tag, req.Tags),
		GroupID:     firstNonEmpty(nested.GroupID, req.GroupID),
		MatchMode:   firstNonEmpty(nested.MatchMode, req.MatchMode),
	})
}

func normalizeLaunchSelector(selector LaunchSelector) LaunchSelector {
	selector.Code = normalizeCode(selector.Code)
	selector.Key = strings.TrimSpace(selector.Key)
	selector.Keywords = normalizeSelectorTerms(appendSelectorTerms(nil, "", selector.Keywords, selector.Keyword))
	selector.Tags = normalizeSelectorTerms(appendSelectorTerms(nil, selector.Tag, selector.Tags))
	selector.ProfileID = strings.TrimSpace(selector.ProfileID)
	selector.ProfileName = strings.TrimSpace(selector.ProfileName)
	selector.GroupID = strings.TrimSpace(selector.GroupID)
	selector.MatchMode = strings.ToLower(strings.TrimSpace(selector.MatchMode))
	if selector.MatchMode == "" {
		selector.MatchMode = defaultLaunchMatchMode(selector)
	}
	selector.Keyword = ""
	selector.Tag = ""
	return selector
}

func (selector LaunchSelector) IsEmpty() bool {
	return selector.Code == "" &&
		selector.Key == "" &&
		selector.ProfileID == "" &&
		selector.ProfileName == "" &&
		selector.GroupID == "" &&
		len(selector.Keywords) == 0 &&
		len(selector.Tags) == 0
}

func (selector LaunchSelector) OnlyCode() bool {
	return selector.Code != "" &&
		selector.Key == "" &&
		selector.ProfileID == "" &&
		selector.ProfileName == "" &&
		selector.GroupID == "" &&
		len(selector.Keywords) == 0 &&
		len(selector.Tags) == 0
}

func (selector LaunchSelector) Validate() error {
	switch selector.MatchMode {
	case "", launchMatchModeUnique, launchMatchModeFirst, launchMatchModeAll:
		return nil
	default:
		return fmt.Errorf("matchMode must be unique, first or all")
	}
}

func defaultLaunchMatchMode(selector LaunchSelector) string {
	if selector.Code != "" || selector.Key != "" || len(selector.Keywords) > 0 {
		return launchMatchModeFirst
	}
	return launchMatchModeUnique
}

func (s *LaunchServer) findProfilesBySelector(selector LaunchSelector) ([]browser.Profile, int, string) {
	if selector.IsEmpty() {
		return nil, http.StatusBadRequest, "selector is required"
	}
	if err := selector.Validate(); err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}
	if s.browserMgr == nil {
		return nil, http.StatusInternalServerError, "advanced profile selector is not available"
	}

	snapshots := s.profileSnapshots()
	if len(snapshots) == 0 {
		return nil, http.StatusNotFound, "profile selector matched no instance"
	}

	if selector.Code != "" {
		profileID, err := s.service.Resolve(selector.Code)
		if err != nil {
			return nil, http.StatusNotFound, "launch code not found"
		}
		filtered := make([]browser.Profile, 0, 1)
		for _, item := range snapshots {
			if item.ProfileId == profileID {
				filtered = append(filtered, item)
				break
			}
		}
		snapshots = filtered
	}

	if selector.ProfileID != "" {
		snapshots = filterProfiles(snapshots, func(item browser.Profile) bool {
			return item.ProfileId == selector.ProfileID
		})
	}

	if selector.ProfileName != "" {
		snapshots = filterProfiles(snapshots, func(item browser.Profile) bool {
			return strings.EqualFold(strings.TrimSpace(item.ProfileName), selector.ProfileName)
		})
	}

	if selector.GroupID != "" {
		snapshots = filterProfiles(snapshots, func(item browser.Profile) bool {
			return strings.TrimSpace(item.GroupId) == selector.GroupID
		})
	}

	if len(selector.Tags) > 0 {
		snapshots = filterProfiles(snapshots, func(item browser.Profile) bool {
			return profileHasAllTags(item, selector.Tags)
		})
	}

	fuzzyQueries := selector.Keywords
	if selector.Key != "" {
		exactMatches := filterProfiles(snapshots, func(item browser.Profile) bool {
			return profileHasExactKeyword(item, selector.Key)
		})
		if len(exactMatches) > 0 {
			snapshots = exactMatches
		} else {
			fuzzyQueries = normalizeSelectorTerms(append([]string{selector.Key}, fuzzyQueries...))
		}
	}

	if len(fuzzyQueries) > 0 {
		snapshots = filterProfiles(snapshots, func(item browser.Profile) bool {
			return profileMatchesAllKeywordQueries(item, fuzzyQueries)
		})
	}

	if len(snapshots) == 0 {
		if selector.OnlyCode() {
			return nil, http.StatusNotFound, "launch code not found"
		}
		return nil, http.StatusNotFound, "profile selector matched no instance"
	}

	sortProfilesForSelector(snapshots)
	return snapshots, http.StatusOK, ""
}

func (s *LaunchServer) findProfileBySelector(selector LaunchSelector) (browser.Profile, int, string) {
	snapshots, status, errMsg := s.findProfilesBySelector(selector)
	if errMsg != "" {
		return browser.Profile{}, status, errMsg
	}
	if len(snapshots) > 1 && selector.MatchMode != launchMatchModeFirst {
		return browser.Profile{}, http.StatusConflict, buildAmbiguousSelectorError(snapshots)
	}
	return snapshots[0], http.StatusOK, ""
}

func (s *LaunchServer) profileSnapshots() []browser.Profile {
	if s.browserMgr == nil {
		return nil
	}

	s.browserMgr.Mutex.Lock()
	items := make([]browser.Profile, 0, len(s.browserMgr.Profiles))
	for _, profile := range s.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		items = append(items, *profile)
	}
	s.browserMgr.Mutex.Unlock()

	if s.service != nil {
		for i := range items {
			if code, err := s.service.EnsureCode(items[i].ProfileId); err == nil {
				items[i].LaunchCode = code
			}
		}
	}
	return items
}

func filterProfiles(items []browser.Profile, keep func(browser.Profile) bool) []browser.Profile {
	filtered := make([]browser.Profile, 0, len(items))
	for _, item := range items {
		if keep(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func profileHasAllTags(profile browser.Profile, required []string) bool {
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

func profileHasExactKeyword(profile browser.Profile, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" || len(profile.Keywords) == 0 {
		return false
	}

	for _, keyword := range profile.Keywords {
		if strings.EqualFold(strings.TrimSpace(keyword), expected) {
			return true
		}
	}
	return false
}

func profileMatchesAllKeywordQueries(profile browser.Profile, queries []string) bool {
	if len(queries) == 0 {
		return true
	}
	if len(profile.Keywords) == 0 {
		return false
	}

	for _, query := range queries {
		queryLower := strings.ToLower(query)
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

func sortProfilesForSelector(items []browser.Profile) {
	sort.Slice(items, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(items[i].ProfileName))
		rightName := strings.ToLower(strings.TrimSpace(items[j].ProfileName))
		if leftName != rightName {
			return leftName < rightName
		}
		return items[i].ProfileId < items[j].ProfileId
	})
}

func buildAmbiguousSelectorError(items []browser.Profile) string {
	// 防止信息泄漏：不暴露profileId、launchCode和profileName等敏感信息
	return fmt.Sprintf(
		"selector matched %d profiles; refine using profileId, groupId, tags, keywords, or set matchMode=first",
		len(items),
	)
}

func appendSelectorTerms(dst []string, single string, many []string, moreSinglesAndSlices ...interface{}) []string {
	if trimmed := strings.TrimSpace(single); trimmed != "" {
		dst = append(dst, trimmed)
	}
	dst = append(dst, many...)
	for _, item := range moreSinglesAndSlices {
		switch v := item.(type) {
		case string:
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				dst = append(dst, trimmed)
			}
		case []string:
			dst = append(dst, v...)
		}
	}
	return dst
}

func normalizeSelectorTerms(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
