# AGENTS.md - AI Coding Agent Guidelines

**Updated:** 2026-01-27 | **Repo:** CalmsToolkit | **Branch:** feature/queue-remediation-tool

## Overview
CalmsToolkit is a flat, multi-binary Go CLI toolkit for media server management
(Plex/Jellyfin streams, Overseerr requests, Sonarr/Radarr queues). Each tool is
enabled via build tags and compiled as a separate binary.

## Build, Test, Tidy
```bash
make build              # Build all binaries into bin/
make build-all          # Cross-compile (linux/darwin/windows/freebsd × amd64/arm64)
make install            # Install to /usr/local/bin (configurable via prefix)
make clean              # Remove build artifacts
make test               # Run tagged test suites sequentially
make tidy               # go mod tidy
go fmt ./...            # REQUIRED before commit
```

### Single Test / Single Tool
```bash
# Run a single test by name (tag required)
go test -tags mediarequests -v -run TestFunctionName ./...
go test -tags queueremediation -v -run TestFunctionName ./...

# Build a single tool
go build -tags mediastreams -o bin/media-streams media-streams.go
go build -tags queueremediation -o bin/queue-remediation \
  queue-remediation.go queue-remediation-shared.go queue-remediation-tui.go
```

### Linting
No dedicated linter is configured. Use `go fmt ./...` for formatting and keep
the codebase consistent with existing patterns.

## Project Structure
```
./
├── media-requests.go      # Overseerr/Jellyseerr interactive menu (tag: mediarequests)
├── media-streams.go       # Plex/Jellyfin unified monitor (tag: mediastreams)
├── media-calendar.go      # Sonarr/Radarr upcoming calendar (tag: mediacalendar)
├── arr-feed.go            # Sonarr/Radarr activity feed (tag: arrfeed)
├── queue-remediation*.go  # Queue fixer + TUI (tag: queueremediation) [3 files]
├── *_test.go              # Co-located tests with matching build tags
├── tooling_stub.go        # Empty stub for untagged builds/IDE support
├── docs/                  # Research, API docs, implementation guides
└── bin/                   # Build output (gitignored)
```

## Build Tag Matrix
| Binary | Tag | Source Files |
|--------|-----|--------------|
| media-requests | `mediarequests` | `media-requests.go` |
| media-streams | `mediastreams` | `media-streams.go` |
| media-calendar | `mediacalendar` | `media-calendar.go` |
| arr-feed | `arrfeed` | `arr-feed.go` |
| queue-remediation | `queueremediation` | `queue-remediation.go`, `queue-remediation-shared.go`, `queue-remediation-tui.go` |

## Code Style
- **Imports**: stdlib first, blank line, then external (tablewriter, bubbletea, lipgloss)
- **Formatting**: always run `go fmt ./...` after edits
- **Build tags**: keep `//go:build <tag>` at top of tool files and tests
- **Types**: exported = UpperCamelCase + doc comment; unexported = lowerCamelCase
- **Constants**: ALL_CAPS for user-facing env vars (`OVERSEERR_TOKEN`)
- **Internal consts**: UpperCamelCase (`StatusPending`)
- **Naming**: avoid stuttering (`request.ID` not `request.RequestID`)
- **Receivers**: 1-2 letters, consistent per type
- **Struct tags**: prefer explicit JSON tags, omit with `json:"-"` when needed

## Error Handling
- Always check and return errors; avoid `_` and empty catch blocks
- Prefer explicit `if err != nil { ... }` blocks
- For CLI exits, print errors to stderr and exit non-zero
- Dry-run mode must not mutate state

## Logging
- CLI output is primarily user-facing; keep logs clear and minimal
- Prefer stderr for error messages

## Testing Patterns
- **Table-driven** tests: `tests := []struct { name string; ... }` with `t.Run()`
- **HTTP mocking**: `httptest.NewServer` for API tests
- **Tag coupling**: test files MUST match source file build tags
- **Coverage**: happy path + error cases (400/401/404/500) + edge cases
- **HAR-aligned**: queue remediation uses real Sonarr/Radarr captures for test data

## Configuration Hierarchy
1. `.env` file at `/opt/apps/compose/.env` (lowest priority)
2. Environment variables (`PLEX_URL`, `SONARR_TOKENS`, etc.)
3. Command-line flags (highest priority)

Multi-instance support: `SONARR_URLS=url1,url2` with matching `SONARR_TOKENS=token1,token2`

## Key Dependencies
- `charmbracelet/bubbletea` + `lipgloss`: TUI framework (media-requests, queue-remediation)
- `olekukonko/tablewriter`: Table formatting
- `golang.org/x/term`: Terminal detection

## API Patterns
- **Sonarr/Radarr**: always include `X-Api-Key` header
- **Manual Import**: use `/api/v3/command` with `ManualImport` name
- **Overseerr**: fallback logic for Bug #3949 (pagination issues)
- **Plex**: XML API at `/status/sessions`; Jellyfin: JSON at `/Sessions`

## Anti-Patterns (Never)
- Commit secrets (tokens in `.env.example` are placeholders only)
- Suppress errors with `_` or empty catch blocks
- Create files without matching build tags when adding to a tool
- Skip `go fmt` before commit
- Modify state in dry-run mode paths

## Documentation Index
- `docs/README_*.md` - tool-specific usage guides
- `docs/SONARR_RADARR_QUEUE_API*.md` - queue API reference
- `docs/MANUAL_IMPORT_*.md` - import workflow research
- `docs/tui_uplift/` - TUI modernization plan

## Cursor/Copilot Rules
- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` files found.
