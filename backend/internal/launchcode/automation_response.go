package launchcode

import "net/http"

type automationAPIResponse[T any] struct {
	OK      bool                `json:"ok"`
	Status  string              `json:"status,omitempty"`
	Message string              `json:"message,omitempty"`
	Error   *automationAPIError `json:"error,omitempty"`
	Data    T                   `json:"data,omitempty"`
}

type automationAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type automationAPIListData[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
	Limit int `json:"limit,omitempty"`
}

type automationAPIItemData[T any] struct {
	Item T `json:"item"`
}

type automationAPIRunData struct {
	Run     interface{} `json:"run,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Result  interface{} `json:"result,omitempty"`
}

func writeAutomationAPIError(w http.ResponseWriter, status int, code string, message string, field string) {
	writeJSON(w, status, automationAPIResponse[struct{}]{
		OK:     false,
		Status: "failed",
		Error: &automationAPIError{
			Code:    code,
			Message: message,
			Field:   field,
		},
	})
}

func writeAutomationAPISuccess[T any](w http.ResponseWriter, status int, message string, data T) {
	writeJSON(w, status, automationAPIResponse[T]{
		OK:      true,
		Status:  "success",
		Message: message,
		Data:    data,
	})
}
