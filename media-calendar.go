//go:build mediacalendar
// +build mediacalendar

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/term"
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
	ColorBold    = "\033[1m"
	ColorOrange  = "\033[0;33m" // Using yellow as orange approximation
)

// Layout and formatting constants
const (
	minComfortableColumnWidth = 45 // Minimum column width for readable content
	minShowTitleLength        = 15 // Minimum show title length before truncation
	minEpisodeTitleLength     = 10 // Minimum episode title length before truncation
	minMovieTitleLength       = 20 // Minimum movie title length before truncation
	minTerminalWidth          = 40 // Minimum terminal width before falling back to vertical layout
)

// Sonarr API structures
type SonarrEpisode struct {
	SeriesID      int       `json:"seriesId"`
	EpisodeID     int       `json:"id"`
	Title         string    `json:"title"`
	AirDate       string    `json:"airDate"`
	AirDateUtc    time.Time `json:"airDateUtc"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
	Monitored     bool      `json:"monitored"`
	HasFile       bool      `json:"hasFile"`
	Series        *Series   `json:"series"`
}

type Series struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SeasonCount int    `json:"seasonCount"`
}

// Radarr API structures
type RadarrMovie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Year        int    `json:"year"`
	ReleaseDate string `json:"physicalRelease"`
	DigitalDate string `json:"digitalRelease"`
	InCinemas   string `json:"inCinemas"`
	Monitored   bool   `json:"monitored"`
	HasFile     bool   `json:"hasFile"`
}

// Queue structures (shared between Sonarr/Radarr)
type QueueResponse struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

type QueueItem struct {
	ID             int             `json:"id"`
	Status         string          `json:"status"`
	TrackedState   string          `json:"trackedDownloadState"`
	StatusMessages []StatusMessage `json:"statusMessages"`
}

type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// Unified calendar item
type CalendarItem struct {
	Type           string // "episode" or "movie"
	Title          string // Episode or movie title
	ShowTitle      string // For episodes: series name
	Year           int
	Season         int
	Episode        int
	AirTime        time.Time
	HasFile        bool
	Monitored      bool
	IsPremiere     bool
	SourceInstance string // Which Sonarr/Radarr instance
}

// Config holds the application configuration
type Config struct {
	SonarrURLs   []string
	SonarrTokens []string
	RadarrURLs   []string
	RadarrTokens []string
	Timeout      time.Duration
	Days         int
	DaysPast     int
	NoColor      bool
	JSONOutput   bool
	WatchMode    bool
	WatchSeconds int
	Debug        bool
}

// QueueIssue represents a service with queue problems
type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

// Summary holds JSON output structure
type Summary struct {
	StartDate   string         `json:"start_date"`
	EndDate     string         `json:"end_date"`
	TotalItems  int            `json:"total_items"`
	Episodes    int            `json:"episodes"`
	Movies      int            `json:"movies"`
	Available   int            `json:"available"`
	Missing     int            `json:"missing"`
	Timestamp   time.Time      `json:"timestamp"`
	QueueIssues []QueueIssue   `json:"queue_issues,omitempty"`
	Items       []CalendarItem `json:"items"`
}

func main() {
	// Command line flags
	var (
		sonarrURLs   = flag.String("sonarr-urls", "", "Comma-separated Sonarr URLs")
		sonarrTokens = flag.String("sonarr-tokens", "", "Comma-separated Sonarr API tokens")
		radarrURLs   = flag.String("radarr-urls", "", "Comma-separated Radarr URLs")
		radarrTokens = flag.String("radarr-tokens", "", "Comma-separated Radarr API tokens")
		timeout      = flag.Duration("timeout", 10*time.Second, "Connection timeout")
		days         = flag.Int("days", 1, "Number of days to display (1 = today only)")
		daysPast     = flag.Int("days-past", 0, "Number of days in the past to display (0 = no past days)")
		noColor      = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput   = flag.Bool("json", false, "Output in JSON format")
		watchMode    = flag.Bool("watch", false, "Continuously monitor calendar")
		watchSeconds = flag.Int("interval", 300, "Watch mode refresh interval in seconds")
		debug        = flag.Bool("debug", false, "Enable debug logging (shows API URLs)")
	)
	flag.Parse()

	// Load configuration
	config := loadConfig(*sonarrURLs, *sonarrTokens, *radarrURLs, *radarrTokens,
		*timeout, *days, *daysPast, *noColor, *jsonOutput, *watchMode, *watchSeconds, *debug)

	// Validate configuration
	if len(config.SonarrURLs) == 0 && len(config.RadarrURLs) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Please set SONARR_URLS/SONARR_TOKENS or RADARR_URLS/RADARR_TOKENS environment variables\n")
		fmt.Fprintf(os.Stderr, "Or use -sonarr-urls/-sonarr-tokens or -radarr-urls/-radarr-tokens flags\n")
		os.Exit(1)
	}

	// Watch mode: continuously monitor
	if config.WatchMode {
		for {
			clearScreen()
			if err := displayCalendar(config); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			time.Sleep(time.Duration(config.WatchSeconds) * time.Second)
		}
	}

	// Single execution
	if err := displayCalendar(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(sonarrURLsFlag, sonarrTokensFlag, radarrURLsFlag, radarrTokensFlag string,
	timeout time.Duration, days int, daysPast int, noColor, jsonOutput, watchMode bool, watchSeconds int, debug bool) Config {

	config := Config{
		Timeout:      timeout,
		Days:         days,
		DaysPast:     daysPast,
		NoColor:      noColor || jsonOutput,
		JSONOutput:   jsonOutput,
		WatchMode:    watchMode,
		WatchSeconds: watchSeconds,
		Debug:        debug || os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
	}

	// Try to load from .env file first
	// Check current directory first, then fallback to /opt/apps/compose/.env
	envPaths := []string{".env", "/opt/apps/compose/.env"}
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			loadEnvFile(envPath, &config)
			break // Use first found .env file
		}
	}

	// Environment variables override .env file
	// Support both plural (SONARR_URLS) and singular (SONARR_URL) variants
	if envURLs := os.Getenv("SONARR_URLS"); envURLs != "" {
		config.SonarrURLs = parseCommaSeparated(envURLs)
	} else if envURL := os.Getenv("SONARR_URL"); envURL != "" {
		// Fallback to singular SONARR_URL
		config.SonarrURLs = []string{envURL}
	}

	if envTokens := os.Getenv("SONARR_TOKENS"); envTokens != "" {
		config.SonarrTokens = parseCommaSeparated(envTokens)
	} else if envToken := os.Getenv("SONARR_API_TOKEN"); envToken != "" {
		// Fallback to singular SONARR_API_TOKEN
		config.SonarrTokens = []string{envToken}
	}

	if envURLs := os.Getenv("RADARR_URLS"); envURLs != "" {
		config.RadarrURLs = parseCommaSeparated(envURLs)
	} else if envURL := os.Getenv("RADARR_URL"); envURL != "" {
		// Fallback to singular RADARR_URL
		config.RadarrURLs = []string{envURL}
	}

	if envTokens := os.Getenv("RADARR_TOKENS"); envTokens != "" {
		config.RadarrTokens = parseCommaSeparated(envTokens)
	} else if envToken := os.Getenv("RADARR_API_TOKEN"); envToken != "" {
		// Fallback to singular RADARR_API_TOKEN
		config.RadarrTokens = []string{envToken}
	}

	// Command line flags override everything
	if sonarrURLsFlag != "" {
		config.SonarrURLs = parseCommaSeparated(sonarrURLsFlag)
	}
	if sonarrTokensFlag != "" {
		config.SonarrTokens = parseCommaSeparated(sonarrTokensFlag)
	}
	if radarrURLsFlag != "" {
		config.RadarrURLs = parseCommaSeparated(radarrURLsFlag)
	}
	if radarrTokensFlag != "" {
		config.RadarrTokens = parseCommaSeparated(radarrTokensFlag)
	}

	// Clean URLs
	for i := range config.SonarrURLs {
		config.SonarrURLs[i] = strings.TrimSuffix(config.SonarrURLs[i], "/")
	}
	for i := range config.RadarrURLs {
		config.RadarrURLs[i] = strings.TrimSuffix(config.RadarrURLs[i], "/")
	}

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
		case "SONARR_URLS":
			config.SonarrURLs = parseCommaSeparated(value)
		case "SONARR_TOKENS":
			config.SonarrTokens = parseCommaSeparated(value)
		case "RADARR_URLS":
			config.RadarrURLs = parseCommaSeparated(value)
		case "RADARR_TOKENS":
			config.RadarrTokens = parseCommaSeparated(value)
		}
	}
}

func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func fetchSonarrCalendar(url, token string, start, end time.Time, debug bool) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		url, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Fetching Sonarr calendar from: %s\n", apiURL)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sonarr at %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sonarr returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var episodes []SonarrEpisode
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, err
	}

	return episodes, nil
}

func fetchRadarrCalendar(url, token string, start, end time.Time, debug bool) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s",
		url, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Fetching Radarr calendar from: %s\n", apiURL)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Radarr at %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Radarr returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var movies []RadarrMovie
	if err := json.Unmarshal(body, &movies); err != nil {
		return nil, err
	}

	return movies, nil
}

func fetchQueue(url, token string, debug bool) (*QueueResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v3/queue", url)

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Fetching queue from: %s\n", apiURL)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to queue at %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("queue API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var queue QueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func aggregateCalendar(config Config) ([]CalendarItem, []QueueIssue, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Adjust start date if looking at past days
	if config.DaysPast > 0 {
		start = start.AddDate(0, 0, -config.DaysPast)
	}

	// End date is start + config.Days (total days to display including past)
	end := start.AddDate(0, 0, config.Days+config.DaysPast)

	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)

	// Track seen series/movies for deduplication
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)

	// Fetch from all Sonarr instances
	for i, url := range config.SonarrURLs {
		if i >= len(config.SonarrTokens) {
			fmt.Fprintf(os.Stderr, "WARNING: No token for Sonarr instance %s\n", url)
			continue
		}
		token := config.SonarrTokens[i]

		episodes, err := fetchSonarrCalendar(url, token, start, end, config.Debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to fetch from Sonarr %s: %v\n", url, err)
			continue
		}

		instanceName := fmt.Sprintf("Sonarr-%d", i+1)

		for _, ep := range episodes {
			if ep.Series == nil {
				continue
			}

			// Deduplicate by series title + season + episode
			key := fmt.Sprintf("%s-S%02dE%02d", ep.Series.Title, ep.SeasonNumber, ep.EpisodeNumber)
			if seenEpisodes[key] {
				continue
			}
			seenEpisodes[key] = true

			// Detect premiere
			isPremiere := ep.SeasonNumber == 1 && ep.EpisodeNumber == 1

			item := CalendarItem{
				Type:           "episode",
				Title:          ep.Title,
				ShowTitle:      ep.Series.Title,
				Year:           ep.Series.Year,
				Season:         ep.SeasonNumber,
				Episode:        ep.EpisodeNumber,
				AirTime:        ep.AirDateUtc,
				HasFile:        ep.HasFile,
				Monitored:      ep.Monitored,
				IsPremiere:     isPremiere,
				SourceInstance: instanceName,
			}
			items = append(items, item)
		}

		// Check queue
		queue, err := fetchQueue(url, token, config.Debug)
		if err == nil && queue != nil {
			errorCount := 0
			for _, item := range queue.Records {
				if item.TrackedState == "importFailed" || item.TrackedState == "importPending" ||
					len(item.StatusMessages) > 0 {
					errorCount++
				}
			}
			if errorCount > 0 {
				queueIssues = append(queueIssues, QueueIssue{
					ServiceName: instanceName,
					URL:         url + "/activity/queue",
					Count:       errorCount,
				})
			}
		}
	}

	// Fetch from all Radarr instances
	for i, url := range config.RadarrURLs {
		if i >= len(config.RadarrTokens) {
			fmt.Fprintf(os.Stderr, "WARNING: No token for Radarr instance %s\n", url)
			continue
		}
		token := config.RadarrTokens[i]

		movies, err := fetchRadarrCalendar(url, token, start, end, config.Debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to fetch from Radarr %s: %v\n", url, err)
			continue
		}

		instanceName := fmt.Sprintf("Radarr-%d", i+1)

		for _, movie := range movies {
			// Deduplicate by title + year
			key := fmt.Sprintf("%s-%d", movie.Title, movie.Year)
			if seenMovies[key] {
				continue
			}
			seenMovies[key] = true

			// Determine release date (prefer digital > physical > cinema)
			var releaseTime time.Time
			if movie.DigitalDate != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.DigitalDate)
			} else if movie.ReleaseDate != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.ReleaseDate)
			} else if movie.InCinemas != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.InCinemas)
			}

			if releaseTime.IsZero() {
				continue
			}

			item := CalendarItem{
				Type:           "movie",
				Title:          movie.Title,
				Year:           movie.Year,
				AirTime:        releaseTime,
				HasFile:        movie.HasFile,
				Monitored:      movie.Monitored,
				IsPremiere:     false,
				SourceInstance: instanceName,
			}
			items = append(items, item)
		}

		// Check queue
		queue, err := fetchQueue(url, token, config.Debug)
		if err == nil && queue != nil {
			errorCount := 0
			for _, item := range queue.Records {
				if item.TrackedState == "importFailed" || item.TrackedState == "importPending" ||
					len(item.StatusMessages) > 0 {
					errorCount++
				}
			}
			if errorCount > 0 {
				queueIssues = append(queueIssues, QueueIssue{
					ServiceName: instanceName,
					URL:         url + "/activity/queue",
					Count:       errorCount,
				})
			}
		}
	}

	// Sort items by air time
	sort.Slice(items, func(i, j int) bool {
		return items[i].AirTime.Before(items[j].AirTime)
	})

	return items, queueIssues, nil
}

func displayCalendar(config Config) error {
	items, queueIssues, err := aggregateCalendar(config)
	if err != nil {
		return err
	}

	if config.JSONOutput {
		return displayJSON(items, queueIssues, config)
	}

	return displayTerminal(items, queueIssues, config)
}

func displayJSON(items []CalendarItem, queueIssues []QueueIssue, config Config) error {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Adjust start date if looking at past days
	if config.DaysPast > 0 {
		start = start.AddDate(0, 0, -config.DaysPast)
	}

	end := start.AddDate(0, 0, config.Days+config.DaysPast)

	episodes := 0
	movies := 0
	available := 0
	missing := 0

	for _, item := range items {
		if item.Type == "episode" {
			episodes++
		} else {
			movies++
		}
		if item.HasFile {
			available++
		} else if item.AirTime.Before(now) {
			missing++
		}
	}

	summary := Summary{
		StartDate:   start.Format("2006-01-02"),
		EndDate:     end.Format("2006-01-02"),
		TotalItems:  len(items),
		Episodes:    episodes,
		Movies:      movies,
		Available:   available,
		Missing:     missing,
		Timestamp:   now,
		QueueIssues: queueIssues,
		Items:       items,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

type calendarLayout struct {
	numColumns     int
	widthPerColumn int
	totalDays      int
	color          func(string) string
}

func displayTerminal(items []CalendarItem, queueIssues []QueueIssue, config Config) error {
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	// Display queue warning banner if issues exist
	if len(queueIssues) > 0 {
		totalIssues := 0
		for _, issue := range queueIssues {
			totalIssues += issue.Count
		}

		fmt.Printf("%s%s⚠️  WARNING: %d items require manual intervention%s\n",
			color(ColorBold), color(ColorRed), totalIssues, color(ColorReset))
		for _, issue := range queueIssues {
			fmt.Printf("%s→ %s: %s%s\n",
				color(ColorYellow), issue.ServiceName, issue.URL, color(ColorReset))
		}
		fmt.Println()
	}

	// Display header
	fmt.Printf("%s%s=== Media Calendar ===%s\n\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Adjust start date if looking at past days
	if config.DaysPast > 0 {
		start = start.AddDate(0, 0, -config.DaysPast)
	}

	// Get terminal width
	termWidth := getTerminalWidth()

	// Minimum width for vertical fallback
	if termWidth < minTerminalWidth {
		return displayVertical(items, config, now, start)
	}

	// Calculate how many columns (days) we can fit
	totalDays := config.Days + config.DaysPast
	numColumns, widthPerColumn := calculateColumnLayout(termWidth, totalDays)

	// Group items by day
	dayGroups := make(map[string][]CalendarItem)
	for _, item := range items {
		dayKey := item.AirTime.Format("2006-01-02")
		dayGroups[dayKey] = append(dayGroups[dayKey], item)
	}

	// Render horizontal calendar
	layout := calendarLayout{
		numColumns:     numColumns,
		widthPerColumn: widthPerColumn,
		totalDays:      totalDays,
		color:          color,
	}
	return renderHorizontalCalendar(dayGroups, config, now, start, layout)
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80 // Default fallback
	}
	return width
}

// calculateColumnLayout determines the optimal number of columns and width per column
// based on the terminal width and total number of days to display
func calculateColumnLayout(termWidth, totalDays int) (numColumns, widthPerColumn int) {
	// Start with all days if possible
	numColumns = totalDays

	// Calculate the available width per column
	// tablewriter uses: | col1 | col2 | col3 |
	// That's: 1 (left border) + (numCols * (1 space + content + 1 space + 1 border))
	// Simplified: 1 + numCols * 3 + total_content_width

	// Work backwards: given terminal width, how much space per column?
	tableBorderOverhead := 1 + (numColumns * 3)
	availableContentWidth := termWidth - tableBorderOverhead

	if availableContentWidth > 0 {
		widthPerColumn = availableContentWidth / numColumns

		// If each column would be too narrow, reduce number of columns
		for widthPerColumn < minComfortableColumnWidth && numColumns > 1 {
			numColumns--
			tableBorderOverhead = 1 + (numColumns * 3)
			availableContentWidth = termWidth - tableBorderOverhead
			if availableContentWidth > 0 {
				widthPerColumn = availableContentWidth / numColumns
			}
		}
	}

	// Ensure at least 1 column
	if numColumns < 1 {
		numColumns = 1
	}

	// Calculate final width per column for formatting decisions
	finalBorderOverhead := 1 + (numColumns * 3)
	finalContentWidth := termWidth - finalBorderOverhead
	widthPerColumn = 60 // default
	if finalContentWidth > 0 && numColumns > 0 {
		widthPerColumn = finalContentWidth / numColumns
	}

	return numColumns, widthPerColumn
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func renderHorizontalCalendar(dayGroups map[string][]CalendarItem, config Config, now time.Time, start time.Time, layout calendarLayout) error {
	// Collect all headers and all day columns
	allHeaders := make([]string, 0)
	allDayColumns := make([][]string, 0)
	maxRows := 0

	for d := 0; d < layout.totalDays; d++ {
		currentDay := start.AddDate(0, 0, d)
		dayKey := currentDay.Format("2006-01-02")
		dayItems := dayGroups[dayKey]

		// Add header
		allHeaders = append(allHeaders, currentDay.Format("Mon 01/02"))

		// Build content for this day
		content := buildDayContent(dayItems, now, config, layout.color, layout.widthPerColumn)
		allDayColumns = append(allDayColumns, content)

		if len(content) > maxRows {
			maxRows = len(content)
		}
	}

	// If we need to wrap to multiple rows, we need to create sections
	// But display them as one continuous table by not adding spacing
	for sectionStart := 0; sectionStart < layout.totalDays; sectionStart += layout.numColumns {
		sectionEnd := sectionStart + layout.numColumns
		if sectionEnd > layout.totalDays {
			sectionEnd = layout.totalDays
		}

		// Create a table for this section
		table := tablewriter.NewWriter(os.Stdout)

		// Extract headers for this section
		sectionHeaders := allHeaders[sectionStart:sectionEnd]
		headerInterfaces := make([]interface{}, len(sectionHeaders))
		for i, h := range sectionHeaders {
			headerInterfaces[i] = h
		}
		table.Header(headerInterfaces...)

		// Build rows for this section
		sectionMaxRows := 0
		for i := sectionStart; i < sectionEnd; i++ {
			if len(allDayColumns[i]) > sectionMaxRows {
				sectionMaxRows = len(allDayColumns[i])
			}
		}

		for row := 0; row < sectionMaxRows; row++ {
			rowData := make([]interface{}, sectionEnd-sectionStart)
			for col := 0; col < sectionEnd-sectionStart; col++ {
				dayIdx := sectionStart + col
				if row < len(allDayColumns[dayIdx]) {
					rowData[col] = allDayColumns[dayIdx][row]
				} else {
					rowData[col] = ""
				}
			}
			table.Append(rowData...)
		}
		table.Render()

		// NO spacing between sections - they should appear attached
	}

	return nil
}

func buildDayContent(dayItems []CalendarItem, now time.Time, config Config, color func(string) string, widthPerColumn int) []string {
	content := make([]string, 0)

	if len(dayItems) == 0 {
		content = append(content, color(ColorGreen)+"No releases"+color(ColorReset))
		return content
	}

	// Sort all items by air time first, then by type (episodes before movies), then by season/episode
	sort.Slice(dayItems, func(i, j int) bool {
		// Primary sort: air time
		if !dayItems[i].AirTime.Equal(dayItems[j].AirTime) {
			return dayItems[i].AirTime.Before(dayItems[j].AirTime)
		}
		// Secondary sort: episodes before movies
		if dayItems[i].Type != dayItems[j].Type {
			return dayItems[i].Type == "episode"
		}
		// Tertiary sort for episodes: season then episode number
		if dayItems[i].Type == "episode" {
			if dayItems[i].Season != dayItems[j].Season {
				return dayItems[i].Season < dayItems[j].Season
			}
			return dayItems[i].Episode < dayItems[j].Episode
		}
		// For movies, maintain stable order
		return false
	})

	// Process items in chronological order with truncation for consecutive same-show episodes
	i := 0
	for i < len(dayItems) {
		item := dayItems[i]

		if item.Type == "movie" {
			content = append(content, formatMovie(item, now, config, color, widthPerColumn))
			i++
			continue
		}

		// For episodes, check if there are consecutive episodes from the same show
		showStart := i
		showEnd := i + 1
		for showEnd < len(dayItems) &&
			dayItems[showEnd].Type == "episode" &&
			dayItems[showEnd].ShowTitle == item.ShowTitle {
			showEnd++
		}
		consecutiveCount := showEnd - showStart

		// If multiple consecutive episodes from same show, show first 2 then collapse
		maxDisplay := 2
		if consecutiveCount > maxDisplay {
			// Show first 2
			for j := showStart; j < showStart+maxDisplay; j++ {
				content = append(content, formatEpisode(dayItems[j], now, config, color, widthPerColumn))
			}
			// Add truncation
			remaining := consecutiveCount - maxDisplay
			content = append(content, color(ColorCyan)+fmt.Sprintf("  + %d more episodes", remaining)+color(ColorReset))
		} else {
			// Show all if only 1-2 consecutive episodes
			for j := showStart; j < showEnd; j++ {
				content = append(content, formatEpisode(dayItems[j], now, config, color, widthPerColumn))
			}
		}

		i = showEnd
	}

	return content
}

func formatEpisode(ep CalendarItem, now time.Time, config Config, color func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(ep, now, config.NoColor)
	timeStr := ep.AirTime.Format("15:04")

	showTitle := ep.ShowTitle
	episodeTitle := ep.Title

	// Multi-line format:
	// Line 1: "HH:MM SHOWNAME - S##E##"
	// Line 2: "       EPISODE_TITLE"

	// Fixed parts for line 1: time(5) + space(1) + " - S##E##"(10) = 16 chars
	fixedCharsLine1 := 16
	maxShowLen := widthPerColumn - fixedCharsLine1

	// Apply minimum sensible length for show title
	if maxShowLen < minShowTitleLength {
		maxShowLen = minShowTitleLength
	}

	// Truncate show title if needed
	if len(showTitle) > maxShowLen {
		showTitle = showTitle[:maxShowLen-3] + "..."
	}

	// Line 2 has leading spaces (7 chars: "       ") for alignment
	indentSpaces := "       "
	maxEpisodeLen := widthPerColumn - len(indentSpaces)

	// Apply minimum sensible length for episode title
	if maxEpisodeLen < minEpisodeTitleLength {
		maxEpisodeLen = minEpisodeTitleLength
	}

	// Truncate episode title if needed
	if len(episodeTitle) > maxEpisodeLen {
		episodeTitle = episodeTitle[:maxEpisodeLen-3] + "..."
	}

	// Build multi-line format using \n
	line1 := fmt.Sprintf("%s %s%s - S%02dE%02d%s",
		timeStr, statusColor, showTitle, ep.Season, ep.Episode, color(ColorReset))
	line2 := fmt.Sprintf("%s%s%s%s",
		indentSpaces, statusColor, episodeTitle, color(ColorReset))

	return line1 + "\n" + line2
}

func formatMovie(movie CalendarItem, now time.Time, config Config, color func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(movie, now, config.NoColor)
	timeStr := movie.AirTime.Format("15:04")

	title := movie.Title

	// Calculate available space
	// Format: "HH:MM TITLE (YYYY)"
	// Fixed parts: time(5) + space(1) + " (####)"(7) = 13 chars
	fixedChars := 13
	maxTitleLen := widthPerColumn - fixedChars

	// Only truncate if necessary
	if maxTitleLen < len(title) {
		if maxTitleLen < minMovieTitleLength {
			maxTitleLen = minMovieTitleLength
		}
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}
	}

	return fmt.Sprintf("%s %s%s (%d)%s",
		timeStr, statusColor, title, movie.Year, color(ColorReset))
}

func displayVertical(items []CalendarItem, config Config, now time.Time, start time.Time) error {
	// Fallback to vertical layout for narrow terminals
	color := func(code string) string {
		if config.NoColor {
			return ""
		}
		return code
	}

	// Group items by day
	dayGroups := make(map[string][]CalendarItem)
	for _, item := range items {
		dayKey := item.AirTime.Format("2006-01-02")
		dayGroups[dayKey] = append(dayGroups[dayKey], item)
	}

	// Display each day
	totalDaysToDisplay := config.Days + config.DaysPast
	for d := 0; d < totalDaysToDisplay; d++ {
		currentDay := start.AddDate(0, 0, d)
		dayKey := currentDay.Format("2006-01-02")
		dayItems := dayGroups[dayKey]

		// Day header
		fmt.Printf("\n%s%s%s%s\n",
			color(ColorBold), color(ColorCyan),
			currentDay.Format("Mon 01/02"),
			color(ColorReset))
		fmt.Printf("%s%s%s%s\n",
			color(ColorBold), color(ColorCyan),
			strings.Repeat("━", 40),
			color(ColorReset))

		if len(dayItems) == 0 {
			fmt.Printf("%sNo scheduled releases%s\n",
				color(ColorGreen), color(ColorReset))
			continue
		}

		// Group episodes by show for truncation
		showEpisodes := make(map[string][]CalendarItem)
		var movieItems []CalendarItem

		for _, item := range dayItems {
			if item.Type == "episode" {
				showEpisodes[item.ShowTitle] = append(showEpisodes[item.ShowTitle], item)
			} else {
				movieItems = append(movieItems, item)
			}
		}

		// Display episodes (with truncation)
		for show, episodes := range showEpisodes {
			maxDisplay := 3
			for i, ep := range episodes {
				if i >= maxDisplay {
					remaining := len(episodes) - maxDisplay
					fmt.Printf("  %s+ %d more episodes%s\n",
						color(ColorCyan), remaining, color(ColorReset))
					break
				}

				statusColor := getStatusColor(ep, now, config.NoColor)
				timeStr := ep.AirTime.Format("15:04")

				// Multi-line format for consistency with horizontal view
				fmt.Printf("  %s%s%s %s%s - S%02dE%02d%s\n",
					color(ColorBold), timeStr, color(ColorReset),
					statusColor, show, ep.Season, ep.Episode, color(ColorReset))
				fmt.Printf("         %s%s%s\n",
					statusColor, ep.Title, color(ColorReset))
			}
		}

		// Display movies
		for _, movie := range movieItems {
			statusColor := getStatusColor(movie, now, config.NoColor)
			timeStr := movie.AirTime.Format("15:04")

			fmt.Printf("  %s%s%s %s%s (%d)%s\n",
				color(ColorBold), timeStr, color(ColorReset),
				statusColor, movie.Title, movie.Year, color(ColorReset))
		}
	}

	return nil
}

func getStatusColor(item CalendarItem, now time.Time, noColor bool) string {
	if noColor {
		return ""
	}

	if item.HasFile {
		return ColorGreen
	}

	if item.AirTime.Before(now) {
		return ColorRed
	}

	if item.IsPremiere {
		return ColorOrange
	}

	return ColorBlue
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
