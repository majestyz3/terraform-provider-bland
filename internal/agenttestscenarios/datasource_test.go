package agenttestscenarios

import "testing"

func TestFilterScenariosByAgentAndPrefix(t *testing.T) {
	out := map[string]any{"data": []any{
		map[string]any{"id": "a", "agent_id": "agent-1", "name": "J1 - Golden"},
		map[string]any{"id": "b", "agent_id": "agent-1", "name": "J2 - Auth"},
		map[string]any{"id": "c", "agent_id": "agent-2", "name": "J1 - Other"},
	}}
	items, ids := filterScenarios(out, "agent-1", "J1")
	if len(items) != 1 || items[0].ID.ValueString() != "a" || ids["J1 - Golden"] != "a" {
		t.Fatalf("unexpected filtered result: %#v %#v", items, ids)
	}
}
