# Agent-targeted test scenarios

Dashboard-created V2 Agent scenarios use the public Agent Testing API and identify the target Agent with `agent_id`.

Use `bland_agent_test_scenario` to create or adopt one scenario, or `data.bland_agent_test_scenarios` to discover all scenarios for an Agent and feed their IDs into `bland_agent_checks`.

The dedicated resource maps:

- `agent_id` -> `agent_id`
- `prompt` -> `tester_persona_prompt`
- `max_turns` -> `max_turns`

Additional documented Agent Testing request fields can be supplied through `extra_json`.

Import an existing dashboard scenario with:

```bash
terraform import bland_agent_test_scenario.agent_targeted <scenario_id>
```

On import the provider reads the scenario and hydrates the Agent ID, name, caller prompt, max turns, description, and documented extra request fields.
