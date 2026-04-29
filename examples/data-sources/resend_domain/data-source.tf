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

# Read an existing Resend domain by ID — useful when the domain was created
# in the Resend dashboard or by another Terraform state and you only want to
# reference its records / capabilities here.
data "resend_domain" "shared" {
  id = "dom_abc123"
}

output "shared_domain_status" {
  description = "Verification status of the shared Resend domain."
  value       = data.resend_domain.shared.status
}
