# CI and release migration

## What changed

- The previous combined quality/test/build job became independently required quality, test, Linux build, and vulnerability jobs.
- golangci-lint, actionlint, module verification, architecture/documentation checks, dependency review, and CodeQL are now enforced.
- All actions are pinned to immutable SHAs and workflows use explicit least-privilege permissions.
- Superseded pull-request runs are cancelled; Go dependency caching remains enabled.
- Dependabot now proposes weekly grouped Go and Actions updates.
- Release Please now prepares semantic release pull requests, and GoReleaser attaches Linux archives and checksums after a release is created.
- Public version history starts at `v1.0.0`; private v6/v7 labels were removed from the changelog.
- The repository is now MIT licensed.

## Maintainer migration checklist

1. Make the repository public, then enable dependency graph, Dependabot alerts/security updates, secret scanning with push protection, and CodeQL.
2. Create and install the narrowly scoped release GitHub App described in `docs/CI_RELEASES.md`.
3. Add `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` as Actions secrets.
4. Merge this infrastructure change and wait for all job context names to appear at least once.
5. Protect `main` with the exact checks and settings documented in `docs/CI_RELEASES.md`.
6. Confirm merge commits and rebase merges are disabled and squash merge is enabled.
7. Merge future feature PRs with Conventional Commit titles.
8. Review and manually merge the first Release Please PR; verify it proposes `v1.0.0` and that both Linux archives and `checksums.txt` appear on the GitHub Release.

Do not manually create the initial tag before Release Please runs. If the release App is unavailable, leave the generated release PR unmerged until credentials are repaired; do not substitute a maintainer PAT in the workflow.
