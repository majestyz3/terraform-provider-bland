terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

resource "bland_eval_agent" "judge" {
  config_json = jsonencode({
    name = "Terraform Demo Judge"
  })
}

resource "bland_eval_agent_version_config" "judge" {
  parent_id  = bland_eval_agent.judge.id
  version_id = bland_eval_agent.judge.current_version_id

  config_json = jsonencode({
    prompt = "Determine whether the agent clearly answered the user's question."
  })
}

resource "bland_eval_agent_publish" "judge" {
  eval_agent_id = bland_eval_agent.judge.id

  triggers = {
    version_id = bland_eval_agent.judge.current_version_id
    config     = sha256(bland_eval_agent_version_config.judge.config_json)
  }

  depends_on = [bland_eval_agent_version_config.judge]
}

output "published_version_id" {
  value = bland_eval_agent_publish.judge.active_version_id
}

output "next_draft_version_id" {
  value = bland_eval_agent_publish.judge.current_version_id
}
