# terraform-provider-resend

Terraform provider for [Resend](https://resend.com) (transactional email). Manages sending domains, API keys and webhooks against Resend's REST API.

This is a fork of the original [chronark/terraform-provider-resend](https://github.com/chronark/terraform-provider-resend) — the original was tied to the renamed `resendlabs/resend-go` org, didn't expose `records` (so DNS could not be wired into Cloudflare et al.), and stubbed out `Update` and `Read`. This fork closes those gaps, tracks the latest `resend/resend-go/v3` SDK, and adds a `resend_webhook` resource. See [CHANGELOG.md](CHANGELOG.md) for the full diff.

## Requirements

- Terraform `>= 1.5`
- Go `>= 1.25` (for development only)
- A Resend API key (`re_…`)

## Installation

### From the Terraform Registry

```hcl
terraform {
  required_providers {
    resend = {
      source  = "armaaar/resend"
      version = "~> 1.0"
    }
  }
}
```

### From a filesystem mirror (cee-app pattern)

For internal monorepos that prefer not to depend on the public Registry, drop the binary into the local mirror path Terraform auto-discovers:

```text
~/.terraform.d/plugins/github.com/armaaar/resend/<version>/<os>_<arch>/terraform-provider-resend_v<version>
```

Then declare the same `source` you used for the install path:

```hcl
terraform {
  required_providers {
    resend = {
      source  = "github.com/armaaar/resend"
      version = "1.0.0"
    }
  }
}
```

A small install script that downloads the right binary from a GitHub Release for the current OS/arch lives in cee-app at `infra/scripts/install-resend-provider.sh` and is also expected to run in CI before `terraform init`.

## Authentication

The provider needs a Resend API key. Either set it inline:

```hcl
provider "resend" {
  api_key = var.resend_api_key
}
```

…or via the `RESEND_API_KEY` environment variable. The config value takes precedence when both are set.

## Resources

### `resend_domain`

Manages a Resend sending domain. The **`records` attribute is the practical reason this resource exists** — pipe it into your DNS provider with `for_each` to verify the domain.

```hcl
resource "resend_domain" "example_com" {
  name               = "example.com"
  region             = "us-east-1" # us-east-1 | eu-west-1 | sa-east-1 | ap-northeast-1
  open_tracking      = true
  click_tracking     = true
  tracking_subdomain = "track"
  tls                = "enforced" # enforced | opportunistic
}

resource "cloudflare_record" "resend" {
  for_each = {
    for r in resend_domain.example_com.records : "${r.record}-${r.name}" => r
  }

  zone_id  = var.cloudflare_zone_id
  name     = each.value.name
  type     = each.value.type
  content  = each.value.value
  priority = each.value.priority
  ttl      = 1
}
```

Caveats:

- `custom_return_path` is settable on Create only. Resend's GET endpoint does not return it, so changes force replacement rather than pretending to drift-detect.
- `tls`, `open_tracking`, `click_tracking`, `tracking_subdomain` round-trip via Read.
- `capabilities.sending`/`receiving` come from a small in-tree HTTP supplement (`internal/resendx`) because the official SDK omits them.

### `resend_api_key`

Manages a Resend API key. Token is returned only on Create — losing it means recreating the key.

```hcl
resource "resend_api_key" "production_sender" {
  name       = "production-sender"
  permission = "sending_access" # full_access | sending_access
  domain_id  = resend_domain.example_com.id
}
```

`name`, `permission` and `domain_id` are all immutable; changing any of them forces replacement.

### `resend_webhook`

Manages a webhook subscription. Resend POSTs to `endpoint` for the listed `events`. The HMAC `signing_secret` is returned on Create and Read; treat it as sensitive credential material.

```hcl
resource "resend_webhook" "deliverability" {
  endpoint = "https://hooks.example.com/resend"
  events = [
    "email.delivered",
    "email.bounced",
    "email.complained",
  ]
}
```

For the authoritative list of events see [Resend's webhook event types](https://resend.com/docs/dashboard/webhooks/event-types).

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for local dev loop, lint/test conventions, and acceptance test setup.

## License

[MPL-2.0](LICENSE)
