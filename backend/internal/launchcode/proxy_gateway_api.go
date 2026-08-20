package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"ant-chrome/backend/internal/gateway"
)

type proxyGatewayRequest struct {
	Code        string                 `json:"code"`
	Key         string                 `json:"key"`
	ProfileID   string                 `json:"profileId"`
	ProfileName string                 `json:"profileName"`
	Keyword     string                 `json:"keyword"`
	Keywords    []string               `json:"keywords"`
	Tag         string                 `json:"tag"`
	Tags        []string               `json:"tags"`
	GroupID     string                 `json:"groupId"`
	MatchMode   string                 `json:"matchMode"`
	Selector    *LaunchSelector        `json:"selector"`
	ProxyID     string                 `json:"proxyId"`
	ProxyConfig string                 `json:"proxyConfig"`
	Routing     *gateway.RoutingConfig `json:"routing"`
	Force       bool                   `json:"force"`
}

func (s *LaunchServer) handleProxyGateway(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/proxy-gateway/switch":
		if r.Method == http.MethodPost {
			s.handleProxyGatewaySwitchOrStatus(w, r)
			return
		}
		writeProxyGatewayError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	case "/api/proxy-gateway/status":
		switch r.Method {
		case http.MethodGet:
			s.handleProxyGatewayStatusQuery(w, r)
		case http.MethodPost:
			s.handleProxyGatewaySwitchOrStatus(w, r)
		default:
			writeProxyGatewayError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	default:
		writeProxyGatewayError(w, http.StatusNotFound, "proxy gateway endpoint not found")
		return
	}
}

func (s *LaunchServer) handleProxyGatewayRouting(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleProxyGatewayRoutingQuery(w, r)
	case http.MethodPut:
		s.handleProxyGatewayRoutingSave(w, r)
	default:
		writeProxyGatewayError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *LaunchServer) handleProxyGatewaySwitchOrStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxy-gateway/switch" && r.URL.Path != "/api/proxy-gateway/status" {
		writeProxyGatewayError(w, http.StatusNotFound, "proxy gateway endpoint not found")
		return
	}
	var req proxyGatewayRequest
	if err := decodeProxyGatewayRequest(r, &req); err != nil {
		writeProxyGatewayError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if r.URL.Path == "/api/proxy-gateway/status" {
		s.writeProxyGatewayStatus(w, req)
		return
	}
	s.writeProxyGatewaySwitch(w, req)
}

func (s *LaunchServer) handleProxyGatewayStatusQuery(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxy-gateway/status" {
		writeProxyGatewayError(w, http.StatusNotFound, "proxy gateway endpoint not found")
		return
	}
	s.writeProxyGatewayStatus(w, proxyGatewayRequestFromQuery(r.URL.Query()))
}

func (s *LaunchServer) handleProxyGatewayRoutingQuery(w http.ResponseWriter, r *http.Request) {
	s.writeProxyGatewayRouting(w, proxyGatewayRequestFromQuery(r.URL.Query()))
}

func (s *LaunchServer) handleProxyGatewayRoutingSave(w http.ResponseWriter, r *http.Request) {
	var req proxyGatewayRequest
	if err := decodeProxyGatewayRequest(r, &req); err != nil {
		writeProxyGatewayError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Routing == nil {
		writeProxyGatewayError(w, http.StatusBadRequest, "routing is required")
		return
	}
	if err := gateway.ValidateRoutingConfig(*req.Routing); err != nil {
		writeProxyGatewayError(w, http.StatusBadRequest, err.Error())
		return
	}
	controller := s.proxyGatewayController()
	if controller == nil {
		writeProxyGatewayError(w, http.StatusServiceUnavailable, "proxy gateway api is unavailable")
		return
	}
	profileID, selector, status, errMsg := s.resolveProxyGatewayProfile(req)
	if errMsg != "" {
		writeProxyGatewayError(w, status, errMsg)
		return
	}
	result, err := controller.SaveRouting(profileID, gateway.NormalizeRoutingConfig(*req.Routing), req.Force)
	if err != nil {
		writeProxyGatewayControllerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"selector":  selector,
		"routing":   gateway.NormalizeRoutingConfig(*req.Routing),
		"gateway":   result,
		"force":     req.Force,
	})
}

func (s *LaunchServer) writeProxyGatewayStatus(w http.ResponseWriter, req proxyGatewayRequest) {
	controller := s.proxyGatewayController()
	if controller == nil {
		writeProxyGatewayError(w, http.StatusServiceUnavailable, "proxy gateway api is unavailable")
		return
	}
	profileID, selector, status, errMsg := s.resolveProxyGatewayProfile(req)
	if errMsg != "" {
		writeProxyGatewayError(w, status, errMsg)
		return
	}
	result, err := controller.Status(profileID)
	if err != nil {
		writeProxyGatewayControllerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"selector":  selector,
		"gateway":   result,
	})
}

func (s *LaunchServer) writeProxyGatewaySwitch(w http.ResponseWriter, req proxyGatewayRequest) {
	controller := s.proxyGatewayController()
	if controller == nil {
		writeProxyGatewayError(w, http.StatusServiceUnavailable, "proxy gateway api is unavailable")
		return
	}
	profileID, selector, status, errMsg := s.resolveProxyGatewayProfile(req)
	if errMsg != "" {
		writeProxyGatewayError(w, status, errMsg)
		return
	}
	result, err := controller.SwitchProxy(profileID, strings.TrimSpace(req.ProxyID), strings.TrimSpace(req.ProxyConfig), req.Force)
	if err != nil {
		writeProxyGatewayControllerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"profileId":   profileID,
		"selector":    selector,
		"gateway":     result.Gateway,
		"appliedLive": result.AppliedLive,
		"force":       req.Force,
	})
}

func (s *LaunchServer) writeProxyGatewayRouting(w http.ResponseWriter, req proxyGatewayRequest) {
	controller := s.proxyGatewayController()
	if controller == nil {
		writeProxyGatewayError(w, http.StatusServiceUnavailable, "proxy gateway api is unavailable")
		return
	}
	profileID, selector, status, errMsg := s.resolveProxyGatewayProfile(req)
	if errMsg != "" {
		writeProxyGatewayError(w, status, errMsg)
		return
	}
	routing, err := controller.GetRouting(profileID)
	if err != nil {
		writeProxyGatewayControllerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"selector":  selector,
		"routing":   gateway.NormalizeRoutingConfig(routing),
	})
}

func (s *LaunchServer) resolveProxyGatewayProfile(req proxyGatewayRequest) (string, LaunchSelector, int, string) {
	selector := mergeProxyGatewaySelector(req)
	if selector.IsEmpty() {
		return "", selector, http.StatusBadRequest, "selector is required"
	}
	if err := validateRuntimeSelector(selector); err != nil {
		return "", selector, http.StatusBadRequest, err.Error()
	}
	target, status, errMsg := s.resolveRuntimeTarget(selector)
	if errMsg != "" {
		return "", selector, status, errMsg
	}
	return target.ProfileID, selector, http.StatusOK, ""
}

func mergeProxyGatewaySelector(req proxyGatewayRequest) LaunchSelector {
	var nested LaunchSelector
	if req.Selector != nil {
		nested = *req.Selector
	}
	return normalizeRuntimeSelector(buildMergedSelector(selectorMergeInput{
		Code:        firstNonEmpty(nested.Code, req.Code),
		Key:         firstNonEmpty(nested.Key, req.Key),
		ProfileID:   firstNonEmpty(nested.ProfileID, req.ProfileID),
		ProfileName: firstNonEmpty(nested.ProfileName, req.ProfileName),
		Keywords:    appendSelectorTerms(nil, "", nested.Keywords, nested.Keyword, req.Keyword, req.Keywords),
		Tags:        appendSelectorTerms(nil, nested.Tag, nested.Tags, req.Tag, req.Tags),
		GroupID:     firstNonEmpty(nested.GroupID, req.GroupID),
		MatchMode:   firstNonEmpty(nested.MatchMode, req.MatchMode),
	}))
}

func proxyGatewayRequestFromQuery(values url.Values) proxyGatewayRequest {
	return proxyGatewayRequest{
		Code:        values.Get("code"),
		Key:         values.Get("key"),
		ProfileID:   values.Get("profileId"),
		ProfileName: values.Get("profileName"),
		Keyword:     values.Get("keyword"),
		Keywords:    append([]string(nil), values["keyword"]...),
		Tag:         values.Get("tag"),
		Tags:        append([]string(nil), values["tag"]...),
		GroupID:     values.Get("groupId"),
		MatchMode:   values.Get("matchMode"),
	}
}

func decodeProxyGatewayRequest(r *http.Request, target *proxyGatewayRequest) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

func mapProxyGatewayErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	status := mapProfileWriteErrorStatus(err)
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "invalid proxy configuration") || strings.Contains(message, "proxy id not found") {
		return http.StatusBadRequest
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusConflict {
		return status
	}
	if strings.Contains(message, "not found") || strings.Contains(message, "not running") {
		return http.StatusNotFound
	}
	return http.StatusBadGateway
}

func writeProxyGatewayError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{"ok": false, "error": strings.TrimSpace(message)})
}

func writeProxyGatewayControllerError(w http.ResponseWriter, err error) {
	status := mapProxyGatewayErrorStatus(err)
	message := "proxy gateway operation failed"
	switch status {
	case http.StatusBadRequest:
		message = "invalid proxy gateway request"
	case http.StatusNotFound:
		message = "profile or proxy gateway not found"
	case http.StatusConflict:
		message = "proxy gateway target is not unique"
	case http.StatusServiceUnavailable:
		message = "proxy gateway api is unavailable"
	}
	writeProxyGatewayError(w, status, message)
}
