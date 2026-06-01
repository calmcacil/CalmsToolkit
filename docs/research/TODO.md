## Completed

- ✅ Unit tests for all tools
- ✅ README.md for project overview
- ✅ Documentation for each tool (docs/user/README_MEDIA_REQUESTS.md, docs/user/README_ARR_FEED.md)

## Known Issues

- API tokens passed in query strings for Plex (`X-Plex-Token`) and Jellyfin (`api_key`) — should use header-based auth where possible
- Several error messages print to stdout instead of stderr
- HTTP response handling boilerplate is duplicated across API fetchers
- CLI input validation is inconsistent across tools
- Skipped manual tests in internal/requests could be converted to proper test tooling
