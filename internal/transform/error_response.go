package transform

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
)

func LogicalErrorStatus(body []byte) (int, bool) {
	var source any
	if err := json.Unmarshal(body, &source); err != nil {
		return 0, false
	}
	value, ok := getPathValue(source, "base_resp.status_code")
	if !ok {
		return 0, false
	}
	code, err := toNumberValue(value)
	if err != nil || code == 0 {
		return 0, false
	}
	statusCode := int(math.Round(code))
	if statusCode >= 400 && statusCode <= 599 {
		return statusCode, true
	}
	return http.StatusBadGateway, true
}

func MapErrorResponse(statusCode int, body []byte, fieldMapJSON, defaultsJSON string, specs []ResponseFieldSpec) ([]byte, error) {
	if HasResponseMapping(fieldMapJSON, defaultsJSON) {
		mapped, _, err := MapResponse(body, fieldMapJSON, defaultsJSON, specs)
		if err != nil {
			return nil, err
		}
		return ensureOpenAIErrorDefaults(statusCode, mapped)
	}
	return defaultOpenAIError(statusCode, body), nil
}

func ensureOpenAIErrorDefaults(statusCode int, body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	errorObj, ok := payload["error"].(map[string]any)
	if !ok {
		errorObj = make(map[string]any)
		payload["error"] = errorObj
	}
	if _, ok := errorObj["message"]; !ok {
		errorObj["message"] = http.StatusText(statusCode)
	}
	if _, ok := errorObj["type"]; !ok {
		errorObj["type"] = "upstream_error"
	}
	if _, ok := errorObj["param"]; !ok {
		errorObj["param"] = ""
	}
	if _, ok := errorObj["code"]; !ok {
		errorObj["code"] = fmt.Sprintf("%d", statusCode)
	}
	return json.Marshal(payload)
}

func defaultOpenAIError(statusCode int, body []byte) []byte {
	var source any
	_ = json.Unmarshal(body, &source)

	message := firstStringPath(source,
		"error.message",
		"message",
		"base_resp.status_msg",
		"base_resp.message",
		"status_msg",
		"detail",
	)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	if message == "" {
		message = "upstream request failed"
	}

	code := firstStringPath(source,
		"error.code",
		"code",
		"base_resp.status_code",
		"status_code",
	)
	if code == "" && statusCode > 0 {
		code = fmt.Sprintf("%d", statusCode)
	}

	errorType := firstStringPath(source, "error.type", "type")
	if errorType == "" {
		errorType = "upstream_error"
	}

	param := firstStringPath(source, "error.param", "param")

	payload, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"param":   param,
			"code":    code,
		},
	})
	return payload
}

func firstStringPath(source any, paths ...string) string {
	for _, path := range paths {
		value, ok := getPathValue(source, path)
		if !ok {
			continue
		}
		text, err := toStringValue(value)
		if err == nil && text != "" {
			return text
		}
	}
	return ""
}
