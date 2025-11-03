# Queue Remediation TUI

A terminal user interface for managing Sonarr and Radarr queue items.

## Features

- **Multi-instance Support**: Connect to multiple Sonarr and Radarr instances
- **Smart Actions**: Automatically categorize queue items and recommend actions
- **Bulk Operations**: Delete, blocklist, or trigger manual imports for multiple items
- **Real-time Updates**: Refresh queue data on demand
- **Filtering**: View all items or only those with issues
- **Detailed Information**: View status messages and error details

## Configuration

Set the following environment variables:

```bash
# Sonarr instances (comma-separated)
SONARR_URLS=http://sonarr1:8989,http://sonarr2:8989
SONARR_TOKENS=token1,token2

# Radarr instances (comma-separated)
RADARR_URLS=http://radarr1:7878,http://radarr2:7878
RADARR_TOKENS=token1,token2

# Optional settings
NO_COLOR=1          # Disable colors
DEBUG=1              # Enable debug mode
```

## Usage

```bash
# Build
go build -o bin/calms-toolkit ./cmd/calms-toolkit

# Run
./bin/calms-toolkit
```

## Controls

### List View
- `↑↓` or `j/k`: Navigate items
- `Enter`: View item details
- `r`: Refresh queue data
- `i`: Toggle "Issues Only" filter
- `Ctrl+C`: Quit

### Detail View
- `F1`: Back to list
- `d`: Delete item (if recommended)
- `m`: Manual import (if recommended)
- `Ctrl+C`: Quit

### Confirmation Dialog
- `Enter`: Confirm action
- `Esc`: Cancel

## Queue Item Categories

The tool automatically categorizes queue items:

- **Failed Downloads**: Items that failed to download (recommended: delete + blocklist)
- **Import Blocked**: Items ready for import but blocked (recommended: manual import)
- **Custom Format Issues**: Releases not meeting custom format criteria (recommended: delete)
- **Quality Issues**: Releases not meeting quality upgrade criteria (recommended: delete)
- **Sample Files**: Sample releases detected (recommended: delete)
- **No Files Found**: Download structure issues (recommended: delete)
- **Normal Downloads**: Healthy downloads (recommended: monitor)

## API Integration

The tool integrates with Sonarr/Radarr v3 API endpoints:

- `GET /api/v3/queue` - Fetch queue items
- `DELETE /api/v3/queue/{id}` - Remove queue items
- `POST /api/v3/command` - Trigger manual import

## Testing

```bash
# Run tests
go test ./internal/tools/queue/...

# Run with coverage
go test -cover ./internal/tools/queue/...
```