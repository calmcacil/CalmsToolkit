package mediarequests

// Step represents the current step in the media request workflow
type Step string

const (
	StepSearch  Step = "search"
	StepSelect  Step = "select"
	StepConfirm Step = "confirm"
	StepSubmit  Step = "submit"
)

// Status codes from the original implementation
const (
	StatusPending  = 1
	StatusApproved = 2
	StatusDeclined = 3
)

const (
	MediaStatusUnknown            = 1
	MediaStatusPending            = 2
	MediaStatusProcessing         = 3
	MediaStatusPartiallyAvailable = 4
	MediaStatusAvailable          = 5
	MediaStatusDeleted            = 6
)

// API structures extracted from original implementation
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

type DownloadStatus struct {
	ExternalID int    `json:"externalId"`
	Status     string `json:"status"`
}

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

type SeasonRequest struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	Status       int    `json:"status"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username,omitempty"`
	PlexUsername string `json:"plexUsername,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Avatar       string `json:"avatar,omitempty"`
}

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

type RequestOverrides struct {
	ServerID   int
	ServerName string
	RootFolder string
}

type ServiceInstance struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Is4k      bool   `json:"is4k"`
	IsDefault bool   `json:"isDefault"`
}

type ServiceProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ServiceRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type ServiceDetails struct {
	Profiles    []ServiceProfile    `json:"profiles"`
	RootFolders []ServiceRootFolder `json:"rootFolders"`
}

type TVDetails struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	NumberOfSeasons  int      `json:"numberOfSeasons"`
	NumberOfEpisodes int      `json:"numberOfEpisodes"`
	Seasons          []Season `json:"seasons"`
}

type Season struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episodeCount"`
	AirDate      string `json:"airDate,omitempty"`
}

// SearchResponse represents the response from search API
type SearchResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"totalPages"`
	TotalResults int            `json:"totalResults"`
	Results      []SearchResult `json:"results"`
}

// RequestsResponse represents the response from requests API
type RequestsResponse struct {
	PageInfo PageInfo       `json:"pageInfo"`
	Results  []MediaRequest `json:"results"`
}

type PageInfo struct {
	Pages    int `json:"pages"`
	PageSize int `json:"pageSize"`
	Results  int `json:"results"`
	Page     int `json:"page"`
}

// Tea messages
type SearchResultsMsg struct {
	Results []SearchResult
	Error   error
}

type SubmitRequestMsg struct {
	Request *MediaRequest
	Error   error
}

type TVDetailsMsg struct {
	Details *TVDetails
	Error   error
}

type ServiceInstancesMsg struct {
	Instances []ServiceInstance
	Error     error
}

type ServiceDetailsMsg struct {
	Details *ServiceDetails
	Error   error
}

type PendingRequestsMsg struct {
	Requests []MediaRequest
	Error    error
}

type ApproveRequestMsg struct {
	Error error
}

type DeclineRequestMsg struct {
	Error error
}
