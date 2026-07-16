# CalmsToolkit

CalmsToolkit is one Linux-focused, SSH-friendly command for a personal media stack. It keeps the project’s compact Unicode cards, semantic colors, and Catppuccin themes while providing safe plain and machine output for scripts.

## Install from GitHub Releases

CalmsToolkit publishes Linux binaries for `amd64` (most Intel/AMD systems) and
`arm64`. This installs the latest release in `/usr/local/bin` and verifies it
against the published checksum before installation:

```bash
(
  set -eu
  repo="https://github.com/calmcacil/CalmsToolkit"
  version="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$repo/releases/latest" | sed 's#.*/##')"
  case "$(uname -m)" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
  archive="calmstoolkit_${version#v}_linux_${arch}.tar.gz"
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"' EXIT
  curl -fL "$repo/releases/download/$version/$archive" -o "$workdir/$archive"
  curl -fL "$repo/releases/download/$version/checksums.txt" -o "$workdir/checksums.txt"
  (cd "$workdir" && sha256sum --ignore-missing --check checksums.txt)
  tar -xzf "$workdir/$archive" -C "$workdir"
  sudo install -m 0755 "$workdir/calmstoolkit" /usr/local/bin/calmstoolkit
)

calmstoolkit version
calmstoolkit config setup
```

You need `curl`, `tar`, and `sha256sum`. For manual downloads, upgrades,
non-root installation, and removal, see the [installation guide](docs/user/INSTALLATION.md).
Configuration is stored at `~/.config/calmstoolkit/config.json` with mode
`0600`; override it with `--config` or `CALMSTOOLKIT_CONFIG`.

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

See [migration](docs/user/MIGRATION_UNIFIED_CLI.md), [CLI behavior](docs/user/CLI_SPEC.md), and [architecture](docs/ARCHITECTURE.md). Public releases follow Semantic Versioning and are generated from Conventional Commit pull-request titles.

## Development

```bash
make check   # format, tidy, vet, race tests, Linux builds
```

New commands must follow [Adding a Tool](docs/ADDING_A_TOOL.md). Never commit the configuration file, tokens, or logs containing credentials.

Pull requests run formatting, lint, race tests, Linux builds, dependency validation, vulnerability analysis, and CodeQL. See [CI and releases](docs/CI_RELEASES.md) for required checks and the automated release flow. CalmsToolkit is available under the [MIT License](LICENSE).
