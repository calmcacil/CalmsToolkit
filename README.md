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
