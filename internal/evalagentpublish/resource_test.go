package evalagentpublish

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApplyFields(t *testing.T) {
	m := Model{}
	out := map[string]any{
		"data": map[string]any{
			"current_version_id": "draft-v2",
			"active_version_id":  "published-v1",
		},
	}
	applyFields(&m, out)
	if m.CurrentVersionID != types.StringValue("draft-v2") {
		t.Fatalf("unexpected current version: %#v", m.CurrentVersionID)
	}
	if m.ActiveVersionID != types.StringValue("published-v1") {
		t.Fatalf("unexpected active version: %#v", m.ActiveVersionID)
	}
}
