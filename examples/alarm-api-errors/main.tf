terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

# The public Bland alarm API documents api_errors as the API-error metric.
# It currently does not document per-agent scoping, a fixed 1-hour window,
# minimum-call counts, or an exact percentage threshold. Do not add those
# dashboard-only concepts to Terraform until Bland exposes them publicly.
resource "bland_alarm" "api_errors" {
  config_json = jsonencode({
    metric_type = "api_errors"
    threshold   = 1.0
  })
}
