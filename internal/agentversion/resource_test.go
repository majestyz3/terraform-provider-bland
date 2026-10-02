package agentversion

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApplyListResultFoundKeepsState(t *testing.T) {
	m := Model{
		ID:           types.StringValue("version-2"),
		AgentID:      types.StringValue("agent-1"),
		Name:         types.StringValue("candidate"),
		SnapshotJSON: types.StringValue(`{"prompt":"hello"}`),
	}
	out := map[string]any{
		"data": []any{
			map[string]any{"id": "version-1", "agent_id": "agent-1", "name": "older", "kind": "manual", "created_at": "2026-10-01T00:00:00Z", "semver": nil},
			map[string]any{"id": "version-2", "agent_id": "agent-1", "name": "candidate", "kind": "manual", "created_at": "2026-10-02T00:00:00Z", "semver": nil},
		},
	}

	if !applyListResult(&m, out) {
		t.Fatal("expected matching version to remain in state")
	}
	if m.ID.ValueString() != "version-2" || m.AgentID.ValueString() != "agent-1" || m.Name.ValueString() != "candidate" || m.SnapshotJSON.ValueString() != `{"prompt":"hello"}` {
		t.Fatalf("immutable state changed: %#v", m)
	}
	if got := m.ResponseJSON.ValueString(); !strings.Contains(got, `"id": "version-2"`) || strings.Contains(got, "snapshot") {
		t.Fatalf("unexpected response_json: %s", got)
	}
}

func TestApplyListResultMissingRemovesState(t *testing.T) {
	m := Model{ID: types.StringValue("missing")}
	out := map[string]any{
		"data": []any{
			map[string]any{"id": "version-1"},
		},
	}

	if applyListResult(&m, out) {
		t.Fatal("expected missing version to be removed from state")
	}
}

func TestVersionsListPathUsesLimit200(t *testing.T) {
	if got, want := versionsListPath("agent-1"), "/v2/agents/agent-1/versions?limit=200"; got != want {
		t.Fatalf("versionsListPath() = %q, want %q", got, want)
	}
}
