# Architecture

`main` constructs the Cobra tree and is the only layer that converts errors into process exits. `internal/cli` derives feature configuration after parsing, honoring changed flags only. `internal/app.Runtime` carries context, streams, logger, clock, HTTP construction, terminal capabilities, and output mode so commands can be tested without a real terminal.

Feature packages fetch and transform domain data. Typed clients own service wire models; shared HTTP policy belongs in `internal/httputil`. Sonarr/Radarr models should converge in `internal/arr`; Plex, Jellyfin, requests, and AniList remain separate domains.

Rendering consumes view models. Terminal primitives and screen ownership belong in `internal/console`; feature packages must not create competing palettes, width logic, or cursor lifecycle. Machine rendering always uses the versioned envelope and never shares stdout with logging.

Dependency direction is `main -> cli -> feature/domain -> foundation`. Foundation packages must not import feature or command packages. No package other than `main` terminates the process.
