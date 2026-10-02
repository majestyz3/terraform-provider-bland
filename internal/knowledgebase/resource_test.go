package knowledgebase

import (
	"encoding/json"
	"testing"
)

func TestExtractID(t *testing.T) {
	tests := []struct {
		name string
		out  map[string]any
		want string
	}{
		{
			name: "data.id",
			out:  map[string]any{"data": map[string]any{"id": "kb-data-id"}},
			want: "kb-data-id",
		},
		{
			name: "data.knowledge_base_id real learn response",
			out: func() map[string]any {
				const raw = `{
  "data": {
    "knowledge_base_id": "a1a16f41-a574-4898-a737-db88d6a3bd65",
    "message": "TEXT KB creation started successfully",
    "success": true,
    "version_id": "7dbb042a-008b-489a-a9ca-ac40aef9adbb"
  },
  "errors": null
}`
				var out map[string]any
				if err := json.Unmarshal([]byte(raw), &out); err != nil {
					t.Fatalf("unmarshal real response: %v", err)
				}
				return out
			}(),
			want: "a1a16f41-a574-4898-a737-db88d6a3bd65",
		},
		{
			name: "top-level id",
			out:  map[string]any{"id": "kb-top-level"},
			want: "kb-top-level",
		},
		{
			name: "missing",
			out:  map[string]any{"data": map[string]any{"message": "no id"}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractID(tt.out); got != tt.want {
				t.Fatalf("extractID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusFields(t *testing.T) {
	out := map[string]any{
		"data": map[string]any{
			"id":            "kb-1",
			"status":        "FAILED",
			"error_message": "bad source",
		},
	}
	status, message := statusFields(out)
	if status != "FAILED" || message != "bad source" {
		t.Fatalf("got status=%q message=%q", status, message)
	}
}
