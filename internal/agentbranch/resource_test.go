package agentbranch

import "testing"

func TestFindBranch(t *testing.T) {
	out := map[string]any{
		"data": []any{
			map[string]any{"id": "b1", "name": "one"},
			map[string]any{"id": "b2", "name": "two"},
		},
	}
	got, ok := findBranch(out, "b2")
	if !ok {
		t.Fatal("expected branch")
	}
	if got["name"] != "two" {
		t.Fatalf("unexpected branch: %#v", got)
	}
}

func TestSplitImportID(t *testing.T) {
	got := splitImportID("agent-id/branch-id")
	if len(got) != 2 || got[0] != "agent-id" || got[1] != "branch-id" {
		t.Fatalf("unexpected split: %#v", got)
	}
	if got := splitImportID("bad"); got != nil {
		t.Fatalf("expected invalid import to return nil: %#v", got)
	}
}
