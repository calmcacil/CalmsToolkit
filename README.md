# CalmsToolkit

Various tools for managing media servers and related services.

## Tools

### Media Requests (Interactive)

An interactive CLI tool for managing media requests through Overseerr/Jellyseerr with an intuitive menu-driven interface.

#### Features

- Interactive Menu: Easy-to-navigate menu system with hotkeys
- Search Media: Search TMDB database for movies and TV shows
- Smart Season Selection: Request all seasons or specific ones for TV shows
- Request Management: View, approve, and decline pending requests
- Status Indicators: Visual feedback for available/requested/pending media
- Root Folder Override: Select custom root folders when approving requests
- Colored Output: ANSI colors for better readability
- JSON Config: Configuration via ~/.config/calmstoolkit/config.json

#### Quick Start

```bash
# Set up configuration
make setup

# Build the binary
make build

# Run interactive menu
./bin/media-requests
```

#### Documentation

- **[docs/README_MEDIA_REQUESTS.md](docs/README_MEDIA_REQUESTS.md)** - Complete documentation and usage guide
- **[docs/OVERSEERR_API_RESEARCH.md](docs/OVERSEERR_API_RESEARCH.md)** - API endpoint reference

### Media Streams Monitor

Monitor active streams on both Plex and Jellyfin servers with detailed information about users, transcoding status, bandwidth, and quality. Supports session history tracking in watch mode.

#### Features

- Multi-Server: Monitor both Plex and Jellyfin servers simultaneously or individually
- Session History: Track recently ended sessions in watch mode (configurable duration)
- Fast: ~10x faster than bash implementations
- Color Output: ANSI colored terminal output with distinct styling for ended sessions
- JSON Output: Machine-readable format for automation and monitoring
- Watch Mode: Continuous real-time monitoring with auto-refresh and history tracking
- Rich Details: Shows transcoding status, bandwidth, quality, codecs, duration, and more
- Audio Support: Handles both video (movies/TV) and audio (music) streams

#### Quick Start

```bash
# Run with both Plex and Jellyfin
./bin/media-streams -server both

# Plex only
./bin/media-streams -server plex -plex-token "your-token"

# Jellyfin only
./bin/media-streams -server jellyfin -jellyfin-token "your-token"

# Watch mode with session history
./bin/media-streams -watch -interval 5 -history-duration 10m

# JSON output for automation
./bin/media-streams -json | jq '.total_streams'
```

### Media Calendar

Display upcoming TV episodes and movie releases from Sonarr and Radarr in a concise calendar view.

#### Features

- Multi-Instance: Monitor multiple Sonarr and Radarr instances simultaneously
- Calendar View: Horizontal and vertical layouts adapt to terminal width
- Filters: Filter by availability, missing, premieres, or monitored status
- Queue Warnings: Alerts for items needing manual intervention
- Watch Mode: Continuous monitoring with auto-refresh
- JSON Output: Machine-readable format for automation

#### Quick Start

```bash
./bin/media-calendar -days 7
./bin/media-calendar -days 7 -days-past 1 -filter missing
./bin/media-calendar -watch -interval 300
./bin/media-calendar -json
```

### Media Airtime

Look up next upcoming and last aired dates for any show or movie in your Sonarr/Radarr library with fuzzy matching and an interactive selection prompt.

#### Features

- Fuzzy Search: Find shows and movies with partial or approximate name matching
- Interactive Selection: Pick from multiple matches with a numbered menu
- Airtime Display: Shows next upcoming airtime or last aired episode with relative dates ("in 3 days", "yesterday")
- Per-Season Detail: Current season detection with episode on-disk status
- Multi-Instance: Searches across all configured Sonarr and Radarr instances
- JSON Output: Machine-readable format for automation

#### Quick Start

```bash
./bin/media-airtime "clarkson"
./bin/media-airtime "clarkson's farm"
./bin/media-airtime -type movie "inception 2010"
./bin/media-airtime -exact "Clarkson's Farm"
./bin/media-airtime -json "dune"
```

### ARR Feed

Monitor Sonarr and Radarr history events (grabbed, imported, failed) in real-time.

#### Features

- Multi-Instance: Monitor multiple Sonarr and Radarr instances simultaneously
- Event Filtering: Show/hide grabbed, imported, failed, deleted, and ignored events
- Watch Mode: Continuous real-time monitoring
- JSON Output: Machine-readable format for automation

#### Quick Start

```bash
./bin/arr-feed
./bin/arr-feed -watch
./bin/arr-feed -show-grabbed -show-failed
./bin/arr-feed -json
```

## Configuration

All tools use a shared JSON configuration file at `~/.config/calmstoolkit/config.json`.

Run `make setup` to generate your configuration interactively.

## Building

```bash
# Current platform
make build

# All platforms (Linux, macOS, Windows, FreeBSD)
make build-all

# Install to /usr/local/bin
make install
```

## Testing

```bash
make test
```

## License

MIT License
