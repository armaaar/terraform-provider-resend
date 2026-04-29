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

# Read an existing Resend webhook. The HMAC signing_secret is returned by
# Resend's GET endpoint — pipe it into your secret store on the consumer side
# so request verification stays in sync with the source of truth.
data "resend_webhook" "deliverability" {
  id = "wh_abc123"
}

output "webhook_signing_secret" {
  description = "HMAC signing secret for the deliverability webhook."
  value       = data.resend_webhook.deliverability.signing_secret
  sensitive   = true
}
