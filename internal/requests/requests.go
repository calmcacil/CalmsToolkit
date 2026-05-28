package requests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

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

// ToolConfig holds configuration for the media requests tool.
type ToolConfig struct {
	ServerURL  string
	APIKey     string
	Timeout    time.Duration
	NoColor    bool
	Theme      string
	Verbose    bool
	JSONOutput bool
	Quiet      bool
	ctx        context.Context
	client     *http.Client
}

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

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.ServerURL = "http://localhost:5055"
		return cfg
	}
	dur, err := time.ParseDuration(tk.General.Timeout)
	if err != nil || dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor
	cfg.Theme = tk.General.Theme
	cfg.ServerURL = strings.TrimSuffix(tk.MediaRequests.OverseerrURL, "/")
	cfg.APIKey = tk.MediaRequests.APIKey
	cfg.Verbose = tk.MediaRequests.Verbose
	return cfg
}

// Run executes the media requests interactive tool.
func Run(cfg ToolConfig) {
	cfg.ctx = context.Background()
	cfg.client = &http.Client{Timeout: cfg.Timeout}

	if cfg.APIKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: API key is not set\n")
		fmt.Fprintf(os.Stderr, "Set api_key in ~/.config/calmstoolkit/config.json or use -token flag\n")
		os.Exit(1)
	}

	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Server URL is not set\n")
		fmt.Fprintf(os.Stderr, "Set overseerr_url in ~/.config/calmstoolkit/config.json or use -url flag\n")
		os.Exit(1)
	}

	if err := testConnection(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to server: %v\n", err)
		os.Exit(1)
	}

	runInteractiveMenu(cfg)
}

func testConnection(cfg ToolConfig) error {
	ctx := cfg.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ServerURL+"/api/v1/auth/me", nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", cfg.APIKey)
	client := cfg.client
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func runInteractiveMenu(cfg ToolConfig) {
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printMainMenu(cfg)

		input, _ := readKeystroke(cfg)

		switch input {
		case "n":
			handleNewRequest(cfg, reader)
		case "w":
			handleViewRequests(cfg, reader)
		case "q":
			fmt.Println("\nGoodbye!")
			return
		default:
			fmt.Println("\nInvalid option. Press any key to continue...")
			readKeystroke(cfg)
		}
	}
}

func printMainMenu(cfg ToolConfig) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s╔══════════════════════════════════════════╗%s\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("%s%s║    Media Requests - Interactive Menu    ║%s\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("%s%s╚══════════════════════════════════════════╝%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	fmt.Printf("%s[N]%s New Request\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[W]%s View Requests\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[Q]%s Quit\n\n", clr(p.Error), clr(p.Reset))
	fmt.Printf("Select an option: ")
}

func handleNewRequest(cfg ToolConfig, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== New Media Request ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("Enter search query (or 'back' to return): ")

	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	if query == "" || strings.ToLower(query) == "back" {
		return
	}

	fmt.Fprintf(os.Stderr, "\n%sSearching...%s\n", clr(p.Warning), clr(p.Reset))
	results, err := searchMedia(cfg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError searching: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo results found.%s\n", clr(p.Warning), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Search Results ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayLimit := 10
	if len(results) > displayLimit {
		results = results[:displayLimit]
	}

	for i, result := range results {
		displaySearchResult(cfg, i+1, result)
	}

	fmt.Printf("\nSelect a number (1-%d) or 'back' to cancel: ", len(results))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(results) {
		fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	selectedMedia := results[selection-1]

	if selectedMedia.MediaInfo != nil {
		status := selectedMedia.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Fprintf(os.Stderr, "\n%sThis media is already available!%s\n", clr(p.Success), clr(p.Reset))
			fmt.Printf("\nPress any key to continue...")
			readKeystroke(cfg)
			return
		}
		if len(selectedMedia.MediaInfo.Requests) > 0 {
			fmt.Fprintf(os.Stderr, "\n%sThis media has already been requested.%s\n", clr(p.Warning), clr(p.Reset))
			fmt.Printf("\nPress any key to continue...")
			readKeystroke(cfg)
			return
		}
	}

	var seasons interface{}
	if selectedMedia.MediaType == "tv" {
		seasons, err = selectSeasons(cfg, selectedMedia, reader)
		if err != nil || seasons == nil {
			return
		}
	}

	overrides, err := selectRootFolderOverride(cfg, selectedMedia, reader)
	if err != nil {
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Confirm Request ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	title := selectedMedia.Title
	if title == "" {
		title = selectedMedia.Name
	}
	year := getYear(selectedMedia)

	fmt.Printf("%sMedia:%s %s", clr(p.Bold), clr(p.Reset), title)
	if year != "" {
		fmt.Printf(" %s(%s)%s", clr(p.Accent), year, clr(p.Reset))
	}
	fmt.Printf("\n")

	fmt.Printf("%sType:%s %s\n", clr(p.Bold), clr(p.Reset), titleCase(selectedMedia.MediaType))

	if selectedMedia.MediaType == "tv" && seasons != nil {
		if seasons == "all" {
			fmt.Printf("%sSeasons:%s All\n", clr(p.Bold), clr(p.Reset))
		} else if seasonList, ok := seasons.([]int); ok {
			fmt.Printf("%sSeasons:%s %v\n", clr(p.Bold), clr(p.Reset), seasonList)
		}
	}

	if overrides != nil {
		if overrides.ServerName != "" {
			fmt.Printf("%sServer:%s %s\n", clr(p.Bold), clr(p.Reset), overrides.ServerName)
		}
		if overrides.RootFolder != "" {
			fmt.Printf("%sRoot Folder:%s %s\n", clr(p.Bold), clr(p.Reset), overrides.RootFolder)
		}
	}

	fmt.Printf("\nSubmit request? (y/n): ")
	confirm := readKeyOrDefault(cfg, "n")

	if confirm != "y" {
		fmt.Fprintf(os.Stderr, "\n%sRequest cancelled.%s\n", clr(p.Warning), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%sSubmitting request...%s\n", clr(p.Warning), clr(p.Reset))
	request, err := createRequest(cfg, selectedMedia, seasons, overrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError creating request: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%s✓ Request submitted successfully!%s\n", clr(p.Success), clr(p.Reset))
	fmt.Fprintf(os.Stderr, "Request ID: %s%d%s\n", clr(p.Accent), request.ID, clr(p.Reset))

	statusText := getStatusText(request.Status)
	fmt.Fprintf(os.Stderr, "Status: %s%s%s\n", clr(p.Warning), statusText, clr(p.Reset))

	fmt.Printf("\nPress any key to continue...")
	readKeystroke(cfg)
}

func handleViewRequests(cfg ToolConfig, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Fprintf(os.Stderr, "%sLoading...%s\n", clr(p.Warning), clr(p.Reset))

	requests, err := getPendingRequests(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching requests: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	if len(requests) == 0 {
		fmt.Printf("%sNo pending requests.%s\n", clr(p.Success), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	for i, req := range requests {
		displayRequestSummary(cfg, i+1, req)
	}

	fmt.Printf("\nSelect a request (1-%d), or 'back' to return: ", len(requests))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(requests) {
		fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	selectedRequest := requests[selection-1]
	handleRequestDetail(cfg, selectedRequest, reader)
}

func handleRequestDetail(cfg ToolConfig, request MediaRequest, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== Request Details ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayRequestDetail(cfg, request)

	fmt.Printf("\n%sActions:%s\n", clr(p.Bold), clr(p.Reset))
	fmt.Printf("%s[A]%s Approve    %s[D]%s Decline    %s[B]%s Back\n\n",
		clr(p.Success), clr(p.Reset),
		clr(p.Error), clr(p.Reset),
		clr(p.Warning), clr(p.Reset))

	fmt.Printf("Select action: ")
	action := readKeyOrDefault(cfg, "b")

	switch action {
	case "a":
		overrides, err := selectRootFolderForApproval(cfg, request, reader)
		if err != nil {
			return
		}

		fmt.Fprintf(os.Stderr, "\n%sApproving request...%s\n", clr(p.Warning), clr(p.Reset))
		if err := approveRequestWithOverrides(cfg, request.ID, overrides); err != nil {
			fmt.Fprintf(os.Stderr, "\n%sError approving: %v%s\n", clr(p.Error), err, clr(p.Reset))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s✓ Request approved!%s\n", clr(p.Success), clr(p.Reset))
			if overrides != nil && overrides.RootFolder != "" {
				fmt.Fprintf(os.Stderr, "%s  Root folder set to: %s%s\n", clr(p.Subdued), overrides.RootFolder, clr(p.Reset))
			}
		}
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)

	case "d":
		fmt.Printf("\n%sAre you sure you want to decline this request? (y/n):%s ", clr(p.Error), clr(p.Reset))
		confirm := readKeyOrDefault(cfg, "n")

		if confirm == "y" {
			fmt.Fprintf(os.Stderr, "\n%sDeclining request...%s\n", clr(p.Warning), clr(p.Reset))
			if err := declineRequest(cfg, request.ID); err != nil {
				fmt.Fprintf(os.Stderr, "\n%sError declining: %v%s\n", clr(p.Error), err, clr(p.Reset))
			} else {
				fmt.Fprintf(os.Stderr, "\n%s✓ Request declined.%s\n", clr(p.Success), clr(p.Reset))
			}
		} else {
			fmt.Fprintf(os.Stderr, "\n%sCancelled.%s\n", clr(p.Warning), clr(p.Reset))
		}
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)

	case "b", "":
		return

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid action.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
	}
}

func displaySearchResult(cfg ToolConfig, index int, result SearchResult) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	title := result.Title
	if title == "" {
		title = result.Name
	}

	year := getYear(result)

	typeIcon := "🎬"
	if result.MediaType == "tv" {
		typeIcon = "📺"
	}

	fmt.Printf("%s%d.%s %s %s%s%s",
		clr(p.Warning), index, clr(p.Reset),
		typeIcon, clr(p.Bold), title, clr(p.Reset))

	if year != "" {
		fmt.Printf(" %s(%s)%s", clr(p.Accent), year, clr(p.Reset))
	}

	if result.MediaInfo != nil {
		status := result.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Printf(" %s[AVAILABLE]%s", clr(p.Success), clr(p.Reset))
		} else if len(result.MediaInfo.Requests) > 0 {
			fmt.Printf(" %s[REQUESTED]%s", clr(p.Warning), clr(p.Reset))
		}
	}

	fmt.Printf("\n")

	if result.Overview != "" && len(result.Overview) > 100 {
		fmt.Printf("   %s%s...%s\n", clr(p.Subdued), result.Overview[:97], clr(p.Reset))
	} else if result.Overview != "" {
		fmt.Printf("   %s%s%s\n", clr(p.Subdued), result.Overview, clr(p.Reset))
	}

	if result.VoteAverage > 0 {
		fmt.Printf("   %sRating: %.1f/10%s\n", clr(p.Subdued), result.VoteAverage, clr(p.Reset))
	}

	fmt.Println()
}

func displayRequestSummary(cfg ToolConfig, index int, request MediaRequest) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}

	fmt.Printf("%s%d.%s [%s] ", clr(p.Warning), index, clr(p.Reset), mediaType)

	fmt.Printf("Request ID: %s%d%s ", clr(p.Accent), request.ID, clr(p.Reset))
	fmt.Printf("(TMDB: %d)", request.Media.TmdbID)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}

	fmt.Printf("\n   %sRequested by:%s %s", clr(p.Subdued), clr(p.Reset), requestedBy)
	fmt.Printf("  %sCreated:%s %s\n", clr(p.Subdued), clr(p.Reset), formatDate(request.CreatedAt))

	fmt.Println()
}

func displayRequestDetail(cfg ToolConfig, request MediaRequest) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%sRequest ID:%s %d\n", clr(p.Bold), clr(p.Reset), request.ID)
	fmt.Printf("%sTMDB ID:%s %d\n", clr(p.Bold), clr(p.Reset), request.Media.TmdbID)

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}
	fmt.Printf("%sType:%s %s\n", clr(p.Bold), clr(p.Reset), mediaType)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}
	fmt.Printf("%sRequested by:%s %s\n", clr(p.Bold), clr(p.Reset), requestedBy)

	fmt.Printf("%sCreated:%s %s\n", clr(p.Bold), clr(p.Reset), formatDate(request.CreatedAt))
	fmt.Printf("%sStatus:%s %s%s%s\n", clr(p.Bold), clr(p.Reset),
		clr(p.Warning), getStatusText(request.Status), clr(p.Reset))

	if len(request.Seasons) > 0 {
		fmt.Printf("%sSeasons requested:%s %d\n", clr(p.Bold), clr(p.Reset), len(request.Seasons))
	}

	if request.Is4k {
		fmt.Printf("%s4K:%s Yes\n", clr(p.Bold), clr(p.Reset))
	}
}

func selectSeasons(cfg ToolConfig, media SearchResult, reader *bufio.Reader) (interface{}, error) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	details, err := getTVDetails(cfg, media.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching TV show details: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	clearScreen()
	fmt.Printf("%s%s=== Select Seasons ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	title := media.Title
	if title == "" {
		title = media.Name
	}
	fmt.Printf("%sTV Show:%s %s\n", clr(p.Bold), clr(p.Reset), title)
	fmt.Printf("%sTotal Seasons:%s %d\n\n", clr(p.Bold), clr(p.Reset), details.NumberOfSeasons)

	fmt.Printf("%s[A]%s Request all seasons\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[S]%s Select specific seasons\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[B]%s Back\n\n", clr(p.Error), clr(p.Reset))

	fmt.Printf("Select option: ")
	option := readKeyOrDefault(cfg, "b")

	switch option {
	case "a":
		return "all", nil

	case "s":
		fmt.Printf("\nEnter season numbers (comma-separated, e.g., 1,2,3): ")
		seasonsStr, _ := reader.ReadString('\n')
		seasonsStr = strings.TrimSpace(seasonsStr)

		if seasonsStr == "" {
			return nil, fmt.Errorf("no seasons specified")
		}

		parts := strings.Split(seasonsStr, ",")
		seasons := make([]int, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			season, err := strconv.Atoi(part)
			if err != nil || season < 1 || season > details.NumberOfSeasons {
				fmt.Fprintf(os.Stderr, "\n%sInvalid season number: %s%s\n", clr(p.Error), part, clr(p.Reset))
				fmt.Printf("\nPress any key to continue...")
				readKeystroke(cfg)
				return nil, fmt.Errorf("invalid season number")
			}
			seasons = append(seasons, season)
		}

		if len(seasons) == 0 {
			return nil, fmt.Errorf("no valid seasons specified")
		}

		return seasons, nil

	case "b", "":
		return nil, fmt.Errorf("cancelled")

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid option.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, fmt.Errorf("invalid option")
	}
}

func selectRootFolderOverride(cfg ToolConfig, media SearchResult, reader *bufio.Reader) (*RequestOverrides, error) {
	p := colors.GetPalette(cfg.Theme)
	mediaType := strings.ToLower(media.MediaType)

	var service string
	var serviceLabel string
	switch mediaType {
	case "movie":
		service = "radarr"
		serviceLabel = "Radarr"
	case "tv":
		service = "sonarr"
		serviceLabel = "Sonarr"
	default:
		return nil, nil
	}

	servers, err := fetchServiceInstances(cfg, service)
	if err != nil {
		clr := func(code string) string {
			if cfg.NoColor {
				return ""
			}
			return code
		}
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s servers: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("\n%sSelect %s destination%s\n", clr(p.Bold), serviceLabel, clr(p.Reset))

	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			fmt.Printf("\nAvailable %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", clr(p.Warning), i+1, clr(p.Reset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", clr(p.Subdued), strings.Join(badges, ", "), clr(p.Reset))
				}
				fmt.Println()
			}

			fmt.Printf("\nSelect a server (1-%d), press Enter to use defaults, or type 'back' to cancel: ", len(servers))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "":
				for i := range servers {
					if servers[i].IsDefault {
						selected = &servers[i]
						break
					}
				}
				if selected == nil {
					selected = &servers[0]
				}
				fmt.Fprintf(os.Stderr, "Using default %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
			case "back", "b":
				return nil, fmt.Errorf("cancelled")
			default:
				index, convErr := strconv.Atoi(input)
				if convErr != nil || index < 1 || index > len(servers) {
					fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
					continue
				}
				selected = &servers[index-1]
			}

			if selected != nil {
				break
			}
		}
	} else {
		selected = &servers[0]
		fmt.Fprintf(os.Stderr, "Using %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
	}

	details, err := fetchServiceDetails(cfg, service, selected.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s details: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	if len(details.RootFolders) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo root folders configured for %s.%s\n", clr(p.Warning), selected.Name, clr(p.Reset))
		fmt.Printf("Press Enter to continue...")
		reader.ReadString('\n')
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	for {
		fmt.Printf("\n%sRoot folders for %s:%s\n", clr(p.Bold), selected.Name, clr(p.Reset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", clr(p.Warning), i+1, clr(p.Reset), folder.Path)
		}

		fmt.Printf("\nSelect a root folder (1-%d), press Enter to use server default, or type 'back' to cancel: ", len(details.RootFolders))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "":
			return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
		case "back", "b":
			return nil, fmt.Errorf("cancelled")
		default:
			index, convErr := strconv.Atoi(input)
			if convErr != nil || index < 1 || index > len(details.RootFolders) {
				fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
				continue
			}
			folder := details.RootFolders[index-1]
			return &RequestOverrides{
				ServerID:   selected.ID,
				ServerName: selected.Name,
				RootFolder: folder.Path,
			}, nil
		}
	}
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

func readKeystroke(cfg ToolConfig) (string, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(strings.ToLower(input)), nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(strings.ToLower(input)), nil
	}

	defer term.Restore(fd, oldState)

	b := make([]byte, 1)
	_, err = os.Stdin.Read(b)
	if err != nil {
		return "", err
	}

	fmt.Printf("%c\n", b[0])

	char := strings.ToLower(string(b[0]))
	return char, nil
}

func readKeyOrDefault(cfg ToolConfig, defaultKey string) string {
	key, err := readKeystroke(cfg)
	if err != nil {
		return defaultKey
	}

	if key == "\n" || key == "\r" {
		return defaultKey
	}

	return key
}

func clearScreen() {
	fmt.Print(colors.HomeCursor)
}

func fetchServiceInstances(cfg ToolConfig, service string) ([]ServiceInstance, error) {
	var endpoint string
	switch service {
	case "radarr":
		endpoint = "/service/radarr"
	case "sonarr":
		endpoint = "/service/sonarr"
	default:
		return nil, nil
	}

	resp, err := makeRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s servers: status %d", service, resp.StatusCode)
	}

	var servers []ServiceInstance
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return nil, err
	}

	return servers, nil
}

func fetchServiceDetails(cfg ToolConfig, service string, id int) (*ServiceDetails, error) {
	if service != "radarr" && service != "sonarr" {
		return nil, fmt.Errorf("unsupported service type: %s", service)
	}

	endpoint := fmt.Sprintf("/service/%s/%d", service, id)
	resp, err := makeRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s details: status %d", service, resp.StatusCode)
	}

	var details ServiceDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return &details, nil
}

const maxBodySize = 10 * 1024 * 1024

func readBodyLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBodySize))
}

func makeRequest(cfg ToolConfig, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := cfg.ServerURL + "/api/v1" + endpoint

	ctx := cfg.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := cfg.client
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return client.Do(req)
}

func searchMedia(cfg ToolConfig, query string) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	endpoint := "/search?" + params.Encode()

	resp, err := makeRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("search failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Results, nil
}

func getTVDetails(cfg ToolConfig, tmdbID int) (*TVDetails, error) {
	endpoint := fmt.Sprintf("/tv/%d", tmdbID)
	resp, err := makeRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get TV details: status %d", resp.StatusCode)
	}

	var details TVDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return &details, nil
}

func createRequest(cfg ToolConfig, media SearchResult, seasons interface{}, overrides *RequestOverrides) (*MediaRequest, error) {
	reqData := CreateRequest{
		MediaType: media.MediaType,
		MediaID:   media.ID,
	}

	if media.MediaType == "tv" && seasons != nil {
		reqData.Seasons = seasons
	}

	if overrides != nil {
		if overrides.ServerID > 0 {
			reqData.ServerID = overrides.ServerID
		}
		if overrides.RootFolder != "" {
			reqData.RootFolder = overrides.RootFolder
		}
	}

	resp, err := makeRequest(cfg, "POST", "/request", reqData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("request creation failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var request MediaRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

func checkUserPermissions(cfg ToolConfig) (*AuthMe, error) {
	resp, err := makeRequest(cfg, "GET", "/auth/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var authMe AuthMe
	if err := json.NewDecoder(resp.Body).Decode(&authMe); err != nil {
		return nil, err
	}

	return &authMe, nil
}

func getRequestCount(cfg ToolConfig) (*RequestCount, error) {
	resp, err := makeRequest(cfg, "GET", "/request/count", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("failed to get request count: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var count RequestCount
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return nil, err
	}

	return &count, nil
}

func getPendingRequests(cfg ToolConfig) ([]MediaRequest, error) {
	p := colors.GetPalette(cfg.Theme)
	var expectedPendingCount int

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "\n=== Diagnostic: Checking pending requests ===\n")

		if authMe, err := checkUserPermissions(cfg); err == nil {
			fmt.Fprintf(os.Stderr, "User ID: %d\n", authMe.ID)
			fmt.Fprintf(os.Stderr, "User Email: %s\n", authMe.Email)
			fmt.Fprintf(os.Stderr, "User Permissions: %d\n", authMe.Permissions)

			const MANAGE_REQUESTS = 16
			const ADMIN = 2
			if (authMe.Permissions & MANAGE_REQUESTS) != 0 {
				fmt.Fprintf(os.Stderr, "✓ Has MANAGE_REQUESTS permission\n")
			} else if (authMe.Permissions & ADMIN) != 0 {
				fmt.Fprintf(os.Stderr, "✓ Has ADMIN permission\n")
			} else {
				fmt.Fprintf(os.Stderr, "⚠ WARNING: May lack MANAGE_REQUESTS (16) or ADMIN (2) permission\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "⚠ Failed to check permissions: %v\n", err)
		}
	}

	if count, err := getRequestCount(cfg); err == nil {
		expectedPendingCount = count.Pending
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Request counts - Pending: %d, Approved: %d, Total: %d\n",
				count.Pending, count.Approved, count.Total)
		}
	} else {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "⚠ Failed to get request count: %v\n", err)
		}
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "===========================================\n\n")
	}

	const pageSize = 50
	skip := 0
	var pending []MediaRequest

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Attempting primary fetch with filter=pending...\n")
	}

	for {
		endpoint := fmt.Sprintf("/request?filter=pending&take=%d&skip=%d", pageSize, skip)

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
		}

		resp, err := makeRequest(cfg, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := readBodyLimited(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get requests: status %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var reqResp RequestsResponse
		if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Page %d: Got %d results (total: %d)\n",
				reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
		}

		pending = append(pending, reqResp.Results...)

		skip += pageSize
		if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
			break
		}
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch complete: %d pending requests fetched\n", len(pending))
	}

	if expectedPendingCount > 0 && len(pending) == 0 {
		clr := func(code string) string {
			if cfg.NoColor {
				return ""
			}
			return code
		}

		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "\n%s⚠ WARNING: Overseerr API bug detected!%s\n", clr(p.Warning), clr(p.Reset))
			fmt.Fprintf(os.Stderr, "Expected %d pending request(s) but filter=pending returned 0 results.\n", expectedPendingCount)
			fmt.Fprintf(os.Stderr, "Activating fallback: fetching all requests and filtering client-side...\n\n")
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "=== Fallback Mode: Fetching filter=all ===\n")
		}

		skip = 0
		var allRequests []MediaRequest

		for {
			endpoint := fmt.Sprintf("/request?filter=all&take=%d&skip=%d", pageSize, skip)

			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
			}

			resp, err := makeRequest(cfg, "GET", endpoint, nil)
			if err != nil {
				return nil, fmt.Errorf("fallback fetch failed: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := readBodyLimited(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("fallback fetch failed: status %d - %s", resp.StatusCode, string(bodyBytes))
			}

			var reqResp RequestsResponse
			if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("fallback decode failed: %w", err)
			}
			resp.Body.Close()

			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Fallback page %d: Got %d results (total: %d)\n",
					reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
			}

			allRequests = append(allRequests, reqResp.Results...)

			skip += pageSize
			if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
				break
			}
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Fallback fetch complete: %d total requests retrieved\n", len(allRequests))
			fmt.Fprintf(os.Stderr, "Filtering for status=%d (PENDING)...\n", StatusPending)
		}

		for _, req := range allRequests {
			if req.Status == StatusPending {
				pending = append(pending, req)
			}
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Client-side filtering complete: %d pending requests found\n", len(pending))
			fmt.Fprintf(os.Stderr, "===========================================\n\n")
		}

		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "%s✓ Fallback successful: Found %d pending request(s)%s\n\n",
				clr(p.Success), len(pending), clr(p.Reset))
		}
	} else if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch successful, no fallback needed.\n\n")
	}

	return pending, nil
}

func approveRequest(cfg ToolConfig, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/approve", requestID)
	resp, err := makeRequest(cfg, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return fmt.Errorf("approval failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func approveRequestWithOverrides(cfg ToolConfig, requestID int, overrides *RequestOverrides) error {
	if overrides != nil && overrides.RootFolder != "" {
		updateData := map[string]interface{}{
			"rootFolder": overrides.RootFolder,
		}
		if overrides.ServerID > 0 {
			updateData["serverId"] = overrides.ServerID
		}

		endpoint := fmt.Sprintf("/request/%d", requestID)
		resp, err := makeRequest(cfg, "PUT", endpoint, updateData)
		if err != nil {
			return fmt.Errorf("failed to set root folder before approval: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := readBodyLimited(resp.Body)
			return fmt.Errorf("failed to set root folder before approval: status %d - %s", resp.StatusCode, string(bodyBytes))
		}
	}

	if err := approveRequest(cfg, requestID); err != nil {
		return err
	}

	return nil
}

func selectRootFolderForApproval(cfg ToolConfig, request MediaRequest, reader *bufio.Reader) (*RequestOverrides, error) {
	p := colors.GetPalette(cfg.Theme)
	clr := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	var service string
	var serviceLabel string
	switch request.Type {
	case "movie":
		service = "radarr"
		serviceLabel = "Radarr"
	case "tv":
		service = "sonarr"
		serviceLabel = "Sonarr"
	default:
		return nil, nil
	}

	servers, err := fetchServiceInstances(cfg, service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s servers: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		return nil, nil
	}

	if len(servers) == 0 {
		return nil, nil
	}

	clearScreen()
	fmt.Printf("%s%s=== Approve Request - Root Folder Override ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayRequestDetail(cfg, request)

	if request.RootFolder != "" {
		fmt.Printf("\n%sCurrent Root Folder:%s %s%s%s\n",
			clr(p.Bold), clr(p.Reset),
			clr(p.Accent), request.RootFolder, clr(p.Reset))
	} else {
		fmt.Printf("\n%sCurrent Root Folder:%s %sNot set (will use server default)%s\n",
			clr(p.Bold), clr(p.Reset),
			clr(p.Subdued), clr(p.Reset))
	}

	fmt.Printf("\n%sWould you like to override the root folder for this request?%s\n", clr(p.Bold), clr(p.Reset))
	fmt.Printf("%s[Y]%s Yes, select root folder\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[N]%s No, use default (proceed with approval)\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[B]%s Back (cancel approval)\n\n", clr(p.Error), clr(p.Reset))

	fmt.Printf("Select option: ")
	option := readKeyOrDefault(cfg, "n")

	switch option {
	case "n", "":
		return nil, nil

	case "b", "back":
		return nil, fmt.Errorf("cancelled")

	case "y", "yes":
		break

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid option.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, fmt.Errorf("invalid option")
	}

	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			clearScreen()
			fmt.Printf("%s%s=== Select %s Server ===%s\n\n", clr(p.Bold), clr(p.Accent), serviceLabel, clr(p.Reset))

			fmt.Printf("Available %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", clr(p.Warning), i+1, clr(p.Reset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", clr(p.Subdued), strings.Join(badges, ", "), clr(p.Reset))
				}
				fmt.Println()
			}

			fmt.Printf("\nSelect a server (1-%d) or type 'back' to cancel: ", len(servers))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "back", "b", "":
				return nil, fmt.Errorf("cancelled")
			default:
				index, convErr := strconv.Atoi(input)
				if convErr != nil || index < 1 || index > len(servers) {
					fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
					fmt.Printf("\nPress Enter to continue...")
					reader.ReadString('\n')
					continue
				}
				selected = &servers[index-1]
			}

			if selected != nil {
				break
			}
		}
	} else {
		selected = &servers[0]
		fmt.Fprintf(os.Stderr, "\nUsing %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
	}

	details, err := fetchServiceDetails(cfg, service, selected.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s details: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, nil
	}

	if len(details.RootFolders) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo root folders configured for %s.%s\n", clr(p.Warning), selected.Name, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	for {
		clearScreen()
		fmt.Printf("%s%s=== Select Root Folder ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

		fmt.Printf("%sServer:%s %s\n\n", clr(p.Bold), clr(p.Reset), selected.Name)
		fmt.Printf("%sRoot folders:%s\n", clr(p.Bold), clr(p.Reset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", clr(p.Warning), i+1, clr(p.Reset), folder.Path)
		}

		fmt.Printf("\nSelect a root folder (1-%d) or type 'back' to cancel: ", len(details.RootFolders))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "back", "b", "":
			return nil, fmt.Errorf("cancelled")
		default:
			index, convErr := strconv.Atoi(input)
			if convErr != nil || index < 1 || index > len(details.RootFolders) {
				fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
				fmt.Printf("\nPress Enter to continue...")
				reader.ReadString('\n')
				continue
			}
			folder := details.RootFolders[index-1]
			return &RequestOverrides{
				ServerID:   selected.ID,
				ServerName: selected.Name,
				RootFolder: folder.Path,
			}, nil
		}
	}
}

func declineRequest(cfg ToolConfig, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/decline", requestID)
	resp, err := makeRequest(cfg, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return fmt.Errorf("decline failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
