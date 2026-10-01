package evalagent

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApplyVersionFields(t *testing.T) {
	m := Model{}
	out := map[string]any{
		"data": map[string]any{
			"agent": map[string]any{
				"current_version_id": "draft-v",
				"active_version_id":  "active-v",
			},
		},
	}
	applyVersionFields(&m, out)
	if m.CurrentVersionID != types.StringValue("draft-v") {
		t.Fatalf("unexpected current version: %#v", m.CurrentVersionID)
	}
	if m.ActiveVersionID != types.StringValue("active-v") {
		t.Fatalf("unexpected active version: %#v", m.ActiveVersionID)
	}
}
