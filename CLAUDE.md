# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CalmsToolkit is a collection of high-performance Go-based monitoring tools for media servers. The toolkit provides ~10x faster performance compared to bash/shell implementations by using Go's native libraries for XML/JSON parsing and HTTP requests.

## Repository Structure

The repository contains three main binaries, each as a standalone Go file:

- `plex-streams.go` - Monitors active Plex media server streams
- `jellyfin-streams.go` - Monitors active Jellyfin media server streams
- `media-streams.go` - Unified monitor for both Plex and Jellyfin servers

Each tool is self-contained with no external Go dependencies (uses only stdlib).

## Building and Running

### Build Commands

```bash
# Build all binaries (outputs to build/ directory)
make build

# Build for all platforms (Linux, macOS, Windows, FreeBSD for amd64/arm64)
make build-all

# Install to ~/.local/bin
make install

# Clean build artifacts
make clean

# Run tests
make test

# Tidy go.mod dependencies
make tidy
```

### Running the Tools

All three tools follow similar command patterns:

**Plex Streams:**
```bash
# With environment variables
export PLEX_URL="http://localhost:32400"
export PLEX_TOKEN="your-token"
./build/plex-streams

# With command-line flags
./build/plex-streams -url "http://plex:32400" -token "your-token"

# Watch mode (continuous monitoring)
./build/plex-streams -watch -interval 5

# JSON output
./build/plex-streams -json
```

**Jellyfin Streams:**
```bash
export JELLYFIN_URL="http://localhost:8096"
export JELLYFIN_TOKEN="your-token"
./build/jellyfin-streams
```

**Media Streams (Unified):**
```bash
./build/media-streams -server both -plex-token "..." -jellyfin-token "..."
./build/media-streams -server plex  # Plex only
./build/media-streams -server jellyfin  # Jellyfin only
```

## Architecture

### Configuration Loading Priority

All tools use a three-tier configuration system (from lowest to highest priority):

1. `.env` file at `/opt/apps/compose/.env` (if exists)
2. Environment variables (`PLEX_URL`, `PLEX_TOKEN`, `JELLYFIN_URL`, `JELLYFIN_TOKEN`)
3. Command-line flags (`-url`, `-token`, etc.)

The `loadConfig()` and `loadEnvFile()` functions implement this hierarchy.

### API Integration

**Plex Integration:**
- Uses XML API: `GET /status/sessions?X-Plex-Token={token}`
- Parses XML response into Go structs using `encoding/xml`
- Handles both `Video` (movies/episodes) and `Track` (music) sessions
- Key structs: `MediaContainer`, `Video`, `Track`, `TranscodeSession`

**Jellyfin Integration:**
- Uses JSON API: `GET /Sessions?api_key={token}`
- Parses JSON response using `encoding/json`
- Filters sessions where `NowPlayingItem != nil` for active streams
- Key structs: `JellyfinSession`, `NowPlayingItem`, `TranscodingInfo`

### Output Modes

Both terminal and JSON output modes are supported via the `-json` flag.

**Terminal Mode:**
- Colorized ANSI output using color constants (`ColorRed`, `ColorGreen`, etc.)
- Displays user, title, client, transcoding status, bandwidth, and quality info
- Summary includes total streams, transcoding count, and total bandwidth
- Color is automatically disabled when `-json` flag is used

**JSON Mode:**
- Outputs structured `Summary` with `StreamInfo` array
- Includes `total_streams`, `transcoding_count`, `total_bandwidth_mbps`, `timestamp`
- Each stream has normalized fields regardless of source server

### Watch Mode

The `-watch` flag enables continuous monitoring with auto-refresh:
- Clears screen using platform-specific commands (`clear` on Unix, `cls` on Windows)
- Refreshes at `-interval` seconds (default: 5)
- Runs indefinitely until interrupted

## Code Patterns

### Transcoding Detection

**Plex:** Check `TranscodeSession != nil` OR check `Media[].Parts[].VideoDecision == "transcode"` OR `AudioDecision == "transcode"`

**Jellyfin:** Check `TranscodingInfo.IsVideoDirect == false` OR `IsAudioDirect == false` OR `PlayState.PlayMethod == "Transcode"`

### Bandwidth Handling

- Plex: `Session.Bandwidth` is in Kbps → divide by 1000 for Mbps
- Jellyfin: `TranscodingInfo.Bitrate` is in bps → divide by 1,000,000 for Mbps

### Resolution Mapping

The `getResolutionName()` function maps pixel heights to friendly names:
- 2160+ → "4K"
- 1440+ → "1440p"
- 1080+ → "1080p"
- 720+ → "720p"
- 480+ → "480p"

This function is duplicated across all three files.

## Development Notes

### Media-Streams Unified Tool

The `media-streams.go` file duplicates and combines the Plex and Jellyfin implementations:
- Uses prefixed struct names (`PlexVideo`, `JellyfinSession`, etc.)
- Implements separate `fetchPlexStreams()` and `fetchJellyfinStreams()` functions
- Merges results into a unified `[]StreamInfo` array with `Server` field
- The `-server` flag controls which APIs to query

### Error Handling

All tools use consistent error handling:
- HTTP client timeouts default to 10 seconds
- Connection errors include the server URL in the message
- Non-200 status codes are reported with the status code
- Configuration validation exits with descriptive error messages

### Cross-Platform Support

Platform differences are handled via `runtime.GOOS`:
- Screen clearing: `clear` for Linux/macOS, `cmd /c cls` for Windows
- Binary extensions: `.exe` suffix added for Windows builds in `build-all` target
- Color output works on all platforms (Windows 10+ supports ANSI)

## Testing and Validation

When making changes:
1. Test with both valid and invalid credentials
2. Test with both active and idle servers (0 streams)
3. Verify JSON output is valid (pipe to `jq`)
4. Test watch mode refresh behavior
5. Verify color/no-color modes work correctly
6. Cross-compile with `make build-all` to catch platform-specific issues
