package jsondatasource

import "testing"

func TestInterpolate(t *testing.T) {
	got := interpolate("/v2/agents/{id}/memory-schema", "agent-123")
	want := "/v2/agents/agent-123/memory-schema"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEncode(t *testing.T) {
	got := encode(map[string]any{"data": map[string]any{"id": "x"}})
	if got == "" {
		t.Fatal("expected JSON output")
	}
}
