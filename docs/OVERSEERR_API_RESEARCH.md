# Overseerr/Jellyseerr API Research

## Overview

Overseerr is a request management and media discovery tool for Plex/Jellyfin ecosystems. Jellyseerr is a fork that works with Jellyfin specifically but uses the same API structure.

**Base URL Format**: `http://[host]:[port]/api/v1`
- Default port: 5055
- Example: `http://localhost:5055/api/v1`

---

## Authentication

Two methods are supported:

### 1. API Key Authentication (Recommended)
Pass the API key in the request header:
```
X-Api-Key: your-api-key-here
```

**Getting your API key:**
- Navigate to Settings > General in the Overseerr web UI
- Find the "API Key" section
- Copy the generated key

### 2. Cookie Authentication
Sign in via `/auth/plex` or `/auth/local` endpoints to receive a session cookie.

---

## Core API Endpoints

### Search for Media

#### Search Movies/TV Shows/People
```
GET /search
```

**Query Parameters:**
- `query` (required, string): Search term
- `page` (optional, number): Page number (default: 1)
- `language` (optional, string): Language code (e.g., "en")

**Response:** Returns array of mixed results (MovieResult, TvResult, PersonResult)

**MovieResult Fields:**
```json
{
  "id": 550,
  "mediaType": "movie",
  "title": "Fight Club",
  "originalTitle": "Fight Club",
  "popularity": 123.45,
  "posterPath": "/path/to/poster.jpg",
  "backdropPath": "/path/to/backdrop.jpg",
  "voteCount": 1234,
  "voteAverage": 8.5,
  "genreIds": [18, 53],
  "overview": "Movie description...",
  "originalLanguage": "en",
  "releaseDate": "1999-10-15",
  "adult": false,
  "video": false,
  "mediaInfo": {
    "id": 123,
    "tmdbId": 550,
    "status": 5,
    "requests": []
  }
}
```

**TvResult Fields:**
```json
{
  "id": 1234,
  "mediaType": "tv",
  "name": "Breaking Bad",
  "originalName": "Breaking Bad",
  "popularity": 234.56,
  "posterPath": "/path/to/poster.jpg",
  "backdropPath": "/path/to/backdrop.jpg",
  "voteCount": 5678,
  "voteAverage": 9.2,
  "genreIds": [18, 80],
  "overview": "TV show description...",
  "originalLanguage": "en",
  "originCountry": ["US"],
  "firstAirDate": "2008-01-20",
  "mediaInfo": {
    "id": 456,
    "tmdbId": 1396,
    "tvdbId": 81189,
    "status": 5,
    "requests": []
  }
}
```

#### Discover Movies
```
GET /discover/movies
```

**Query Parameters:**
- `page` (number)
- `language` (string)
- `genre` (string): Genre ID
- `studio` (string): Studio ID
- `keywords` (string): Keyword IDs
- `sortBy` (string): Sort field
- `primaryReleaseDateGte` (string): Release date from
- `primaryReleaseDateLte` (string): Release date to
- `withRuntimeGte` (number): Minimum runtime
- `withRuntimeLte` (number): Maximum runtime
- `voteAverageGte` (number): Minimum rating
- `voteAverageLte` (number): Maximum rating
- `watchRegion` (string): Region code
- `watchProviders` (string): Provider IDs

#### Discover TV Shows
```
GET /discover/tv
```

Similar parameters to movies, with TV-specific options:
- `network` (string): Network ID
- `firstAirDateGte/Lte` (string): Air date range

#### Trending Content
```
GET /discover/trending
```

Returns currently trending movies and TV shows.

#### Upcoming Releases
```
GET /discover/movies/upcoming
GET /discover/tv/upcoming
```

---

### Request Management

#### Create New Request
```
POST /request
```

**Request Body:**
```json
{
  "mediaType": "movie",  // Required: "movie" or "tv"
  "mediaId": 550,        // Required: TMDB ID
  "tvdbId": 81189,       // Optional: TVDB ID (for TV shows)
  "seasons": [1, 2, 3],  // Optional: Array of season numbers OR "all"
  "is4k": false,         // Optional: Request 4K version
  "serverId": 1,         // Optional: Radarr/Sonarr server ID
  "profileId": 1,        // Optional: Quality profile ID
  "rootFolder": "/media/movies",  // Optional: Storage path
  "languageProfileId": 1,         // Optional: Language profile (TV only)
  "userId": 1,           // Optional: User ID (admin only)
  "tags": [1, 2]         // Optional: Tag IDs
}
```

**Minimal Example (Movie):**
```json
{
  "mediaType": "movie",
  "mediaId": 550
}
```

**TV Show Example (All Seasons):**
```json
{
  "mediaType": "tv",
  "mediaId": 1396,
  "seasons": "all"
}
```

**TV Show Example (Specific Seasons):**
```json
{
  "mediaType": "tv",
  "mediaId": 1396,
  "seasons": [1, 2, 3]
}
```

**Response:** Returns MediaRequest object with ID and status

**Permissions:**
- Requires `REQUEST` permission
- Auto-approved for users with `ADMIN` or `AUTO_APPROVE` permissions

**Error Responses:**
- 403: Permission denied or quota exceeded
- 409: Duplicate request already exists
- 202: No seasons available (accepted but not processed)

#### Get All Requests
```
GET /request
```

**Query Parameters:**
- `take` (number): Results per page
- `skip` (number): Results to skip (for pagination)
- `filter` (string): Filter criteria ("all", "approved", "pending", "available")
- `sort` (string): Sort field (default: "added")
- `requestedBy` (number): Filter by user ID

**Response:**
```json
{
  "pageInfo": {
    "pages": 5,
    "pageSize": 20,
    "results": 95,
    "page": 1
  },
  "results": [
    {
      "id": 123,
      "status": 2,
      "media": {
        "id": 456,
        "tmdbId": 550,
        "status": 2
      },
      "createdAt": "2023-10-15T10:30:00.000Z",
      "updatedAt": "2023-10-15T11:00:00.000Z",
      "requestedBy": {
        "id": 1,
        "email": "user@example.com",
        "username": "user123"
      },
      "modifiedBy": null,
      "is4k": false,
      "serverId": 1,
      "profileId": 1,
      "rootFolder": "/media/movies"
    }
  ]
}
```

**Permissions:**
- Users with `ADMIN` or `MANAGE_REQUESTS` see all requests
- Regular users see only their own requests

#### Get Single Request
```
GET /request/{requestId}
```

**Response:** Returns MediaRequest object

#### Update Request Status (Approve/Deny)
```
POST /request/{requestId}/{status}
```

**Path Parameters:**
- `requestId` (required): Request ID
- `status` (required): New status - "approve" or "decline"

**Examples:**
```
POST /request/123/approve
POST /request/123/decline
```

**Permissions:** Requires `MANAGE_REQUESTS` or `ADMIN` permission

**Response:** Returns updated MediaRequest object

#### Update Request Details
```
PUT /request/{requestId}
```

**Request Body:**
```json
{
  "serverId": 2,
  "profileId": 3,
  "rootFolder": "/media/movies-4k",
  "tags": [1, 2, 3]
}
```

**Permissions:** Requires `MANAGE_REQUESTS` permission

#### Delete Request
```
DELETE /request/{requestId}
```

**Permissions:**
- Users with `MANAGE_REQUESTS` can delete any request
- Regular users can only delete their own pending requests

#### Get Request Counts
```
GET /request/count
```

**Response:**
```json
{
  "pending": 5,
  "approved": 12,
  "processing": 3,
  "available": 45,
  "total": 65
}
```

#### Retry Failed Request
```
POST /request/{requestId}/retry
```

Attempts to re-send a failed request to Radarr/Sonarr.

---

### User Management

#### Get Current User
```
GET /auth/me
```

Returns the authenticated user's information including permissions.

**Response:**
```json
{
  "id": 1,
  "email": "user@example.com",
  "username": "user123",
  "plexUsername": "PlexUser",
  "permissions": 2048,
  "avatar": "/path/to/avatar.jpg",
  "requestCount": 15,
  "movieQuotaLimit": null,
  "movieQuotaDays": null,
  "tvQuotaLimit": null,
  "tvQuotaDays": null
}
```

#### Get User Requests
```
GET /user/{userId}/requests
```

**Query Parameters:**
- `take` (number)
- `skip` (number)

#### Get User Quota
```
GET /user/{userId}/quota
```

Returns current quota usage for the user.

---

### Movie & TV Details

#### Get Movie Details
```
GET /movie/{movieId}
```

Returns detailed movie information including cast, crew, ratings, and availability status.

#### Get TV Show Details
```
GET /tv/{tvId}
```

Returns detailed TV show information.

#### Get Season Details
```
GET /tv/{tvId}/season/{seasonId}
```

Returns season details and episode list.

#### Get Recommendations
```
GET /movie/{movieId}/recommendations
GET /tv/{tvId}/recommendations
```

Returns similar/recommended content.

---

### Service Configuration

#### Get Radarr Servers
```
GET /service/radarr
```

Returns list of configured Radarr servers (non-sensitive data only).

#### Get Sonarr Servers
```
GET /service/sonarr
```

Returns list of configured Sonarr servers (non-sensitive data only).

#### Get Server Profiles
```
GET /service/radarr/{radarrId}
GET /service/sonarr/{sonarrId}
```

Returns quality profiles and root folders for the specified server.

---

## Status Codes and Enums

### Media Status (MediaInfo.status)
Numeric values representing media availability:
- `1` = UNKNOWN
- `2` = PENDING
- `3` = PROCESSING
- `4` = PARTIALLY_AVAILABLE
- `5` = AVAILABLE
- `6` = DELETED

### Request Status (MediaRequest.status)
Numeric values representing request state:
- `1` = PENDING_APPROVAL
- `2` = APPROVED
- `3` = DECLINED

### Media Type
String values:
- `"movie"` = Movie
- `"tv"` = TV Show

---

## Go Implementation Guide

### Struct Definitions

```go
// Request creation
type CreateRequest struct {
    MediaType         string   `json:"mediaType"`          // Required: "movie" or "tv"
    MediaId           int      `json:"mediaId"`            // Required: TMDB ID
    TvdbId            *int     `json:"tvdbId,omitempty"`   // Optional: TVDB ID
    Seasons           interface{} `json:"seasons,omitempty"` // []int or "all"
    Is4k              *bool    `json:"is4k,omitempty"`
    ServerId          *int     `json:"serverId,omitempty"`
    ProfileId         *int     `json:"profileId,omitempty"`
    RootFolder        *string  `json:"rootFolder,omitempty"`
    LanguageProfileId *int     `json:"languageProfileId,omitempty"`
    UserId            *int     `json:"userId,omitempty"`
    Tags              []int    `json:"tags,omitempty"`
}

// Media request response
type MediaRequest struct {
    Id          int          `json:"id"`
    Status      int          `json:"status"`
    Media       *MediaInfo   `json:"media,omitempty"`
    CreatedAt   *string      `json:"createdAt,omitempty"`
    UpdatedAt   *string      `json:"updatedAt,omitempty"`
    RequestedBy *User        `json:"requestedBy,omitempty"`
    ModifiedBy  *User        `json:"modifiedBy,omitempty"`
    Is4k        *bool        `json:"is4k,omitempty"`
    ServerId    *int         `json:"serverId,omitempty"`
    ProfileId   *int         `json:"profileId,omitempty"`
    RootFolder  *string      `json:"rootFolder,omitempty"`
}

// Media information
type MediaInfo struct {
    Id        int             `json:"id"`
    TmdbId    int             `json:"tmdbId"`
    TvdbId    *int            `json:"tvdbId,omitempty"`
    Status    int             `json:"status"`
    Requests  []MediaRequest  `json:"requests,omitempty"`
    CreatedAt *string         `json:"createdAt,omitempty"`
    UpdatedAt *string         `json:"updatedAt,omitempty"`
}

// Search results
type MovieResult struct {
    Id               int        `json:"id"`
    MediaType        string     `json:"mediaType"`
    Title            string     `json:"title"`
    OriginalTitle    *string    `json:"originalTitle,omitempty"`
    Popularity       *float64   `json:"popularity,omitempty"`
    PosterPath       *string    `json:"posterPath,omitempty"`
    BackdropPath     *string    `json:"backdropPath,omitempty"`
    VoteCount        *int       `json:"voteCount,omitempty"`
    VoteAverage      *float64   `json:"voteAverage,omitempty"`
    GenreIds         []int      `json:"genreIds,omitempty"`
    Overview         *string    `json:"overview,omitempty"`
    OriginalLanguage *string    `json:"originalLanguage,omitempty"`
    ReleaseDate      *string    `json:"releaseDate,omitempty"`
    Adult            *bool      `json:"adult,omitempty"`
    Video            *bool      `json:"video,omitempty"`
    MediaInfo        *MediaInfo `json:"mediaInfo,omitempty"`
}

type TvResult struct {
    Id               int        `json:"id"`
    MediaType        string     `json:"mediaType"`
    Name             string     `json:"name"`
    OriginalName     *string    `json:"originalName,omitempty"`
    Popularity       *float64   `json:"popularity,omitempty"`
    PosterPath       *string    `json:"posterPath,omitempty"`
    BackdropPath     *string    `json:"backdropPath,omitempty"`
    VoteCount        *int       `json:"voteCount,omitempty"`
    VoteAverage      *float64   `json:"voteAverage,omitempty"`
    GenreIds         []int      `json:"genreIds,omitempty"`
    Overview         *string    `json:"overview,omitempty"`
    OriginalLanguage *string    `json:"originalLanguage,omitempty"`
    OriginCountry    []string   `json:"originCountry,omitempty"`
    FirstAirDate     *string    `json:"firstAirDate,omitempty"`
    MediaInfo        *MediaInfo `json:"mediaInfo,omitempty"`
}

// User information
type User struct {
    Id             int     `json:"id"`
    Email          string  `json:"email"`
    Username       *string `json:"username,omitempty"`
    PlexUsername   *string `json:"plexUsername,omitempty"`
    Permissions    int     `json:"permissions"`
    Avatar         *string `json:"avatar,omitempty"`
    RequestCount   int     `json:"requestCount"`
}
```

### Example HTTP Requests

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    baseURL = "http://localhost:5055/api/v1"
    apiKey  = "your-api-key-here"
)

// HTTP client with timeout
var client = &http.Client{
    Timeout: 10 * time.Second,
}

// Helper to make authenticated requests
func makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
    var reqBody io.Reader
    if body != nil {
        jsonData, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        reqBody = bytes.NewBuffer(jsonData)
    }

    req, err := http.NewRequest(method, baseURL+endpoint, reqBody)
    if err != nil {
        return nil, err
    }

    req.Header.Set("X-Api-Key", apiKey)
    req.Header.Set("Content-Type", "application/json")

    return client.Do(req)
}

// Search for media
func searchMedia(query string) ([]interface{}, error) {
    resp, err := makeRequest("GET", fmt.Sprintf("/search?query=%s", query), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("search failed: %s", resp.Status)
    }

    var result struct {
        Results []interface{} `json:"results"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Results, nil
}

// Create a movie request
func requestMovie(tmdbId int) (*MediaRequest, error) {
    reqData := CreateRequest{
        MediaType: "movie",
        MediaId:   tmdbId,
    }

    resp, err := makeRequest("POST", "/request", reqData)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return nil, fmt.Errorf("request creation failed: %s", resp.Status)
    }

    var request MediaRequest
    if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
        return nil, err
    }

    return &request, nil
}

// Request TV show (all seasons)
func requestTvShow(tmdbId int, seasons interface{}) (*MediaRequest, error) {
    reqData := CreateRequest{
        MediaType: "tv",
        MediaId:   tmdbId,
        Seasons:   seasons, // Can be "all" or []int{1, 2, 3}
    }

    resp, err := makeRequest("POST", "/request", reqData)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return nil, fmt.Errorf("request creation failed: %s", resp.Status)
    }

    var request MediaRequest
    if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
        return nil, err
    }

    return &request, nil
}

// Get all pending requests
func getPendingRequests() ([]MediaRequest, error) {
    resp, err := makeRequest("GET", "/request?filter=pending", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to get requests: %s", resp.Status)
    }

    var result struct {
        Results []MediaRequest `json:"results"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Results, nil
}

// Approve a request
func approveRequest(requestId int) (*MediaRequest, error) {
    resp, err := makeRequest("POST", fmt.Sprintf("/request/%d/approve", requestId), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("approval failed: %s", resp.Status)
    }

    var request MediaRequest
    if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
        return nil, err
    }

    return &request, nil
}

// Decline a request
func declineRequest(requestId int) (*MediaRequest, error) {
    resp, err := makeRequest("POST", fmt.Sprintf("/request/%d/decline", requestId), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("decline failed: %s", resp.Status)
    }

    var request MediaRequest
    if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
        return nil, err
    }

    return &request, nil
}

// Delete a request
func deleteRequest(requestId int) error {
    resp, err := makeRequest("DELETE", fmt.Sprintf("/request/%d", requestId), nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
        return fmt.Errorf("deletion failed: %s", resp.Status)
    }

    return nil
}

// Get current user
func getCurrentUser() (*User, error) {
    resp, err := makeRequest("GET", "/auth/me", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to get user: %s", resp.Status)
    }

    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }

    return &user, nil
}
```

---

## Common Workflows

### 1. Search and Request a Movie
```go
// Search for the movie
results, _ := searchMedia("Fight Club")

// Find the movie result (check mediaType == "movie")
// Then request it using the ID
request, _ := requestMovie(550) // TMDB ID

fmt.Printf("Request created with ID: %d, Status: %d\n", request.Id, request.Status)
```

### 2. Request TV Show (All Seasons)
```go
request, _ := requestTvShow(1396, "all") // Breaking Bad, all seasons
```

### 3. Request Specific TV Seasons
```go
seasons := []int{1, 2, 3}
request, _ := requestTvShow(1396, seasons) // Breaking Bad, seasons 1-3
```

### 4. Approve Pending Requests
```go
pending, _ := getPendingRequests()
for _, req := range pending {
    fmt.Printf("Request ID %d: %s\n", req.Id, req.Media.TmdbId)
    // Approve it
    approveRequest(req.Id)
}
```

### 5. List All Requests with Filter
```go
// Get all requests
resp, _ := makeRequest("GET", "/request?take=50&skip=0&filter=all&sort=added", nil)

// Get only approved
resp, _ := makeRequest("GET", "/request?filter=approved", nil)

// Get user's requests
resp, _ := makeRequest("GET", "/request?requestedBy=1", nil)
```

---

## Important Notes

1. **TMDB IDs vs Internal IDs**:
   - Use TMDB IDs when creating requests (`mediaId` field)
   - The `id` field in responses is Overseerr's internal ID

2. **Permissions System**:
   - Regular users can only see/manage their own requests
   - `ADMIN` and `MANAGE_REQUESTS` permissions allow managing all requests
   - `AUTO_APPROVE` permission bypasses approval workflow

3. **4K Requests**:
   - 4K requests are tracked separately from standard requests
   - Set `is4k: true` in request body
   - Requires separate Radarr/Sonarr 4K server configuration

4. **TV Show Seasons**:
   - Can request all seasons with `"seasons": "all"`
   - Or specific seasons with `"seasons": [1, 2, 3]`
   - Seasons array uses season numbers, not IDs

5. **Rate Limiting**:
   - Implement appropriate delays between API calls
   - Consider caching search results

6. **Error Handling**:
   - Always check HTTP status codes
   - 403: Permission denied
   - 404: Resource not found
   - 409: Duplicate request
   - 429: Rate limited

7. **Jellyseerr Compatibility**:
   - Jellyseerr uses the same API structure
   - Default port: 5055
   - Just change the base URL to your Jellyseerr instance

---

## Reference Links

- **Official API Documentation**: https://api-docs.overseerr.dev/
- **GitHub Repository**: https://github.com/sct/overseerr
- **Go Client Library**: https://github.com/devopsarr/overseerr-go
- **Jellyseerr Fork**: https://github.com/Fallenbagel/jellyseerr
