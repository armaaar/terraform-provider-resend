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

# Read metadata for an existing API key. The token itself is never returned by
# Resend after creation — you can only fetch the key's name and timestamps.
data "resend_api_key" "production" {
  id = "ak_abc123"
}

output "production_key_last_used" {
  description = "Timestamp of the production API key's last use, or null if never used."
  value       = data.resend_api_key.production.last_used_at
}
