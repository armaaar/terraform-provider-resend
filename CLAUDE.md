# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Terraform provider for [Resend](https://resend.com) (transactional email). Built on the **Terraform Plugin Framework** (`hashicorp/terraform-plugin-framework`), not the older Plugin SDK v2. Forked from `chronark/terraform-provider-resend`; published to the Terraform Registry as `registry.terraform.io/armaaar/resend`. Wraps `github.com/resend/resend-go/v3`, with a small in-tree HTTP supplement (`internal/resendx`) for the handful of REST fields the SDK still omits.

Currently exposes three resources and no data sources: `resend_domain`, `resend_api_key`, `resend_webhook`.

## Common commands

```shell
# Build / install the provider binary into $GOPATH/bin
go install

# Run acceptance tests (requires RESEND_API_KEY + RESEND_TEST_BASE_DOMAIN;
# creates real resources, costs $)
make testacc                                   # all tests
TF_ACC=1 RESEND_API_KEY=… RESEND_TEST_BASE_DOMAIN=… \
  go test ./internal/provider/ -run TestAccDomainResource -v   # single test

# Unit tests only (no live API)
go test ./...

# Regenerate registry docs from schema + examples (writes to docs/)
go generate ./...

# Lint (matches CI; v2 config)
golangci-lint run

# Run the provider under a debugger
go run . -debug
```

`RESEND_API_KEY` is required for any acceptance test (the provider reads it as the `RESEND_API_KEY` env-var fallback, and `testAccPreCheck` asserts it's set). `RESEND_TEST_BASE_DOMAIN` is required for the domain test (subdomains under it are randomised per run). `RESEND_TEST_AP_REGION=1` opts into the `ap-northeast-1` region test, which not all Resend accounts have access to.

## Architecture

- [main.go](main.go) — entry point. `providerserver.Serve` registers `provider.New(version)` at the address `registry.terraform.io/armaaar/resend`. Hosts the `go:generate` directives that drive doc generation.
- [internal/provider/provider.go](internal/provider/provider.go) — `ResendProvider`. Reads the `api_key` attribute (falling back to `RESEND_API_KEY`), constructs a `*resend.Client` **and** a `*resendx.Client` (the HTTP supplement, version-stamped for User-Agent), bundles them in a `providerClients` struct, and stashes that in `resp.ResourceData` / `resp.DataSourceData` for each resource's `Configure` to type-assert.
- [internal/provider/domain_resource.go](internal/provider/domain_resource.go) — `resend_domain`. Records list, capabilities, tls, custom_return_path, tracking flags. Update is implemented; immutable attributes use `RequiresReplace()`. The `applyExtState` helper merges resendx-supplied fields on Create/Read/Update.
- [internal/provider/api_key_resource.go](internal/provider/api_key_resource.go) — `resend_api_key`. Read paginates `ApiKeys.ListWithOptions(ctx, &resend.ListOptions{After: cursor})` until the matching id is found or `HasMore` is false; missing → `RemoveResource`.
- [internal/provider/webhook_resource.go](internal/provider/webhook_resource.go) — `resend_webhook`. Full CRUD via the SDK; `signing_secret` round-trips because v3.6.0's `Webhook` struct exposes it on Get. 404 detection uses string-matching on the SDK's untyped errors.
- [internal/resendx/](internal/resendx/) — minimal HTTP client wrapping `https://api.resend.com`. Sets `Authorization: Bearer …`, `User-Agent: terraform-provider-resend/<version>`, and decodes 4xx/5xx into `*APIError`. Provides `GetDomain` (returns the supplemental `tls` + `capabilities` fields) and `IsNotFound(err)`.
- [tools/tools.go](tools/tools.go) — Go tools-pattern stub (`//go:build tools`) that pins `tfplugindocs` as a build dependency so `go generate` resolves it.
- [docs/](docs/) — **generated** from resource `MarkdownDescription` fields plus example `.tf` files under [examples/resources/](examples/resources/) and [examples/provider/](examples/provider/). Don't hand-edit `docs/`; edit the schema descriptions or examples and re-run `go generate`. CI's `generate` job fails the build if `docs/` drifts from sources.

### Commits and versioning

- This repo uses [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, `ci:`, `docs:`, `refactor:`, `style:`, `test:`, optionally with a scope and a `!` for breaking changes). The recent history is consistent with this — match it.
- **No** `Co-Authored-By` trailers, **no** "Generated with Claude Code" footers.
- Releases follow [SemVer](https://semver.org/) and are cut by `goreleaser` (see [.goreleaser.yml](.goreleaser.yml)). `feat:` → minor bump, `fix:` → patch bump, anything with `!` or `BREAKING CHANGE:` → major bump.

### Things to know when editing

- Schema descriptions live in `MarkdownDescription` on each `schema.Attribute`; these flow into the generated registry docs, so they should read as user-facing documentation, not internal notes.
- After any schema change, re-run `go generate ./...` and stage the `docs/` drift in the same commit. CI will fail otherwise.
- Acceptance tests in CI run with `max-parallel: 1` across the Terraform-version matrix because they create real Resend resources — keep tests serial-safe and clean up resources via the framework's automatic Delete step.
- The Domain `Update` path uses the SDK's `SetOpenTracking(bool)` / `SetClickTracking(bool)` setters rather than direct field assignment. The SDK has a custom `MarshalJSON` that drops `false` values from omitempty fields without those setters.
- Resend's REST API returns `tls` and `capabilities` on `GET /domains/:id` but the official SDK's `Domain` struct omits both. `internal/resendx` exists solely to fill that gap; do not add fields the SDK already exposes.
- The SDK's webhook errors are untyped strings; 404 detection is currently a `strings.Contains` on `"not_found"` / `"404"`. If the SDK ever adds typed errors, switch to `errors.As`.
- The module path is `github.com/armaaar/terraform-provider-resend`; if you fork into a different org, update `go.mod`, every `internal/` import, `main.go` `Address`, and every `examples/*/source =` in one commit, then re-tag.
