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

arr-feed uses a shared JSON configuration file at `~/.config/calmstoolkit/config.json`.

Run `make setup` to generate your configuration interactively.

### JSON Configuration

```json
{
  "version": 1,
  "sonarr_instances": [
    {"name": "Sonarr HD", "url": "http://localhost:8989", "api_key": "your-key"}
  ],
  "radarr_instances": [
    {"name": "Radarr HD", "url": "http://localhost:7878", "api_key": "your-key"}
  ],
  "general": {
    "timeout": "30s"
  },
  "arr_feed": {
    "poll_interval": "5s",
    "history_window": "1h",
    "show_grabbed": true,
    "show_imported": true,
    "show_failed": true,
    "show_deleted": false,
    "show_ignored": false,
    "max_events": 50
  }
}
```

### CLI Flags

All configuration values can be overridden via flags:

```bash
arr-feed -watch -duration 24h -poll 2s
```

Full flag list:

- `-poll` - Poll interval for watch mode (default: 5s)
- `-duration` - History lookback window (default: 1h)
- `-timeout` - HTTP request timeout (default: 30s)
- `-no-color` - Disable colored output
- `-json` - Output JSON instead of table
- `-watch` - Continuous monitoring mode
- `-show-grabbed` - Show grabbed events (default: true)
- `-show-imported` - Show imported events (default: true)
- `-show-failed` - Show failed events (default: true)
- `-show-deleted` - Show deleted events (default: false)
- `-show-ignored` - Show ignored events (default: false)
- `-events` - Maximum number of events to display (1-100, default: 50)
- `-quiet` - Suppress error output in watch mode

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

### JSON Output

Get machine-readable output for scripting:

```bash
arr-feed -json | jq '.[] | select(.action == "Failed")'
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

## Development

### Run Tests

```bash
make test
```

Runs the full test suite including arr-feed tests.

## Technical Details

- **Dependencies**: stdlib only
- **API Version**: Sonarr/Radarr v3 API (`/api/v3/history/since`)
- **Architecture**: Concurrent fetching with goroutines, event deduplication, time-based sorting

## See Also

- [ARR_FEED_SPEC.md](ARR_FEED_SPEC.md) - Complete technical specification
- [media-calendar](../README.md) - Related tool for calendar view
- [Sonarr API Docs](https://github.com/Sonarr/Sonarr/wiki/API)
- [Radarr API Docs](https://github.com/Radarr/Radarr/wiki/API)

## License

Part of CalmsToolkit - See repository root for license information.
