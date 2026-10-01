package agentexperiment

import "testing"

func TestMutableConfig(t *testing.T) {
	in := map[string]any{
		"variant_version_id": "immutable",
		"baseline_env":       "production",
		"traffic_percentage": 25,
		"run_hours":          nil,
		"variant_call_quota": 100,
	}
	got := mutableConfig(in)
	if _, ok := got["variant_version_id"]; ok {
		t.Fatal("variant_version_id must not be sent on update")
	}
	if _, ok := got["baseline_env"]; ok {
		t.Fatal("baseline_env must not be sent on update")
	}
	if got["traffic_percentage"] != 25 {
		t.Fatalf("missing traffic_percentage: %#v", got)
	}
	if got["variant_call_quota"] != 100 {
		t.Fatalf("missing variant_call_quota: %#v", got)
	}
}

func TestExperimentID(t *testing.T) {
	out := map[string]any{"data": map[string]any{"experiment": map[string]any{"id": "exp-1"}}}
	if got := experimentID(out); got != "exp-1" {
		t.Fatalf("got %q", got)
	}
}
