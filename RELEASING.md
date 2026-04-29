# Releasing

End-to-end runbook for cutting a release of `armaaar/resend`. This file assumes the audience has read [CONTRIBUTING.md](CONTRIBUTING.md).

The mechanics:

- The `release.yml` workflow runs on every pushed tag matching `v*`.
- It invokes `goreleaser` (v2) to build cross-platform binaries, sign the SHA256 checksum file with GPG, and upload everything as a GitHub Release.
- The Terraform Registry polls the repo for new tags, fetches the GitHub Release artifacts, validates the GPG signature against the public key on file, and indexes the version.
- An optional **acceptance** workflow (`acceptance.yml`) is `workflow_dispatch`-only and must be run by a human before tagging — it runs `TF_ACC=1 go test` against a real Resend account.

## One-time setup

### 1. Generate a GPG key for release signing

```sh
gpg --full-generate-key   # RSA 4096, no expiry (or 5y if you want forced rotation)
gpg --list-secret-keys --keyid-format LONG
# note the fingerprint after sec   rsa4096/<FINGERPRINT>
```

Export both halves:

```sh
# public — paste into the Terraform Registry
gpg --armor --export <FINGERPRINT>

# private — paste into the GitHub repo secret
gpg --armor --export-secret-keys --pinentry-mode loopback \
    --passphrase "$PASSPHRASE" <FINGERPRINT>
```

### 2. Add GitHub repository secrets

`https://github.com/armaaar/terraform-provider-resend/settings/secrets/actions`:

| Secret | Value |
| --- | --- |
| `GPG_PRIVATE_KEY` | The full ASCII-armored private key block (BEGIN/END lines included) |
| `PASSPHRASE` | The GPG passphrase (omit if your key has none — but tighten access regardless) |
| `RESEND_API_KEY` | Resend API key for the **acceptance test** workflow only — never used by `release.yml` |
| `RESEND_TEST_BASE_DOMAIN` | Base domain for randomised acceptance subdomains |

`GITHUB_TOKEN` is auto-injected by Actions; do not create one.

### 3. Register the provider on the Terraform Registry

1. Sign in at `https://registry.terraform.io/sign-in` with the GitHub account that owns this repo.
2. **User Settings → Signing Keys → Add a Signing Key** → paste the ASCII-armored public key from step 1.
3. **Publish → Provider → armaaar/terraform-provider-resend** → confirm. The Registry scans for tags matching `v*.*.*`; the first scan may find nothing, that's fine — it'll re-scan after the first tag is pushed.

### 4. Confirm the repo is public

The Registry can only fetch public repos. Filesystem-mirror consumers (cee-app) work either way, but the public Registry path requires public visibility.

## Cutting a release

Pre-flight (run locally on a clean clone of `main`):

```sh
git pull --ff-only origin main
git status                                 # nothing pending
go mod tidy && git diff --exit-code go.*   # no dep drift
go vet ./... && golangci-lint run          # clean
go test ./...                              # unit tests + resendx pass
go generate ./... && git diff --exit-code  # docs in sync with schema
goreleaser check                           # .goreleaser.yml valid
goreleaser release --snapshot --clean      # local build of every OS/arch into dist/
```

Then run the **acceptance workflow** against the current `main`:

1. `https://github.com/armaaar/terraform-provider-resend/actions/workflows/acceptance.yml` → **Run workflow** → branch `main` → **Run**.
2. Wait for green. This burns Resend API quota — run it deliberately, not on every push.

Then tag and push:

```sh
# 1. flip CHANGELOG's "Unreleased" header to the version + date
$EDITOR CHANGELOG.md

# 2. commit the changelog
git commit -am "chore: prepare v1.0.0"

# 3. annotated tag (lightweight tags are silently ignored by goreleaser)
git tag -a v1.0.0 -m "v1.0.0"

# 4. push commit + tag
git push origin main
git push origin v1.0.0
```

The release workflow takes ~2-4 minutes. Watch it at `https://github.com/armaaar/terraform-provider-resend/actions`.

## Post-release verification

In order — each step depends on the previous:

| Layer | Check |
| --- | --- |
| Tag pushed | `git ls-remote --tags origin v1.0.0` returns the SHA |
| Workflow succeeded | `gh run list --workflow=release.yml -L 1` shows green |
| GitHub Release created | `gh release view v1.0.0` lists ~10 OS/arch zips, `_SHA256SUMS`, `_SHA256SUMS.sig`, `_manifest.json` |
| GPG signature valid | Download both files, `gpg --verify <SHA256SUMS.sig> <SHA256SUMS>` says *Good signature* |
| Registry indexed | `https://registry.terraform.io/providers/armaaar/resend/1.0.0` renders the docs (usually within 10 minutes of the workflow finishing) |
| End-to-end | Throwaway Terraform config with `source = "armaaar/resend", version = "1.0.0"`, `terraform init`, `terraform plan` against a real Resend account |

## Hotfix release

Identical to the first release with shorter prep:

```sh
git pull --ff-only origin main
# ...fix...
git commit -am "fix(domain): handle empty records on partially_failed status"
# bump CHANGELOG, tag, push
$EDITOR CHANGELOG.md
git commit -am "chore: prepare v1.0.1"
git tag -a v1.0.1 -m "v1.0.1"
git push origin main v1.0.1
```

Existing v1.0.0 stays available; consumers with `~> 1.0` or no constraint get the upgrade on the next `terraform init -upgrade`.

## Failure modes and recovery

### "Goreleaser failed: invalid GPG signature"

The signing step in `release.yml` couldn't import the private key. Likely causes: `GPG_PRIVATE_KEY` secret is malformed (missing BEGIN/END lines, line endings stripped, base64-encoded by accident), or `PASSPHRASE` doesn't match the key. Re-export and re-paste both.

### "Registry shows: signature verification failed"

The release uploaded fine, but the Registry's check against the public key on file failed. Causes: the public key in your Registry settings is for a different key than the one signing the release. Re-export the public key from the *same* keyring you exported the private key from, replace it in Registry settings, and trigger a re-scan by pushing a small fixup tag (e.g. `v1.0.1` with a no-op commit).

### "Tag pushed but no workflow run"

The workflow's trigger is `tags: ['v*']`. Ensure your tag matches exactly (`v1.0.0`, not `1.0.0` or `release-1.0.0`). Lightweight tags (created with `git tag v1.0.0` without `-a`) trigger the workflow but are silently skipped by goreleaser. Always use `git tag -a`.

### "Goreleaser builds locally but fails in CI"

Usually a Go version mismatch. CI's `setup-go` reads `go.mod`'s `go` directive; locally you might be running a different toolchain. Pin `go-version` explicitly in the workflow if drift recurs.

### "I tagged the wrong commit"

You **cannot** safely move a tag once the Registry has indexed it. Path of least pain: ship the right code as `v1.0.1` and let the version constraint trickle through. If the bad release hasn't been indexed yet (workflow failed early, no GitHub Release exists), you can `git push origin :refs/tags/v1.0.0` and re-tag — but verify nothing is published before doing so.

### "I want to delete a release"

You can't. Once published to the Registry, a version is immutable. You can `gh release delete v1.0.0` from GitHub itself, but the Registry has cached the artifacts and will continue serving them. Treat any tagged version as permanent.

## Renaming the module path

If the fork ever moves (e.g. to a real GitHub org), this becomes a brand-new provider on the Registry — `v1.0.0` under `armaaar/resend` and any new namespace are unrelated as far as the Registry is concerned. Plan accordingly:

1. Update `go.mod`'s module declaration.
2. Update every internal import.
3. Update `main.go`'s `Address`.
4. Update every `examples/*/source =`.
5. Update `RELEASING.md` and `CONTRIBUTING.md`'s namespace references.
6. Re-do this entire one-time-setup section under the new namespace (new GPG key not required, but a new Registry "publish a provider" step is).

Tag a fresh major (`v2.0.0`) under the new namespace; existing consumers will need to change their `source =` to migrate.
