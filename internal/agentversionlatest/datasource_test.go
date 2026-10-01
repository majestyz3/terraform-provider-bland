package agentversionlatest

import "testing"

func TestLatestVersionResponseShape(t *testing.T) {
	out := map[string]any{
		"data": map[string]any{
			"id": "version-1",
			"snapshot": map[string]any{
				"prompt": "hello",
			},
		},
	}
	data, ok := out["data"].(map[string]any)
	if !ok || data["id"] != "version-1" {
		t.Fatalf("unexpected data: %#v", out)
	}
	if got := encode(data["snapshot"]); got == "" || got == "null" {
		t.Fatalf("unexpected snapshot encoding: %q", got)
	}
}
