# Changelog

## [6.0.0] — 2026-06-01

Architecture refactor — no user-facing API changes.

### Added
- `--version` flag on every binary (injected via ldflags).
- `internal/buildinfo` package with version/commit/date metadata.
- `internal/cmdutil` package for shared flag parsing, theme validation, and config loading.
- `internal/slogx` package with structured logging (text/JSON) via `log/slog`.
- `internal/core` package with `CommonConfig` embedded in every tool — removes 5x duplication of timeout/no-color/theme parsing.
- `internal/datex` package with flexible `DateTime` JSON type.
- `httputil.DoJSONWithRetry` / `DoXMLWithRetry` with exponential backoff and `Retry-After` respect.
- `httputil.Client.MaxBodySize` field (configurable per tool).
- `make clean-all` target for hygiene.
- CI coverage report step.
- Per-package test coverage for `core`, `datex`, `slogx`.

### Changed
- `docs/` reorganized into `docs/user/` and `docs/research/`.
- Dev docs (`AGENT_QUICKSTART.md`, `CLAUDE.md`, `WORKTREE_*`) moved to `.opencode/`.
- `internal/requests` split into `model.go`, `client.go`, `menu.go`, `render.go`.
- `internal/feed` split into `model.go`, `client.go`, `render.go`.
- `internal/streams` split into `model.go`, `client.go`, `render.go`.
- All 5 `cmd/*/main.go` files refactored to use `cmdutil` (net -120 LOC).
- All 5 `ToolConfig` structs now embed `core.CommonConfig`.
- Response body in error messages truncated to 512 bytes.

### Removed
- Stray `media-airtime` binary at repo root.
- Hard-coded 10 MiB body cap in `DoRequest`.
- Duplicated theme/flag/validation code in every `main.go`.
- Duplicated timeout/no-color/theme parsing in every `BuildToolConfig`.

### Fixed
- Error messages now consistently go to stderr (via slog).
- Theme validation centralized (5x → 1x).
