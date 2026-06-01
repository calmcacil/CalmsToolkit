# Contributing

## Quick start

```bash
make build    # Compile all tools
make test     # Run all tests
make lint     # golangci-lint
make fmt      # gofmt
make clean    # Remove build artifacts
make clean-all # Remove build artifacts + stray binaries
```

## Code conventions

- `data` with `gofmt` before committing (`make fmt`).
- `go vet ./...` must pass.
- Test with `go test -race -count=1 ./...`.
- All `cmd/*/main.go` files use `internal/cmdutil` for shared flag parsing.
- New tool config structs embed `internal/core.CommonConfig`.
- Shared HTTP calls use `internal/httputil` with retry.
- Logging goes through `log/slog` via `internal/slogx`.
- Build metadata is injected via `internal/buildinfo` and `-ldflags -X`.

## Project structure

```
cmd/<tool>/main.go       — Thin entry point (flag parsing + cfg wiring)
internal/<feature>/      — Feature packages (airtime, calendar, feed, requests, streams)
internal/cmdutil/        — Shared flag/validation helpers
internal/config/         — ToolkitConfig, ArrInstance, Load/Save/Validate
internal/core/           — CommonConfig embedded in every tool
internal/colors/         — ANSI palette system (default, catppuccin)
internal/datex/          — Flexible JSON date/time type
internal/httputil/       — HTTP client wrapper with retry
internal/slogx/          — Structured logger (text/JSON)
internal/buildinfo/      — Version/commit/date injected at build
```

## Per-binary tests

Each `cmd/<tool>/` has a shallow `main_test.go`. Feature tests live in `internal/<feature>/<feature>_test.go`. Use `httptest.Server` for API fixtures.

## CI

See `.github/workflows/ci.yml`. Runs: `gofmt`, `go mod tidy` diff, `go vet`, `go test -race -coverprofile`, `go build ./cmd/...`.
