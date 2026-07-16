# CLI behavior specification

Configuration precedence is `defaults < configuration file < environment < explicitly supplied flags`. An omitted boolean or interval flag never overwrites its configured value. The config path is explicit `--config`, then `CALMSTOOLKIT_CONFIG`, then `~/.config/calmstoolkit/config.json`.

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
