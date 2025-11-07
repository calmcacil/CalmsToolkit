# Queue Remediation Tool - Complete Usage Guide

**Last Updated:** November 1, 2025  
**Tool:** `queue-remediation`  
**Purpose:** Automatically detect and fix stuck queue items in Sonarr and Radarr

---

## Table of Contents

1. [Overview](#overview)
2. [How It Works](#how-it-works)
3. [Installation](#installation)
4. [Configuration](#configuration)
5. [Usage Examples](#usage-examples)
6. [Remediation Actions Explained](#remediation-actions-explained)
7. [Command-Line Flags](#command-line-flags)
8. [Troubleshooting](#troubleshooting)
9. [Best Practices](#best-practices)
10. [FAQ](#faq)

---

## Overview

The Queue Remediation tool is an intelligent CLI utility that monitors your Sonarr and Radarr download queues, identifies problematic items, and automatically applies the appropriate fix. It eliminates the need for manual queue management by handling common issues like:

- ✅ Downloads that completed but won't import
- ✅ Quality/custom format mismatches stuck in queue
- ✅ Sample files blocking real downloads
- ✅ Failed downloads that need to be removed
- ✅ Items matched by ID but requiring manual import

### Key Features

- **🛡️ Safe by Default**: Dry-run mode lets you see what would happen before making changes
- **🎯 Smart Classification**: Intelligently categorizes queue items and chooses the right action
- **🔄 Multi-Instance Support**: Handles multiple Sonarr and Radarr servers simultaneously
- **📊 Two API Modes**: Command API (fire-and-forget) or REST API (precise control)
- **📝 Detailed Logging**: Verbose and debug modes for troubleshooting
- **⚡ Fast & Efficient**: Processes entire queue in seconds

---

## How It Works

### Detection Phase

The tool queries the `/api/v3/queue` endpoint on each configured instance and analyzes:
- Download status (`downloading`, `completed`, `failed`)
- Tracked download state (`importPending`, `importBlocked`, etc.)
- Status messages (quality mismatches, custom format scores, etc.)
- Output paths and download IDs

### Classification Phase

Each queue item is classified into one of three categories:

1. **DELETE (with optional blocklist)**
   - Failed downloads
   - Quality not an upgrade
   - Custom format not an upgrade
   - Sample files detected
   - No files found in download

2. **MANUAL_IMPORT**
   - Completed downloads in `importBlocked` state
   - Items matched to series/movie by ID but not auto-imported

3. **MONITOR (no action)**
   - Active downloads progressing normally
   - Items without issues

### Remediation Phase

Based on classification, the tool executes:

- **DELETE**: Removes queue item via `DELETE /api/v3/queue/{id}`
  - Optionally adds to blocklist to prevent re-download
  - Removes from download client if requested

- **MANUAL_IMPORT**: Triggers import via one of two methods:
  - **Command API** (default): POSTs `DownloadedEpisodesScan`/`DownloadedMoviesScan` command
  - **REST API** (opt-in): Scans folder, builds import requests with explicit IDs, executes import

- **MONITOR**: No action, item logs to output as "downloading normally"

---

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/calmcacil/CalmsToolkit.git
cd CalmsToolkit

# Build the binary
make build

# The binary will be in bin/queue-remediation
```

### Build for Specific Platform

```bash
# Build for current platform
go build -tags queueremediation -o queue-remediation queue-remediation.go

# Build for Linux (from macOS/Windows)
GOOS=linux GOARCH=amd64 go build -tags queueremediation -o queue-remediation-linux queue-remediation.go

# Build for Windows (from macOS/Linux)
GOOS=windows GOARCH=amd64 go build -tags queueremediation -o queue-remediation.exe queue-remediation.go
```

### Install to System Path (Optional)

```bash
# Copy to /usr/local/bin
sudo cp bin/queue-remediation /usr/local/bin/

# Make executable
sudo chmod +x /usr/local/bin/queue-remediation

# Now you can run from anywhere
queue-remediation -dry-run
```

---

## Configuration

The tool supports three configuration methods (in order of precedence):

### 1. Command-Line Flags (Highest Priority)

```bash
./queue-remediation \
  -sonarr-urls "http://sonarr1:8989,http://sonarr2:8989" \
  -sonarr-tokens "token1,token2" \
  -radarr-urls "http://radarr:7878" \
  -radarr-tokens "token" \
  -dry-run
```

### 2. Environment Variables

```bash
export SONARR_URLS="http://localhost:8989"
export SONARR_TOKENS="your-sonarr-api-key"
export RADARR_URLS="http://localhost:7878"
export RADARR_TOKENS="your-radarr-api-key"
export USE_REST_API="true"

./queue-remediation -dry-run
```

### 3. .env File (Lowest Priority)

Create a `.env` file in the current directory or `/opt/apps/compose/.env`:

```bash
# Sonarr Configuration
SONARR_URLS=http://192.168.1.10:8989/sonarr,http://192.168.1.10:8990/sonarr-4k
SONARR_TOKENS=api-key-1,api-key-2

# Radarr Configuration
RADARR_URLS=http://192.168.1.10:7878/radarr
RADARR_TOKENS=api-key-3

# Optional: Use REST API for manual imports
USE_REST_API=true
```

### Getting API Keys

**Sonarr/Radarr:**
1. Open Sonarr/Radarr web UI
2. Go to Settings → General
3. Copy the API Key from the Security section

---

## Usage Examples

### Example 1: Dry-Run Analysis (Recommended First Step)

```bash
./queue-remediation -dry-run
```

**Output:**
```
[DRY-RUN] Analyzing queue items...

[DRY-RUN] Sonarr1 - Item #780680553 (The.Lowdown.S01E06...)
[DRY-RUN]   → Would MANUAL_IMPORT to /downloads/Sonarr/... - matched to series by ID

[DRY-RUN] Sonarr1 - Item #1628563380 (Mayor.of.Kingstown.S02E07.REPACK...)
[DRY-RUN]   → Would DELETE (blocklist=true) - custom format not an upgrade

=== DRY-RUN SUMMARY ===
Total items: 5
  Would delete: 1
  Would manual import: 2
  Monitoring: 2
```

### Example 2: Execute Remediation (Production Mode)

```bash
./queue-remediation -verbose
```

**Output:**
```
[INFO] Fetching queue from Sonarr1...
[INFO] Found 5 queue items
[INFO] Successfully triggered manual import via Command API for queue item #780680553
[INFO] Deleted queue item #1628563380 (blocklisted)
[INFO] Remediation complete: 2 items processed, 3 monitoring
```

### Example 3: REST API Mode (More Precise)

```bash
./queue-remediation -use-rest-api -verbose
```

**Output:**
```
[INFO] Using REST API for manual import (queue item #780680553: The.Lowdown.S01E06)
[VERBOSE] Scanning for manual import: GET http://sonarr:8989/api/v3/manualimport?folder=/downloads/...
[VERBOSE] Scan response: status 200, found 1 files (0 with rejections, 1 importable)
[VERBOSE] Accepted file: Series=The Lowdown, Season=1, Episodes=1, Quality=WEBDL-1080p
[VERBOSE] Executing manual import: POST http://sonarr:8989/api/v3/manualimport (1 files)
[INFO] Successfully imported 1/1 files via REST API
```

### Example 4: Multiple Instances

```bash
./queue-remediation \
  -sonarr-urls "http://sonarr-hd:8989,http://sonarr-4k:8990" \
  -sonarr-tokens "token1,token2" \
  -radarr-urls "http://radarr-hd:7878,http://radarr-4k:7879" \
  -radarr-tokens "token3,token4" \
  -dry-run
```

### Example 5: Debug Mode (Troubleshooting)

```bash
./queue-remediation -debug
```

**Output includes:**
```
[DEBUG] Scan response body: [{"id":12345,"path":"/downloads/...","series":{"id":42,"title":"The Lowdown"},...}]
[DEBUG] Import request payload:
[
  {
    "path": "/downloads/The.Lowdown.S01E06...",
    "seriesId": 42,
    "seasonNumber": 1,
    "episodeIds": [123],
    "quality": {...},
    "downloadId": "abc123"
  }
]
[DEBUG] Import response body: [{"id":12345,"rejections":[]}]
```

### Example 6: Timeout Configuration

```bash
# Increase timeout for slow networks
./queue-remediation -timeout 60s -dry-run
```

---

## Remediation Actions Explained

### DELETE Actions

**Triggers:**
- Download status is `failed`
- Status message: "custom format not an upgrade"
- Status message: "quality not an upgrade"
- Status message: "sample file detected"
- Status message: "no files found"

**What Happens:**
1. Removes item from Sonarr/Radarr queue
2. Removes download from download client (if present)
3. Optionally adds release to blocklist (prevents re-download)

**When Blocklist is Applied:**
- Quality/custom format rejections: **YES** (prevents re-downloading incompatible releases)
- Sample files: **NO** (allows retry with real files)
- Failed downloads: **NO** (allows retry)

**Example:**
```
Item: Mayor.of.Kingstown.S02E07.REPACK.MULTI.1080p.WEB.H264-HiggsBoson
Reason: Custom format score: -10000 (below threshold)
Action: DELETE + BLOCKLIST
Result: Cleared from queue, Sonarr searches for better release
```

### MANUAL_IMPORT Actions

**Triggers:**
- `trackedDownloadState` is `importBlocked`
- Status message: "matched to series by ID" (but didn't auto-import)

**What Happens (Enhanced REST API Mode - with `-use-rest-api`):**
1. **Server-Side Filtering**: `GET /api/v3/manualimport?seriesId={id}` or `?movieId={id}`
2. **Client-Side Validation**: Verifies each file belongs to correct series/movie
3. **Precise Import**: `POST /api/v3/manualimport` with validated files only
4. **Detailed Reporting**: Success/failure status per file

**What Happens (Command API Fallback - default or when REST fails):**
1. **Exact Path Usage**: Uses precise `OutputPath` from queue item
2. **Background Scan**: POSTs `DownloadedEpisodesScan`/`DownloadedMoviesScan` command
3. **Import Mode**: Includes `importMode: Move` for proper file handling
4. **Asynchronous**: Sonarr/Radarr processes scan in background

**Example:**
```
Item: The.Lowdown.S01E06.1080p.AMZN.WEB-DL.DDP5.1.H.264-FLUX
Reason: Matched to series by ID, importBlocked
Action: MANUAL_IMPORT (REST API)
Result: Scanned folder, found 1 file, imported successfully
```

### MONITOR Actions

**Triggers:**
- Status is `downloading` or `queued`
- No blocking status messages
- Download progressing normally

**What Happens:**
- Tool logs item as "downloading normally"
- No actions taken
- Item continues downloading

**Example:**
```
Item: Mayor.of.Kingstown.S02E01.PROPER.MULTI.1080p.WEB.H264-HiggsBoson
Reason: Downloading normally (45% complete)
Action: MONITOR
Result: No action, download continues
```

---

## Command-Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-sonarr-urls` | string | `""` | Comma-separated Sonarr instance URLs |
| `-sonarr-tokens` | string | `""` | Comma-separated Sonarr API tokens (must match URL count) |
| `-radarr-urls` | string | `""` | Comma-separated Radarr instance URLs |
| `-radarr-tokens` | string | `""` | Comma-separated Radarr API tokens (must match URL count) |
| `-timeout` | duration | `30s` | HTTP request timeout (e.g., `10s`, `1m`, `90s`) |
| `-dry-run` | bool | `false` | Show what would be done without making changes |
| `-use-rest-api` | bool | `false` | Use enhanced REST API for manual imports with ID validation (recommended) |
| `-verbose` | bool | `false` | Show verbose logging (API calls, filtering decisions) |
| `-debug` | bool | `false` | Show debug logging (full payloads, implies `-verbose`) |

### Flag Examples

```bash
# Dry-run with verbose output
./queue-remediation -dry-run -verbose

# Production mode with REST API and debug logging
./queue-remediation -use-rest-api -debug

# Custom timeout for slow networks
./queue-remediation -timeout 2m -dry-run

# Single instance, command-line configuration
./queue-remediation \
  -sonarr-urls "http://localhost:8989" \
  -sonarr-tokens "your-token" \
  -dry-run
```

---

## Troubleshooting

### Issue: "number of Sonarr URLs (2) must match number of Sonarr tokens (1)"

**Cause:** URL count and token count don't match

**Solution:**
```bash
# Ensure equal counts
export SONARR_URLS="http://sonarr1:8989,http://sonarr2:8989"
export SONARR_TOKENS="token1,token2"  # Two tokens for two URLs
```

### Issue: "all instances failed to fetch queue"

**Possible Causes:**
1. Incorrect URL (check for typos, port numbers)
2. Invalid API token
3. Network connectivity issues
4. Sonarr/Radarr not running

**Solution:**
```bash
# Test connectivity manually
curl -H "X-Api-Key: your-token" http://localhost:8989/api/v3/queue

# Check logs with debug mode
./queue-remediation -debug
```

### Issue: Manual import fails with "failed to scan for manual import: status 400"

**Possible Causes:**
1. Invalid folder path
2. Path doesn't exist on Sonarr/Radarr server
3. Permissions issue

**Solution:**
```bash
# Verify path exists in Sonarr/Radarr logs
# Check System → Logs in Sonarr/Radarr UI

# Fall back to Command API
./queue-remediation -verbose  # Don't use -use-rest-api
```

### Issue: Items keep coming back to queue after deletion

**Cause:** Sonarr/Radarr is re-downloading the same release

**Solution:**
1. Ensure blocklist is enabled in your quality profile
2. Check that tool is using `blocklist=true` (it should for quality/custom format rejections)
3. Manually blocklist the release in Sonarr/Radarr UI if needed

### Issue: "Failed to read input" errors (media-requests tool)

**Cause:** Recent bug fixes addressed this - ensure you're on latest version

**Solution:**
```bash
# Rebuild from latest source
git pull
make build
```

---

## Best Practices

### 1. Always Start with Dry-Run

```bash
# FIRST: See what would happen
./queue-remediation -dry-run -verbose

# THEN: Execute if comfortable with actions
./queue-remediation -verbose
```

### 2. Use REST API for Stuck Imports

If you have many import-blocked items, REST API mode provides better success rates:

```bash
./queue-remediation -use-rest-api -verbose
```

### 3. Run Regularly (Automation)

Create a cron job or systemd timer:

```bash
# Cron: Run every 6 hours
0 */6 * * * /usr/local/bin/queue-remediation -verbose >> /var/log/queue-remediation.log 2>&1

# Systemd timer (queue-remediation.timer)
[Unit]
Description=Queue Remediation Timer

[Timer]
OnCalendar=*-*-* 00,06,12,18:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

### 4. Monitor Logs for Patterns

If you see recurring issues:
- Frequent quality rejections → Review quality profiles
- Many import-blocked items → Check path mappings and permissions
- Sample file detections → Consider different indexers

### 5. Use Verbose Mode for First Few Runs

Until you're confident the tool is working as expected:

```bash
./queue-remediation -verbose
```

### 6. Keep Configuration in .env File

Easier than environment variables or command-line flags:

```bash
# /opt/apps/compose/.env or .env in working directory
SONARR_URLS=http://localhost:8989
SONARR_TOKENS=your-token
USE_REST_API=true
```

---

## FAQ

### Q: Is it safe to run without dry-run?

**A:** Yes, the tool only takes actions that are safe and reversible:
- DELETE removes queue items (but Sonarr/Radarr can re-download)
- MANUAL_IMPORT triggers Sonarr/Radarr's built-in import process
- MONITOR does nothing

However, **always run dry-run first** to verify the tool's decisions match your expectations.

### Q: Will this delete my downloaded files?

**A:** No. The tool only removes items from the *queue* (the download tracking list). Downloaded files remain on disk unless:
1. You have "Remove Completed Downloads" enabled in your download client, AND
2. The tool removes the item from the download client (which it does)

### Q: What's the difference between Command API and REST API mode?

**Command API (default/fallback):**
- Uses exact `OutputPath` from queue item
- Fires background scan with `importMode: Move`
- Sonarr/Radarr handles all matching and importing
- Fire-and-forget approach with minimal overhead

**Enhanced REST API (with `-use-rest-api`):**
- Server-side filtering with `seriesId`/`movieId` parameters
- Client-side validation prevents wrong file imports
- Precise import with detailed success/failure reporting
- Falls back to Command API if scan fails

**Recommendation:** Use enhanced REST API (`-use-rest-api`) for better success rates and data safety.

### Q: Can I run this on multiple machines?

**A:** Yes, but avoid running simultaneously on the same instances:
- Queue state changes between runs
- Could cause race conditions
- Use locking or scheduling to prevent overlap

### Q: How do I exclude specific instances?

**A:** Simply omit them from configuration:

```bash
# Only remediate Sonarr1 and Radarr1
export SONARR_URLS="http://sonarr1:8989"
export SONARR_TOKENS="token1"
export RADARR_URLS="http://radarr1:7878"
export RADARR_TOKENS="token3"
```

### Q: What if I want to manually import instead of auto?

**A:** The tool respects Sonarr/Radarr's import settings. If you prefer:
1. Run with `-dry-run` to identify stuck items
2. Manually import via Sonarr/Radarr UI
3. Use the tool only for cleanup (deletes)

### Q: Does this work with Sonarr v4 / Radarr v5?

**A:** Yes, the tool uses the v3 API which is stable across versions:
- Sonarr v3, v4: ✅ Supported
- Radarr v3, v4, v5: ✅ Supported

### Q: Can I customize which actions are taken?

**A:** Currently no - the tool uses intelligent defaults. If you need custom logic:
1. Fork the repository
2. Modify `mapStatusToAction()` function in `queue-remediation.go`
3. Build your custom version

---

## Support and Contributing

### Reporting Issues

Found a bug or have a feature request? Please open an issue on GitHub:
- Include full command used
- Include output with `-debug` flag
- Include Sonarr/Radarr version

### Contributing

Pull requests welcome! Please:
1. Follow existing code style (`go fmt`)
2. Add tests for new functionality
3. Update documentation

### License

MIT License - feel free to use and modify

---

**Last Updated:** November 1, 2025  
**Version:** 1.0.0  
**Author:** CalmsToolkit  
**Repository:** https://github.com/calmcacil/CalmsToolkit
