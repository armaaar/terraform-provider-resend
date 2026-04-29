# Contributing

## Requirements

- Go `>= 1.25` (the toolchain in `go.mod` pins `go1.26.2`).
- Terraform `>= 1.5` if you want to run acceptance tests against a real Resend account.
- `golangci-lint` `v2.x` for the lint pass.

## Local dev loop

Install the provider into your `$GOPATH/bin` so a Terraform `dev_overrides` block can pick it up:

```sh
go install
```

Add a `~/.terraformrc` block pointing the source at your local binary. Replace `<gopath>` with `go env GOPATH`:

```hcl
provider_installation {
  dev_overrides {
    "armaaar/resend" = "<gopath>/bin"
  }
  direct {}
}
```

After that, any `terraform plan` / `terraform apply` against a config that uses `armaaar/resend` runs against your local build with no `terraform init` required.

## Build, lint, test

```sh
go build ./...
go vet ./...
golangci-lint run
go test ./...                # unit tests; acceptance tests skip without TF_ACC
```

## Acceptance tests

Acceptance tests create real resources at Resend, which costs money and pollutes the account. Run them deliberately:

```sh
TF_ACC=1 \
  RESEND_API_KEY=re_… \
  RESEND_TEST_BASE_DOMAIN=tf-acc.example.com \
  go test -v -timeout 30m ./internal/provider/...
```

Required env:

| Variable | Why |
| --- | --- |
| `TF_ACC=1` | Tells `helper/resource.Test` to actually run |
| `RESEND_API_KEY` | Bearer token for the account that will own the test resources |
| `RESEND_TEST_BASE_DOMAIN` | The provider's domain test creates random subdomains under this base. Use a domain whose DNS you do not actually need to verify so cleanup is cheap |

Optional:

| Variable | Why |
| --- | --- |
| `RESEND_TEST_AP_REGION=1` | Opt in to the `TestAccDomainResource_apNortheast` test (region availability is account-tier dependent) |

## Documentation

Resource and provider docs under `docs/` are **generated** by `tfplugindocs`. Edit the `MarkdownDescription` strings in the schema definitions, then regenerate:

```sh
go generate ./...
```

Commit the regenerated files alongside the schema change. CI fails the build if `docs/` drifts from the source.

## Commit style

Conventional Commits (`feat`, `fix`, `chore`, `docs`, `test`, `ci`). Scopes are optional but `feat(domain): …` / `fix(api_key): …` read well in the changelog.

Do **not** add `Co-Authored-By` trailers or any "Generated with …" footers.

## Renaming the module path

If you fork this repo to a different org, update in one commit:

- `module github.com/armaaar/terraform-provider-resend` in `go.mod`
- Every import under `internal/`
- `Address` in `main.go`
- Every `source = "registry.terraform.io/armaaar/resend"` in `examples/`

Then re-tag (Registry pulls the new namespace from the tag).
