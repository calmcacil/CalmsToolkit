# CLI behavior specification

Configuration precedence is `defaults < configuration file < environment < explicitly supplied flags`. An omitted boolean or interval flag never overwrites its configured value. The config path is explicit `--config`, then `CALMSTOOLKIT_CONFIG`, then `~/.config/calmstoolkit/config.json`.

## Configuration setup

`calmstoolkit config setup` is the canonical onboarding and editing interface.
Its menu supports editing one section, running every section in sequence,
validating and saving, or quitting without saving. Pressing Enter retains the
displayed value; `-` clears an optional value. Input is validated before the
flow advances. Secrets are displayed as `configured`, and environment-only
credentials are not copied into the configuration file. `--force` starts from
defaults, while an existing file is otherwise edited in place. `--defaults`
writes defaults non-interactively (`--force` is required to replace a file).

The setup flow exposes every persisted user setting:

| Section | Settings |
|---|---|
| General | HTTP timeout, color disablement, theme |
| Sonarr instances | name, API URL, optional external/browser URL, API key; add/edit/remove multiple instances |
| Radarr instances | name, API URL, optional external/browser URL, API key; add/edit/remove multiple instances |
| Media streams | enabled server type, Plex URL/token, Jellyfin URL/token, watch interval, history duration |
| Media requests | Overseerr/Jellyseerr URL, API key, verbose diagnostics |
| Media calendar | future days, past days, watch interval, debug diagnostics |
| Media airtime | result limit, past-day window, future-day window, debug diagnostics |
| Arr feed | poll interval, history window, grabbed/imported/failed/deleted/ignored/subtitle visibility, maximum events |
| AniSearch | mapping download URL, optional cache path, result limit |

`version` is managed by the application and is not prompted. Credentials can
instead be overlaid with `CALMSTOOLKIT_PLEX_TOKEN`,
`CALMSTOOLKIT_JELLYFIN_TOKEN`, `CALMSTOOLKIT_REQUESTS_API_KEY`, and per-instance
`CALMSTOOLKIT_SONARR_<NORMALIZED_NAME>_API_KEY` or
`CALMSTOOLKIT_RADARR_<NORMALIZED_NAME>_API_KEY`. Legacy `PLEX_TOKEN`,
`JELLYFIN_TOKEN`, and `OVERSEERR_API_KEY` remain fallback names.

## Completion

`calmstoolkit completion <shell>` writes a Cobra-generated completion script to
stdout. Supported shells are `bash`, `zsh`, and `fish`. The scripts
discover the complete command tree, local and persistent flags, and registered
values for finite options including output mode, theme, stream server, and
airtime search type. See the [installation guide](INSTALLATION.md#shell-completions)
for installation commands.

Sonarr and Radarr instances distinguish the API address from the optional
browser-facing address:

```json
{
  "name": "Sonarr",
  "url": "http://sonarr:8989",
  "external_url": "https://sonarr.example.com",
  "api_key": "..."
}
```

CalmsToolkit connects to `url`. User-facing links, such as calendar queue
warnings, use `external_url` when present and otherwise fall back to `url`.
Both fields may include a reverse-proxy path and are normalized without a
trailing slash.

Global output modes are `auto`, `terminal`, `plain`, `json`, and `ndjson`. Auto selects terminal only for a capable UTF-8 TTY. `NO_COLOR`, `--no-color`, or `TERM=dumb` disables color; limited terminals use ASCII/plain output. JSON is one snapshot. NDJSON is one envelope per line and is mandatory for machine watch output. Interactive requests rejects both machine modes.

Every machine record has:

```json
{"schema_version":"1","generated_at":"2026-07-15T12:00:00Z","command":"streams","partial":false,"warnings":[],"data":{}}
```

Stdout contains views or machine records only. Prompts, logs, warnings, and redacted debug diagnostics go to stderr. ANSI control sequences are forbidden in plain, JSON, and NDJSON.

Exit codes are 0 success/best-effort, 1 operational failure, 2 usage or configuration error, 3 partial result with `--strict`, and 130 interruption. Multi-source commands show successful data plus warnings by default; machine envelopes set `partial` and include source warnings.

Terminal dashboards refresh inline and never use the alternate screen. One screen owner controls clearing, cursor visibility/restoration, resize, and cancellation.
