terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

variable "agent_id" {
  type        = string
  description = "Existing Bland V2 Agent ID."
}

variable "snapshot_json" {
  type        = string
  description = "Version snapshot, typically exported with data.bland_agent_version_latest and then templated in the config repository."
}

variable "agent_targeted_scenario_ids" {
  type        = list(string)
  description = "Scenario IDs that target this V2 agent. Pathway-targeted scenario IDs are not accepted by the checks API."
  default     = []
}

resource "bland_agent_version" "release" {
  agent_id      = var.agent_id
  name          = "Terraform release candidate"
  snapshot_json = var.snapshot_json
}

resource "bland_agent_checks" "staging" {
  agent_id    = var.agent_id
  environment = "staging"

  config_json = jsonencode({
    scenario_ids = var.agent_targeted_scenario_ids
  })
}

resource "bland_agent_release" "staging" {
  agent_id   = var.agent_id
  version_id = bland_agent_version.release.id
  bump       = "patch"

  depends_on = [bland_agent_checks.staging]
}

output "release_version" {
  value = bland_agent_release.staging.version_number
}

output "environment_pins" {
  value = bland_agent_release.staging.environments_json
}

# Production promotion is intentionally omitted from the automatic example.
# Bland's promote endpoint does not run required checks for you. Gate a check
# run separately, then opt in to bland_agent_promotion only when you deliberately
# want Terraform to perform the staging -> production promotion.
