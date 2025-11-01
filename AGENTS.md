# AGENTS.md - AI Coding Agent Guidelines

## Build & Test Commands
- Build all: `make build` (outputs to `bin/`)
- Build all platforms: `make build-all`
- Test all: `make test` (runs all tagged test suites)
- Test single file: `go test -tags mediarequests -v -run TestFunctionName` (requires matching build tag)
- Format code: `go fmt ./...` (REQUIRED before commit)
- Tidy deps: `make tidy` or `go mod tidy`

## Code Style
- **Imports**: stdlib first, then external deps (tablewriter, term), blank line between groups
- **Formatting**: Use `go fmt ./...` on all changes; 100% idiomatic Go
- **Types**: Exported = UpperCamelCase + doc comment; unexported = lowerCamelCase
- **Constants**: ALL_CAPS for env vars shown to users (e.g., `OVERSEERR_TOKEN`); UpperCamelCase for internal consts (e.g., `StatusPending`)
- **Error handling**: Always check errors; prefer explicit `if err != nil` over inline checks
- **Naming**: Avoid stuttering (e.g., `request.RequestID` → `request.ID`); follow Go conventions (receiver names 1-2 letters)

## Testing
- Use table-driven tests (`tests := []struct { name string; ... }`)
- Place tests in `*_test.go` beside feature code with matching `//go:build` tag
- Mock HTTP with `httptest.NewServer`; cover happy path + failure modes
- Run full suite (`make test`) before committing

## Project Structure
- Five CLI binaries at root: `media-requests.go`, `media-streams.go`, `media-calendar.go`, `arr-feed.go`, `queue-remediation.go`
- Each requires build tag (e.g., `//go:build mediarequests`) and matching test file tag
- Research/docs in `docs/`; env template in `.env.example`; never commit secrets
