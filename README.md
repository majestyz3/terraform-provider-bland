# Terraform Provider for Bland AI

A from-scratch Terraform provider targeting Bland's current public REST APIs, including the 2026 V2 Agent lifecycle.

> Status: **alpha.** It is scoped first to reproducibly manage the synthetic SCAN Health member-services demo. Validate against a Bland test organization before production use.

## Repository

Provider source code lives here:

```text
github.com/majestyz3/terraform-provider-bland
```

The separate `github.com/majestyz3/bland` repository is reserved for Terraform configuration specific to the SCAN interview demo and is **not** the provider source repository.

## Current resource families

### V2 Agent lifecycle
- `bland_agent`
- `bland_agent_version`
- `bland_agent_branch`
- `bland_agent_disposition`
- `bland_agent_experiment`
- `bland_agent_identity`
- `bland_agent_inbound_binding`
- `bland_agent_variable`
- `bland_agent_checks`
- `bland_disposition_extractor`
- `bland_tool`

### Read-only data sources
- `data.bland_agent_info`
- `data.bland_agent_environments`
- `data.bland_agent_inbound`
- `data.bland_agent_memory_schema`
- `data.bland_agent_versions`
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
- `bland_eval_workbench`
- `bland_alarm`
- Eval Agent/Workbench editable-version configuration resources

Fast-moving Bland objects use full-fidelity JSON attributes so newly-added API fields can be represented without destructive provider flattening. Shared JSON-backed resources now support standard Terraform import by Bland object ID.

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
- Publish/promote/rollback remains an explicit release-management action rather than an implicit `terraform apply` side effect.
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
