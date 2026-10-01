package agentpromotion

import "testing"

func TestProductionVersionID(t *testing.T) {
	envs := []any{
		map[string]any{"env_type": "staging", "current_version_id": "staging-v"},
		map[string]any{"env_type": "production", "current_version_id": "prod-v"},
	}
	if got := productionVersionID(envs); got != "prod-v" {
		t.Fatalf("got %q want %q", got, "prod-v")
	}
}
