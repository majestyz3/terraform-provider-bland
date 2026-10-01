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
  description = "Existing Bland V2 Agent built in the dashboard."
}

data "bland_agent_version_latest" "dashboard_export" {
  agent_id = var.agent_id
}

output "version_id" {
  value = data.bland_agent_version_latest.dashboard_export.id
}

output "snapshot_json" {
  value = data.bland_agent_version_latest.dashboard_export.snapshot_json
}
