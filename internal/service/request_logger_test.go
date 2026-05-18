package service

import "testing"

func TestNormalizeJSONBString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty becomes json null", in: "", want: "null"},
		{name: "valid object unchanged", in: `{"ok":true}`, want: `{"ok":true}`},
		{name: "valid array unchanged", in: `[1,2,3]`, want: `[1,2,3]`},
		{name: "invalid json wrapped as raw", in: `not-json`, want: `{"raw":"not-json"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeJSONBString(tt.in); got != tt.want {
				t.Fatalf("normalizeJSONBString(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestJSONBStringEmptyBody(t *testing.T) {
	if got := jsonbString(nil, 1024); got != "null" {
		t.Fatalf("jsonbString(nil) = %s, want null", got)
	}
}
