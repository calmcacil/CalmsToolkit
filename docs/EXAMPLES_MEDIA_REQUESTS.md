# Media Requests - Usage Examples

This document provides practical examples for using the `media-requests` interactive CLI tool.

## Table of Contents

- [Initial Setup](#initial-setup)
- [Basic Workflows](#basic-workflows)
- [Advanced Scenarios](#advanced-scenarios)
- [Integration Examples](#integration-examples)
- [Common Issues](#common-issues)

## Initial Setup

### Setup Wizard (Recommended)

```bash
# Generate configuration interactively
make setup

# Build the tool
make build

# Run the tool
./bin/media-requests
```

### Command-Line Flags

```bash
# Quick connection to local Overseerr
./bin/media-requests \
  -url "http://localhost:5055" \
  -token "your-api-key"
```

### Getting Your API Key

1. Log into Overseerr/Jellyseerr web interface
2. Navigate to **Settings** > **General**
3. Find the **API Key** section
4. Copy the generated key

## Basic Workflows

### Request a Movie

```
$ ./bin/media-requests

╔══════════════════════════════════════════╗
║    Media Requests - Interactive Menu    ║
╚══════════════════════════════════════════╝

[N] New Request
[W] View Requests
[Q] Quit

Select an option: n

=== New Media Request ===

Enter search query (or 'back' to return): inception

=== Search Results ===

1. 🎬 Inception (2010)
   A thief who steals corporate secrets...
   Rating: 8.4/10

2. 🎬 Inception: The Documentary (2016)
   ...

Select a number (1-10) or 'back' to cancel: 1

=== Confirm Request ===

Media: Inception (2010)
Type: Movie

Submit request? (y/n): y

✓ Request submitted successfully!
Request ID: 42
Status: Pending Approval
```

### Request a TV Show

```
$ ./bin/media-requests

[N] New Request
Enter search query: breaking bad

=== Search Results ===

1. 📺 Breaking Bad (2008)
2. 🎬 Breaking Bad: El Camino (2019)

Select: 1

=== Select Seasons ===

TV Show: Breaking Bad
Total Seasons: 5

[A] Request all seasons
[S] Select specific seasons
[B] Back

Select option: s
Enter season numbers: 1,2,3,4,5

=== Confirm Request ===

Media: Breaking Bad (2008)
Type: Tv
Seasons: [1 2 3 4 5]

Submit request? (y/n): y
```

## Advanced Scenarios

### Approving a Request with a Root Folder Override

When approving a request, you can override the destination root folder:

```
=== Pending Requests ===

1. [Movie] Request ID: 42 (TMDB: 27205)
   Requested by: jane_smith  Created: 2025-01-15 11:45

Select a request (1-1): 1

=== Request Details ===

Request ID: 42
TMDB ID: 27205
Type: Movie
Requested by: jane_smith
Created: 2025-01-15 11:45
Status: Pending Approval

Actions:
[A] Approve    [D] Decline    [B] Back

Select action: a

=== Approve Request - Root Folder Override ===

...request details...
Current Root Folder: Not set (will use server default)

Would you like to override the root folder for this request?
[Y] Yes, select root folder
[N] No, use default (proceed with approval)
[B] Back (cancel approval)

Select option: y

=== Select Root Folder ===

Server: Radarr HD

Root folders:
1. /data/movies/4k
2. /data/movies/hd

Select a root folder (1-2): 1

✓ Request approved!
  Root folder set to: /data/movies/4k
```

## Integration Examples

### Non-Interactive Mode

While the primary interface is interactive, you can pipe input for basic automation:

```bash
echo -e "n\nThe Matrix\n1\ny\nq" | ./bin/media-requests -url "http://localhost:5055" -token "key"
```

### Container/Docker

```bash
docker run -it --rm \
  -v ~/.config/calmstoolkit:/root/.config/calmstoolkit \
  calmstoolkit/media-requests
```

### SSH Tunnel

```bash
ssh -L 5055:localhost:5055 user@remote-server
./bin/media-requests
```

## Common Issues

### "ERROR: API key is not set"

Set your API key in the config file (`~/.config/calmstoolkit/config.json`) or use `-token` flag.

### "ERROR: Failed to connect to server"

- Verify the server is running
- Check the URL is correct (including http:// or https://)
- Ensure no firewall is blocking the connection

### "Invalid API key"

Generate a new API key in Overseerr/Jellyseerr Settings > General.

### Overseerr API Bug Warning

If you see a warning about an Overseerr API bug, this is the tool automatically working around a known issue. No action needed.

### Output Doesn't Look Right

- Use `-no-color` if ANSI colors don't display correctly
- Use `-json` for machine-readable output
