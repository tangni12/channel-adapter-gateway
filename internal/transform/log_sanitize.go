package transform

import (
	"encoding/json"
	"strings"
)

const redactedLargeValue = "xxxx"

type LogSanitizeTarget string

const (
	LogTargetOfficialRequest  LogSanitizeTarget = "official_request"
	LogTargetUpstreamRequest  LogSanitizeTarget = "upstream_request"
	LogTargetUpstreamResponse LogSanitizeTarget = "upstream_response"
	LogTargetOfficialResponse LogSanitizeTarget = "official_response"
	LogTargetResponseUsage    LogSanitizeTarget = "response_usage"
)

func SanitizeLogPayload(endpoint string, target LogSanitizeTarget, body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}

	switch endpoint {
	case OpenAIEndpointImagesGenerations, OpenAIEndpointImagesEdits:
		sanitizeOpenAIImagePayload(target, payload)
	default:
		sanitizeCommonLargeFields(payload)
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return normalized
}

func sanitizeOpenAIImagePayload(target LogSanitizeTarget, payload any) {
	switch target {
	case LogTargetOfficialResponse, LogTargetUpstreamResponse:
		redactPaths(payload, "data[].b64_json")
	case LogTargetOfficialRequest, LogTargetUpstreamRequest:
		redactPaths(payload, "image")
		redactPaths(payload, "image[]")
		redactPaths(payload, "images[]")
		redactPaths(payload, "mask")
	}
	sanitizeCommonLargeFields(payload)
}

func sanitizeCommonLargeFields(payload any) {
	switch value := payload.(type) {
	case map[string]any:
		for key, item := range value {
			lowerKey := strings.ToLower(key)
			if shouldRedactFieldName(lowerKey) {
				value[key] = redactValue(item)
				continue
			}
			sanitizeCommonLargeFields(item)
		}
	case []any:
		for _, item := range value {
			sanitizeCommonLargeFields(item)
		}
	}
}

func shouldRedactFieldName(key string) bool {
	return key == "b64_json" ||
		key == "base64" ||
		key == "image_base64" ||
		key == "mask_base64" ||
		key == "audio_base64" ||
		key == "video_base64"
}

func redactPaths(payload any, paths ...string) {
	for _, path := range paths {
		redactPath(payload, parsePath(path))
	}
}

func redactPath(current any, tokens []pathToken) {
	if len(tokens) == 0 {
		return
	}
	token := tokens[0]
	switch value := current.(type) {
	case map[string]any:
		next, ok := value[token.name]
		if !ok {
			return
		}
		if len(tokens) == 1 {
			value[token.name] = redactValue(next)
			return
		}
		if token.array {
			items, ok := toAnySlice(next)
			if !ok {
				return
			}
			for _, item := range items {
				redactPath(item, tokens[1:])
			}
			return
		}
		redactPath(next, tokens[1:])
	case []any:
		for _, item := range value {
			redactPath(item, tokens)
		}
	}
}

func redactValue(value any) any {
	switch v := value.(type) {
	case string:
		return redactedLargeValue
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = redactValue(item)
		}
		return result
	case map[string]any:
		clone := make(map[string]any, len(v)+1)
		for key, item := range v {
			clone[key] = item
		}
		return clone
	default:
		return redactedLargeValue
	}
}
