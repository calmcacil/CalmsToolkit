# Activity feed

Install `calmstoolkit` using the [GitHub Releases installation guide](INSTALLATION.md)
before following these examples.

`calmstoolkit feed` combines Sonarr and Radarr history. It shows grabbed, imported, and failed events by default and can include deleted, ignored, or subtitle details.

```bash
calmstoolkit feed --duration 24h
calmstoolkit feed --watch --interval 2s
calmstoolkit --output ndjson feed --watch | jq -c '.data'
```

Configure instances under `sonarr_instances` and `radarr_instances`. A failed source does not discard successful sources; use global `--strict` when partial output must fail automation.
