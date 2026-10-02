# Terraform Provider for Bland AI

A Terraform provider targeting Bland's current public REST APIs, including the 2026 V2 Agent lifecycle.

> Status: **alpha.** not to be used in production 

## Repository

Provider source code lives here:

```text
github.com/majestyz3/terraform-provider-bland
```

The separate `github.com/majestyz3/bland` repository contains the runnable SCAN interview demo that consumes this provider. Its CI builds this provider and exercises a full Terraform apply/zero-diff-plan/destroy lifecycle against a deterministic mock Bland API.

## Current resource families

### V2 Agent lifecycle
- `bland_agent`
- `bland_agent_version`
- `bland_agent_release`
- `bland_agent_promotion` (explicit opt-in; production promotion is normally kept manual)
- `bland_agent_branch`
- `bland_agent_disposition`
- `bland_agent_experiment`
- `bland_agent_identity`
- `bland_agent_inbound_binding`
- `bland_agent_variable`
- `bland_agent_checks`
- `bland_agent_test_scenario`
- `bland_disposition_extractor`
- `bland_tool`

### Read-only data sources
- `data.bland_agent_info`
- `data.bland_agent_environments`
- `data.bland_agent_inbound`
- `data.bland_agent_memory_schema`
- `data.bland_agent_versions`
- `data.bland_agent_version_latest`
- `data.bland_agent_test_scenarios`
- `data.bland_alarms`
- `data.bland_agent_branches`
- `data.bland_agent_dispositions`
- `data.bland_disposition_extractors`
- `data.bland_disposition_judge_catalog`
- `data.bland_disposition_variable_catalog`
- `data.bland_disposition_model_profile_catalog`
- `data.bland_disposition_extractor_catalog`

### Shared / compatibility
- `bland_persona`
- `bland_conversational_pathway`
- `bland_knowledge_base`
- `bland_guard_rail`
- `bland_test_scenario`
- `bland_eval_agent`
- `bland_eval_agent_publish`
- `bland_eval_workbench`
- `bland_alarm`
- Eval Agent/Workbench editable-version configuration resources

Fast-moving Bland objects use full-fidelity JSON attributes so newly-added API fields can be represented without destructive provider flattening. Shared JSON-backed resources now support standard Terraform import by Bland object ID.

`bland_knowledge_base` waits for Bland's asynchronous processing to reach `COMPLETED` before creation succeeds. The wait is configurable with `ready_timeout` (default `5m`) and the current processing state is exposed as `status`.

## Recommended workflow

A practical migration path from a dashboard-built Agent into Terraform is:

1. Build and validate the Agent in the Bland dashboard once.
2. Export the latest saved Agent Version with `data.bland_agent_version_latest`.
3. Store that normalized `snapshot_json` in your configuration repository.
4. Replace environment-specific IDs inside the stored snapshot with Terraform template variables. A common pattern is to keep `snapshot.json.tftpl` in the repo and render it with `templatefile()`, for example:

```hcl
locals {
  snapshot_json = templatefile("${path.module}/snapshot.json.tftpl", {
    knowledge_base_id = bland_knowledge_base.member_services.id
  })
}

resource "bland_agent_version" "release" {
  agent_id      = var.agent_id
  name          = "Terraform release candidate"
  snapshot_json = local.snapshot_json
}
```

5. Configure pre-release scenario IDs with `bland_agent_checks`.
6. Explicitly publish the saved version to staging with `bland_agent_release`.
7. Run and evaluate your required checks separately before production. Bland's public promote endpoint does not execute or gate those checks automatically.
8. Normally promote staging to production manually. Use `bland_agent_promotion` only when you intentionally want Terraform apply to perform that production promotion.

Publishing and promotion are therefore modeled as explicit opt-in resources rather than side effects of editing an Agent Version.

## Authentication

```bash
export BLAND_API_KEY="..."
```

For local development, build and install this provider from this repository using Terraform's provider development override mechanism. Registry installation instructions will be added when the provider is published.

Set `BLAND_BASE_URL` (or provider `base_url`) for a compatible private/self-hosted endpoint.

## Safety and design

- No private dashboard endpoints.
- Examples never purchase numbers, place calls, send SMS, or configure a real transfer destination.
- Agent Versions are immutable.
- Publish/promote/rollback remains an explicit release-management action rather than an implicit Agent-Version side effect. `bland_agent_release` and `bland_agent_promotion` perform those actions only when explicitly declared.
- Disposition and extractor drafts are desired state; publishing them remains an explicit release action.
- Agent inbound-number attachment is managed explicitly with matching attach/detach payloads.
- Destroying an experiment stops it; Bland retains its completed experiment history.
- Destroying an agent identity only removes it from Terraform state because Bland does not expose an identity delete endpoint.
- Runtime transcripts, recordings, test runs, and eval results are not modeled as desired-state resources.

## Prior art

Inspired by James Hiester's experimental `terraform-provider-bland`, which demonstrated early Pathway, Knowledge Base, and Secret management. This implementation is written from scratch around Bland's current public API and V2 Agent lifecycle.

## Bland references

- https://docs.bland.ai/api-v2/overview
- https://docs.bland.ai/api-v2/post/agents
- https://docs.bland.ai/api-v2/post/agents-id-versions
- https://docs.bland.ai/api-v2/put/agents-id-environments-env-variables-key
- https://docs.bland.ai/api-v2/put/agents-id-environments-env-checks
- https://docs.bland.ai/api-v2/post/tools
- https://docs.bland.ai/llms.txt

## Examples

Acceptance-style configuration examples live under `examples/`:

- `examples/export-dashboard-agent` — export the latest dashboard-built Agent snapshot.
- `examples/eval-agent-publish` — configure the current Eval Agent draft and publish it explicitly.
- `examples/agent-release` — create an immutable Agent Version, configure staging checks, and publish to staging.
- `examples/knowledge-base-readiness` — wait for asynchronous knowledge ingestion.
- `examples/alarm-api-errors` — configure the documented public `api_errors` alarm metric.
- `examples/test-scenario-agent` — create or import a V2 Agent-targeted scenario using the observed public `agent_id` field.
- `examples/adopt-dashboard-demo` — full existing-Agent adoption flow: export snapshot, import dashboard scenarios, configure judges/checks, and publish a minor staging release.

`bland_agent_test_scenario` is the typed V2-Agent scenario resource. Dashboard-created Agent scenarios are linked with the public response field `agent_id`; the provider maps `prompt` to `tester_persona_prompt` and supports clean imports of existing dashboard scenarios. `data.bland_agent_test_scenarios` filters the organization scenario list by `agent_id` and exposes both a typed list and a name-to-ID map for `bland_agent_checks`.

`bland_alarm` remains full-fidelity JSON for the public Alarm API, but now reconstructs the documented request fields on import so existing public-API alarms can be adopted cleanly. `data.bland_alarms` lists visible alarms and exposes an optional name-to-ID map when the API returns names. The supplied organization export contained no alarms, so examples do not invent dashboard-only scope/window/minimum-call fields or a missing alarm ID.
