# Unified CLI migration

CalmsToolkit now installs only `calmstoolkit`. The JSON configuration remains compatible.

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
