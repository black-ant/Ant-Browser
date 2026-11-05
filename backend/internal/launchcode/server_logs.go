package launchcode

import (
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

func (s *LaunchServer) appendLaunchLog(method, path, clientIP, code string, selector LaunchSelector, params LaunchRequestParams, ok bool, status int, errMsg, profileID, profileName string, startAt time.Time) {
	entry := LaunchCallRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		Method:      method,
		Path:        path,
		ClientIP:    clientIP,
		Code:        strings.TrimSpace(code),
		Selector:    selector,
		ProfileID:   profileID,
		ProfileName: profileName,
		Params:      params,
		OK:          ok,
		Status:      status,
		Error:       errMsg,
		DurationMs:  time.Since(startAt).Milliseconds(),
	}

	s.logMu.Lock()
	s.callLogs = append(s.callLogs, entry)
	if len(s.callLogs) > 500 {
		s.callLogs = append([]LaunchCallRecord(nil), s.callLogs[len(s.callLogs)-500:]...)
	}
	s.logMu.Unlock()

	log := logger.New("LaunchServer")
	if ok {
		log.Debug("Launch API 调用", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("profile_id", profileID), logger.F("status", status), logger.F("duration_ms", entry.DurationMs))
		return
	}
	log.Warn("Launch API 调用失败", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("status", status), logger.F("error", errMsg), logger.F("duration_ms", entry.DurationMs))
}

func (s *LaunchServer) listLaunchLogs(limit int) []LaunchCallRecord {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > len(s.callLogs) {
		limit = len(s.callLogs)
	}
	if limit == 0 {
		return []LaunchCallRecord{}
	}

	out := make([]LaunchCallRecord, 0, limit)
	for i := len(s.callLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.callLogs[i])
	}
	return out
}
