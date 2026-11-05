package launchcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"ant-chrome/backend/internal/automation"
)

const automationPublicHookRoutePrefix = "/api/automation/hooks/"

func (s *LaunchServer) handleAutomationPublicHook(w http.ResponseWriter, r *http.Request) {
	hookPath, ok := parseAutomationPublicHookPath(r.URL.Path)
	if !ok {
		writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
		return
	}

	record, err := s.findAutomationPublicHookScript(hookPath)
	if err != nil {
		if err == errAutomationHookServiceUnavailable {
			writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
			return
		}
		if os.IsNotExist(err) {
			writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
			return
		}
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	if !record.PublicAPI.Enabled {
		writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
		return
	}

	if r.Method != record.PublicAPI.Method {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	runner, ok := s.starter.(AutomationScriptRunner)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body", "")
		return
	}

	input, err := buildAutomationPublicHookRunRequest(*record, r, body)
	if err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), automationRequestErrorField(err))
		return
	}

	run, err := runner.AutomationScriptRunWithOptions(input)
	if err != nil {
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	writeAutomationPublicHookResponse(w, *record, run)
}

var errAutomationHookServiceUnavailable = automationRequestError("automation hook service unavailable")

func (s *LaunchServer) findAutomationPublicHookScript(hookPath string) (*automation.ScriptRecord, error) {
	lister, ok := s.starter.(AutomationScriptLister)
	if !ok {
		return nil, errAutomationHookServiceUnavailable
	}

	items, err := lister.AutomationScriptList()
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if normalizeAutomationPublicHookPath(item.PublicAPI.Path) != hookPath {
			continue
		}
		record := item
		return &record, nil
	}

	return nil, os.ErrNotExist
}

func parseAutomationPublicHookPath(urlPath string) (string, bool) {
	trimmed := strings.TrimSpace(urlPath)
	if !strings.HasPrefix(trimmed, automationPublicHookRoutePrefix) {
		return "", false
	}

	trimmed = strings.TrimPrefix(trimmed, automationPublicHookRoutePrefix)
	trimmed = normalizeAutomationPublicHookPath(trimmed)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func normalizeAutomationPublicHookPath(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}

	cleaned := strings.Trim(path.Clean("/"+value), "/")
	if cleaned == "" || cleaned == "." {
		return ""
	}
	return strings.ToLower(cleaned)
}

func buildAutomationPublicHookRunRequest(record automation.ScriptRecord, r *http.Request, body []byte) (automation.ScriptRunRequest, error) {
	if shouldApplyAutomationPublicHookVariables(record) {
		resolvedBody, err := resolveAutomationPublicHookRequestBody(record, body)
		if err != nil {
			return automation.ScriptRunRequest{}, err
		}
		body = resolvedBody
	}

	input, err := decodeAutomationPublicHookRequestBody(body)
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	targetMode, targetInput, selectorText, useScriptSelector, err := resolveAutomationPublicHookInstance(input)
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	if err := validateAutomationTimeoutMs(input.TimeoutMs); err != nil {
		return automation.ScriptRunRequest{}, err
	}
	params := mergeAutomationPublicHookDefaultParamsObject(record, input.Params)
	paramsText, err := encodeAutomationPublicHookJSONObject(params)
	if err != nil {
		return automation.ScriptRunRequest{}, badAutomationRequest("params must be a JSON object")
	}

	return automation.ScriptRunRequest{
		ScriptID:          record.ID,
		SelectorText:      selectorText,
		TargetMode:        targetMode,
		TargetInput:       targetInput,
		ParamsText:        paramsText,
		UseScriptSelector: useScriptSelector,
		UseScriptParams:   false,
		TimeoutMs:         resolveAutomationPublicHookTimeout(r, input.TimeoutMs, record.PublicAPI.TimeoutMs),
	}, nil
}

type automationPublicHookRequestBody struct {
	Code      string                        `json:"code"`
	Instance  *automationPublicHookInstance `json:"instance"`
	Params    map[string]interface{}        `json:"params"`
	TimeoutMs int                           `json:"timeoutMs"`
}

type automationPublicHookInstance struct {
	Type               string                          `json:"type"`
	Selector           automation.ScriptTargetSelector `json:"selector"`
	TemplateSelector   automation.ScriptTargetSelector `json:"templateSelector"`
	CreateNameTemplate string                          `json:"createNameTemplate"`
	ProfileName        string                          `json:"profileName"`
}

func decodeAutomationPublicHookRequestBody(body []byte) (automationPublicHookRequestBody, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return automationPublicHookRequestBody{}, nil
	}

	var input automationPublicHookRequestBody
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return automationPublicHookRequestBody{}, badAutomationRequest("invalid request body")
	}
	if input.Params == nil {
		input.Params = map[string]interface{}{}
	}
	return input, nil
}

func resolveAutomationPublicHookInstance(input automationPublicHookRequestBody) (string, any, string, bool, error) {
	legacyCode := strings.TrimSpace(input.Code)
	if input.Instance == nil {
		if legacyCode == "" {
			return "", nil, "", true, nil
		}
		encodedSelectorText, err := encodeAutomationPublicHookJSONObject(map[string]interface{}{"code": legacyCode})
		if err != nil {
			return "", nil, "", true, badAutomationRequest("code is invalid")
		}
		return "", nil, encodedSelectorText, false, nil
	}

	if legacyCode != "" {
		return "", nil, "", true, badAutomationRequest("code and instance cannot be used together")
	}

	switch strings.ToLower(strings.TrimSpace(input.Instance.Type)) {
	case "script-default":
		return "", nil, "", true, nil
	case "existing", "rotate":
		selector := input.Instance.Selector
		if automationPublicHookTargetSelectorEmpty(selector) {
			return "", nil, "", true, badAutomationRequest("instance.selector is required")
		}
		return strings.ToLower(strings.TrimSpace(input.Instance.Type)), selector, "", false, nil
	case "create":
		targetInput := map[string]interface{}{
			"templateSelector": input.Instance.TemplateSelector,
		}
		if automationPublicHookTargetSelectorEmpty(input.Instance.TemplateSelector) {
			return "", nil, "", true, badAutomationRequest("instance.templateSelector is required")
		}
		if name := strings.TrimSpace(input.Instance.CreateNameTemplate); name != "" {
			targetInput["createNameTemplate"] = name
		} else if name := strings.TrimSpace(input.Instance.ProfileName); name != "" {
			targetInput["profileName"] = name
		}
		return "create", targetInput, "", false, nil
	case "":
		return "", nil, "", true, badAutomationRequest("instance.type is required")
	default:
		return "", nil, "", true, badAutomationRequest("instance.type is unsupported")
	}
}

func automationPublicHookTargetSelectorEmpty(selector automation.ScriptTargetSelector) bool {
	return strings.TrimSpace(selector.Code) == "" &&
		strings.TrimSpace(selector.ProfileID) == "" &&
		strings.TrimSpace(selector.ProfileName) == "" &&
		strings.TrimSpace(selector.GroupID) == "" &&
		len(selector.Keywords) == 0 &&
		len(selector.Tags) == 0
}

func encodeAutomationPublicHookJSONObject(obj map[string]interface{}) (string, error) {
	encoded, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func shouldApplyAutomationPublicHookVariables(record automation.ScriptRecord) bool {
	if strings.TrimSpace(record.PublicAPI.RequestBodyText) == "" {
		return false
	}
	for _, variable := range record.PublicAPI.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		if strings.Contains(record.PublicAPI.RequestBodyText, "{{"+name+"}}") || strings.Contains(record.PublicAPI.RequestBodyText, "${"+name+"}") {
			return true
		}
	}
	return false
}

func replaceAutomationPublicHookPlaceholderValue(bodyText string, name string, rawValue interface{}) string {
	value := strings.TrimSpace(formatAutomationPublicHookVariableValue(rawValue))
	escapedValue := escapeAutomationPublicHookJSONString(value)
	for _, placeholder := range []string{"{{" + name + "}}", "${" + name + "}"} {
		bodyText = strings.ReplaceAll(bodyText, placeholder, escapedValue)
	}
	return bodyText
}

func resolveAutomationPublicHookRequestBody(record automation.ScriptRecord, body []byte) ([]byte, error) {
	config := record.PublicAPI
	input, err := decodeAutomationPublicHookRequestBody(body)
	if err != nil {
		return nil, err
	}
	values := input.Params

	bodyText := replaceAutomationPublicHookPlaceholderValue(config.RequestBodyText, "code", automationPublicHookInstanceCode(input))
	for _, variable := range config.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		placeholders := []string{"{{" + name + "}}", "${" + name + "}"}
		used := false
		for _, placeholder := range placeholders {
			if strings.Contains(bodyText, placeholder) {
				used = true
				break
			}
		}
		if !used {
			continue
		}

		rawValue := automationPublicHookVariableDefaultValue(variable, input)
		if incomingValue, ok := values[name]; ok {
			rawValue = incomingValue
		}
		value := strings.TrimSpace(formatAutomationPublicHookVariableValue(rawValue))
		if variable.Required && value == "" {
			return nil, badAutomationRequest("missing required variable: " + name)
		}
		escapedValue := escapeAutomationPublicHookJSONString(value)
		for _, placeholder := range placeholders {
			bodyText = strings.ReplaceAll(bodyText, placeholder, escapedValue)
		}
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(bodyText), &decoded); err != nil {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	decodedBody, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	mergedBody := mergeAutomationPublicHookDefaultParams(record, decodedBody)
	encoded, err := json.Marshal(mergedBody)
	if err != nil {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	return encoded, nil
}

func automationPublicHookInstanceCode(input automationPublicHookRequestBody) string {
	if code := strings.TrimSpace(input.Code); code != "" {
		return code
	}
	if input.Instance == nil {
		return ""
	}
	return strings.TrimSpace(input.Instance.Selector.Code)
}

func automationPublicHookVariableDefaultValue(variable automation.ScriptPublicAPIVariable, input automationPublicHookRequestBody) interface{} {
	if strings.TrimSpace(variable.Name) == "code" {
		if code := automationPublicHookInstanceCode(input); code != "" {
			return code
		}
	}
	return variable.DefaultValue
}

func mergeAutomationPublicHookDefaultParams(record automation.ScriptRecord, body map[string]interface{}) map[string]interface{} {
	defaultParams, ok := parseAutomationPublicHookJSONObject(record.ParamsText)
	if !ok || len(defaultParams) == 0 {
		return body
	}

	if record.PublicAPI.RequestMode == "params-only" {
		return mergeAutomationPublicHookJSONObjects(defaultParams, body)
	}

	rawParams, ok := body["params"].(map[string]interface{})
	if !ok {
		return body
	}

	nextBody := make(map[string]interface{}, len(body))
	for key, value := range body {
		nextBody[key] = value
	}
	nextBody["params"] = mergeAutomationPublicHookJSONObjects(defaultParams, rawParams)
	return nextBody
}

func mergeAutomationPublicHookDefaultParamsObject(record automation.ScriptRecord, param map[string]interface{}) map[string]interface{} {
	if param == nil {
		param = map[string]interface{}{}
	}
	defaultParams, ok := parseAutomationPublicHookJSONObject(record.ParamsText)
	if !ok || len(defaultParams) == 0 {
		return param
	}
	return mergeAutomationPublicHookJSONObjects(defaultParams, param)
}

func parseAutomationPublicHookJSONObject(text string) (map[string]interface{}, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false
	}
	var value interface{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]interface{})
	return object, ok
}

func mergeAutomationPublicHookJSONObjects(base map[string]interface{}, patch map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(patch))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range patch {
		baseObject, baseOK := merged[key].(map[string]interface{})
		patchObject, patchOK := value.(map[string]interface{})
		if baseOK && patchOK {
			merged[key] = mergeAutomationPublicHookJSONObjects(baseObject, patchObject)
			continue
		}
		merged[key] = value
	}
	return merged
}

func formatAutomationPublicHookVariableValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64, bool, int, int64, json.Number:
		return strings.TrimSpace(strings.Trim(fmt.Sprint(typed), "\""))
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func escapeAutomationPublicHookJSONString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	text := string(encoded)
	if len(text) >= 2 {
		return text[1 : len(text)-1]
	}
	return text
}

func decodeAutomationRunAPIRequestBody(body []byte) (automationScriptRunAPIRequest, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return automationScriptRunAPIRequest{}, nil
	}

	var req automationScriptRunAPIRequest
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return automationScriptRunAPIRequest{}, badAutomationRequest("invalid request body")
	}
	return req, nil
}

func decodeJSONObjectBody(body []byte, fieldName string) (map[string]interface{}, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}

	var value interface{}
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}
	return obj, true, nil
}
