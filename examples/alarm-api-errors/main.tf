terraform {
  required_providers {
    bland = {
      source = "majestyz3/bland"
    }
  }
}

provider "bland" {}

# The supplied GET /v1/alarms export returned an empty alarms array, so there is
# no public API payload for the dashboard alert named "Member API failures above 5%".
# This example therefore reproduces only the documented public API portion:
# api_errors with Normal sensitivity (threshold 1.0). It intentionally leaves out
# agent scope, a 1-hour window, minimum-call count, severity, and notification
# settings because none appeared in the supplied response.
resource "bland_alarm" "api_errors" {
  config_json = jsonencode({
    metric_type = "api_errors"
    threshold   = 1.0
  })
}
