# Repository Guidelines

## Project Structure & Module Organization
CalmsToolkit is a Go module with one Linux CLI at `cmd/calmstoolkit`. Never add a standalone binary. Cobra registration lives in `internal/cli`, runtime injection in `internal/app`, shared rendering in `internal/console`, configuration in `internal/config`, and feature/domain behavior elsewhere under `internal/`. Research notes are under `docs/research/` and normative user docs under `docs/user/`.

## Build, Test, and Development Commands
Use `make build`, `make build-all` (Linux amd64/arm64), `make test`, and `make tidy`. `make check` is the full gate. Run commands with `go run ./cmd/calmstoolkit <command>`. Configuration is managed through `calmstoolkit config setup|validate` and `doctor`.

For CI-equivalent validation also run the pinned golangci-lint, actionlint, and govulncheck commands in `docs/CI_RELEASES.md`. Do not change workflows without updating the audit/migration docs and retaining explicit permissions plus immutable action SHAs.

## Coding Style & Naming Conventions
Format every change with `gofmt` (or `go fmt ./...`) before committing; the project mirrors idiomatic Go spacing and grouping. Keep exported types, funcs, and consts in UpperCamelCase with doc comments, use lowerCamelCase for locals, and reserve ALL_CAPS for environment variable labels surfaced to users. Group related constants and config structs to keep CLI flag wiring readable.

## Testing Guidelines
Place table-driven tests in `_test.go` files beside the feature under test and cover both happy paths and API failure modes (mock HTTP responses where feasible). Target at least the functionality added in each PR; run `make test` or `go test ./...` before pushing, and capture notable coverage regressions in the PR notes.

New commands must test terminal, plain, JSON/NDJSON, failure, and cancellation paths. Foundational packages `app`, `console`, `config`, `httputil`, and new domain clients target at least 80% coverage. JSON/NDJSON tests must assert schema validity and absence of ANSI sequences.

## Commit & Pull Request Guidelines
Use Conventional Commit subjects and PR titles (`feat(calendar): add filters`). Squash incidental fixups locally so each commit is reviewable; the final PR title becomes the squash commit and drives Release Please. Pull requests should link the relevant issue, summarize behavioral changes, call out new flags or env vars, and include screenshots or sample CLI output when they clarify UX changes. Flag breaking or security-sensitive configuration updates explicitly.

## Configuration & Secrets
The shared config file at `~/.config/calmstoolkit/config.json` stores API keys and tokens. It is created with `0600` permissions; never commit it. Local overrides should rely on shell exports or direnv, and scrub logs or fixtures before sharing them in reviews.

## Mandatory architecture boundaries

Only `cmd/calmstoolkit/main.go` may call `os.Exit`. Commands return errors through the shared runtime. Use `internal/console` instead of feature-local width, box, palette, or screen-control frameworks, and use typed clients over `internal/httputil`. Logs and diagnostics go to stderr; machine stdout contains envelopes only. Interface changes require user docs and migration notes. Follow `docs/ADDING_A_TOOL.md` and `docs/ARCHITECTURE.md`.
