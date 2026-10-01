# Terraform Provider for Bland AI

A from-scratch Terraform provider targeting Bland's current public REST APIs, including the 2026 V2 Agent lifecycle.

> Status: **alpha.** It is scoped first to reproducibly manage the synthetic SCAN Health member-services demo. Validate against a Bland test organization before production use.

## Current resource families

### V2 Agent lifecycle
- `bland_agent`
- `bland_agent_version`
- `bland_agent_variable`
- `bland_agent_checks`
- `bland_tool`

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

Fast-moving Bland objects use full-fidelity JSON attributes so newly-added API fields can be represented without destructive provider flattening.

## Authentication

```bash
export BLAND_API_KEY="..."
```

```hcl
terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}
provider "bland" {}
```

Set `BLAND_BASE_URL` (or provider `base_url`) for a compatible private/self-hosted endpoint.

## Safety and design

- No private dashboard endpoints.
- Examples never purchase numbers, place calls, send SMS, or configure a real transfer destination.
- Agent Versions are immutable.
- Publish/promote/rollback remains an explicit release-management action rather than an implicit `terraform apply` side effect.
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
