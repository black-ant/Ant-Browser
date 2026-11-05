package launchcode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/automation"
)

func writeAutomationPublicHookResponse(w http.ResponseWriter, record automation.ScriptRecord, run *automation.ScriptRunRecord) {
	_ = record
	parsedPayload, resultPayload, hasResult := decodeAutomationRunPayloadValue(run.ResultText)
	if run.Status != "success" {
		writeJSON(w, http.StatusOK, compactAutomationPublicHookFailure(run))
		return
	}

	response := map[string]interface{}{
		"ok":      true,
		"status":  run.Status,
		"summary": run.Summary,
		"message": run.Summary,
		"data":    map[string]interface{}{},
		"result":  map[string]interface{}{},
	}

	if hasResult {
		data := compactAutomationPublicHookData(resultPayload, run)
		response["data"] = data
		response["result"] = data
	} else if parsedPayload != nil {
		data := compactAutomationPublicHookData(parsedPayload, run)
		response["data"] = data
		response["result"] = data
	}

	writeJSON(w, http.StatusOK, response)
}

func compactAutomationPublicHookFailure(run *automation.ScriptRunRecord) map[string]interface{} {
	response := map[string]interface{}{
		"ok":      false,
		"status":  run.Status,
		"summary": run.Summary,
		"message": run.Summary,
		"data":    map[string]interface{}{},
		"result":  map[string]interface{}{},
	}
	if strings.TrimSpace(run.Error) != "" {
		response["error"] = run.Error
	}
	return response
}

func compactAutomationPublicHookData(payload interface{}, run *automation.ScriptRunRecord) interface{} {
	data := compactAutomationPublicHookResult(payload, run)
	delete(data, "ok")
	delete(data, "summary")
	return data
}

func compactAutomationPublicHookResult(payload interface{}, run *automation.ScriptRunRecord) map[string]interface{} {
	obj, ok := payload.(map[string]interface{})
	if !ok {
		result := map[string]interface{}{"ok": true}
		if strings.TrimSpace(run.Summary) != "" {
			result["summary"] = run.Summary
		}
		if payload != nil {
			result["result"] = payload
		}
		return result
	}

	if !hasAutomationPublicHookDownloadField(obj) {
		result := make(map[string]interface{}, len(obj)+1)
		result["ok"] = true
		for key, value := range obj {
			if key != "ok" && value != nil {
				result[key] = value
			}
		}
		if _, exists := result["summary"]; !exists && strings.TrimSpace(run.Summary) != "" {
			result["summary"] = run.Summary
		}
		return result
	}

	result := map[string]interface{}{"ok": true}
	for _, key := range []string{"downloadAddress", "downloadPath", "outputPath", "sourceImageUrl", "sourceDownloadUrl", "screenshotPath", "pageScreenshotPath", "contentType", "imageWidth", "imageHeight", "status", "summary", "error"} {
		if value, exists := obj[key]; exists && value != nil {
			result[key] = value
		}
	}
	return result
}

func hasAutomationPublicHookDownloadField(obj map[string]interface{}) bool {
	for _, key := range []string{"downloadAddress", "downloadPath", "outputPath"} {
		if value, exists := obj[key]; exists && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return true
		}
	}
	return false
}

func decodeAutomationRunPayloadValue(raw string) (interface{}, interface{}, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil, false
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, nil, false
	}

	if obj, ok := payload.(map[string]interface{}); ok {
		result, exists := obj["result"]
		return payload, result, exists
	}
	return payload, nil, false
}
