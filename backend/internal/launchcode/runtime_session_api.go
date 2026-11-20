package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"ant-chrome/backend/internal/browser"
)

const (
	defaultRuntimeSessionTimeout = 45 * time.Second
	minRuntimeSessionTimeout     = 1 * time.Second
	maxRuntimeSessionTimeout     = 2 * time.Minute
)

type RuntimeSessionRequest struct {
	RuntimeRequest
	LaunchRequestParams
	TimeoutMs int `json:"timeoutMs"`
}

func decodeRuntimeSessionRequest(r *http.Request) (RuntimeSessionRequest, int, string) {
	if r.Method != http.MethodPost {
		return RuntimeSessionRequest{}, http.StatusMethodNotAllowed, "method not allowed"
	}

	var req RuntimeSessionRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return RuntimeSessionRequest{}, http.StatusBadRequest, "invalid request body"
	}
	return req, http.StatusOK, ""
}

func normalizeRuntimeSessionTimeout(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return defaultRuntimeSessionTimeout
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout < minRuntimeSessionTimeout {
		return minRuntimeSessionTimeout
	}
	if timeout > maxRuntimeSessionTimeout {
		return maxRuntimeSessionTimeout
	}
	return timeout
}

func (s *LaunchServer) handleRuntimeSession(w http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	clientIP := remoteIP(r.RemoteAddr)
	selector := LaunchSelector{}

	req, status, errMsg := decodeRuntimeSessionRequest(r)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, status, errMsg, "", "", startAt)
		return
	}

	selector = mergeRuntimeSelector(req.RuntimeRequest)
	if selector.IsEmpty() {
		errMsg = "selector is required"
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, req.LaunchRequestParams, false, http.StatusBadRequest, errMsg, "", "", startAt)
		return
	}
	if err := validateRuntimeSelector(selector); err != nil {
		errMsg = err.Error()
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, selector.Code, selector, req.LaunchRequestParams, false, http.StatusBadRequest, errMsg, "", "", startAt)
		return
	}

	req.LaunchRequestParams = normalizeLaunchRequestParams(req.LaunchRequestParams)
	profile, launchCode, status, errMsg := s.launchBySelector(selector, req.LaunchRequestParams)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, false, status, errMsg, "", "", startAt)
		return
	}

	waitTimeout := normalizeRuntimeSessionTimeout(req.TimeoutMs)
	profile, ready, err := s.prepareRuntimeSession(profile, waitTimeout)
	if err != nil {
		writeJSON(w, mapProfileWriteErrorStatus(err), map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, false, mapProfileWriteErrorStatus(err), err.Error(), "", "", startAt)
		return
	}
	if profile == nil {
		errMsg = "runtime session is not available"
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, false, http.StatusServiceUnavailable, errMsg, "", "", startAt)
		return
	}
	if launchCode != "" && profile.LaunchCode == "" {
		profile.LaunchCode = launchCode
	}

	responseStatus := http.StatusAccepted
	if ready {
		responseStatus = http.StatusOK
	}
	payload := s.runtimeSessionPayload(profile, waitTimeout, ready)
	writeJSON(w, responseStatus, payload)
	s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, ready, responseStatus, "", profile.ProfileId, profile.ProfileName, startAt)
}

func (s *LaunchServer) prepareRuntimeSession(profile *browser.Profile, timeout time.Duration) (*browser.Profile, bool, error) {
	return s.waitRuntimeDebugReady(profile, timeout, s.SetActiveProfile)
}

// waitRuntimeDebugReady 等实例调试端口就绪，最多等 timeout。
// onReady 非空时在确认就绪的那一刻回调（runtime session 用它切换统一入口的活动实例）。
func (s *LaunchServer) waitRuntimeDebugReady(profile *browser.Profile, timeout time.Duration, onReady func(*browser.Profile)) (*browser.Profile, bool, error) {
	if onReady == nil {
		onReady = func(*browser.Profile) {}
	}
	normalized := s.normalizeRuntimeProfile(profile)
	if normalized == nil {
		return nil, false, nil
	}
	if normalized.DebugReady {
		onReady(normalized)
		return normalized, true, nil
	}
	if timeout <= 0 {
		return normalized, false, nil
	}

	if waiter, ok := s.starter.(BrowserDebugWaiter); ok {
		waited, ready, err := waiter.WaitInstanceDebugReady(normalized.ProfileId, normalized.DebugPort, timeout)
		if err != nil {
			return nil, false, err
		}
		if waited != nil {
			normalized = s.normalizeRuntimeProfile(waited)
		}
		if normalized != nil && ready && normalized.DebugReady {
			onReady(normalized)
			return normalized, true, nil
		}
		return normalized, normalized != nil && normalized.DebugReady, nil
	}

	deadline := time.Now().Add(timeout)
	for {
		snapshot, _, errMsg := s.statusProfile(normalized.ProfileId)
		if errMsg != "" {
			return nil, false, runtimeRequestError(errMsg)
		}
		if snapshot != nil {
			normalized = snapshot
		}
		if normalized != nil && normalized.DebugReady {
			onReady(normalized)
			return normalized, true, nil
		}
		if time.Now().After(deadline) {
			return normalized, false, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (s *LaunchServer) runtimeSessionPayload(profile *browser.Profile, timeout time.Duration, ready bool) map[string]interface{} {
	payload := s.profileRuntimePayload(profile)
	payload["ready"] = ready
	payload["timeoutMs"] = timeout.Milliseconds()
	payload["waitTimedOut"] = !ready
	payload["retryable"] = !ready
	return payload
}
