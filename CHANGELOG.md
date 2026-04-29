# Changelog

## 1.0.0 — 2026-04-29

Initial fork release. Forked from `chronark/terraform-provider-resend` and substantially overhauled.

### Breaking from the upstream

- Module path changed from `github.com/chronark/terraform-provider-resend` to `github.com/armaaar/terraform-provider-resend`. Provider source becomes `registry.terraform.io/armaaar/resend`.
- `resend_domain.dns_provider` removed — Resend's API does not actually return this field on Get.

### Features

- **`resend_domain.records` is now wired up** as a Computed nested list. Every record has `record`, `name`, `type`, `ttl`, `status`, `value` and `priority`. This is the headline reason the fork exists — pipe `records` into `cloudflare_record` (or any DNS provider) with `for_each`.
- New `resend_domain` fields: `open_tracking`, `click_tracking`, `tracking_subdomain`, `tls`, `custom_return_path`, and a Computed `capabilities` object exposing `sending` / `receiving`.
- `resend_domain` `Update` is implemented (was previously stubbed). Changes to `open_tracking`, `click_tracking`, `tracking_subdomain` and `tls` apply in-place.
- `resend_domain.region` accepts `ap-northeast-1` in addition to the previously-documented regions.
- New `resend_webhook` resource — full CRUD plus `signing_secret` round-trip on Read and Import.
- Added paginated `Read` to `resend_api_key` so out-of-band deletion is reflected at the next refresh.
- New data sources mirroring every resource: `data.resend_domain` / `data.resend_domains` / `data.resend_api_key` / `data.resend_api_keys` / `data.resend_webhook` / `data.resend_webhooks`. Singular forms look up by ID; plural forms paginate the list endpoint. The domain singular includes the supplemental `tls` + `capabilities` from `resendx`; the webhook singular includes `signing_secret`.

### Fixes

- Removed `log.Println(os.Environ())` from provider `Configure`. The previous behaviour leaked the entire process environment into Terraform's log on every plan/apply.
- Removed the API key from the `tflog.Info` banner (the previous "Creating Resend API client `<key>`" leaked the key on every Configure).
- Provider now actually honours the `RESEND_API_KEY` env var fallback. Previously the schema marked `api_key` `Required`, and the `NewClient` calls used the config value directly even when the resolved local was different.

### Internal

- Bumped Go floor to 1.25 and `terraform-plugin-framework` to v1.19.0.
- Migrated SDK import from `github.com/resendlabs/resend-go v1.7.0` to `github.com/resend/resend-go/v3 v3.6.0`.
- All SDK calls now go through the context-aware variants (`*WithContext`).
- Added an in-tree HTTP supplement package `internal/resendx` for the few fields Resend's REST API exposes but the SDK omits (`tls`, `capabilities`).
