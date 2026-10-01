terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

variable "agent_id" {
  type = string
}

resource "bland_agent_test_scenario" "agent_targeted" {
  agent_id  = var.agent_id
  name      = "Agent-targeted member question"
  prompt    = "You are a synthetic member. Ask one concise question, answer verification prompts truthfully, and end the test after the agent answers."
  max_turns = 20

  extra_json = jsonencode({
    category                  = "CUSTOM"
    bland_tone_enabled        = false
    is_required_for_promotion = false
    input_messages            = []
    assertions                = []
  })
}
