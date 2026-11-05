package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/automation"
)

const (
	automationMinTimeoutMs = 1000
	automationMaxTimeoutMs = 30 * 60 * 1000
)

type automationScriptRunAPIRequest struct {
	ScriptID          string          `json:"scriptId"`
	Selector          json.RawMessage `json:"selector"`
	TargetInput       json.RawMessage `json:"targetInput"`
	Params            json.RawMessage `json:"params"`
	UseScriptSelector *bool           `json:"useScriptSelector"`
	UseScriptParams   *bool           `json:"useScriptParams"`
	TimeoutMs         int             `json:"timeoutMs"`
}

type automationScriptSummary struct {
	ID           string                           `json:"id"`
	Name         string                           `json:"name"`
	Description  string                           `json:"description"`
	Type         string                           `json:"type"`
	Status       string                           `json:"status"`
	EntryFile    string                           `json:"entryFile"`
	Tags         []string                         `json:"tags"`
	Selector     map[string]interface{}           `json:"selector"`
	Params       map[string]interface{}           `json:"params"`
	Notes        string                           `json:"notes"`
	TargetConfig automation.ScriptTargetConfig    `json:"targetConfig"`
	PublicAPI    automation.ScriptPublicAPIConfig `json:"publicAPI"`
	CreatedAt    string                           `json:"createdAt"`
	UpdatedAt    string                           `json:"updatedAt"`
}

type automationScriptDetail struct {
	automationScriptSummary
	PackageFormat   string                  `json:"packageFormat"`
	ManifestVersion int                     `json:"manifestVersion"`
	Source          automation.ScriptSource `json:"source"`
}

func (s *LaunchServer) handleAutomationScripts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	lister, ok := s.starter.(AutomationScriptLister)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	items, err := lister.AutomationScriptList()
	if err != nil {
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	result := make([]automationScriptSummary, 0, len(items))
	for _, item := range items {
		result = append(result, summarizeAutomationScript(item))
	}

	writeAutomationAPISuccess(w, http.StatusOK, "", automationAPIListData[automationScriptSummary]{
		Items: result,
		Count: len(result),
	})
}

func (s *LaunchServer) handleAutomationScriptByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	scriptID, ok := parseAutomationScriptPathID(r.URL.Path)
	if !ok {
		writeAutomationAPIError(w, http.StatusNotFound, "not_found", "script not found", "")
		return
	}

	getter, ok := s.starter.(AutomationScriptGetter)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	item, err := getter.AutomationScriptGet(scriptID)
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if os.IsNotExist(err) {
			writeAutomationAPIError(w, http.StatusNotFound, "not_found", "script not found", "")
			return
		}
		if strings.Contains(strings.ToLower(message), "script id is invalid") || strings.Contains(strings.ToLower(message), "script id is required") {
			writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", message, "scriptId")
			return
		}
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", message, "")
		return
	}

	writeAutomationAPISuccess(w, http.StatusOK, "", automationAPIItemData[automationScriptDetail]{
		Item: detailAutomationScript(*item),
	})
}

func (s *LaunchServer) handleAutomationScriptRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	runner, ok := s.starter.(AutomationScriptRunner)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	var req automationScriptRunAPIRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body", "")
		return
	}

	input, err := normalizeAutomationRunRequest(req)
	if err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), automationRequestErrorField(err))
		return
	}

	run, err := runner.AutomationScriptRunWithOptions(input)
	if err != nil {
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	data := automationAPIRunData{
		Run:     run,
		Summary: run.Summary,
	}
	if result := decodeAutomationRunResult(run.ResultText); result != nil {
		data.Result = result
	}
	writeAutomationAPISuccess(w, http.StatusOK, run.Summary, data)
}

func (s *LaunchServer) handleAutomationScriptRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	lister, ok := s.starter.(AutomationScriptRunLister)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}

	items, err := lister.AutomationScriptRunList(limit)
	if err != nil {
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	writeAutomationAPISuccess(w, http.StatusOK, "", automationAPIListData[automation.ScriptRunRecord]{
		Items: items,
		Count: len(items),
		Limit: limit,
	})
}

func summarizeAutomationScript(record automation.ScriptRecord) automationScriptSummary {
	return automationScriptSummary{
		ID:           strings.TrimSpace(record.ID),
		Name:         strings.TrimSpace(record.Name),
		Description:  strings.TrimSpace(record.Description),
		Type:         strings.TrimSpace(record.Type),
		Status:       strings.TrimSpace(record.Status),
		EntryFile:    strings.TrimSpace(record.EntryFile),
		Tags:         append([]string(nil), record.Tags...),
		Selector:     parseJSONObjectText(record.SelectorText),
		Params:       parseJSONObjectText(record.ParamsText),
		Notes:        strings.TrimSpace(record.Notes),
		TargetConfig: record.TargetConfig,
		PublicAPI:    record.PublicAPI,
		CreatedAt:    strings.TrimSpace(record.CreatedAt),
		UpdatedAt:    strings.TrimSpace(record.UpdatedAt),
	}
}

func detailAutomationScript(record automation.ScriptRecord) automationScriptDetail {
	return automationScriptDetail{
		automationScriptSummary: summarizeAutomationScript(record),
		PackageFormat:           strings.TrimSpace(record.PackageFormat),
		ManifestVersion:         record.ManifestVersion,
		Source:                  record.Source,
	}
}

func parseAutomationScriptPathID(path string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/automation/scripts/")
	path = strings.Trim(path, "/")
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}

func normalizeAutomationRunRequest(req automationScriptRunAPIRequest) (automation.ScriptRunRequest, error) {
	scriptID := strings.TrimSpace(req.ScriptID)
	if scriptID == "" {
		return automation.ScriptRunRequest{}, badAutomationRequest("scriptId is required")
	}

	selector, hasSelector, err := decodeJSONObjectRaw(req.Selector, "selector")
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	targetInput, hasTargetInput, err := decodeJSONObjectRaw(req.TargetInput, "targetInput")
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	params, hasParams, err := decodeJSONObjectRaw(req.Params, "params")
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}

	useScriptSelector, err := resolveUseScriptField("selector", req.UseScriptSelector, hasSelector)
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	useScriptParams, err := resolveUseScriptField("params", req.UseScriptParams, hasParams)
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	if err := validateAutomationTimeoutMs(req.TimeoutMs); err != nil {
		return automation.ScriptRunRequest{}, err
	}

	selectorText := ""
	if !useScriptSelector {
		encodedSelector, err := json.Marshal(selector)
		if err != nil {
			return automation.ScriptRunRequest{}, badAutomationRequest("selector must be a JSON object")
		}
		selectorText = string(encodedSelector)
	}

	paramsText := ""
	if !useScriptParams {
		encodedParams, err := json.Marshal(params)
		if err != nil {
			return automation.ScriptRunRequest{}, badAutomationRequest("params must be a JSON object")
		}
		paramsText = string(encodedParams)
	}

	return automation.ScriptRunRequest{
		ScriptID:          scriptID,
		SelectorText:      selectorText,
		TargetInput:       targetInput,
		ParamsText:        paramsText,
		UseScriptSelector: useScriptSelector && !hasTargetInput,
		UseScriptParams:   useScriptParams,
		TimeoutMs:         req.TimeoutMs,
	}, nil
}

func validateAutomationTimeoutMs(timeoutMs int) error {
	if timeoutMs == 0 {
		return nil
	}
	if timeoutMs < automationMinTimeoutMs || timeoutMs > automationMaxTimeoutMs {
		return badAutomationRequest("timeoutMs must be between 1000 and 1800000")
	}
	return nil
}

func resolveUseScriptField(name string, explicit *bool, hasObject bool) (bool, error) {
	if explicit == nil {
		return !hasObject, nil
	}
	if *explicit && hasObject {
		return false, badAutomationRequest(name + " conflicts with useScript" + upperFirst(name) + "=true")
	}
	if !*explicit && !hasObject {
		return false, badAutomationRequest(name + " is required when useScript" + upperFirst(name) + "=false")
	}
	return *explicit, nil
}

func decodeJSONObjectRaw(raw json.RawMessage, fieldName string) (map[string]interface{}, bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, false, nil
	}

	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}
	return obj, true, nil
}

func decodeAutomationRunResult(raw string) interface{} {
	_, result, ok := decodeAutomationRunPayloadValue(raw)
	if !ok {
		return nil
	}
	return result
}

func parseJSONObjectText(text string) map[string]interface{} {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	var value interface{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return obj
}

func upperFirst(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func badAutomationRequest(message string) error {
	return automationRequestError(strings.TrimSpace(message))
}

func automationRequestErrorField(err error) string {
	message := strings.TrimSpace(err.Error())
	for _, field := range []string{"scriptId", "selector", "targetInput", "params"} {
		if strings.Contains(message, field) {
			return field
		}
	}
	return ""
}

type automationRequestError string

func (e automationRequestError) Error() string {
	return strings.TrimSpace(string(e))
}
