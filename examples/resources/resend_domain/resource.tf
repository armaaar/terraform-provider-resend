terraform {
  required_version = ">= 1.5"

  required_providers {
    resend = {
      source  = "armaaar/resend"
      version = "~> 1.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }
}

provider "resend" {}

variable "cloudflare_zone_id" {
  description = "Cloudflare DNS zone ID under which the Resend verification records will be created."
  type        = string
}

resource "resend_domain" "example_com" {
  name               = "example.com"
  region             = "us-east-1"
  open_tracking      = true
  click_tracking     = true
  tracking_subdomain = "track"
  tls                = "enforced"
}

# Pipe Resend's records straight into Cloudflare. Resend's record class
# (SPF / DKIM / Tracking / TrackingCAA) keeps neighbours unique inside the map.
resource "cloudflare_record" "resend" {
  for_each = {
    for r in resend_domain.example_com.records : "${r.record}-${r.name}" => r
  }

  zone_id  = var.cloudflare_zone_id
  name     = each.value.name
  type     = each.value.type
  content  = each.value.value
  priority = each.value.priority
  ttl      = 1 # auto
}
