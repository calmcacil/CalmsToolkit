//go:build mediarequests
// +build mediarequests

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[0;31m"
	ColorGreen   = "\033[0;32m"
	ColorYellow  = "\033[0;33m"
	ColorBlue    = "\033[0;34m"
	ColorMagenta = "\033[0;35m"
	ColorCyan    = "\033[0;36m"
	ColorGray    = "\033[0;90m"
	ColorBold    = "\033[1m"
)

// ANSI control sequences
const (
	AnsiClearScreen = "\033[2J"   // Clear entire screen
	AnsiHomeCursor  = "\033[H"    // Move cursor to home position (0,0)
	AnsiHideCursor  = "\033[?25l" // Hide cursor
	AnsiShowCursor  = "\033[?25h" // Show cursor
)

// Status codes
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

// API structures
type Config struct {
	ServerURL string
	APIKey    string
	Timeout   time.Duration
	NoColor   bool
}

type SearchResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"totalPages"`
	TotalResults int            `json:"totalResults"`
	Results      []SearchResult `json:"results"`
}

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

type AuthMe struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	Permissions int    `json:"permissions"`
}

type RequestCount struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Total    int `json:"total"`
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

// Global flag for verbose diagnostics
var verbose bool

func main() {
	var (
		serverURL  = flag.String("url", "", "Overseerr/Jellyseerr server URL")
		apiKey     = flag.String("token", "", "API key/token")
		timeout    = flag.Duration("timeout", 30*time.Second, "Connection timeout")
		noColor    = flag.Bool("no-color", false, "Disable colored output")
		verbosePtr = flag.Bool("verbose", false, "Enable verbose diagnostic output")
	)
	flag.Parse()

	// Set global verbose flag
	verbose = *verbosePtr

	config := loadConfig(*serverURL, *apiKey, *timeout, *noColor)

	// Validate configuration
	if config.APIKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: API key is not set\n")
		fmt.Fprintf(os.Stderr, "Set OVERSEERR_TOKEN or JELLYSEERR_TOKEN environment variable, or use -token flag\n")
		os.Exit(1)
	}

	if config.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Server URL is not set\n")
		fmt.Fprintf(os.Stderr, "Set OVERSEERR_URL or JELLYSEERR_URL environment variable, or use -url flag\n")
		os.Exit(1)
	}

	// Test connection
	if err := testConnection(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to server: %v\n", err)
		os.Exit(1)
	}

	// Run interactive menu
	runInteractiveMenu(config)
}

func loadConfig(serverURL, apiKey string, timeout time.Duration, noColor bool) Config {
	config := Config{
		ServerURL: "http://localhost:5055",
		APIKey:    "",
		Timeout:   timeout,
		NoColor:   noColor,
	}

	// Load from .env file
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, &config)
	}

	// Environment variables override .env file
	if envURL := os.Getenv("OVERSEERR_URL"); envURL != "" {
		config.ServerURL = envURL
	} else if envURL := os.Getenv("JELLYSEERR_URL"); envURL != "" {
		config.ServerURL = envURL
	}

	if envToken := os.Getenv("OVERSEERR_TOKEN"); envToken != "" {
		config.APIKey = envToken
	} else if envToken := os.Getenv("JELLYSEERR_TOKEN"); envToken != "" {
		config.APIKey = envToken
	}

	// Command line flags override everything
	if serverURL != "" {
		config.ServerURL = serverURL
	}
	if apiKey != "" {
		config.APIKey = apiKey
	}

	config.ServerURL = strings.TrimSuffix(config.ServerURL, "/")

	return config
}

func loadEnvFile(path string, config *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "OVERSEERR_URL", "JELLYSEERR_URL":
			config.ServerURL = value
		case "OVERSEERR_TOKEN", "JELLYSEERR_TOKEN":
			config.APIKey = value
		}
	}
}

func testConnection(config Config) error {
	client := &http.Client{Timeout: config.Timeout}
	req, err := http.NewRequest("GET", config.ServerURL+"/api/v1/auth/me", nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", config.APIKey)
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

func runInteractiveMenu(config Config) {
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printMainMenu(config)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "n":
			handleNewRequest(config, reader)
		case "w":
			handleViewRequests(config, reader)
		case "q":
			fmt.Println("\nGoodbye!")
			return
		default:
			fmt.Println("\nInvalid option. Press Enter to continue...")
			reader.ReadString('\n')
		}
	}
}

func printMainMenu(config Config) {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s╔══════════════════════════════════════════╗%s\n", color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("%s%s║    Media Requests - Interactive Menu    ║%s\n", color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("%s%s╚══════════════════════════════════════════╝%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	fmt.Printf("%s[N]%s New Request\n", color(ColorGreen), color(ColorReset))
	fmt.Printf("%s[W]%s View Requests\n", color(ColorYellow), color(ColorReset))
	fmt.Printf("%s[Q]%s Quit\n\n", color(ColorRed), color(ColorReset))
	fmt.Printf("Select an option: ")
}

func handleNewRequest(config Config, reader *bufio.Reader) {
	clearScreen()
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== New Media Request ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("Enter search query (or 'back' to return): ")

	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	if query == "" || strings.ToLower(query) == "back" {
		return
	}

	// Search for media
	fmt.Printf("\n%sSearching...%s\n", color(ColorYellow), color(ColorReset))
	results, err := searchMedia(config, query)
	if err != nil {
		fmt.Printf("\n%sError searching: %v%s\n", color(ColorRed), err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(results) == 0 {
		fmt.Printf("\n%sNo results found.%s\n", color(ColorYellow), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Display results
	clearScreen()
	fmt.Printf("%s%s=== Search Results ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	displayLimit := 10
	if len(results) > displayLimit {
		results = results[:displayLimit]
	}

	for i, result := range results {
		displaySearchResult(config, i+1, result)
	}

	fmt.Printf("\nSelect a number (1-%d) or 'back' to cancel: ", len(results))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(results) {
		fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	selectedMedia := results[selection-1]

	// Check if already available or requested
	if selectedMedia.MediaInfo != nil {
		status := selectedMedia.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Printf("\n%sThis media is already available!%s\n", color(ColorGreen), color(ColorReset))
			fmt.Printf("\nPress Enter to continue...")
			reader.ReadString('\n')
			return
		}
		if len(selectedMedia.MediaInfo.Requests) > 0 {
			fmt.Printf("\n%sThis media has already been requested.%s\n", color(ColorYellow), color(ColorReset))
			fmt.Printf("\nPress Enter to continue...")
			reader.ReadString('\n')
			return
		}
	}

	// Handle TV show season selection
	var seasons interface{}
	if selectedMedia.MediaType == "tv" {
		seasons, err = selectSeasons(config, selectedMedia, reader)
		if err != nil || seasons == nil {
			return
		}
	}

	overrides, err := selectRootFolderOverride(config, selectedMedia, reader)
	if err != nil {
		return
	}

	// Confirm and submit request
	clearScreen()
	fmt.Printf("%s%s=== Confirm Request ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	title := selectedMedia.Title
	if title == "" {
		title = selectedMedia.Name
	}
	year := getYear(selectedMedia)

	fmt.Printf("%sMedia:%s %s", color(ColorBold), color(ColorReset), title)
	if year != "" {
		fmt.Printf(" %s(%s)%s", color(ColorCyan), year, color(ColorReset))
	}
	fmt.Printf("\n")

	fmt.Printf("%sType:%s %s\n", color(ColorBold), color(ColorReset), strings.Title(selectedMedia.MediaType))

	if selectedMedia.MediaType == "tv" && seasons != nil {
		if seasons == "all" {
			fmt.Printf("%sSeasons:%s All\n", color(ColorBold), color(ColorReset))
		} else if seasonList, ok := seasons.([]int); ok {
			fmt.Printf("%sSeasons:%s %v\n", color(ColorBold), color(ColorReset), seasonList)
		}
	}

	if overrides != nil {
		if overrides.ServerName != "" {
			fmt.Printf("%sServer:%s %s\n", color(ColorBold), color(ColorReset), overrides.ServerName)
		}
		if overrides.RootFolder != "" {
			fmt.Printf("%sRoot Folder:%s %s\n", color(ColorBold), color(ColorReset), overrides.RootFolder)
		}
	}

	fmt.Printf("\nSubmit request? (y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Printf("\n%sRequest cancelled.%s\n", color(ColorYellow), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Submit request
	fmt.Printf("\n%sSubmitting request...%s\n", color(ColorYellow), color(ColorReset))
	request, err := createRequest(config, selectedMedia, seasons, overrides)
	if err != nil {
		fmt.Printf("\n%sError creating request: %v%s\n", color(ColorRed), err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Printf("\n%s✓ Request submitted successfully!%s\n", color(ColorGreen), color(ColorReset))
	fmt.Printf("Request ID: %s%d%s\n", color(ColorCyan), request.ID, color(ColorReset))

	statusText := getStatusText(request.Status)
	fmt.Printf("Status: %s%s%s\n", color(ColorYellow), statusText, color(ColorReset))

	fmt.Printf("\nPress Enter to continue...")
	reader.ReadString('\n')
}

func handleViewRequests(config Config, reader *bufio.Reader) {
	clearScreen()
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("%sLoading...%s\n", color(ColorYellow), color(ColorReset))

	requests, err := getPendingRequests(config)
	if err != nil {
		fmt.Printf("\n%sError fetching requests: %v%s\n", color(ColorRed), err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	if len(requests) == 0 {
		fmt.Printf("%sNo pending requests.%s\n", color(ColorGreen), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	for i, req := range requests {
		displayRequestSummary(config, i+1, req)
	}

	fmt.Printf("\nSelect a request (1-%d), or 'back' to return: ", len(requests))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(requests) {
		fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return
	}

	selectedRequest := requests[selection-1]
	handleRequestDetail(config, selectedRequest, reader)
}

func handleRequestDetail(config Config, request MediaRequest, reader *bufio.Reader) {
	clearScreen()
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== Request Details ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	displayRequestDetail(config, request)

	fmt.Printf("\n%sActions:%s\n", color(ColorBold), color(ColorReset))
	fmt.Printf("%s[A]%s Approve    %s[D]%s Decline    %s[B]%s Back\n\n",
		color(ColorGreen), color(ColorReset),
		color(ColorRed), color(ColorReset),
		color(ColorYellow), color(ColorReset))

	fmt.Printf("Select action: ")
	action, _ := reader.ReadString('\n')
	action = strings.TrimSpace(strings.ToLower(action))

	switch action {
	case "a":
		// Prompt for root folder override before approving
		overrides, err := selectRootFolderForApproval(config, request, reader)
		if err != nil {
			// User cancelled
			return
		}

		fmt.Printf("\n%sApproving request...%s\n", color(ColorYellow), color(ColorReset))
		if err := approveRequestWithOverrides(config, request.ID, overrides); err != nil {
			fmt.Printf("\n%sError approving: %v%s\n", color(ColorRed), err, color(ColorReset))
		} else {
			fmt.Printf("\n%s✓ Request approved!%s\n", color(ColorGreen), color(ColorReset))
			if overrides != nil && overrides.RootFolder != "" {
				fmt.Printf("%s  Root folder set to: %s%s\n", color(ColorGray), overrides.RootFolder, color(ColorReset))
			}
		}
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')

	case "d":
		fmt.Printf("\n%sAre you sure you want to decline this request? (y/n):%s ", color(ColorRed), color(ColorReset))
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))

		if confirm == "y" || confirm == "yes" {
			fmt.Printf("\n%sDeclining request...%s\n", color(ColorYellow), color(ColorReset))
			if err := declineRequest(config, request.ID); err != nil {
				fmt.Printf("\n%sError declining: %v%s\n", color(ColorRed), err, color(ColorReset))
			} else {
				fmt.Printf("\n%s✓ Request declined.%s\n", color(ColorGreen), color(ColorReset))
			}
		} else {
			fmt.Printf("\n%sCancelled.%s\n", color(ColorYellow), color(ColorReset))
		}
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')

	case "b", "":
		return

	default:
		fmt.Printf("\n%sInvalid action.%s\n", color(ColorRed), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
	}
}

func displaySearchResult(config Config, index int, result SearchResult) {
	color := func(code string) string {
		if config.NoColor {
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
		color(ColorYellow), index, color(ColorReset),
		typeIcon, color(ColorBold), title, color(ColorReset))

	if year != "" {
		fmt.Printf(" %s(%s)%s", color(ColorCyan), year, color(ColorReset))
	}

	// Show status if available
	if result.MediaInfo != nil {
		status := result.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Printf(" %s[AVAILABLE]%s", color(ColorGreen), color(ColorReset))
		} else if len(result.MediaInfo.Requests) > 0 {
			fmt.Printf(" %s[REQUESTED]%s", color(ColorYellow), color(ColorReset))
		}
	}

	fmt.Printf("\n")

	if result.Overview != "" && len(result.Overview) > 100 {
		fmt.Printf("   %s%s...%s\n", color(ColorGray), result.Overview[:97], color(ColorReset))
	} else if result.Overview != "" {
		fmt.Printf("   %s%s%s\n", color(ColorGray), result.Overview, color(ColorReset))
	}

	if result.VoteAverage > 0 {
		fmt.Printf("   %sRating: %.1f/10%s\n", color(ColorGray), result.VoteAverage, color(ColorReset))
	}

	fmt.Println()
}

func displayRequestSummary(config Config, index int, request MediaRequest) {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}

	fmt.Printf("%s%d.%s [%s] ", color(ColorYellow), index, color(ColorReset), mediaType)

	// Get title from media info - would need to fetch details
	fmt.Printf("Request ID: %s%d%s ", color(ColorCyan), request.ID, color(ColorReset))
	fmt.Printf("(TMDB: %d)", request.Media.TmdbID)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}

	fmt.Printf("\n   %sRequested by:%s %s", color(ColorGray), color(ColorReset), requestedBy)
	fmt.Printf("  %sCreated:%s %s\n", color(ColorGray), color(ColorReset), formatDate(request.CreatedAt))

	fmt.Println()
}

func displayRequestDetail(config Config, request MediaRequest) {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("%sRequest ID:%s %d\n", color(ColorBold), color(ColorReset), request.ID)
	fmt.Printf("%sTMDB ID:%s %d\n", color(ColorBold), color(ColorReset), request.Media.TmdbID)

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}
	fmt.Printf("%sType:%s %s\n", color(ColorBold), color(ColorReset), mediaType)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}
	fmt.Printf("%sRequested by:%s %s\n", color(ColorBold), color(ColorReset), requestedBy)

	fmt.Printf("%sCreated:%s %s\n", color(ColorBold), color(ColorReset), formatDate(request.CreatedAt))
	fmt.Printf("%sStatus:%s %s%s%s\n", color(ColorBold), color(ColorReset),
		color(ColorYellow), getStatusText(request.Status), color(ColorReset))

	if len(request.Seasons) > 0 {
		fmt.Printf("%sSeasons requested:%s %d\n", color(ColorBold), color(ColorReset), len(request.Seasons))
	}

	if request.Is4k {
		fmt.Printf("%s4K:%s Yes\n", color(ColorBold), color(ColorReset))
	}
}

func selectSeasons(config Config, media SearchResult, reader *bufio.Reader) (interface{}, error) {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	// Fetch TV show details to get season count
	details, err := getTVDetails(config, media.ID)
	if err != nil {
		fmt.Printf("\n%sError fetching TV show details: %v%s\n", color(ColorRed), err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, err
	}

	clearScreen()
	fmt.Printf("%s%s=== Select Seasons ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	title := media.Title
	if title == "" {
		title = media.Name
	}
	fmt.Printf("%sTV Show:%s %s\n", color(ColorBold), color(ColorReset), title)
	fmt.Printf("%sTotal Seasons:%s %d\n\n", color(ColorBold), color(ColorReset), details.NumberOfSeasons)

	fmt.Printf("%s[A]%s Request all seasons\n", color(ColorGreen), color(ColorReset))
	fmt.Printf("%s[S]%s Select specific seasons\n", color(ColorYellow), color(ColorReset))
	fmt.Printf("%s[B]%s Back\n\n", color(ColorRed), color(ColorReset))

	fmt.Printf("Select option: ")
	option, _ := reader.ReadString('\n')
	option = strings.TrimSpace(strings.ToLower(option))

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
				fmt.Printf("\n%sInvalid season number: %s%s\n", color(ColorRed), part, color(ColorReset))
				fmt.Printf("\nPress Enter to continue...")
				reader.ReadString('\n')
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
		fmt.Printf("\n%sInvalid option.%s\n", color(ColorRed), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, fmt.Errorf("invalid option")
	}
}

func selectRootFolderOverride(config Config, media SearchResult, reader *bufio.Reader) (*RequestOverrides, error) {
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

	servers, err := fetchServiceInstances(config, service)
	if err != nil {
		color := func(code string) string {
			if config.NoColor {
				return ""
			}
			return code
		}
		fmt.Printf("\n%sError fetching %s servers: %v%s\n", color(ColorRed), serviceLabel, err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	fmt.Printf("\n%sSelect %s destination%s\n", color(ColorBold), serviceLabel, color(ColorReset))

	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			fmt.Printf("\nAvailable %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", color(ColorYellow), i+1, color(ColorReset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", color(ColorGray), strings.Join(badges, ", "), color(ColorReset))
				}
				fmt.Println()
			}

			fmt.Printf("\nSelect a server (1-%d), press Enter to use defaults, or type 'back' to cancel: ", len(servers))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "":
				// Select default server or fall back to first server
				for i := range servers {
					if servers[i].IsDefault {
						selected = &servers[i]
						break
					}
				}
				if selected == nil {
					selected = &servers[0]
				}
				fmt.Printf("Using default %s server: %s%s%s\n", serviceLabel, color(ColorBold), selected.Name, color(ColorReset))
			case "back", "b":
				return nil, fmt.Errorf("cancelled")
			default:
				index, convErr := strconv.Atoi(input)
				if convErr != nil || index < 1 || index > len(servers) {
					fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
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
		fmt.Printf("Using %s server: %s%s%s\n", serviceLabel, color(ColorBold), selected.Name, color(ColorReset))
	}

	details, err := fetchServiceDetails(config, service, selected.ID)
	if err != nil {
		fmt.Printf("\n%sError fetching %s details: %v%s\n", color(ColorRed), serviceLabel, err, color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, err
	}

	if len(details.RootFolders) == 0 {
		fmt.Printf("\n%sNo root folders configured for %s.%s\n", color(ColorYellow), selected.Name, color(ColorReset))
		fmt.Printf("Press Enter to continue...")
		reader.ReadString('\n')
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	for {
		fmt.Printf("\n%sRoot folders for %s:%s\n", color(ColorBold), selected.Name, color(ColorReset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", color(ColorYellow), i+1, color(ColorReset), folder.Path)
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
				fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
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

func clearScreen() {
	fmt.Print(AnsiHomeCursor)
	fmt.Print(AnsiClearScreen)
}

// API functions

func fetchServiceInstances(config Config, service string) ([]ServiceInstance, error) {
	var endpoint string
	switch service {
	case "radarr":
		endpoint = "/service/radarr"
	case "sonarr":
		endpoint = "/service/sonarr"
	default:
		return nil, nil
	}

	resp, err := makeRequest(config, "GET", endpoint, nil)
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

func fetchServiceDetails(config Config, service string, id int) (*ServiceDetails, error) {
	if service != "radarr" && service != "sonarr" {
		return nil, fmt.Errorf("unsupported service type: %s", service)
	}

	endpoint := fmt.Sprintf("/service/%s/%d", service, id)
	resp, err := makeRequest(config, "GET", endpoint, nil)
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

func makeRequest(config Config, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := config.ServerURL + "/api/v1" + endpoint

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: config.Timeout}
	return client.Do(req)
}

func searchMedia(config Config, query string) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	endpoint := "/search?" + params.Encode()

	resp, err := makeRequest(config, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Results, nil
}

func getTVDetails(config Config, tmdbID int) (*TVDetails, error) {
	endpoint := fmt.Sprintf("/tv/%d", tmdbID)
	resp, err := makeRequest(config, "GET", endpoint, nil)
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

func createRequest(config Config, media SearchResult, seasons interface{}, overrides *RequestOverrides) (*MediaRequest, error) {
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

	resp, err := makeRequest(config, "POST", "/request", reqData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request creation failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var request MediaRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

func checkUserPermissions(config Config) (*AuthMe, error) {
	resp, err := makeRequest(config, "GET", "/auth/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var authMe AuthMe
	if err := json.NewDecoder(resp.Body).Decode(&authMe); err != nil {
		return nil, err
	}

	return &authMe, nil
}

func getRequestCount(config Config) (*RequestCount, error) {
	resp, err := makeRequest(config, "GET", "/request/count", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get request count: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var count RequestCount
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return nil, err
	}

	return &count, nil
}

func getPendingRequests(config Config) ([]MediaRequest, error) {
	var expectedPendingCount int

	if verbose {
		fmt.Fprintf(os.Stderr, "\n=== Diagnostic: Checking pending requests ===\n")

		if authMe, err := checkUserPermissions(config); err == nil {
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

	if count, err := getRequestCount(config); err == nil {
		expectedPendingCount = count.Pending
		if verbose {
			fmt.Fprintf(os.Stderr, "Request counts - Pending: %d, Approved: %d, Total: %d\n",
				count.Pending, count.Approved, count.Total)
		}
	} else {
		if verbose {
			fmt.Fprintf(os.Stderr, "⚠ Failed to get request count: %v\n", err)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "===========================================\n\n")
	}

	const pageSize = 50
	skip := 0
	var pending []MediaRequest

	if verbose {
		fmt.Fprintf(os.Stderr, "Attempting primary fetch with filter=pending...\n")
	}

	for {
		endpoint := fmt.Sprintf("/request?filter=pending&take=%d&skip=%d", pageSize, skip)

		if verbose {
			fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
		}

		resp, err := makeRequest(config, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get requests: status %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var reqResp RequestsResponse
		if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if verbose {
			fmt.Fprintf(os.Stderr, "Page %d: Got %d results (total: %d)\n",
				reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
		}

		pending = append(pending, reqResp.Results...)

		skip += pageSize
		if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
			break
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch complete: %d pending requests fetched\n", len(pending))
	}

	if expectedPendingCount > 0 && len(pending) == 0 {
		color := func(code string) string {
			if config.NoColor {
				return ""
			}
			return code
		}

		fmt.Fprintf(os.Stderr, "\n%s⚠ WARNING: Overseerr API bug detected!%s\n", color(ColorYellow), color(ColorReset))
		fmt.Fprintf(os.Stderr, "Expected %d pending request(s) but filter=pending returned 0 results.\n", expectedPendingCount)
		fmt.Fprintf(os.Stderr, "Activating fallback: fetching all requests and filtering client-side...\n\n")

		if verbose {
			fmt.Fprintf(os.Stderr, "=== Fallback Mode: Fetching filter=all ===\n")
		}

		skip = 0
		var allRequests []MediaRequest

		for {
			endpoint := fmt.Sprintf("/request?filter=all&take=%d&skip=%d", pageSize, skip)

			if verbose {
				fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
			}

			resp, err := makeRequest(config, "GET", endpoint, nil)
			if err != nil {
				return nil, fmt.Errorf("fallback fetch failed: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("fallback fetch failed: status %d - %s", resp.StatusCode, string(bodyBytes))
			}

			var reqResp RequestsResponse
			if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("fallback decode failed: %w", err)
			}
			resp.Body.Close()

			if verbose {
				fmt.Fprintf(os.Stderr, "Fallback page %d: Got %d results (total: %d)\n",
					reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
			}

			allRequests = append(allRequests, reqResp.Results...)

			skip += pageSize
			if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
				break
			}
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Fallback fetch complete: %d total requests retrieved\n", len(allRequests))
			fmt.Fprintf(os.Stderr, "Filtering for status=%d (PENDING)...\n", StatusPending)
		}

		for _, req := range allRequests {
			if req.Status == StatusPending {
				pending = append(pending, req)
			}
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Client-side filtering complete: %d pending requests found\n", len(pending))
			fmt.Fprintf(os.Stderr, "===========================================\n\n")
		}

		fmt.Fprintf(os.Stderr, "%s✓ Fallback successful: Found %d pending request(s)%s\n\n",
			color(ColorGreen), len(pending), color(ColorReset))
	} else if verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch successful, no fallback needed.\n\n")
	}

	return pending, nil
}

func approveRequest(config Config, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/approve", requestID)
	resp, err := makeRequest(config, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approval failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// approveRequestWithOverrides approves a request and optionally updates it with rootFolder override.
// It first approves the request via POST, then updates it via PUT if overrides are provided.
func approveRequestWithOverrides(config Config, requestID int, overrides *RequestOverrides) error {
	// First, approve the request
	if err := approveRequest(config, requestID); err != nil {
		return err
	}

	// If no overrides or no rootFolder specified, we're done
	if overrides == nil || overrides.RootFolder == "" {
		return nil
	}

	// Update the request with the rootFolder override
	updateData := map[string]interface{}{
		"rootFolder": overrides.RootFolder,
	}

	endpoint := fmt.Sprintf("/request/%d", requestID)
	resp, err := makeRequest(config, "PUT", endpoint, updateData)
	if err != nil {
		return fmt.Errorf("approved but failed to update root folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approved but root folder update failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// selectRootFolderForApproval prompts the user to optionally override the root folder when approving a request.
// Returns the selected overrides or nil if the user chooses to use defaults or cancels.
func selectRootFolderForApproval(config Config, request MediaRequest, reader *bufio.Reader) (*RequestOverrides, error) {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	// Determine service type from request
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
		// Unknown type, proceed without override option
		return nil, nil
	}

	// Fetch available service instances
	servers, err := fetchServiceInstances(config, service)
	if err != nil {
		fmt.Printf("\n%sError fetching %s servers: %v%s\n", color(ColorRed), serviceLabel, err, color(ColorReset))
		fmt.Printf("Proceeding with approval without overrides...\n")
		return nil, nil
	}

	if len(servers) == 0 {
		// No servers configured, proceed without overrides
		return nil, nil
	}

	// Show override prompt
	clearScreen()
	fmt.Printf("%s%s=== Approve Request - Root Folder Override ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

	displayRequestDetail(config, request)

	fmt.Printf("\n%sWould you like to override the root folder for this request?%s\n", color(ColorBold), color(ColorReset))
	fmt.Printf("%s[Y]%s Yes, select root folder\n", color(ColorGreen), color(ColorReset))
	fmt.Printf("%s[N]%s No, use default (proceed with approval)\n", color(ColorYellow), color(ColorReset))
	fmt.Printf("%s[B]%s Back (cancel approval)\n\n", color(ColorRed), color(ColorReset))

	fmt.Printf("Select option: ")
	option, _ := reader.ReadString('\n')
	option = strings.TrimSpace(strings.ToLower(option))

	switch option {
	case "n", "":
		// Proceed without overrides
		return nil, nil

	case "b", "back":
		// Cancel approval
		return nil, fmt.Errorf("cancelled")

	case "y", "yes":
		// Continue to root folder selection
		break

	default:
		fmt.Printf("\n%sInvalid option.%s\n", color(ColorRed), color(ColorReset))
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, fmt.Errorf("invalid option")
	}

	// Select server
	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			clearScreen()
			fmt.Printf("%s%s=== Select %s Server ===%s\n\n", color(ColorBold), color(ColorCyan), serviceLabel, color(ColorReset))

			fmt.Printf("Available %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", color(ColorYellow), i+1, color(ColorReset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", color(ColorGray), strings.Join(badges, ", "), color(ColorReset))
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
					fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
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
		fmt.Printf("\nUsing %s server: %s%s%s\n", serviceLabel, color(ColorBold), selected.Name, color(ColorReset))
	}

	// Fetch server details to get root folders
	details, err := fetchServiceDetails(config, service, selected.ID)
	if err != nil {
		fmt.Printf("\n%sError fetching %s details: %v%s\n", color(ColorRed), serviceLabel, err, color(ColorReset))
		fmt.Printf("Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return nil, nil
	}

	if len(details.RootFolders) == 0 {
		fmt.Printf("\n%sNo root folders configured for %s.%s\n", color(ColorYellow), selected.Name, color(ColorReset))
		fmt.Printf("Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress Enter to continue...")
		reader.ReadString('\n')
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	// Select root folder
	for {
		clearScreen()
		fmt.Printf("%s%s=== Select Root Folder ===%s\n\n", color(ColorBold), color(ColorCyan), color(ColorReset))

		fmt.Printf("%sServer:%s %s\n\n", color(ColorBold), color(ColorReset), selected.Name)
		fmt.Printf("%sRoot folders:%s\n", color(ColorBold), color(ColorReset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", color(ColorYellow), i+1, color(ColorReset), folder.Path)
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
				fmt.Printf("\n%sInvalid selection.%s\n", color(ColorRed), color(ColorReset))
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

func declineRequest(config Config, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/decline", requestID)
	resp, err := makeRequest(config, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("decline failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
