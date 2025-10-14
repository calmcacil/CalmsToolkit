# CalmsToolkit
Various tools for my server and associated apps.

## Tools

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
