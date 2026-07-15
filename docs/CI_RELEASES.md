# CI and releases

## Pull request validation

Every pull request to `main` runs `CI` and `Security`. The required status contexts are:

- `CI / Quality`
- `CI / Tests`
- `CI / Linux builds`
- `CI / Vulnerabilities`
- `Security / Dependency review`
- `Security / CodeQL (Go)`

`Quality` is the fast correctness and maintainability gate. `Tests` catches behavior and data-race regressions. `Linux builds` verifies both supported distribution targets. `Vulnerabilities` detects reachable known Go vulnerabilities. Dependency review blocks newly introduced vulnerable dependencies at moderate severity or higher, while CodeQL supplies semantic security analysis.

Contributors should run `make check` and the commands below before pushing:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --timeout=5m
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
```

Pull request titles must use Conventional Commits because squash merging turns that title into the release input. Examples are `fix(config): preserve environment precedence` and `feat!: unify the CLI`.

## Required repository settings

Configure a branch ruleset or protection rule for `main` with:

- Require a pull request before merging, with zero required approvals.
- Dismiss stale approvals if approvals are added later.
- Require all six status checks above and require the branch to be up to date.
- Require conversation resolution and linear history.
- Apply the rule to administrators; disallow force pushes and branch deletion.
- Allow squash merging only. Disable merge commits and rebase merging so the PR title is the one release input.
- Enable auto-merge if desired, but keep Release Please release PRs manually merged.
- Do not require signed commits yet: GitHub squash commits and automation identity make enforcement awkward without materially improving this threat model.
- Do not enable a merge queue yet: the repository's traffic does not justify its operational complexity.

Repository Actions settings should leave the default workflow token at **read repository contents** and disallow workflows from approving pull requests. Allow only actions used by the checked-in workflows, or require full-length SHA pinning through organization/repository policy where available.

Dependency graph, Dependabot alerts/security updates, secret scanning with push protection, and CodeQL default capabilities should be enabled when the repository becomes public.

## Release model

The project uses SemVer, Conventional Commit squash titles, Release Please, and GoReleaser.

- `fix` produces a patch release.
- `feat` produces a minor release.
- `!` or a `BREAKING CHANGE` footer produces a major release.
- Documentation, test, CI, and chore changes remain in the release PR but normally do not force a release alone.

Release Please is preferred over semantic-release because it is GitHub-native, keeps a human-reviewable release PR, and does not introduce a Node release runtime. Changesets is strongest for multi-package repositories and would add manual files to every PR. Pure GoReleaser handles artifacts well but does not replace release-version/changelog management. Release Please plus GoReleaser cleanly separates version policy from binary production.

The initial manifest is `0.0.0`; the first breaking feature release becomes public `v1.0.0`. Private v6/v7 development labels are intentionally not part of the public release history.

### Release credentials

Create a GitHub App installed only on `calmcacil/CalmsToolkit` with repository permissions:

- Contents: read and write
- Pull requests: read and write
- Metadata: read (implicit)

Store its App ID as `RELEASE_APP_ID` and its PEM private key as `RELEASE_APP_PRIVATE_KEY` in Actions secrets. Do not expose either as a repository variable or environment variable in pull-request jobs.

After a normal PR is squash-merged, Release Please opens or updates its release PR. Review and manually squash-merge that PR. The resulting trusted `main` run creates the tag and GitHub Release, then GoReleaser attaches:

- `calmstoolkit_<version>_linux_amd64.tar.gz`
- `calmstoolkit_<version>_linux_arm64.tar.gz`
- `checksums.txt`

There is no package registry publication, OIDC trust, SBOM, or provenance attestation in the initial design.

## Maintenance

Dependabot opens weekly grouped updates for Go dependencies and GitHub Actions. It prefixes their titles with `build(deps):` and `ci(deps):`, respectively, so generated PRs pass the Conventional Commit gate. Action updates must remain pinned to an immutable SHA and retain a version comment. Review upstream release notes before merging major action changes.

To test the artifact definition without publishing:

```bash
go run github.com/goreleaser/goreleaser/v2@v2.12.7 check
go run github.com/goreleaser/goreleaser/v2@v2.12.7 release --snapshot --clean
```
