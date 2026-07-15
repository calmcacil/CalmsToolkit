# CalmsToolkit

CalmsToolkit is one Linux-focused, SSH-friendly command for a personal media stack. It keeps the project’s compact Unicode cards, semantic colors, and Catppuccin themes while providing safe plain and machine output for scripts.

## Install

```bash
make build
sudo make install
calmstoolkit config setup
```

`make build-all` produces static Linux amd64 and arm64 binaries. Configuration remains at `~/.config/calmstoolkit/config.json` with mode `0600`; override it with `--config` or `CALMSTOOLKIT_CONFIG`.

## Commands

```text
calmstoolkit streams                 Plex/Jellyfin sessions
calmstoolkit calendar                Sonarr/Radarr releases
calmstoolkit requests                Interactive Overseerr/Jellyseerr requests
calmstoolkit airtime <query>         Library airtime lookup
calmstoolkit feed                    Sonarr/Radarr activity
calmstoolkit anime <query>           AniList search with TVDB mapping
calmstoolkit config setup|validate   Configuration management
calmstoolkit doctor                  Local and service diagnostics
calmstoolkit version                 Build information
```

Run `calmstoolkit <command> --help` for feature flags. Global flags are `--config`, `--output`, `--theme`, `--no-color`, `--timeout`, `--debug`, `--quiet`, and `--strict`.

Output is `auto`, `terminal`, `plain`, `json`, or `ndjson`. Auto uses the rich terminal view only on a capable UTF-8 TTY and otherwise selects plain output. Watch commands require NDJSON rather than JSON for machine output. Diagnostics always go to stderr.

```bash
calmstoolkit streams --server plex
calmstoolkit calendar --days 7 --filter missing
calmstoolkit feed --watch --output ndjson | jq -c .data
calmstoolkit anime "Frieren" --output json
```

See [migration](docs/user/MIGRATION_UNIFIED_CLI.md), [CLI behavior](docs/user/CLI_SPEC.md), and [architecture](docs/ARCHITECTURE.md).

## Development

```bash
make check   # format, tidy, vet, race tests, Linux builds
```

New commands must follow [Adding a Tool](docs/ADDING_A_TOOL.md). Never commit the configuration file, tokens, or logs containing credentials.
