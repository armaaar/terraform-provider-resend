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

# List every API key in the account — useful for audit / inventory output.
data "resend_api_keys" "all" {}

output "stale_api_keys" {
  description = "API keys that have never been used."
  value       = [for k in data.resend_api_keys.all.api_keys : k.name if k.last_used_at == null]
}
