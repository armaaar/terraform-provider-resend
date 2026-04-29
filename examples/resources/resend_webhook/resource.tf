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

resource "resend_webhook" "deliverability" {
  endpoint = "https://hooks.example.com/resend"
  events = [
    "email.delivered",
    "email.bounced",
    "email.complained",
  ]
}
