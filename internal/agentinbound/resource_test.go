package agentinbound

import "testing"

func TestDecodeRejectsEmpty(t *testing.T) {
	if _, err := decode("{}"); err == nil {
		t.Fatal("expected empty binding to fail")
	}
}

func TestBasePath(t *testing.T) {
	got := basePath("agent/with/slash")
	want := "/v2/agents/agent%2Fwith%2Fslash/inbound"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
