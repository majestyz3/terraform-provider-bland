package knowledgebase

import "testing"

func TestStatusFields(t *testing.T) {
	out := map[string]any{
		"data": map[string]any{
			"status":        "FAILED",
			"error_message": "bad source",
		},
	}
	status, message := statusFields(out)
	if status != "FAILED" || message != "bad source" {
		t.Fatalf("got status=%q message=%q", status, message)
	}
}
