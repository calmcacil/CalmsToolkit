# media-requests

Interactive CLI tool for managing media requests through Overseerr/Jellyseerr.

## Overview

`media-requests` provides an interactive menu-driven interface to:
- Search for movies and TV shows
- Create new media requests
- View pending requests
- Approve or decline requests

This tool communicates directly with the Overseerr/Jellyseerr API, providing a streamlined command-line experience for media request management.

## Features

- **Interactive Menu System**: Easy-to-navigate menu with hotkeys (N/W/Q)
- **Search Media**: Search TMDB database through Overseerr/Jellyseerr
- **Smart Season Selection**: For TV shows, select all seasons or specific ones
- **Request Management**: View, approve, and decline pending requests
- **Status Indicators**: Visual feedback for available/requested/pending media
- **Colored Output**: ANSI colors for better readability (can be disabled)
- **Flexible Configuration**: Environment variables, .env files, or command-line flags

## Installation

### Build from Source

```bash
# Build for current platform
make build

# Install to ~/.local/bin (or /usr/local/bin)
make install

# Build for all platforms
make build-all
```

The binary will be available as `bin/media-requests`.

### Pre-built Binaries

Download pre-built binaries from the [Releases](https://github.com/calmcacil/CalmsToolkit/releases) page.

## Configuration

### Environment Variables

The tool supports configuration via environment variables:

```bash
# Overseerr configuration
export OVERSEERR_URL="http://localhost:5055"
export OVERSEERR_TOKEN="your-api-key-here"

# OR Jellyseerr configuration
export JELLYSEERR_URL="http://localhost:5055"
export JELLYSEERR_TOKEN="your-api-key-here"
```

### Configuration File

You can also use a `.env` file at `/opt/apps/compose/.env`:

```bash
OVERSEERR_URL=http://overseerr:5055
OVERSEERR_TOKEN=your-api-key-here
```

### Command-Line Flags

Flags override environment variables:

```bash
./bin/media-requests \
  -url "http://overseerr:5055" \
  -token "your-api-key-here"
```

### Getting Your API Key

1. Log into Overseerr/Jellyseerr web interface
2. Navigate to **Settings** > **General**
3. Find the **API Key** section
4. Copy the generated key

## Usage

### Basic Usage

```bash
# Run with environment variables
./bin/media-requests

# Run with command-line flags
./bin/media-requests -url "http://overseerr:5055" -token "your-token"

# Disable colored output
./bin/media-requests -no-color
```

### Interactive Menu

When you launch the tool, you'll see the main menu:

```
╔══════════════════════════════════════════╗
║    Media Requests - Interactive Menu    ║
╚══════════════════════════════════════════╝

[N] New Request
[W] View Requests
[Q] Quit

Select an option:
```

#### Creating a New Request (N)

1. **Search**: Enter a movie or TV show name
   ```
   Enter search query (or 'back' to return): breaking bad
   ```

2. **Select Media**: Choose from search results
   ```
   1. 📺 Breaking Bad (2008) [AVAILABLE]
      A high school chemistry teacher turned meth cook...
      Rating: 9.2/10

   2. 🎬 Breaking Bad: The Movie (2017)
      ...

   Select a number (1-10) or 'back' to cancel: 1
   ```

3. **Choose Seasons** (TV shows only):
   ```
   [A] Request all seasons
   [S] Select specific seasons
   [B] Back

   Select option: s
   Enter season numbers (comma-separated, e.g., 1,2,3): 1,2,3
   ```

4. **Confirm Request**:
   ```
   Media: Breaking Bad (2008)
   Type: Tv
   Seasons: [1 2 3]

   Submit request? (y/n): y

   ✓ Request submitted successfully!
   Request ID: 42
   Status: Pending Approval
   ```

#### Viewing Requests (W)

1. **List Pending Requests**:
   ```
   === Pending Requests ===

   1. [TV Show] Request ID: 42 (TMDB: 1396)
      Requested by: john_doe  Created: 2025-01-15 10:30

   2. [Movie] Request ID: 43 (TMDB: 550)
      Requested by: jane_smith  Created: 2025-01-15 11:45

   Select a request (1-2), or 'back' to return:
   ```

2. **View Request Details**:
   ```
   === Request Details ===

   Request ID: 42
   TMDB ID: 1396
   Type: TV Show
   Requested by: john_doe
   Created: 2025-01-15 10:30
   Status: Pending Approval
   Seasons requested: 3
   ```

3. **Take Action**:
   ```
   Actions:
   [A] Approve    [D] Decline    [B] Back

   Select action: a

   Approving request...
   ✓ Request approved!
   ```

## Command-Line Options

```bash
Usage of media-requests:
  -url string
        Overseerr/Jellyseerr server URL
  -token string
        API key/token
  -timeout duration
        Connection timeout (default 30s)
  -no-color
        Disable colored output
```

## Configuration Priority

Configuration is loaded in the following order (highest priority last):

1. `.env` file at `/opt/apps/compose/.env`
2. Environment variables (`OVERSEERR_URL`, `OVERSEERR_TOKEN`)
3. Command-line flags (`-url`, `-token`)

## API Endpoints Used

The tool interacts with the following Overseerr/Jellyseerr API endpoints:

- `GET /api/v1/auth/me` - Verify authentication
- `GET /api/v1/search` - Search for media
- `GET /api/v1/tv/{id}` - Get TV show details (season count)
- `POST /api/v1/request` - Create new request
- `GET /api/v1/request` - Get pending requests
- `POST /api/v1/request/{id}/approve` - Approve request
- `POST /api/v1/request/{id}/decline` - Decline request

## Permissions

Different actions require different permission levels in Overseerr/Jellyseerr:

- **Creating Requests**: Requires `REQUEST` permission
- **Viewing All Requests**: Requires `ADMIN` or `MANAGE_REQUESTS` permission
- **Approving/Declining**: Requires `ADMIN` or `MANAGE_REQUESTS` permission

Regular users can only view and manage their own requests.

## Examples

### Search and Request a Movie

```bash
$ ./bin/media-requests

# Select [N] for New Request
# Enter: "Fight Club"
# Select: 1 (the 1999 movie)
# Confirm: y
```

### Request All Seasons of a TV Show

```bash
$ ./bin/media-requests

# Select [N] for New Request
# Enter: "Breaking Bad"
# Select: 1
# Choose: [A] Request all seasons
# Confirm: y
```

### Approve Pending Requests

```bash
$ ./bin/media-requests

# Select [W] for View Requests
# Select: 1 (first pending request)
# Action: [A] Approve
```

### Request Specific TV Seasons

```bash
$ ./bin/media-requests

# Select [N] for New Request
# Enter: "Game of Thrones"
# Select: 1
# Choose: [S] Select specific seasons
# Enter: 1,2,3,4
# Confirm: y
```

## Troubleshooting

### "ERROR: API key is not set"

Make sure you've set either:
- `OVERSEERR_TOKEN` or `JELLYSEERR_TOKEN` environment variable
- Provided `-token` flag
- Added the token to `/opt/apps/compose/.env`

### "ERROR: Failed to connect to server"

Verify that:
- The server URL is correct
- Overseerr/Jellyseerr is running and accessible
- The API key is valid
- There are no firewall/network issues

### "invalid API key"

Your API key may be incorrect or expired. Generate a new one from Settings > General in the web UI.

### "search failed: status 404"

The search endpoint couldn't be reached. Verify your server URL includes the protocol (http:// or https://).

### Colorized Output Not Working

Colors work on:
- Linux/macOS terminals
- Windows 10+ (with ANSI support)

If colors aren't displaying correctly, use the `-no-color` flag.

## Technical Details

### Architecture

- **Single Binary**: No external dependencies, uses only Go stdlib
- **Interactive CLI**: Menu-driven interface with stdin/stdout
- **ANSI Colors**: Supports color-coded output for clarity
- **HTTP Client**: 30-second default timeout for API calls
- **Error Handling**: Comprehensive error messages with context

### Media Status Codes

- `1` - Unknown
- `2` - Pending
- `3` - Processing
- `4` - Partially Available
- `5` - Available
- `6` - Deleted

### Request Status Codes

- `1` - Pending Approval
- `2` - Approved
- `3` - Declined

## Compatibility

- **Overseerr**: All versions with API v1
- **Jellyseerr**: All versions (uses same API as Overseerr)
- **Platforms**: Linux, macOS, Windows, FreeBSD
- **Architectures**: amd64, arm64

## Performance

- **Startup Time**: < 100ms
- **Search Latency**: Depends on Overseerr/TMDB response time
- **Memory Usage**: ~10MB
- **Binary Size**: ~6MB (compressed)

## Security Considerations

- **API Keys**: Never commit API keys to version control
- **HTTPS**: Use HTTPS URLs in production for encrypted communication
- **Permissions**: Ensure users have appropriate Overseerr permissions
- **Tokens**: API tokens should be treated as passwords

## Related Tools

- **media-streams**: Monitor active Plex/Jellyfin streams
- **media-calendar**: Display upcoming media releases

## Support

For issues, feature requests, or questions:
- GitHub Issues: [CalmsToolkit Issues](https://github.com/calmcacil/CalmsToolkit/issues)
- See also: [OVERSEERR_API_RESEARCH.md](docs/OVERSEERR_API_RESEARCH.md)

## License

Part of CalmsToolkit - see main repository for license information.
