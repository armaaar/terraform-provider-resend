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

resource "resend_domain" "example_com" {
  name = "example.com"
}

# A least-privilege key scoped to one domain — pair this with a verified
# resend_domain so revoking the key only impacts that domain's traffic.
resource "resend_api_key" "production_sender" {
  name       = "production-sender"
  permission = "sending_access"
  domain_id  = resend_domain.example_com.id
}
