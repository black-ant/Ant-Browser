package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

// ProfileWriteRequest 用于创建/更新实例配置。
// profile 为持久化配置；start 为本次自动启动的临时参数。
type ProfileWriteRequest struct {
	Profile    *browser.ProfileInput `json:"profile"`
	LaunchCode string                `json:"launchCode"`
	AutoLaunch bool                  `json:"autoLaunch"`
	Start      *LaunchRequestParams  `json:"start"`
}

type profileCreator interface {
	CreateProfile(input browser.ProfileInput) (*browser.Profile, error)
}

type profileUpdater interface {
	UpdateProfile(profileID string, input browser.ProfileInput) (*browser.Profile, error)
}

type profileDeleter interface {
	DeleteProfile(profileID string) error
}

func (s *LaunchServer) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListProfiles(w, r)
	case http.MethodPost:
		s.handleCreateProfile(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
	}
}

func (s *LaunchServer) handleProfileByID(w http.ResponseWriter, r *http.Request) {
	profileID, ok := parseProfilePathID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":    false,
			"error": "profile not found",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetProfile(w, r, profileID)
	case http.MethodPut:
		s.handleUpdateProfile(w, r, profileID)
	case http.MethodDelete:
		s.handleDeleteProfile(w, r, profileID)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
	}
}

// handleCreateProfile POST /api/profiles
func (s *LaunchServer) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	log := logger.New("LaunchServer")
	startAt := time.Now()

	req, status, errMsg := decodeProfileWriteRequest(r)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	input := normalizeProfileInput(*req.Profile)
	profile, launchCode, status, errMsg := s.createProfile(input, resolveRequestedLaunchCode(req.LaunchCode, input.LaunchCode))
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	launchedProfile, launched, launchErr := s.maybeAutoLaunchProfile(profile, req)
	if launchErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":          false,
			"created":     true,
			"updated":     false,
			"launched":    false,
			"profileId":   profile.ProfileId,
			"profileName": profile.ProfileName,
			"launchCode":  launchCode,
			"profile":     profile,
			"error":       launchErr.Error(),
		})
		log.Warn("Profile API 创建后自动启动失败",
			logger.F("profile_id", profile.ProfileId),
			logger.F("profile_name", profile.ProfileName),
			logger.F("launch_code", launchCode),
			logger.F("duration_ms", time.Since(startAt).Milliseconds()),
			logger.F("error", launchErr.Error()),
		)
		return
	}
	if launched {
		mergeProfileRuntime(profile, launchedProfile)
		s.SetActiveProfile(profile)
	}

	writeJSON(w, http.StatusCreated, s.profileWriteSuccessPayload(profile, launchCode, true, false, launched))
	log.Info("Profile API 创建实例",
		logger.F("profile_id", profile.ProfileId),
		logger.F("profile_name", profile.ProfileName),
		logger.F("launch_code", launchCode),
		logger.F("auto_launch", launched),
		logger.F("duration_ms", time.Since(startAt).Milliseconds()),
	)
}

func (s *LaunchServer) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	items, status, errMsg := s.listProfiles()
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"count": len(items),
		"items": items,
	})
}

func (s *LaunchServer) handleGetProfile(w http.ResponseWriter, _ *http.Request, profileID string) {
	profile, status, errMsg := s.profileSnapshotByID(profileID)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"profileId":   profile.ProfileId,
		"profileName": profile.ProfileName,
		"launchCode":  profile.LaunchCode,
		"profile":     profile,
	})
}

func (s *LaunchServer) handleUpdateProfile(w http.ResponseWriter, r *http.Request, profileID string) {
	log := logger.New("LaunchServer")
	startAt := time.Now()

	previous, status, errMsg := s.profileSnapshotByID(profileID)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	req, status, errMsg := decodeProfileWriteRequest(r)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	input := normalizeProfileInput(*req.Profile)
	profile, launchCode, status, errMsg := s.updateProfile(profileID, input, resolveRequestedLaunchCode(req.LaunchCode, input.LaunchCode), previous)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}

	launchedProfile, launched, launchErr := s.maybeAutoLaunchProfile(profile, req)
	if launchErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":          false,
			"created":     false,
			"updated":     true,
			"launched":    false,
			"profileId":   profile.ProfileId,
			"profileName": profile.ProfileName,
			"launchCode":  launchCode,
			"profile":     profile,
			"error":       launchErr.Error(),
		})
		log.Warn("Profile API 更新后自动启动失败",
			logger.F("profile_id", profile.ProfileId),
			logger.F("profile_name", profile.ProfileName),
			logger.F("launch_code", launchCode),
			logger.F("duration_ms", time.Since(startAt).Milliseconds()),
			logger.F("error", launchErr.Error()),
		)
		return
	}
	if launched {
		mergeProfileRuntime(profile, launchedProfile)
		s.SetActiveProfile(profile)
	}

	writeJSON(w, http.StatusOK, s.profileWriteSuccessPayload(profile, launchCode, false, true, launched))
	log.Info("Profile API 更新实例",
		logger.F("profile_id", profile.ProfileId),
		logger.F("profile_name", profile.ProfileName),
		logger.F("launch_code", launchCode),
		logger.F("auto_launch", launched),
		logger.F("duration_ms", time.Since(startAt).Milliseconds()),
	)
}

func (s *LaunchServer) handleDeleteProfile(w http.ResponseWriter, _ *http.Request, profileID string) {
	profile, status, errMsg := s.profileSnapshotByID(profileID)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		return
	}
	if profile.Running {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"ok":    false,
			"error": "running profile cannot be deleted",
		})
		return
	}

	if err := s.deleteProfileInternal(profileID); err != nil {
		writeJSON(w, mapProfileWriteErrorStatus(err), map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	if s.service != nil {
		_ = s.service.Remove(profileID)
	}
	s.ClearActiveProfile(profileID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"deleted":     true,
		"profileId":   profileID,
		"profileName": profile.ProfileName,
		"launchCode":  profile.LaunchCode,
	})
}

func (s *LaunchServer) createProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, int, string) {
	profile, err := s.createProfileInternal(input)
	if err != nil {
		return nil, "", mapProfileWriteErrorStatus(err), err.Error()
	}
	if profile == nil {
		return nil, "", http.StatusInternalServerError, "profile creation returned nil profile"
	}

	launchCode, status, errMsg := s.applyRequestedLaunchCode(profile.ProfileId, strings.TrimSpace(profile.LaunchCode), requestedCode)
	if errMsg != "" {
		_ = s.deleteCreatedProfile(profile.ProfileId)
		return nil, "", status, errMsg
	}
	profile.LaunchCode = launchCode
	return profile, launchCode, http.StatusCreated, ""
}

func (s *LaunchServer) updateProfile(profileID string, input browser.ProfileInput, requestedCode string, previous *browser.Profile) (*browser.Profile, string, int, string) {
	profile, err := s.updateProfileInternal(profileID, input)
	if err != nil {
		return nil, "", mapProfileWriteErrorStatus(err), err.Error()
	}
	if profile == nil {
		return nil, "", http.StatusInternalServerError, "profile update returned nil profile"
	}

	currentCode := ""
	if previous != nil {
		currentCode = strings.TrimSpace(previous.LaunchCode)
	}
	launchCode, status, errMsg := s.applyRequestedLaunchCode(profile.ProfileId, currentCode, requestedCode)
	if errMsg != "" {
		if rollbackErr := s.rollbackProfileUpdate(profileID, previous); rollbackErr != nil {
			logger.New("LaunchServer").Warn("Profile API 更新回滚失败",
				logger.F("profile_id", profileID),
				logger.F("error", rollbackErr.Error()),
			)
		}
		return nil, "", status, errMsg
	}
	profile.LaunchCode = launchCode
	return profile, launchCode, http.StatusOK, ""
}

func (s *LaunchServer) maybeAutoLaunchProfile(profile *browser.Profile, req ProfileWriteRequest) (*browser.Profile, bool, error) {
	if profile == nil || !req.AutoLaunch {
		return nil, false, nil
	}

	params := LaunchRequestParams{}
	if req.Start != nil {
		params = LaunchRequestParams{
			LaunchArgs:           normalizeStringSlice(req.Start.LaunchArgs),
			StartURLs:            normalizeStringSlice(req.Start.StartURLs),
			SkipDefaultStartURLs: req.Start.SkipDefaultStartURLs,
		}
	}

	launchedProfile, err := s.launchProfile(profile.ProfileId, params)
	if err != nil {
		return nil, false, err
	}
	return launchedProfile, true, nil
}

func (s *LaunchServer) createProfileInternal(input browser.ProfileInput) (*browser.Profile, error) {
	if creator, ok := s.starter.(profileCreator); ok {
		return creator.CreateProfile(input)
	}
	if s.browserMgr != nil {
		return s.browserMgr.Create(input)
	}
	return nil, http.ErrNotSupported
}

func (s *LaunchServer) updateProfileInternal(profileID string, input browser.ProfileInput) (*browser.Profile, error) {
	if updater, ok := s.starter.(profileUpdater); ok {
		return updater.UpdateProfile(profileID, input)
	}
	if s.browserMgr != nil {
		return s.browserMgr.Update(profileID, input)
	}
	return nil, http.ErrNotSupported
}

func (s *LaunchServer) deleteCreatedProfile(profileID string) error {
	if deleter, ok := s.starter.(profileDeleter); ok {
		return deleter.DeleteProfile(profileID)
	}
	if s.browserMgr != nil {
		return s.browserMgr.Delete(profileID)
	}
	return nil
}

func (s *LaunchServer) deleteProfileInternal(profileID string) error {
	return s.deleteCreatedProfile(profileID)
}

func (s *LaunchServer) rollbackProfileUpdate(profileID string, previous *browser.Profile) error {
	if previous == nil {
		return nil
	}
	_, err := s.updateProfileInternal(profileID, profileToInput(previous))
	return err
}

func (s *LaunchServer) listProfiles() ([]browser.Profile, int, string) {
	if s.browserMgr == nil {
		return nil, http.StatusServiceUnavailable, "profile catalog is not available"
	}

	items := s.browserMgr.List()
	for i := range items {
		items[i].LaunchCode = s.resolveProfileLaunchCode(items[i].ProfileId, items[i].LaunchCode)
	}
	return items, http.StatusOK, ""
}

func (s *LaunchServer) profileSnapshotByID(profileID string) (*browser.Profile, int, string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, http.StatusNotFound, "profile not found"
	}
	if s.browserMgr == nil {
		return nil, http.StatusServiceUnavailable, "profile catalog is not available"
	}

	s.browserMgr.Mutex.Lock()
	profile, ok := s.browserMgr.Profiles[profileID]
	var snapshot browser.Profile
	if ok && profile != nil {
		snapshot = *profile
	}
	s.browserMgr.Mutex.Unlock()
	if !ok {
		return nil, http.StatusNotFound, "profile not found"
	}

	snapshot.LaunchCode = s.resolveProfileLaunchCode(snapshot.ProfileId, snapshot.LaunchCode)
	return &snapshot, http.StatusOK, ""
}

func (s *LaunchServer) applyRequestedLaunchCode(profileID, currentCode, requestedCode string) (string, int, string) {
	currentCode = strings.TrimSpace(currentCode)
	requestedCode = strings.TrimSpace(requestedCode)
	if requestedCode == "" {
		return s.resolveProfileLaunchCode(profileID, currentCode), http.StatusOK, ""
	}
	if s.service == nil {
		return "", http.StatusServiceUnavailable, "launch code service is unavailable"
	}

	code, err := s.service.SetCode(profileID, requestedCode)
	if err != nil {
		return "", mapProfileWriteErrorStatus(err), err.Error()
	}
	return code, http.StatusOK, ""
}

func (s *LaunchServer) resolveProfileLaunchCode(profileID, currentCode string) string {
	if trimmed := strings.TrimSpace(currentCode); trimmed != "" {
		return trimmed
	}
	if s.service == nil || strings.TrimSpace(profileID) == "" {
		return ""
	}
	code, err := s.service.EnsureCode(profileID)
	if err != nil {
		return ""
	}
	return code
}

func (s *LaunchServer) profileWriteSuccessPayload(profile *browser.Profile, launchCode string, created bool, updated bool, launched bool) map[string]interface{} {
	payload := map[string]interface{}{
		"ok":          true,
		"created":     created,
		"updated":     updated,
		"launched":    launched,
		"profileId":   profile.ProfileId,
		"profileName": profile.ProfileName,
		"launchCode":  launchCode,
		"profile":     profile,
	}

	if !launched {
		return payload
	}

	for key, value := range s.launchSuccessPayload(profile, launchCode) {
		payload[key] = value
	}
	payload["created"] = created
	payload["updated"] = updated
	payload["launched"] = true
	payload["profile"] = profile
	return payload
}

func decodeProfileWriteRequest(r *http.Request) (ProfileWriteRequest, int, string) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return ProfileWriteRequest{}, http.StatusMethodNotAllowed, "method not allowed"
	}

	var req ProfileWriteRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return ProfileWriteRequest{}, http.StatusBadRequest, "invalid request body"
	}
	if req.Profile == nil {
		return ProfileWriteRequest{}, http.StatusBadRequest, "profile is required"
	}
	return req, http.StatusOK, ""
}

func normalizeProfileInput(input browser.ProfileInput) browser.ProfileInput {
	return browser.ProfileInput{
		ProfileName:     strings.TrimSpace(input.ProfileName),
		UserDataDir:     sanitizeAPIUserDataDir(input.UserDataDir),
		CoreId:          strings.TrimSpace(input.CoreId),
		FingerprintArgs: normalizeStringSlice(input.FingerprintArgs),
		ProxyId:         strings.TrimSpace(input.ProxyId),
		ProxyConfig:     strings.TrimSpace(input.ProxyConfig),
		LaunchArgs:      normalizeStringSlice(input.LaunchArgs),
		Tags:            normalizeStringSlice(input.Tags),
		Keywords:        normalizeStringSlice(input.Keywords),
		GroupId:         strings.TrimSpace(input.GroupId),
		LaunchCode:      strings.TrimSpace(input.LaunchCode),
		AccountIds:      normalizeStringSlice(input.AccountIds),
	}
}

// sanitizeAPIUserDataDir 约束通过 HTTP API 传入的 UserDataDir。
//
// 该接口是未鉴权的外部攻击面（profile 写入），若放任调用方指定绝对路径或包含
// ".." 的相对路径，可将浏览器用户数据目录写入文件系统任意位置（如覆盖系统文件
// 或越权读写）。这里只允许「单层、无分隔符」的相对目录名，其余一律丢弃并回退到
// 由 ProfileId 派生的受管默认目录（UserDataDir 为空时的既有行为）。
//
// 注意：本地可信的 Wails UI 走 App.CreateProfile，不经过本函数，因此用户在桌面端
// 仍可使用绝对路径自定义数据目录。
func sanitizeAPIUserDataDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	// 绝对路径（含 Windows 盘符 / UNC）一律拒绝。
	if filepath.IsAbs(dir) || strings.HasPrefix(dir, `\\`) || strings.Contains(dir, ":") {
		return ""
	}
	// 归一化后若逃逸出单层目录（含 ".." 或路径分隔符），一律拒绝。
	cleaned := filepath.ToSlash(filepath.Clean(dir))
	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") ||
		strings.Contains(cleaned, "/") {
		return ""
	}
	return cleaned
}

func profileToInput(profile *browser.Profile) browser.ProfileInput {
	if profile == nil {
		return browser.ProfileInput{}
	}
	return browser.ProfileInput{
		ProfileName:     strings.TrimSpace(profile.ProfileName),
		UserDataDir:     strings.TrimSpace(profile.UserDataDir),
		CoreId:          strings.TrimSpace(profile.CoreId),
		FingerprintArgs: append([]string{}, profile.FingerprintArgs...),
		ProxyId:         strings.TrimSpace(profile.ProxyId),
		ProxyConfig:     strings.TrimSpace(profile.ProxyConfig),
		LaunchArgs:      append([]string{}, profile.LaunchArgs...),
		Tags:            append([]string{}, profile.Tags...),
		Keywords:        append([]string{}, profile.Keywords...),
		GroupId:         strings.TrimSpace(profile.GroupId),
		LaunchCode:      strings.TrimSpace(profile.LaunchCode),
		AccountIds:      nil, // 回滚时不恢复账号关联
	}
}

func resolveRequestedLaunchCode(topLevelCode, profileCode string) string {
	if strings.TrimSpace(topLevelCode) != "" {
		return topLevelCode
	}
	return profileCode
}

func mergeProfileRuntime(target, runtimeProfile *browser.Profile) {
	if target == nil || runtimeProfile == nil {
		return
	}
	target.Running = runtimeProfile.Running
	target.Status = runtimeProfile.Status
	target.DebugPort = runtimeProfile.DebugPort
	target.DebugReady = runtimeProfile.DebugReady
	target.Pid = runtimeProfile.Pid
	target.RuntimeWarning = runtimeProfile.RuntimeWarning
	target.LastError = runtimeProfile.LastError
	target.LastStartAt = runtimeProfile.LastStartAt
	target.LastStopAt = runtimeProfile.LastStopAt
}

func parseProfilePathID(path string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/profiles/")
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}

func mapProfileWriteErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case msg == strings.ToLower(strings.TrimSpace(http.ErrNotSupported.Error())):
		return http.StatusServiceUnavailable
	case strings.Contains(msg, "profile not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "running profile cannot be deleted"):
		return http.StatusConflict
	case strings.Contains(msg, "launch code already exists"):
		return http.StatusConflict
	case strings.Contains(msg, "launch code format invalid"),
		strings.Contains(msg, "launch code must be"):
		return http.StatusBadRequest
	case strings.Contains(msg, "实例数量已达上限"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
