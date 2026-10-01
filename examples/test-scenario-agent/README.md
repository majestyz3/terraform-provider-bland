# Agent-targeted test scenario: public API gap

Bland V2 environment checks require scenario IDs that target the V2 Agent rather than a Pathway.

The public Agent Testing create/get documentation currently documents Pathway and Persona targeting, but does not document the field used by dashboard-created V2 Agent-targeted scenarios. This provider intentionally does not invent or depend on private dashboard fields.

The existing `bland_test_scenario` resource passes its entire `config_json` body through unchanged on both create and update, so no provider change is required once the public field is confirmed.

To inspect the dashboard-created scenario requested for this provider work, run:

```bash
curl --fail-with-body --silent --show-error \
  -H "authorization: $BLAND_API_KEY" \
  -H "accept: application/json" \
  https://api.bland.ai/v1/agent-testing/scenarios/a3b73e78-af76-4be4-9918-5b860523e532 | jq .
```

Once Bland documents the returned agent-targeting field in the public API reference, the Terraform shape can be:

```hcl
resource "bland_test_scenario" "agent_targeted" {
  config_json = jsonencode({
    name                  = "Agent-targeted member question"
    tester_persona_prompt = "Ask the agent a simple synthetic member-services question."
    max_turns             = 6

    # Add the documented V2-agent target field here after it is confirmed.
  })
}
```
