terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

resource "bland_knowledge_base" "demo" {
  ready_timeout = "5m"

  config_json = jsonencode({
    type        = "text"
    name        = "Terraform readiness example"
    description = "Synthetic, non-sensitive example content."
    text        = "This is synthetic example content created by Terraform."
  })
}

output "knowledge_base_id" {
  value = bland_knowledge_base.demo.id
}

output "status" {
  value = bland_knowledge_base.demo.status
}
