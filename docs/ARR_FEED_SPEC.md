# arr-feed Technical Specification

## Overview
`arr-feed` is a CLI tool that monitors Sonarr and Radarr history APIs to display a live feed of recent arr activity including imports, downloads, failures, and deletions.

## Architecture

### 1. Data Structures

#### HistoryEvent
Main event structure for unified display:
```go
type HistoryEvent struct {
    Server        string    // "sonarr" or "radarr"
    When          time.Time // Event timestamp
    Action        string    // Human-readable action (Imported, Grabbed, Failed, etc.)
    Title         string    // Series or Movie name
    Episode       string    // "S01E05" format (TV only, empty for movies)
    EpisodeTitle  string    // Episode name (TV only, empty for movies)
    Quality       string    // Quality profile name
    Formats       []string  // Custom format names
    SourceTitle   string    // Release name for grabbed/imported events
    EventType     string    // Raw eventType from API
    ID            int       // History record ID
}
```

#### Sonarr API Structures
```go
type SonarrHistoryResponse struct {
    Page          int              `json:"page"`
    PageSize      int              `json:"pageSize"`
    TotalRecords  int              `json:"totalRecords"`
    Records       []SonarrHistory  `json:"records"`
}

type SonarrHistory struct {
    EpisodeID     int                    `json:"episodeId"`
    SeriesID      int                    `json:"seriesId"`
    SourceTitle   string                 `json:"sourceTitle"`
    Quality       SonarrQuality          `json:"quality"`
    QualityCutoff bool                   `json:"qualityCutoffNotMet"`
    Date          string                 `json:"date"`
    EventType     string                 `json:"eventType"`
    Data          map[string]interface{} `json:"data"`
    Episode       *SonarrEpisode         `json:"episode,omitempty"`
    Series        *SonarrSeries          `json:"series,omitempty"`
    ID            int                    `json:"id"`
}

type SonarrQuality struct {
    Quality       SonarrQualityItem `json:"quality"`
    CustomFormats []CustomFormat    `json:"customFormats"`
    Revision      QualityRevision   `json:"revision,omitempty"`
}

type SonarrQualityItem struct {
    ID         int    `json:"id"`
    Name       string `json:"name"`
    Source     string `json:"source,omitempty"`
    Resolution int    `json:"resolution,omitempty"`
}

type CustomFormat struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type QualityRevision struct {
    Version  int  `json:"version"`
    Real     int  `json:"real"`
    IsRepack bool `json:"isRepack"`
}

type SonarrEpisode struct {
    ID            int    `json:"id"`
    SeasonNumber  int    `json:"seasonNumber"`
    EpisodeNumber int    `json:"episodeNumber"`
    Title         string `json:"title"`
}

type SonarrSeries struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
}
```

#### Radarr API Structures
```go
type RadarrHistoryResponse struct {
    Page         int             `json:"page"`
    PageSize     int             `json:"pageSize"`
    TotalRecords int             `json:"totalRecords"`
    Records      []RadarrHistory `json:"records"`
}

type RadarrHistory struct {
    MovieID       int                    `json:"movieId"`
    SourceTitle   string                 `json:"sourceTitle"`
    Quality       RadarrQuality          `json:"quality"`
    QualityCutoff bool                   `json:"qualityCutoffNotMet"`
    Date          string                 `json:"date"`
    EventType     string                 `json:"eventType"`
    Data          map[string]interface{} `json:"data"`
    Movie         *RadarrMovie           `json:"movie,omitempty"`
    ID            int                    `json:"id"`
}

type RadarrQuality struct {
    Quality       RadarrQualityItem `json:"quality"`
    CustomFormats []CustomFormat    `json:"customFormats"`
    Revision      QualityRevision   `json:"revision,omitempty"`
}

type RadarrQualityItem struct {
    ID         int    `json:"id"`
    Name       string `json:"name"`
    Source     string `json:"source,omitempty"`
    Resolution int    `json:"resolution,omitempty"`
}

type RadarrMovie struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
    Year  int    `json:"year"`
}
```

### 2. Configuration

#### Config Structure
```go
type Config struct {
    SonarrURLs    []string
    SonarrTokens  []string
    RadarrURLs    []string
    RadarrTokens  []string
    PollInterval  time.Duration
    HistoryWindow time.Duration
    Timeout       time.Duration
    NoColor       bool
}
```

#### Environment Variables
```
SONARR_URLS          # Comma-separated URLs
SONARR_TOKENS        # Comma-separated API tokens (matching order)
RADARR_URLS          # Comma-separated URLs
RADARR_TOKENS        # Comma-separated API tokens (matching order)
ARR_FEED_POLL_INTERVAL     # Default: 5s
ARR_FEED_HISTORY_DURATION  # Default: 1h
ARR_FEED_TIMEOUT           # Default: 30s
```

#### CLI Flags
```
-sonarr-urls     string   Sonarr URLs (comma-separated)
-sonarr-tokens   string   Sonarr API tokens (comma-separated)
-radarr-urls     string   Radarr URLs (comma-separated)
-radarr-tokens   string   Radarr API tokens (comma-separated)
-poll            duration Poll interval (watch mode)
-duration        duration History lookback window
-timeout         duration HTTP request timeout
-no-color        bool     Disable colored output
-json            bool     Output JSON instead of table
-watch           bool     Continuous monitoring mode
-show-grabbed    bool     Show grabbed events (default: true)
-show-imported   bool     Show imported events (default: true)
-show-failed     bool     Show failed events (default: true)
-show-deleted    bool     Show deleted events (default: true)
-show-ignored    bool     Show ignored events (default: false)
```

### 3. API Integration

#### Sonarr History Endpoint
```
GET /api/v3/history/since?date=<ISO8601>&includeEpisode=true&includeSeries=true
```

#### Radarr History Endpoint
```
GET /api/v3/history/since?date=<ISO8601>&includeMovie=true
```

#### Event Type Mapping

##### Sonarr Events
- `grabbed` → "Grabbed"
- `downloadFolderImported` → "Imported"
- `downloadFailed` → "Failed"
- `episodeFileDeleted` → "Deleted"
- `episodeFileRenamed` → "Renamed"
- `downloadIgnored` → "Ignored"
- `seriesFolderImported` → "Bulk Import"

##### Radarr Events
- `grabbed` → "Grabbed"
- `downloadFolderImported` → "Imported"
- `downloadFailed` → "Failed"
- `movieFileDeleted` → "Deleted"
- `movieFileRenamed` → "Renamed"
- `downloadIgnored` → "Ignored"

### 4. Output Formatting

#### Table Mode (Default)
```
When          | Action   | Series/Movie           | Episode | Episode Title    | Quality    | Formats
------------- | -------- | ---------------------- | ------- | ---------------- | ---------- | ---------------
5 minutes ago | Imported | Breaking Bad           | S01E05  | Gray Matter      | Bluray-720 | AMZN, DV
12 hours ago  | Grabbed  | The Matrix             |         |                  | Bluray-1080| IMAX
1 day ago     | Failed   | Better Call Saul       | S03E02  | Witness          | WEB-DL     |
```

Colors:
- **Imported**: Green
- **Grabbed**: Cyan
- **Failed**: Red
- **Deleted**: Yellow
- **Ignored**: Gray
- **Renamed**: Blue

#### JSON Mode
```json
[
  {
    "server": "sonarr",
    "when": "2025-10-17T10:30:00Z",
    "action": "Imported",
    "title": "Breaking Bad",
    "episode": "S01E05",
    "episodeTitle": "Gray Matter",
    "quality": "Bluray-720p",
    "formats": ["AMZN", "DV"],
    "sourceTitle": "Breaking.Bad.S01E05.720p.BluRay.x264-DEMAND"
  }
]
```

### 5. Implementation Flow

#### Initialization
1. Load configuration (3-tier: .env → env vars → flags)
2. Validate URLs and tokens match in count
3. Test connectivity to all configured instances
4. Initialize last-seen timestamp (now - history window)

#### Single Run Mode (Default)
1. Fetch history from all Sonarr instances (parallel goroutines)
2. Fetch history from all Radarr instances (parallel goroutines)
3. Parse and convert to unified HistoryEvent structures
4. Merge and sort events by timestamp (descending)
5. Filter based on event type flags
6. Render table or JSON output
7. Exit

#### Watch Mode (-watch)
1. Clear screen and hide cursor
2. Execute single run logic
3. Store last-seen event timestamp
4. Sleep for poll interval
5. On next iteration, use `since` parameter with last timestamp
6. Append new events to in-memory cache (max 100 recent events)
7. Re-render screen with updated relative times
8. Repeat until Ctrl+C

#### Error Handling
- If Sonarr instance fails: log warning, continue with other instances
- If Radarr instance fails: log warning, continue with other instances
- If all instances fail: exit with error
- Network timeout: use configured timeout value
- API errors: display status code and response body

### 6. Time Display Logic

Format relative timestamps:
- < 1 minute: "Just now"
- < 60 minutes: "X minutes ago" (round up)
- < 24 hours: "X hours ago" (round up)
- < 7 days: "X days ago"
- >= 7 days: "YYYY-MM-DD HH:MM"

### 7. Testing Strategy

#### Unit Tests (arr-feed_test.go)
- Config loading from env/flags/file
- Event type mapping (Sonarr/Radarr → Action)
- Time formatting (relative time display)
- Episode formatting (S##E##)
- Quality extraction
- Custom format extraction
- Event merging and sorting

#### Mock HTTP Responses
- Sonarr history with various event types
- Radarr history with various event types
- Error responses (401, 404, 500)
- Timeout scenarios

#### Integration Tests
- Multi-instance configuration
- Event deduplication across instances
- Filter flag combinations
- Watch mode polling logic

### 8. Build Integration

#### Makefile Updates
```makefile
BINARY_ARRFEED=arr-feed

build:
    $(GOBUILD) $(LDFLAGS) -tags arrfeed -o $(BUILD_DIR)/$(BINARY_ARRFEED) arr-feed.go

test:
    $(GOTEST) -tags arrfeed -v ./...
```

#### .env.example Updates
Add Sonarr/Radarr configuration examples as shown in Configuration section above.

## Dependencies

All stdlib except:
- `golang.org/x/term` (already in go.mod for terminal handling)

No additional external dependencies required.
