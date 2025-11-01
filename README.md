# CalmsToolkit
Various tools for my server and associated apps.

## Tools

### Media Requests (Interactive)

An interactive CLI tool for managing media requests through Overseerr/Jellyseerr with an intuitive menu-driven interface.

#### Features

- 🎯 **Interactive Menu**: Easy-to-navigate menu system with hotkeys
- 🔍 **Search Media**: Search TMDB database for movies and TV shows
- 📺 **Smart Season Selection**: Request all seasons or specific ones for TV shows
- ✅ **Request Management**: View, approve, and decline pending requests
- 🎨 **Status Indicators**: Visual feedback for available/requested/pending media
- 🌈 **Colored Output**: ANSI colors for better readability
- 🔧 **Flexible Config**: Environment variables, .env files, or command-line flags
- 🚀 **No Dependencies**: Single static binary using only Go stdlib

#### Quick Start

```bash
# Build the binary
make build

# Set up configuration
export OVERSEERR_URL="http://localhost:5055"
export OVERSEERR_TOKEN="your-api-key"

# Run interactive menu
./bin/media-requests

# Or use command-line flags
./bin/media-requests -url "http://overseerr:5055" -token "your-token"
```

#### Menu Options

- **[N] New Request**: Search and request movies or TV shows
- **[W] View Requests**: View, approve, or decline pending requests
- **[Q] Quit**: Exit the application

#### Documentation

- **[README_MEDIA_REQUESTS.md](docs/README_MEDIA_REQUESTS.md)** - Complete documentation and usage guide
- **[OVERSEERR_API_RESEARCH.md](docs/OVERSEERR_API_RESEARCH.md)** - API endpoint reference

### Media Streams Monitor (Unified)

A high-performance Go-based tool for monitoring active streams on both Plex and Jellyfin servers with detailed information about users, transcoding status, bandwidth, and quality. Supports session history tracking in watch mode.

#### Features

- 🎬 **Multi-Server**: Monitor both Plex and Jellyfin servers simultaneously or individually
- ⏳ **Session History**: Track recently ended sessions in watch mode (configurable duration)
- ⚡ **Fast**: ~10x faster than bash implementations
- 🔧 **Portable**: Single static binary, no dependencies required
- 🎨 **Color Output**: Beautiful ANSI colored terminal output with distinct styling for ended sessions
- 🔄 **Cross-Platform**: Native support for Linux, macOS, Windows, and FreeBSD
- 📊 **JSON Output**: Machine-readable format for automation and monitoring
- 👀 **Watch Mode**: Continuous real-time monitoring with auto-refresh and history tracking
- 📈 **Rich Details**: Shows transcoding status, bandwidth, quality, codecs, duration, and more
- 🎵 **Full Support**: Handles both video (movies/TV) and audio (music) streams

#### Quick Start

```bash
# Build the binary
make build

# Run with both Plex and Jellyfin
./bin/media-streams -server both

# Plex only
./bin/media-streams -server plex -plex-token "your-token"

# Jellyfin only
./bin/media-streams -server jellyfin -jellyfin-token "your-token"

# Watch mode with session history (default: 5 minutes)
./bin/media-streams -watch -interval 5 -history-duration 10m

# JSON output for automation
./bin/media-streams -json | jq '.total_streams'
```

### Queue Remediation

An intelligent tool for automatically detecting and fixing stuck queue items in Sonarr and Radarr. Identifies problematic downloads (failed imports, quality mismatches, sample files) and applies appropriate remediation actions.

#### Features

- 🔍 **Smart Detection**: Automatically identifies stuck/blocked queue items
- 🎯 **Targeted Actions**: Deletes failed items, triggers manual imports, or monitors active downloads
- 🛡️ **Safe Mode**: Dry-run mode shows what would happen without making changes
- 🔄 **Multi-Instance**: Supports multiple Sonarr and Radarr servers simultaneously
- 📊 **REST API Support**: Optional REST API mode for precise manual import control
- 🚨 **Blocklist Management**: Automatically blocklists problematic releases
- 📝 **Detailed Logging**: Verbose and debug modes for troubleshooting
- ⚡ **Fast**: Processes entire queue in seconds

#### Quick Start

```bash
# Build the binary
make build

# Set up configuration
export SONARR_URLS="http://localhost:8989"
export SONARR_TOKENS="your-api-key"
export RADARR_URLS="http://localhost:7878"
export RADARR_TOKENS="your-api-key"

# Dry-run mode (shows what would happen without making changes)
./bin/queue-remediation -dry-run

# Run with verbose logging
./bin/queue-remediation -verbose

# Use REST API for manual imports (more precise)
./bin/queue-remediation -use-rest-api -verbose
```

#### Remediation Actions

The tool intelligently classifies queue items and applies appropriate actions:

- **DELETE + BLOCKLIST**: Quality/custom format mismatches, sample files, failed downloads
- **MANUAL_IMPORT**: Completed downloads stuck in import-blocked state
- **MONITOR**: Active downloads progressing normally (no action needed)

#### Documentation

- **[README_QUEUEFIX.md](docs/README_QUEUEFIX.md)** - Complete usage guide and troubleshooting
- **[QUEUE_REMEDIATION_IMPLEMENTATION_GUIDE.md](docs/QUEUE_REMEDIATION_IMPLEMENTATION_GUIDE.md)** - Technical implementation details
- **[SONARR_RADARR_QUEUE_API.md](docs/SONARR_RADARR_QUEUE_API.md)** - API reference

### Plex Streams Monitor

A high-performance Go-based tool for monitoring active Plex streams with detailed information about users, transcoding status, bandwidth, and quality.

#### Features

- ⚡ **Fast**: ~10x faster than bash/xmlstarlet implementation (~25ms vs ~250ms)
- 🔧 **Portable**: Single static binary, no dependencies required
- 🎨 **Color Output**: Beautiful ANSI colored terminal output
- 🔄 **Cross-Platform**: Native support for Linux, macOS, Windows, and FreeBSD
- 📊 **JSON Output**: Machine-readable format for automation and monitoring
- 👀 **Watch Mode**: Continuous real-time monitoring with auto-refresh
- 📈 **Rich Details**: Shows transcoding status, bandwidth, quality, codecs, and more
- 🎵 **Full Support**: Handles both video (movies/TV) and audio (music) streams

#### Quick Start

```bash
# Build the binary
make build

# Or manually
go build -ldflags "-s -w" -o plex-streams plex-streams.go

# Run with environment variables
export PLEX_URL="http://localhost:32400"
export PLEX_TOKEN="your-plex-token"
./plex-streams

# Or with command-line flags
./plex-streams -url "http://plex:32400" -token "your-token"

# Watch mode (continuous monitoring)
./plex-streams -watch -interval 5

# JSON output for automation
./plex-streams -json | jq '.total_streams'
```

#### Documentation

- **[PLEX-STREAMS-README.md](PLEX-STREAMS-README.md)** - Complete documentation and usage guide
- **[MIGRATION.md](MIGRATION.md)** - Migration guide from bash version with performance comparisons
- **[EXAMPLES.md](EXAMPLES.md)** - Practical examples and integration patterns

#### Performance

| Metric | Bash Version | Go Version | Improvement |
|--------|-------------|------------|-------------|
| Execution Time | ~250ms | ~25ms | **10x faster** |
| Memory Usage | ~8MB | ~3MB | **63% less** |
| Dependencies | 4 packages | 0 | **None** |

#### Building

```bash
# Current platform
make build

# All platforms (Linux, macOS, Windows, FreeBSD)
make build-all

# Install to /usr/local/bin
make install
```

## License

MIT License - Feel free to use and modify
