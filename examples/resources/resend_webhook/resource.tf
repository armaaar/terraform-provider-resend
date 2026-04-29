terraform {
  required_providers {
    resend = {
      source = "registry.terraform.io/armaaar/resend"
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
