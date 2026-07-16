# Media requests

Install `calmstoolkit` using the [GitHub Releases installation guide](INSTALLATION.md)
before following these examples.

`calmstoolkit requests` is the interactive Overseerr/Jellyseerr request manager. It supports search, season selection, viewing and moderating pending requests, and root-folder choice during approval.

Configure `media_requests.overseerr_url` and `media_requests.api_key`, or set `CALMSTOOLKIT_REQUESTS_API_KEY` (`OVERSEERR_API_KEY` remains a legacy fallback).

```bash
calmstoolkit requests
calmstoolkit requests --url http://localhost:5055 --token "$TOKEN"
calmstoolkit --no-color requests
```

Because the workflow prompts for choices, JSON and NDJSON are rejected. Tokens supplied on a command line may be visible in process listings; the config file or environment is preferable.
