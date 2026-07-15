# Adding a tool

- Register a Cobra constructor in `internal/cli`; do not add a binary.
- Derive defaults from `internal/config`, then apply only explicitly changed flags.
- Accept the shared runtime/context and return errors.
- Model fetched data separately from terminal view models.
- Use `internal/console` for terminal, plain, JSON, and NDJSON output.
- Use typed service clients on the shared HTTP foundation; redact credentials.
- Test terminal widths 40, 60, 80, 100, and 120, including Unicode and no-color.
- Test JSON/NDJSON schema, ANSI absence, failures, partial data, and cancellation.
- Add HTTP fixtures for malformed, oversized, retry, and authentication responses.
- Update command docs and migration notes, then run `make check`.
