package transform

import (
	"encoding/json"
	"testing"
)

func TestNormalizeOpenAIUsageForImagesKeepsMappedOfficialFields(t *testing.T) {
	body := []byte(`{
		"created": 1,
		"usage": {
			"input_tokens": 941,
			"input_tokens_details": {"image_tokens": 900, "text_tokens": 41},
			"output_tokens": 1756,
			"output_tokens_details": {"image_tokens": 1756, "text_tokens": 0},
			"total_tokens": 2697
		}
	}`)

	normalized, usageBytes, err := NormalizeOpenAIUsageForEndpoint(body, OpenAIEndpointImagesGenerations)
	if err != nil {
		t.Fatalf("NormalizeOpenAIUsageForEndpoint returned error: %v", err)
	}

	usage := decodeUsage(t, normalized)
	if got := usage["input_tokens"].(float64); got != 941 {
		t.Fatalf("input_tokens = %v, want 941", got)
	}
	if got := usage["output_tokens"].(float64); got != 1756 {
		t.Fatalf("output_tokens = %v, want 1756", got)
	}
	if got := usage["total_tokens"].(float64); got != 2697 {
		t.Fatalf("total_tokens = %v, want 2697", got)
	}

	var loggedUsage map[string]any
	if err := json.Unmarshal(usageBytes, &loggedUsage); err != nil {
		t.Fatalf("usageBytes is not valid json: %v", err)
	}
	if got := loggedUsage["output_tokens"].(float64); got != 1756 {
		t.Fatalf("logged output_tokens = %v, want 1756", got)
	}
}

func TestNormalizeOpenAIUsageForImagesOnlyFillsTotalTokens(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":41,"output_tokens":1756}}`)

	normalized, _, err := NormalizeOpenAIUsageForEndpoint(body, OpenAIEndpointImagesEdits)
	if err != nil {
		t.Fatalf("NormalizeOpenAIUsageForEndpoint returned error: %v", err)
	}

	usage := decodeUsage(t, normalized)
	if got := usage["input_tokens"].(float64); got != 41 {
		t.Fatalf("input_tokens = %v, want 41", got)
	}
	if got := usage["output_tokens"].(float64); got != 1756 {
		t.Fatalf("output_tokens = %v, want 1756", got)
	}
	if got := usage["total_tokens"].(float64); got != 1797 {
		t.Fatalf("total_tokens = %v, want 1797", got)
	}
}

func TestNormalizeOpenAIUsageForImagesDoesNotMapLegacyFields(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":41,"completion_tokens":1756}}`)

	normalized, _, err := NormalizeOpenAIUsageForEndpoint(body, OpenAIEndpointImagesEdits)
	if err != nil {
		t.Fatalf("NormalizeOpenAIUsageForEndpoint returned error: %v", err)
	}

	usage := decodeUsage(t, normalized)
	if _, ok := usage["input_tokens"]; ok {
		t.Fatal("images post-processing must not map prompt_tokens to input_tokens")
	}
	if _, ok := usage["output_tokens"]; ok {
		t.Fatal("images post-processing must not map completion_tokens to output_tokens")
	}
}

func TestNormalizeOpenAIUsageForLegacyKeepsChatUsageFields(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":10,"output_tokens":20}}`)

	normalized, _, err := NormalizeOpenAIUsageForEndpoint(body, "openai.chat.completions")
	if err != nil {
		t.Fatalf("NormalizeOpenAIUsageForEndpoint returned error: %v", err)
	}

	usage := decodeUsage(t, normalized)
	if got := usage["prompt_tokens"].(float64); got != 10 {
		t.Fatalf("prompt_tokens = %v, want 10", got)
	}
	if got := usage["completion_tokens"].(float64); got != 20 {
		t.Fatalf("completion_tokens = %v, want 20", got)
	}
	if got := usage["total_tokens"].(float64); got != 30 {
		t.Fatalf("total_tokens = %v, want 30", got)
	}
}

func decodeUsage(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body is not valid json: %v", err)
	}
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatal("usage is missing")
	}
	return usage
}
