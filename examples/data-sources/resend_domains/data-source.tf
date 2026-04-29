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

# List every Resend domain in the account — handy for inventory dashboards or
# enforcing fleet-wide policies.
data "resend_domains" "all" {}

output "verified_domains" {
  description = "Names of all domains that Resend has fully verified."
  value       = [for d in data.resend_domains.all.domains : d.name if d.status == "verified"]
}
