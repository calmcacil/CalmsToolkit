# GitHub Actions audit

Date: 2026-07-15

This audit records the state before the CI redesign and the resulting controls. The repository is intended to be public; no workflow may trust code from a fork.

## Previous state

The repository had one workflow, `.github/workflows/ci.yml`, named `CI`.

| Area | Previous behavior | Assessment |
| --- | --- | --- |
| Triggers | Pushes and pull requests targeting `main` | Safe trigger choice; duplicate runs were possible around merges, and there was no manual diagnostic trigger. |
| Permissions | Top-level `contents: read` | Good least-privilege default. No secrets were used and no privileged PR event was used. |
| Execution | One `Quality, Test & Build` job on `ubuntu-latest` | Safe for public forks, but serialized unrelated checks and delayed feedback. |
| Actions | `actions/checkout@v4` and `actions/setup-go@v5` | Maintainer-controlled actions, but mutable major tags did not provide supply-chain immutability. |
| Formatting | `gofmt` diff check | Appropriate and inexpensive. |
| Modules | `go mod tidy` plus a diff check | Useful, but `go mod verify` was absent. |
| Static analysis | `go vet` | Useful but narrow; the checked-in golangci-lint policy was not enforced. |
| Tests | Race tests with coverage | Strong unit gate. Coverage was displayed but not persisted or summarized clearly. |
| Builds | `go build ./cmd/...` | Verified the host build, not both supported Linux architectures. |
| Security | None | No dependency review, CodeQL, or Go vulnerability analysis. |
| Release | None | Tags, notes, version selection, and binaries were manual or undefined. |

There was no `pull_request_target`, no self-hosted runner, no secret use, no artifact ingestion, and no untrusted code running with write permissions. The old workflow therefore had no direct secret-exposure path. Its main security weakness was missing analysis and mutable action references rather than an actively privileged design.

## Identified issues

- CI did not enforce the repository's golangci-lint policy, action syntax, module checksums, command/documentation registration, or the single-entrypoint architecture.
- A single job made failures less obvious and prevented independent required checks.
- Supported Linux `amd64` and `arm64` release builds were not both validated.
- There was no dependency-change review, CodeQL scan, or `govulncheck` gate.
- Action references were not pinned to immutable commit SHAs.
- No dependency update automation existed for Go modules or actions.
- No release automation, release credential boundary, or reproducible artifact configuration existed.
- Superseded runs were cancelled only by workflow/ref; the new grouping makes PR intent explicit.
- The v6/v7 changelog described private development history even though public versioning must begin at v1.

## Resulting workflows

### CI

`.github/workflows/ci.yml` runs for pull requests and pushes to `main`, with a manual trigger for maintainers. It has four independently visible jobs:

- **Quality**: formatting, tidy/diff, module verification, vet, golangci-lint, actionlint, PR-title convention, architecture boundaries, and command documentation.
- **Tests**: race-enabled tests and a coverage job summary.
- **Linux builds**: static builds for Linux `amd64` and `arm64` with injected build information.
- **Vulnerabilities**: pinned `govulncheck` across all packages.

The split makes every required gate explicit while setup-go caching and concurrency cancellation limit repeat work. Tests are not sharded because this repository is small and repeated Go setup would cost more than it saves.

### Security

`.github/workflows/security.yml` runs dependency review on pull requests and CodeQL on pull requests, `main`, a weekly schedule, and manual dispatch. Dependency review does not check out or execute pull-request code. CodeQL receives only the scopes it requires; all other jobs retain `contents: read`.

### Release

`.github/workflows/release.yml` runs only from the canonical repository's trusted `main` ref. Release Please uses a short-lived GitHub App installation token to maintain a release PR and create the tag/release after that PR is manually merged. GoReleaser runs only when Release Please reports a newly created release, checks out that exact tag without persisting credentials, and attaches Linux archives plus checksums.

The default `GITHUB_TOKEN` stays read-only. The release app should be installed only on this repository with **Contents: read/write** and **Pull requests: read/write**. Its private key is stored only as an Actions secret. Fork pull requests cannot invoke the release workflow or access these secrets.

## Security analysis

- Every third-party action is pinned to a full commit SHA with a version comment for reviewability.
- There is no `pull_request_target`, privileged checkout of pull-request code, or self-hosted runner assumption.
- Pull-request jobs receive no secrets and only a read-only token. Shell interpolation uses environment variables rather than embedding untrusted PR fields in scripts.
- No pull-request artifact is consumed by a privileged workflow. Release binaries are built from an immutable tag created on protected `main`.
- Go's module cache is managed by `setup-go`. Fork pull requests can restore eligible caches but cannot write repository caches, limiting cache-poisoning risk.
- Release credentials use a short-lived GitHub App token rather than a long-lived PAT. OIDC is unnecessary because no external package registry or cloud provider is used.
- Dependabot keeps action SHAs and Go dependencies reviewable and current.
- License checking in dependency review is deliberately disabled: automated license classification is noisy for this small binary, while vulnerability severity `moderate` or above is enforced.

## Runtime and cost assessment

Public repositories using standard GitHub-hosted runners receive free, effectively unlimited standard runner minutes under GitHub's current plan model. Costs can still arise from larger runners, paid storage beyond allowances, external/self-hosted infrastructure, or abuse that violates acceptable-use limits. Plan terms can change and should be checked before enabling larger runners.

This design uses only standard `ubuntu-latest` runners. It cancels superseded CI/security runs, caches downloaded Go modules, avoids uploading routine artifacts, and keeps scheduled security work weekly. Fast checks remain on every PR because path-skipping them would create ambiguous required checks and documentation/config changes can affect delivery. Release work runs only after a merge and only builds when a release was actually created.

## Deliberate omissions

- No spell checker: the repository is small and the false-positive/configuration burden exceeds its current value.
- No generated-file gate: the repository has no committed code generation contract.
- No separate integration matrix: HTTP/service behavior uses deterministic Go tests; live credentials must never run on public PRs.
- No SBOM, provenance, signing, package publication, larger runners, or merge queue at this stage. These add operational cost without a current distribution requirement.
