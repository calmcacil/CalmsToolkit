package requests

import (
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/core"
)

// ToolConfig holds configuration for the media requests tool.
type ToolConfig struct {
	core.CommonConfig
	ServerURL string
	APIKey    string
	Verbose   bool
}

const (
	// StatusPending indicates a media request is awaiting approval.
	StatusPending = 1
	// StatusApproved indicates a media request has been approved.
	StatusApproved = 2
	// StatusDeclined indicates a media request has been declined.
	StatusDeclined = 3
)

const (
	// MediaStatusUnknown indicates the media availability is unknown.
	MediaStatusUnknown = 1
	// MediaStatusPending indicates the media request is pending.
	MediaStatusPending = 2
	// MediaStatusProcessing indicates the media is being processed.
	MediaStatusProcessing = 3
	// MediaStatusPartiallyAvailable indicates partial media availability.
	MediaStatusPartiallyAvailable = 4
	// MediaStatusAvailable indicates the media is fully available.
	MediaStatusAvailable = 5
	// MediaStatusDeleted indicates the media record has been deleted.
	MediaStatusDeleted = 6
)

// SearchResponse is the Overseerr search API response.
type SearchResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"totalPages"`
	TotalResults int            `json:"totalResults"`
	Results      []SearchResult `json:"results"`
}

// SearchResult represents a single search result from Overseerr.
type SearchResult struct {
	ID            int        `json:"id"`
	MediaType     string     `json:"mediaType"`
	Title         string     `json:"title,omitempty"`
	Name          string     `json:"name,omitempty"`
	OriginalTitle string     `json:"originalTitle,omitempty"`
	OriginalName  string     `json:"originalName,omitempty"`
	Overview      string     `json:"overview,omitempty"`
	PosterPath    string     `json:"posterPath,omitempty"`
	ReleaseDate   string     `json:"releaseDate,omitempty"`
	FirstAirDate  string     `json:"firstAirDate,omitempty"`
	VoteAverage   float64    `json:"voteAverage,omitempty"`
	Popularity    float64    `json:"popularity,omitempty"`
	MediaInfo     *MediaInfo `json:"mediaInfo,omitempty"`
}

// MediaInfo contains availability and request info for a media item.
type MediaInfo struct {
	ID             int              `json:"id"`
	TmdbID         int              `json:"tmdbId"`
	TvdbID         int              `json:"tvdbId,omitempty"`
	Status         int              `json:"status"`
	Requests       []MediaRequest   `json:"requests,omitempty"`
	DownloadStatus []DownloadStatus `json:"downloadStatus,omitempty"`
	CreatedAt      string           `json:"createdAt,omitempty"`
	UpdatedAt      string           `json:"updatedAt,omitempty"`
}

// DownloadStatus describes the download state for a media item.
type DownloadStatus struct {
	ExternalID int    `json:"externalId"`
	Status     string `json:"status"`
}

// MediaRequest represents a user media request in Overseerr.
type MediaRequest struct {
	ID          int             `json:"id"`
	Status      int             `json:"status"`
	Media       MediaInfo       `json:"media"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
	RequestedBy User            `json:"requestedBy"`
	ModifiedBy  *User           `json:"modifiedBy,omitempty"`
	Is4k        bool            `json:"is4k"`
	ServerID    int             `json:"serverId,omitempty"`
	ProfileID   int             `json:"profileId,omitempty"`
	RootFolder  string          `json:"rootFolder,omitempty"`
	Seasons     []SeasonRequest `json:"seasons,omitempty"`
	Type        string          `json:"type,omitempty"`
}

// SeasonRequest represents a season-level request.
type SeasonRequest struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	Status       int    `json:"status"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// User represents an Overseerr user.
type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username,omitempty"`
	PlexUsername string `json:"plexUsername,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Avatar       string `json:"avatar,omitempty"`
}

// AuthMe is the response from the /auth/me endpoint.
type AuthMe struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	Permissions int    `json:"permissions"`
}

// RequestCount summarizes request counts.
type RequestCount struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Total    int `json:"total"`
}

// CreateRequest is the payload for creating a new media request.
type CreateRequest struct {
	MediaType  string      `json:"mediaType"`
	MediaID    int         `json:"mediaId"`
	TvdbID     int         `json:"tvdbId,omitempty"`
	Seasons    interface{} `json:"seasons,omitempty"`
	Is4k       bool        `json:"is4k,omitempty"`
	ServerID   int         `json:"serverId,omitempty"`
	ProfileID  int         `json:"profileId,omitempty"`
	RootFolder string      `json:"rootFolder,omitempty"`
}

// RequestOverrides specifies server/root folder overrides for a request.
type RequestOverrides struct {
	ServerID   int
	ServerName string
	RootFolder string
}

// ServiceInstance represents a configured Sonarr/Radarr server instance.
type ServiceInstance struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Is4k      bool   `json:"is4k"`
	IsDefault bool   `json:"isDefault"`
}

// ServiceProfile is a quality profile on a Sonarr/Radarr instance.
type ServiceProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ServiceRootFolder is a root folder path on a Sonarr/Radarr instance.
type ServiceRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// ServiceDetails contains profiles and root folders for a service instance.
type ServiceDetails struct {
	Profiles    []ServiceProfile    `json:"profiles"`
	RootFolders []ServiceRootFolder `json:"rootFolders"`
}

// RequestsResponse is the paginated response from /request endpoints.
type RequestsResponse struct {
	PageInfo PageInfo       `json:"pageInfo"`
	Results  []MediaRequest `json:"results"`
}

// PageInfo contains pagination metadata.
type PageInfo struct {
	Pages    int `json:"pages"`
	PageSize int `json:"pageSize"`
	Results  int `json:"results"`
	Page     int `json:"page"`
}

// TVDetails contains season/episode info for a TV show.
type TVDetails struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	NumberOfSeasons  int      `json:"numberOfSeasons"`
	NumberOfEpisodes int      `json:"numberOfEpisodes"`
	Seasons          []Season `json:"seasons"`
}

// Season represents a single TV season.
type Season struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episodeCount"`
	AirDate      string `json:"airDate,omitempty"`
}

func getYear(result SearchResult) string {
	if result.ReleaseDate != "" && len(result.ReleaseDate) >= 4 {
		return result.ReleaseDate[:4]
	}
	if result.FirstAirDate != "" && len(result.FirstAirDate) >= 4 {
		return result.FirstAirDate[:4]
	}
	return ""
}

func getStatusText(status int) string {
	switch status {
	case StatusPending:
		return "Pending Approval"
	case StatusApproved:
		return "Approved"
	case StatusDeclined:
		return "Declined"
	default:
		return "Unknown"
	}
}

func formatDate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("2006-01-02 15:04")
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
