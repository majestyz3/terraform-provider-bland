package dispositionextractor

import "testing"

func TestExtractorID(t *testing.T) {
	cases := []struct {
		in   map[string]any
		want string
	}{
		{map[string]any{"data": map[string]any{"id": "ex-1"}}, "ex-1"},
		{map[string]any{"data": map[string]any{"extractor": map[string]any{"id": "ex-2"}}}, "ex-2"},
		{map[string]any{"id": "ex-3"}, "ex-3"},
	}
	for _, tc := range cases {
		if got := extractorID(tc.in); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}

func TestDraftBody(t *testing.T) {
	draft := map[string]any{"draft": map[string]any{"name": "x"}}
	if got := draftBody(draft); got["name"] != "x" {
		t.Fatalf("unexpected draft body: %#v", got)
	}

	def := map[string]any{"definition": map[string]any{"type": "string"}}
	got := draftBody(def)
	nested, ok := got["definition"].(map[string]any)
	if !ok || nested["type"] != "string" {
		t.Fatalf("unexpected definition body: %#v", got)
	}
}

func TestDecodeRejectsEmpty(t *testing.T) {
	if _, err := decode("{}"); err == nil {
		t.Fatal("expected empty config to fail")
	}
}
