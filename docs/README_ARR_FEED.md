# arr-feed

A CLI tool for monitoring Sonarr and Radarr activity feeds in real-time. Display recent downloads, imports, failures, and deletions from your *arr instances.

## Features

- **Multi-Instance Support**: Monitor multiple Sonarr and Radarr instances simultaneously
- **Real-Time Monitoring**: Watch mode continuously polls for new activity
- **Flexible Filtering**: Show/hide specific event types (grabbed, imported, failed, deleted, ignored)
- **Color-Coded Output**: Easy-to-read colored table format with action-specific colors
- **JSON Export**: Machine-readable output for automation and integration
- **Relative Timestamps**: Human-friendly time display ("5 minutes ago", "2 hours ago")
- **Custom Format Display**: Shows custom formats applied to each release
- **Episode Information**: Full episode details for TV shows (S01E05 format)

## Installation

### Build from Source

```bash
make build
```

This creates `bin/arr-feed` ready for use.

### Cross-Platform Build

```bash
make build-all
```

Builds binaries for Linux, macOS, Windows, and FreeBSD (amd64 and arm64).

### Install System-Wide

```bash
sudo make install
```

Installs to `/usr/local/bin` (customize with `prefix=/path/to/install`).

## Configuration

arr-feed uses a 3-tier configuration system: `.env` file → environment variables → CLI flags (highest priority).

### Environment Variables

```bash
# Sonarr Configuration (reuses SONARR_URLS and SONARR_TOKENS from media-calendar)
SONARR_URLS="http://localhost:8989,http://sonarr2:8989"
SONARR_TOKENS="your-api-key-1,your-api-key-2"

# Radarr Configuration (reuses RADARR_URLS and RADARR_TOKENS from media-calendar)
RADARR_URLS="http://localhost:7878"
RADARR_TOKENS="your-api-key-3"

# Optional: arr-feed specific settings
ARR_FEED_POLL_INTERVAL=5s          # Watch mode refresh rate
ARR_FEED_HISTORY_DURATION=1h       # How far back to look
ARR_FEED_TIMEOUT=30s               # HTTP request timeout
```

### CLI Flags

All environment variables can be overridden via flags:

```bash
arr-feed -sonarr-urls "http://localhost:8989" \
         -sonarr-tokens "your-api-key" \
         -radarr-urls "http://localhost:7878" \
         -radarr-tokens "your-api-key" \
         -duration 24h \
         -watch
```

Full flag list:

- `-sonarr-urls` - Sonarr instance URLs (comma-separated)
- `-sonarr-tokens` - Sonarr API tokens (comma-separated, matching order)
- `-radarr-urls` - Radarr instance URLs (comma-separated)
- `-radarr-tokens` - Radarr API tokens (comma-separated, matching order)
- `-poll` - Poll interval for watch mode (default: 5s)
- `-duration` - History lookback window (default: 1h)
- `-timeout` - HTTP request timeout (default: 30s)
- `-no-color` - Disable colored output
- `-json` - Output JSON instead of table
- `-watch` - Continuous monitoring mode
- `-show-grabbed` - Show grabbed events (default: true)
- `-show-imported` - Show imported events (default: true)
- `-show-failed` - Show failed events (default: true)
- `-show-deleted` - Show deleted events (default: true)
- `-show-ignored` - Show ignored events (default: false)

## Usage Examples

### Basic Usage

Show the last hour of activity:

```bash
arr-feed
```

### Watch Mode

Continuously monitor for new activity:

```bash
arr-feed -watch
```

### Extended History

Show the last 24 hours:

```bash
arr-feed -duration 24h
```

### Filter by Event Type

Show only failed downloads:

```bash
arr-feed -show-grabbed=false -show-imported=false -show-deleted=false
```

Show only imports and grabs (hide failures):

```bash
arr-feed -show-failed=false
```

### JSON Output

Get machine-readable output for scripting:

```bash
arr-feed -json | jq '.[] | select(.action == "Failed")'
```

### Multi-Instance Monitoring

Monitor multiple Sonarr and Radarr instances:

```bash
arr-feed -sonarr-urls "http://sonarr1:8989,http://sonarr2:8989" \
         -sonarr-tokens "token1,token2" \
         -radarr-urls "http://radarr1:7878,http://radarr2:7878" \
         -radarr-tokens "token3,token4"
```

### Custom Poll Interval

Watch mode with faster updates:

```bash
arr-feed -watch -poll 2s
```

## Output Format

### Table Mode (Default)

```
When            | Action   | Series/Movie                   | Episode | Episode Title        | Quality         | Formats            
------------------------------------------------------------------------------------------------------------------------------------
5 minutes ago   | Imported | Breaking Bad                   | S01E05  | Gray Matter          | Bluray-720p     | AMZN, DV           
12 hours ago    | Grabbed  | The Matrix (1999)              |         |                      | Bluray-1080p    | IMAX               
1 day ago       | Failed   | Better Call Saul               | S03E02  | Witness              | WEB-DL-1080p    |                    
2 days ago      | Deleted  | Game of Thrones                | S08E06  | The Iron Throne      | HDTV-720p       |                    

Total events: 4
```

### Color Legend

- **Green**: Imported, Bulk Import
- **Cyan**: Grabbed
- **Red**: Failed
- **Yellow**: Deleted
- **Gray**: Ignored
- **Blue**: Renamed

### JSON Mode

```json
[
  {
    "server": "sonarr",
    "when": "2025-10-17T10:30:00Z",
    "action": "Imported",
    "title": "Breaking Bad",
    "episode": "S01E05",
    "episodeTitle": "Gray Matter",
    "quality": "Bluray-720p",
    "formats": ["AMZN", "DV"],
    "sourceTitle": "Breaking.Bad.S01E05.720p.BluRay.x264-DEMAND",
    "eventType": "downloadFolderImported",
    "id": 12345
  }
]
```

## Event Types

### Sonarr Events

- `grabbed` → **Grabbed**: Episode added to download client
- `downloadFolderImported` → **Imported**: Episode successfully imported
- `downloadFailed` → **Failed**: Download failed or was rejected
- `episodeFileDeleted` → **Deleted**: Episode file was deleted
- `episodeFileRenamed` → **Renamed**: Episode file was renamed
- `downloadIgnored` → **Ignored**: Download was ignored (disabled by default)
- `seriesFolderImported` → **Bulk Import**: Entire series folder imported

### Radarr Events

- `grabbed` → **Grabbed**: Movie added to download client
- `downloadFolderImported` → **Imported**: Movie successfully imported
- `downloadFailed` → **Failed**: Download failed or was rejected
- `movieFileDeleted` → **Deleted**: Movie file was deleted
- `movieFileRenamed` → **Renamed**: Movie file was renamed
- `downloadIgnored` → **Ignored**: Download was ignored (disabled by default)

## Error Handling

- **Partial Failures**: If one instance fails, arr-feed continues with remaining instances
- **Complete Failure**: Exits with error if all instances fail
- **Network Timeouts**: Respects the configured timeout value
- **API Errors**: Displays HTTP status code and error message

## Performance

- **Parallel Fetching**: All instances are queried concurrently using goroutines
- **Efficient Polling**: Watch mode only fetches events since last poll
- **Event Caching**: Maintains in-memory cache of last 100 events in watch mode
- **Minimal Dependencies**: Uses only Go stdlib, no external dependencies

## Troubleshooting

### No events showing

1. Check your API keys are correct
2. Verify URLs are accessible (no trailing slashes)
3. Ensure events exist in the configured time window
4. Try increasing `-duration` (e.g., `-duration 24h`)

### Connection errors

1. Verify Sonarr/Radarr is running and accessible
2. Check firewall rules
3. Confirm API endpoint: `http://your-server:8989/api/v3/history/since`
4. Test with: `curl -H "X-Api-Key: YOUR_KEY" "http://localhost:8989/api/v3/system/status"`

### Missing data in output

1. Check your filter flags (e.g., `-show-ignored=true` to see ignored events)
2. Verify Sonarr/Radarr is configured to include episode/series data
3. Some fields may be empty for certain event types

### Watch mode not updating

1. Verify poll interval is reasonable (not too fast)
2. Check for errors in the output
3. Confirm new activity is actually happening in Sonarr/Radarr

## Integration Examples

### Send Failed Downloads to Discord

```bash
#!/bin/bash
while true; do
  arr-feed -duration 5m -show-grabbed=false -show-imported=false -show-deleted=false -json | \
    jq -r '.[] | "Failed: \(.title) - \(.sourceTitle)"' | \
    while read line; do
      curl -X POST "https://discord.com/api/webhooks/YOUR_WEBHOOK" \
        -H "Content-Type: application/json" \
        -d "{\"content\": \"$line\"}"
    done
  sleep 300
done
```

### Log All Activity to File

```bash
arr-feed -watch -json >> /var/log/arr-feed.log
```

### Count Events by Type

```bash
arr-feed -duration 24h -json | jq -r '.[].action' | sort | uniq -c
```

### Monitor for Specific Show

```bash
arr-feed -watch -json | jq -r 'select(.title | contains("Breaking Bad"))'
```

## Development

### Run Tests

```bash
make test
```

Runs the full test suite including arr-feed tests.

### Test Individual Components

```bash
go test -tags arrfeed -v ./arr-feed_test.go ./arr-feed.go
```

### Build for Development

```bash
go build -tags arrfeed -o bin/arr-feed arr-feed.go
```

## Technical Details

- **Build Tag**: `arrfeed`
- **Dependencies**: stdlib only
- **API Version**: Sonarr/Radarr v3 API (`/api/v3/history/since`)
- **Test Coverage**: 10 test functions covering core functionality
- **Architecture**: Concurrent fetching with goroutines, event deduplication, time-based sorting

## See Also

- [ARR_FEED_SPEC.md](ARR_FEED_SPEC.md) - Complete technical specification
- [media-calendar](README.md) - Related tool for calendar view
- [Sonarr API Docs](https://github.com/Sonarr/Sonarr/wiki/API)
- [Radarr API Docs](https://github.com/Radarr/Radarr/wiki/API)

## License

Part of CalmsToolkit - See repository root for license information.
