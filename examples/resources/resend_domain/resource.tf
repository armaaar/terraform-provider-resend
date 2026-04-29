terraform {
  required_providers {
    resend = {
      source = "registry.terraform.io/armaaar/resend"
    }
    cloudflare = {
      source = "cloudflare/cloudflare"
    }
  }
}

provider "resend" {}


resource "resend_domain" "example_com" {
  name               = "example.com"
  region             = "us-east-1"
  open_tracking      = true
  click_tracking     = true
  tracking_subdomain = "track"
  tls                = "enforced"
}

# Pipe Resend's records straight into Cloudflare. Resend's Record.Record value
# (SPF / DKIM / Tracking / TrackingCAA) keeps neighbours unique inside the map.
resource "cloudflare_record" "resend" {
  for_each = {
    for r in resend_domain.example_com.records : "${r.record}-${r.name}" => r
  }

  zone_id  = "your-cloudflare-zone-id"
  name     = each.value.name
  type     = each.value.type
  content  = each.value.value
  priority = each.value.priority
  ttl      = 1 # auto
}
