terraform {
  required_version = ">= 1.5"

  required_providers {
    resend = {
      source  = "armaaar/resend"
      version = "~> 1.0"
    }
  }
}

provider "resend" {}

# List every webhook in the account. The signing_secret is intentionally
# missing from this list endpoint — fetch each webhook via data.resend_webhook
# if you need it.
data "resend_webhooks" "all" {}

output "disabled_webhooks" {
  description = "Endpoints of webhooks currently in disabled state."
  value       = [for w in data.resend_webhooks.all.webhooks : w.endpoint if w.status == "disabled"]
}
