package agenttestscenario

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBodyFromModelUsesAgentTarget(t *testing.T) {
	m := Model{
		AgentID:   types.StringValue("agent-1"),
		Name:      types.StringValue("Test A"),
		Prompt:    types.StringValue("Act like a caller."),
		MaxTurns:  types.Int64Value(20),
		ExtraJSON: types.StringValue(`{"category":"CUSTOM","is_required_for_promotion":true}`),
	}
	body, err := bodyFromModel(m)
	if err != nil {
		t.Fatal(err)
	}
	if body["agent_id"] != "agent-1" || body["scenario_type"] != "AGENT" {
		t.Fatalf("agent target not preserved: %#v", body)
	}
	if body["tester_persona_prompt"] != "Act like a caller." || body["category"] != "CUSTOM" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestHydrateImportedScenario(t *testing.T) {
	m := Model{}
	out := map[string]any{"data": map[string]any{
		"id": "scenario-1", "agent_id": "agent-1", "name": "Test A",
		"tester_persona_prompt": "Caller instructions", "max_turns": float64(20),
	}}
	if err := hydrate(&m, out); err != nil {
		t.Fatal(err)
	}
	if m.AgentID.ValueString() != "agent-1" || m.Name.ValueString() != "Test A" || m.Prompt.ValueString() != "Caller instructions" || m.MaxTurns.ValueInt64() != 20 {
		t.Fatalf("unexpected hydrated model: %#v", m)
	}
}

func TestMaxTurnsRange(t *testing.T) {
	if validateMaxTurns(0) == nil || validateMaxTurns(51) == nil {
		t.Fatal("expected out-of-range max_turns to fail")
	}
	if err := validateMaxTurns(20); err != nil {
		t.Fatal(err)
	}
}
