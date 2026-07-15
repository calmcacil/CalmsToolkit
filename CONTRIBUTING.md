# Contributing

Run `make check` before submitting changes. It formats source, verifies module tidiness, runs vet and race tests, and builds Linux amd64/arm64 artifacts.

There is one executable: `cmd/calmstoolkit`. Register commands with Cobra in `internal/cli`; put behavior in a feature or domain package; inject `internal/app.Runtime`; use `internal/console` for terminal/machine output and `internal/httputil` or a typed domain client for HTTP. Only `cmd/calmstoolkit/main.go` may call `os.Exit`.

Tests should cover terminal/plain/machine output, failure, cancellation, and HTTP fixtures where applicable. JSON and NDJSON stdout must contain only documented envelopes; diagnostics and logs belong on stderr. Never log or fixture tokens, API keys, authorization headers, or secret-bearing URLs.

Update command help, user docs, the migration guide when an interface changes, and `docs/ADDING_A_TOOL.md` checklists. Use imperative commit subjects such as `Add calendar filters`.
