package transform

import (
	"encoding/json"
	"testing"
)

func TestMapErrorResponseUsesConfiguredMiniMaxFields(t *testing.T) {
	body := []byte(`{"created":0,"trace_id":"065a","base_resp":{"status_msg":"Your request was rejected by the safety system.","status_code":400}}`)

	mapped, err := MapErrorResponse(
		400,
		body,
		`{"error.message":"base_resp.status_msg","error.code":"base_resp.status_code"}`,
		`{"error.type":"content_policy_violation","error.param":""}`,
		[]ResponseFieldSpec{
			{Name: "error.message", Type: "string", Required: true},
			{Name: "error.type", Type: "string"},
			{Name: "error.param", Type: "string"},
			{Name: "error.code", Type: "string"},
		},
	)
	if err != nil {
		t.Fatalf("MapErrorResponse returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal(mapped, &payload); err != nil {
		t.Fatalf("invalid mapped json: %v", err)
	}
	if payload["error"]["message"] != "Your request was rejected by the safety system." {
		t.Fatalf("unexpected message: %#v", payload["error"]["message"])
	}
	if payload["error"]["type"] != "content_policy_violation" {
		t.Fatalf("unexpected type: %#v", payload["error"]["type"])
	}
	if payload["error"]["code"] != "400" {
		t.Fatalf("unexpected code: %#v", payload["error"]["code"])
	}
}

func TestMapErrorResponseDefaultsUnknownUpstreamShape(t *testing.T) {
	mapped, err := MapErrorResponse(503, []byte(`{"unexpected":"shape"}`), `{}`, `{}`, nil)
	if err != nil {
		t.Fatalf("MapErrorResponse returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal(mapped, &payload); err != nil {
		t.Fatalf("invalid mapped json: %v", err)
	}
	if payload["error"]["message"] != "Service Unavailable" {
		t.Fatalf("unexpected message: %#v", payload["error"]["message"])
	}
	if payload["error"]["type"] != "upstream_error" {
		t.Fatalf("unexpected type: %#v", payload["error"]["type"])
	}
	if payload["error"]["code"] != "503" {
		t.Fatalf("unexpected code: %#v", payload["error"]["code"])
	}
}

func TestLogicalErrorStatusDetectsMiniMaxBaseResp(t *testing.T) {
	status, ok := LogicalErrorStatus([]byte(`{"base_resp":{"status_code":400,"status_msg":"rejected"}}`))
	if !ok {
		t.Fatal("expected logical error")
	}
	if status != 400 {
		t.Fatalf("unexpected status: %d", status)
	}
}
