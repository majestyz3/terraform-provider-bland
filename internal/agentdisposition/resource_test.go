package agentdisposition

import "testing"

func TestDispositionID(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"direct data", map[string]any{"data": map[string]any{"id": "disp-1"}}, "disp-1"},
		{"nested data", map[string]any{"data": map[string]any{"disposition": map[string]any{"id": "disp-2"}}}, "disp-2"},
		{"top level", map[string]any{"id": "disp-3"}, "disp-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dispositionID(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestDraftDefinition(t *testing.T) {
	direct := map[string]any{"data": map[string]any{"definition": map[string]any{"type": "boolean"}}}
	if got, ok := draftDefinition(direct); !ok || got["type"] != "boolean" {
		t.Fatalf("unexpected direct draft: %#v %v", got, ok)
	}

	nested := map[string]any{"data": map[string]any{"draft": map[string]any{"definition": map[string]any{"type": "string"}}}}
	if got, ok := draftDefinition(nested); !ok || got["type"] != "string" {
		t.Fatalf("unexpected nested draft: %#v %v", got, ok)
	}
}

func TestDecodeDefinitionRejectsEmpty(t *testing.T) {
	if _, err := decodeDefinition("{}"); err == nil {
		t.Fatal("expected empty definition to fail")
	}
}
