# Unified CLI migration

CalmsToolkit now installs only `calmstoolkit`. The JSON configuration remains compatible.
Install or upgrade it from [GitHub Releases](INSTALLATION.md); replacing the
binary does not overwrite the existing configuration file.

| Previous command | New command |
|---|---|
| `media-streams` | `calmstoolkit streams` |
| `media-calendar` | `calmstoolkit calendar` |
| `media-requests` | `calmstoolkit requests` |
| `media-airtime` | `calmstoolkit airtime` |
| `arr-feed` | `calmstoolkit feed` |
| `anisearch` | `calmstoolkit anime` |
| `calmstoolkit-setup` | `calmstoolkit config setup` |

Single-dash long flags are now standard double-dash Cobra flags. Replace `-json` with `--output json`; a watching script must use `--output ndjson`. Feed’s `-poll` is `--interval`. Version is a command: `calmstoolkit version`.

Optional shell aliases are not installed automatically:

```bash
alias arr-streams='calmstoolkit streams'
alias arr-calendar='calmstoolkit calendar'
alias arr-feed='calmstoolkit feed'
```

Before removing old scripts, walk through `config validate`, `doctor`, one terminal invocation, one piped invocation, and every JSON consumer. JSON now uses the envelope documented in `CLI_SPEC.md`; read feature fields from `.data`.

## Browser-facing Sonarr and Radarr URLs

Existing instance configuration remains valid. If an instance's `url` uses a
hostname that only resolves from the CalmsToolkit host or container network,
add an optional `external_url` for links shown to the user:

```json
{
  "name": "Sonarr",
  "url": "http://sonarr:8989",
  "external_url": "https://sonarr.example.com",
  "api_key": "..."
}
```

API requests and `doctor` continue to use `url`; calendar queue warnings use
`external_url`. When `external_url` is omitted, links continue to use `url`.
