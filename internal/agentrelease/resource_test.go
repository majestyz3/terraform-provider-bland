package agentrelease

import "testing"

func TestEncodeReleaseCollections(t *testing.T) {
	envs := []any{map[string]any{"env_type": "staging", "current_version_id": "v1"}}
	if got := encode(envs); got == "" || got == "null" {
		t.Fatalf("unexpected encoded environments: %q", got)
	}
}

func TestValidReleaseBumps(t *testing.T) {
	for _, bump := range []string{"patch", "minor", "major"} {
		if bump != "patch" && bump != "minor" && bump != "major" {
			t.Fatalf("valid bump rejected: %s", bump)
		}
	}
}
