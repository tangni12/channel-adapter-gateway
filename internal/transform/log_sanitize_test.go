package transform

import (
	"encoding/json"
	"testing"
)

func TestSanitizeLogPayloadRedactsOpenAIImageBase64Response(t *testing.T) {
	body := []byte(`{
		"created": 1,
		"data": [
			{"url": "https://example.com/a.png", "b64_json": "abc123abc123"},
			{"b64_json": "xyz789"}
		],
		"usage": {"input_tokens": 41, "output_tokens": 1756}
	}`)

	sanitized := SanitizeLogPayload(OpenAIEndpointImagesGenerations, LogTargetOfficialResponse, body)

	var payload map[string]any
	if err := json.Unmarshal(sanitized, &payload); err != nil {
		t.Fatalf("sanitized payload is not valid json: %v", err)
	}
	data := payload["data"].([]any)
	first := data[0].(map[string]any)
	if first["url"] != "https://example.com/a.png" {
		t.Fatalf("url changed: %v", first["url"])
	}
	if first["b64_json"] != redactedLargeValue {
		t.Fatalf("b64_json was not redacted: %v", first["b64_json"])
	}
	usage := payload["usage"].(map[string]any)
	if usage["output_tokens"].(float64) != 1756 {
		t.Fatalf("usage changed: %v", usage)
	}
}

func TestSanitizeLogPayloadRedactsCommonNestedBase64Fields(t *testing.T) {
	body := []byte(`{
		"result": {
			"images": [{"base64": "very-large-value"}],
			"metadata": {"id": "keep-me"}
		}
	}`)

	sanitized := SanitizeLogPayload("unknown.endpoint", LogTargetUpstreamResponse, body)

	var payload map[string]any
	if err := json.Unmarshal(sanitized, &payload); err != nil {
		t.Fatalf("sanitized payload is not valid json: %v", err)
	}
	result := payload["result"].(map[string]any)
	images := result["images"].([]any)
	image := images[0].(map[string]any)
	if image["base64"] != redactedLargeValue {
		t.Fatalf("base64 was not redacted: %v", image["base64"])
	}
	metadata := result["metadata"].(map[string]any)
	if metadata["id"] != "keep-me" {
		t.Fatalf("metadata changed: %v", metadata)
	}
}
