package jsonresource

import "testing"

func TestExtractID(t *testing.T) {
	got, ok := extractID(map[string]any{"data": map[string]any{"id": "abc"}}, nil)
	if !ok || got != "abc" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestSelectKeys(t *testing.T) {
	in := map[string]any{"name": "n", "description": "d", "nodes": []any{1}}
	got := selectKeys(in, []string{"name", "description"})
	if len(got) != 2 || got["name"] != "n" {
		t.Fatalf("unexpected %#v", got)
	}
}

func TestSelectKeysPassesThroughAllFieldsWhenUnfiltered(t *testing.T) {
	in := map[string]any{
		"name":      "agent-targeted scenario",
		"max_turns": 6,
		"agent_id":  "agent-example",
	}
	got := selectKeys(in, nil)
	if len(got) != len(in) || got["agent_id"] != "agent-example" || got["max_turns"] != 6 {
		t.Fatalf("unexpected pass-through body %#v", got)
	}
}
