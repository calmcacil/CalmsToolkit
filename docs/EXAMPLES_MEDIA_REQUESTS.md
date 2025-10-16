# Media Requests - Usage Examples

This document provides practical examples for using the `media-requests` interactive CLI tool.

## Table of Contents

- [Initial Setup](#initial-setup)
- [Basic Workflows](#basic-workflows)
- [Advanced Scenarios](#advanced-scenarios)
- [Integration Examples](#integration-examples)
- [Common Issues](#common-issues)

## Initial Setup

### Quick Setup (Environment Variables)

```bash
# For Overseerr
export OVERSEERR_URL="http://localhost:5055"
export OVERSEERR_TOKEN="your-api-key-from-settings"

# For Jellyseerr
export JELLYSEERR_URL="http://localhost:5055"
export JELLYSEERR_TOKEN="your-api-key-from-settings"

# Run the tool
./bin/media-requests
```

### Docker/Compose Setup

If you're using Docker Compose, add to your `.env` file:

```env
# /opt/apps/compose/.env
OVERSEERR_URL=http://overseerr:5055
OVERSEERR_TOKEN=REMOVED_SECRET
```

The tool will automatically read this file.

### Remote Server Setup

```bash
# Connect to remote Overseerr instance
./bin/media-requests \
  -url "https://overseerr.yourdomain.com" \
  -token "your-api-key"
```

## Basic Workflows

### Example 1: Request a Popular Movie

```
1. Launch the tool:
   $ ./bin/media-requests

2. Select [N] for New Request

3. Search for a movie:
   Enter search query: avengers endgame

4. Review results:
   1. 🎬 Avengers: Endgame (2019)
      After the devastating events of Avengers: Infinity War...
      Rating: 8.3/10

5. Select the movie:
   Select a number (1-10): 1

6. Confirm request:
   Media: Avengers: Endgame (2019)
   Type: Movie

   Submit request? (y/n): y

7. Success!
   ✓ Request submitted successfully!
   Request ID: 123
   Status: Pending Approval
```

### Example 2: Request All Seasons of a TV Show

```
1. Launch the tool and select [N]

2. Search:
   Enter search query: breaking bad

3. Select the show:
   1. 📺 Breaking Bad (2008)
   Select a number: 1

4. Choose season option:
   [A] Request all seasons
   [S] Select specific seasons
   [B] Back

   Select option: a

5. Confirm:
   Media: Breaking Bad (2008)
   Type: Tv
   Seasons: All

   Submit request? (y/n): y
```

### Example 3: Request Specific Seasons

```
1. Launch and select [N]

2. Search: the office

3. Select: 1 (US version)

4. Choose specific seasons:
   Select option: s

   Enter season numbers (comma-separated): 1,2,3,4,5

5. Confirm:
   Media: The Office (2005)
   Type: Tv
   Seasons: [1 2 3 4 5]

   Submit request? (y/n): y
```

### Example 4: Approve Pending Requests

```
1. Launch and select [W] for View Requests

2. Review pending requests:
   === Pending Requests ===

   1. [TV Show] Request ID: 42 (TMDB: 1396)
      Requested by: john_doe  Created: 2025-01-15 10:30

   2. [Movie] Request ID: 43 (TMDB: 550)
      Requested by: jane_smith  Created: 2025-01-15 11:45

3. Select a request:
   Select a request (1-2): 1

4. View details:
   === Request Details ===

   Request ID: 42
   TMDB ID: 1396
   Type: TV Show
   Requested by: john_doe
   Created: 2025-01-15 10:30
   Status: Pending Approval
   Seasons requested: 3

5. Take action:
   [A] Approve    [D] Decline    [B] Back

   Select action: a

6. Confirmation:
   ✓ Request approved!
```

### Example 5: Decline a Request

```
1. Launch and select [W]

2. Select a request: 2

3. Choose decline:
   Select action: d

4. Confirm:
   Are you sure you want to decline this request? (y/n): y

   Declining request...
   ✓ Request declined.
```

## Advanced Scenarios

### Scenario 1: Check if Media is Already Available

When you search, the tool shows status indicators:

```
Search results for "inception":

1. 🎬 Inception (2010) [AVAILABLE]
   A thief who steals corporate secrets...

2. 🎬 Inception: The Making (2010) [REQUESTED]
   Documentary about the making of...
```

- `[AVAILABLE]` - Already on your server, no need to request
- `[REQUESTED]` - Already requested by someone
- No indicator - Available to request

### Scenario 2: Request 4K Content

Currently, the tool submits requests with default settings. To request 4K:

1. Submit the request normally through the tool
2. Have an admin modify the request in Overseerr web UI to set `is4k: true`
3. Or modify the code to add 4K selection (see [Contributing](#contributing))

### Scenario 3: Batch Request Management

For managing multiple requests:

```bash
# Create a simple script
#!/bin/bash

# Approve all pending requests (requires admin access)
# Note: This would need to be automated separately as the tool is interactive

# Instead, use the tool's interactive mode:
./bin/media-requests

# Then in the menu:
# [W] -> Review each request -> [A] to approve
```

### Scenario 4: Search with Special Characters

```
Search queries work well with special characters:

- "lord of the rings"
- "star wars: episode v"
- "2001: a space odyssey"
- "amélie"
```

### Scenario 5: Cancel a Search

At any point during a search or request:

```
Enter search query (or 'back' to return): back

# Or press Enter on an empty line to cancel
Select a number (1-10) or 'back' to cancel: [Enter]
```

## Integration Examples

### Example 1: Systemd Service (Auto-start)

```ini
# /etc/systemd/system/media-requests.service
[Unit]
Description=Media Requests Interactive Tool
After=network.target

[Service]
Type=simple
User=mediauser
Environment="OVERSEERR_URL=http://localhost:5055"
Environment="OVERSEERR_TOKEN=your-token"
ExecStart=/usr/local/bin/media-requests
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### Example 2: Docker Container

```dockerfile
FROM alpine:latest

COPY bin/media-requests /usr/local/bin/media-requests
RUN chmod +x /usr/local/bin/media-requests

ENV OVERSEERR_URL=http://overseerr:5055
ENV OVERSEERR_TOKEN=your-token

ENTRYPOINT ["/usr/local/bin/media-requests"]
```

### Example 3: SSH Remote Access

```bash
# Access from remote machine
ssh user@server -t "./bin/media-requests"

# Or with port forwarding
ssh user@server -L 5055:overseerr:5055
./bin/media-requests -url "http://localhost:5055" -token "your-token"
```

### Example 4: Scheduled Request Review

```bash
#!/bin/bash
# review-requests.sh

# Send notification when new requests are pending
# (This would need custom scripting to parse request counts)

source /opt/apps/compose/.env

# You could extend the tool to support --list-pending-count flag
# for automation purposes
```

## Common Issues

### Issue 1: "API key is not set"

**Solution:**
```bash
# Verify your environment variables
echo $OVERSEERR_TOKEN

# Or set them:
export OVERSEERR_TOKEN="your-key-here"

# Or use flags:
./bin/media-requests -token "your-key-here"
```

### Issue 2: Search Returns No Results

**Possible causes:**
- Typo in search query
- TMDB database doesn't have the content
- Network/API timeout

**Solution:**
```bash
# Try different search terms:
# Instead of: "lotr"
# Try: "lord of the rings"

# Check if Overseerr is accessible:
curl http://localhost:5055/api/v1/status
```

### Issue 3: "Permission denied" When Approving

**Cause:** User doesn't have `MANAGE_REQUESTS` or `ADMIN` permission in Overseerr.

**Solution:**
1. Log into Overseerr web UI as admin
2. Go to Settings > Users
3. Grant user appropriate permissions

### Issue 4: Colors Not Displaying

**Solution:**
```bash
# Disable colors explicitly:
./bin/media-requests -no-color

# Or check terminal support:
echo $TERM  # Should be xterm-256color or similar
```

### Issue 5: Connection Timeout

**Solution:**
```bash
# Increase timeout:
./bin/media-requests -timeout 60s

# Check network:
ping overseerr-host
curl http://overseerr:5055/api/v1/status
```

## Tips and Tricks

### Tip 1: Quick Navigation

Memorize the hotkeys:
- `N` - New request (fast access to search)
- `W` - View/manage requests
- `Q` - Quit
- `back` - Cancel any operation

### Tip 2: Efficient TV Show Requests

For ongoing series, request all seasons first. You can always add new seasons later as they're released.

### Tip 3: Search Precision

Be specific with search terms:
- Good: "breaking bad"
- Less good: "bb" or "break"

Include year for disambiguation:
- "dune 2021" vs "dune 1984"

### Tip 4: Batch Approvals

If you have many pending requests:
1. Open the tool
2. Press `W` to view requests
3. Quickly review and approve/decline each one
4. Use hotkeys `A` and `D` for speed

### Tip 5: Status Indicators

Pay attention to status indicators in search results:
- `[AVAILABLE]` - Don't waste time requesting
- `[REQUESTED]` - Someone already asked for it
- No indicator - Safe to request

## Contributing

Want to add features? The code is open source!

Ideas for contributions:
- Add 4K request option during request creation
- Add ability to request specific episodes (not just seasons)
- Add request history/statistics
- Add user profile management
- Add search filters (genre, year, rating)

See the main repository for contribution guidelines.

## More Information

- [README_MEDIA_REQUESTS.md](README_MEDIA_REQUESTS.md) - Full documentation
- [OVERSEERR_API_RESEARCH.md](docs/OVERSEERR_API_RESEARCH.md) - API reference
- [GitHub Issues](https://github.com/calmcacil/CalmsToolkit/issues) - Report bugs or request features
