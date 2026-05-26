//go:build mediacalendar

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

const (
	minComfortableColumnWidth = 45
	minShowTitleLength        = 15
	minEpisodeTitleLength     = 10
	minMovieTitleLength       = 20
	minTerminalWidth          = 40
	maxDisplayPerShow         = 2
	maxDisplayPerShowVertical = 3
)

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

type CalendarItem struct {
	Type           string
	Title          string
	ShowTitle      string
	Year           int
	Season         int
	Episode        int
	AirTime        time.Time
	HasFile        bool
	Monitored      bool
	IsPremiere     bool
	SourceInstance string
}

type CalendarToolConfig struct {
	SonarrInstances []ArrInstance
	RadarrInstances []ArrInstance
	Timeout         time.Duration
	Days            int
	DaysPast        int
	NoColor         bool
	JSONOutput      bool
	WatchMode       bool
	WatchSeconds    int
	Debug           bool
	NoBanner        bool
	Quiet           bool
	Filter          string
	MonitoredOnly   bool
}

type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

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

type fetchEpisodeResult struct {
	Instance string
	Episodes []SonarrEpisode
	Err      error
}

type fetchMovieResult struct {
	Instance string
	Movies   []RadarrMovie
	Err      error
}

type fetchQueueResult struct {
	Instance string
	Queue    *QueueResponse
	Err      error
}

func BuildCalendarToolConfig(tk *ToolkitConfig) CalendarToolConfig {
	cfg := CalendarToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.Days = 1
		cfg.WatchSeconds = 300
		return cfg
	}

	dur, err := time.ParseDuration(tk.General.Timeout)
	if err != nil || dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor

	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)

	if tk.MediaCalendar.Days > 0 {
		cfg.Days = tk.MediaCalendar.Days
	} else {
		cfg.Days = 1
	}
	cfg.DaysPast = tk.MediaCalendar.DaysPast
	if tk.MediaCalendar.WatchInterval > 0 {
		cfg.WatchSeconds = tk.MediaCalendar.WatchInterval
	} else {
		cfg.WatchSeconds = 300
	}
	cfg.Debug = tk.MediaCalendar.Debug

	return cfg
}

func calculateDateRange(days, daysPast int) (start, end time.Time) {
	now := time.Now()
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if daysPast > 0 {
		start = start.AddDate(0, 0, -daysPast)
	}
	end = start.AddDate(0, 0, days+daysPast)
	return start, end
}

func main() {
	tk, err := LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := BuildCalendarToolConfig(tk)

	days := flag.Int("days", cfg.Days, "Number of days to display")
	daysPast := flag.Int("days-past", cfg.DaysPast, "Number of past days to include")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP connection timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	watchMode := flag.Bool("watch", false, "Continuously monitor calendar")
	watchSeconds := flag.Int("interval", cfg.WatchSeconds, "Watch mode refresh interval in seconds")
	debug := flag.Bool("debug", cfg.Debug, "Enable debug logging")
	noBanner := flag.Bool("no-banner", false, "Suppress the banner header")
	quiet := flag.Bool("quiet", false, "Suppress queue warnings")
	filter := flag.String("filter", "", "Filter: missing,available,premieres,monitored (comma-separated)")
	monitoredOnly := flag.Bool("monitored-only", false, "Only show monitored items")
	flag.Parse()

	cfg.Days = *days
	cfg.DaysPast = *daysPast
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *jsonOutput
	cfg.JSONOutput = *jsonOutput
	cfg.WatchMode = *watchMode
	cfg.WatchSeconds = *watchSeconds
	cfg.Debug = *debug
	cfg.NoBanner = *noBanner
	cfg.Quiet = *quiet
	cfg.Filter = *filter
	cfg.MonitoredOnly = *monitoredOnly

	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	if cfg.WatchMode {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		for {
			fmt.Print(AnsiClearScreen + AnsiHomeCursor)
			if err := displayCalendar(ctx, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			select {
			case <-ctx.Done():
				fmt.Println("\nShutting down.")
				return
			case <-time.After(time.Duration(cfg.WatchSeconds) * time.Second):
			}
		}
	}

	ctx := context.Background()
	if err := displayCalendar(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func displayCalendar(ctx context.Context, cfg CalendarToolConfig) error {
	items, queueIssues, err := aggregateCalendar(ctx, cfg)
	if err != nil {
		return err
	}

	if cfg.JSONOutput {
		return displayJSON(items, queueIssues, cfg)
	}

	return displayTerminal(items, queueIssues, cfg)
}

func aggregateCalendar(ctx context.Context, cfg CalendarToolConfig) ([]CalendarItem, []QueueIssue, error) {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        20,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
		},
	}

	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)
	var mu sync.Mutex
	var qMu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)

	// Fetch Sonarr instances concurrently
	for _, inst := range cfg.SonarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchSonarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenEpisodes)
		})
	}

	// Fetch Radarr instances concurrently
	for _, inst := range cfg.RadarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchRadarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenMovies)
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	// Sort items by air time
	sort.Slice(items, func(i, j int) bool {
		return items[i].AirTime.Before(items[j].AirTime)
	})

	return items, queueIssues, nil
}

func fetchSonarrInstance(ctx context.Context, client *http.Client, inst ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
	episodes, err := fetchSonarrCalendar(ctx, client, inst, start, end, debug)
	if err != nil {
		return fmt.Errorf("Sonarr %s: %w", inst.Name, err)
	}

	mu.Lock()
	for _, ep := range episodes {
		if ep.Series == nil {
			continue
		}
		key := fmt.Sprintf("%s-S%02dE%02d", ep.Series.Title, ep.SeasonNumber, ep.EpisodeNumber)
		if seen[key] {
			continue
		}
		seen[key] = true

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
			IsPremiere:     ep.SeasonNumber == 1 && ep.EpisodeNumber == 1,
			SourceInstance: inst.Name,
		}
		*items = append(*items, item)
	}
	mu.Unlock()

	// Check queue
	queue, err := fetchQueue(ctx, client, inst, debug)
	if err != nil {
		return nil // queue check is best-effort
	}

	if queue != nil {
		errorCount := 0
		for _, qItem := range queue.Records {
			if qItem.TrackedState == "importFailed" || qItem.TrackedState == "importPending" ||
				len(qItem.StatusMessages) > 0 {
				errorCount++
			}
		}
		if errorCount > 0 {
			qMu.Lock()
			*queueIssues = append(*queueIssues, QueueIssue{
				ServiceName: inst.Name,
				URL:         inst.URL + "/activity/queue",
				Count:       errorCount,
			})
			qMu.Unlock()
		}
	}

	return nil
}

func fetchRadarrInstance(ctx context.Context, client *http.Client, inst ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
	movies, err := fetchRadarrCalendar(ctx, client, inst, start, end, debug)
	if err != nil {
		return fmt.Errorf("Radarr %s: %w", inst.Name, err)
	}

	mu.Lock()
	for _, movie := range movies {
		key := fmt.Sprintf("%s-%d", movie.Title, movie.Year)
		if seen[key] {
			continue
		}
		seen[key] = true

		var releaseTime time.Time
		if movie.DigitalDate != "" {
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.DigitalDate)
		}
		if releaseTime.IsZero() && movie.ReleaseDate != "" {
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.ReleaseDate)
		}
		if releaseTime.IsZero() && movie.InCinemas != "" {
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.InCinemas)
		}
		if releaseTime.IsZero() {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Skipping %s (%d) - no valid release date\n", movie.Title, movie.Year)
			}
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
			SourceInstance: inst.Name,
		}
		*items = append(*items, item)
	}
	mu.Unlock()

	queue, err := fetchQueue(ctx, client, inst, debug)
	if err != nil {
		return nil
	}

	if queue != nil {
		errorCount := 0
		for _, qItem := range queue.Records {
			if qItem.TrackedState == "importFailed" || qItem.TrackedState == "importPending" ||
				len(qItem.StatusMessages) > 0 {
				errorCount++
			}
		}
		if errorCount > 0 {
			qMu.Lock()
			*queueIssues = append(*queueIssues, QueueIssue{
				ServiceName: inst.Name,
				URL:         inst.URL + "/activity/queue",
				Count:       errorCount,
			})
			qMu.Unlock()
		}
	}

	return nil
}

func fetchSonarrCalendar(ctx context.Context, client *http.Client, inst ArrInstance, start, end time.Time, debug bool) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
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

func fetchRadarrCalendar(ctx context.Context, client *http.Client, inst ArrInstance, start, end time.Time, debug bool) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
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

func fetchQueue(ctx context.Context, client *http.Client, inst ArrInstance, debug bool) (*QueueResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v3/queue?pageSize=1", inst.URL)

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Queue %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
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

func displayJSON(items []CalendarItem, queueIssues []QueueIssue, cfg CalendarToolConfig) error {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)
	now := time.Now()

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

func displayTerminal(items []CalendarItem, queueIssues []QueueIssue, cfg CalendarToolConfig) error {
	color := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	now := time.Now()
	start, _ := calculateDateRange(cfg.Days, cfg.DaysPast)

	if !cfg.Quiet && len(queueIssues) > 0 {
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

	if !cfg.NoBanner {
		fmt.Printf("%s%s=== Media Calendar ===%s\n\n",
			color(ColorBold), color(ColorCyan), color(ColorReset))
	}

	items = applyFilters(items, cfg)

	if len(items) == 0 {
		fmt.Printf("%sNo items match the current filters.%s\n",
			color(ColorGreen), color(ColorReset))
		return nil
	}

	termWidth := getTerminalWidth()
	if termWidth < minTerminalWidth {
		return displayVertical(items, cfg, now, start)
	}

	totalDays := cfg.Days + cfg.DaysPast
	numColumns, widthPerColumn := calculateColumnLayout(termWidth, totalDays)

	dayGroups := make(map[string][]CalendarItem)
	for _, item := range items {
		dayKey := item.AirTime.Format("2006-01-02")
		dayGroups[dayKey] = append(dayGroups[dayKey], item)
	}

	layout := calendarLayout{
		numColumns:     numColumns,
		widthPerColumn: widthPerColumn,
		totalDays:      totalDays,
		color:          color,
	}
	if err := renderHorizontalCalendar(dayGroups, cfg, now, start, layout); err != nil {
		return err
	}

	displaySummary(items, now, color)

	return nil
}

func applyFilters(items []CalendarItem, cfg CalendarToolConfig) []CalendarItem {
	if cfg.MonitoredOnly {
		filtered := make([]CalendarItem, 0, len(items))
		for _, item := range items {
			if item.Monitored {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	if cfg.Filter != "" {
		filters := strings.Split(cfg.Filter, ",")
		filtered := make([]CalendarItem, 0, len(items))
		for _, item := range items {
			match := false
			for _, f := range filters {
				switch strings.TrimSpace(strings.ToLower(f)) {
				case "missing":
					if !item.HasFile && item.AirTime.Before(time.Now()) {
						match = true
					}
				case "available":
					if item.HasFile {
						match = true
					}
				case "premieres":
					if item.IsPremiere {
						match = true
					}
				case "monitored":
					if item.Monitored {
						match = true
					}
				}
				if match {
					break
				}
			}
			if match {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	return items
}

func displaySummary(items []CalendarItem, now time.Time, color func(string) string) {
	episodes := 0
	movies := 0
	available := 0
	missing := 0
	future := 0

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
	future = len(items) - available - missing

	fmt.Printf("\n%s%s%d items%s (%d episodes, %d movies) — ",
		color(ColorBold), color(ColorCyan), len(items), color(ColorReset), episodes, movies)
	fmt.Printf("%s%d available%s, %s%d missing%s, %d upcoming\n",
		color(ColorGreen), available, color(ColorReset),
		color(ColorRed), missing, color(ColorReset),
		future)
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

func calculateColumnLayout(termWidth, totalDays int) (numColumns, widthPerColumn int) {
	numColumns = totalDays

	tableBorderOverhead := 1 + (numColumns * 3)
	availableContentWidth := termWidth - tableBorderOverhead

	if availableContentWidth > 0 {
		widthPerColumn = availableContentWidth / numColumns
		for widthPerColumn < minComfortableColumnWidth && numColumns > 1 {
			numColumns--
			tableBorderOverhead = 1 + (numColumns * 3)
			availableContentWidth = termWidth - tableBorderOverhead
			if availableContentWidth > 0 {
				widthPerColumn = availableContentWidth / numColumns
			}
		}
	}

	if numColumns < 1 {
		numColumns = 1
	}

	finalBorderOverhead := 1 + (numColumns * 3)
	finalContentWidth := termWidth - finalBorderOverhead
	widthPerColumn = 60
	if finalContentWidth > 0 && numColumns > 0 {
		widthPerColumn = finalContentWidth / numColumns
	}

	return numColumns, widthPerColumn
}

func truncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func renderHorizontalCalendar(dayGroups map[string][]CalendarItem, cfg CalendarToolConfig, now time.Time, start time.Time, layout calendarLayout) error {
	allHeaders := make([]string, 0)
	allDayColumns := make([][]string, 0)
	maxRows := 0

	for d := 0; d < layout.totalDays; d++ {
		currentDay := start.AddDate(0, 0, d)
		dayKey := currentDay.Format("2006-01-02")
		dayItems := dayGroups[dayKey]

		allHeaders = append(allHeaders, currentDay.Format("Mon 01/02"))

		content := buildDayContent(dayItems, now, cfg, layout.color, layout.widthPerColumn)
		allDayColumns = append(allDayColumns, content)

		if len(content) > maxRows {
			maxRows = len(content)
		}
	}

	for sectionStart := 0; sectionStart < layout.totalDays; sectionStart += layout.numColumns {
		sectionEnd := sectionStart + layout.numColumns
		if sectionEnd > layout.totalDays {
			sectionEnd = layout.totalDays
		}

		table := tablewriter.NewWriter(os.Stdout)

		sectionHeaders := allHeaders[sectionStart:sectionEnd]
		headerInterfaces := make([]interface{}, len(sectionHeaders))
		for i, h := range sectionHeaders {
			headerInterfaces[i] = h
		}
		table.Header(headerInterfaces...)

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
			table.Append(rowData)
		}
		table.Render()
	}

	return nil
}

func buildDayContent(dayItems []CalendarItem, now time.Time, cfg CalendarToolConfig, color func(string) string, widthPerColumn int) []string {
	if len(dayItems) == 0 {
		return []string{color(ColorGreen) + "No releases" + color(ColorReset)}
	}

	sortedItems := slices.Clone(dayItems)

	sort.Slice(sortedItems, func(i, j int) bool {
		if !sortedItems[i].AirTime.Equal(sortedItems[j].AirTime) {
			return sortedItems[i].AirTime.Before(sortedItems[j].AirTime)
		}
		if sortedItems[i].Type != sortedItems[j].Type {
			return sortedItems[i].Type == "episode"
		}
		if sortedItems[i].Type == "episode" {
			if sortedItems[i].Season != sortedItems[j].Season {
				return sortedItems[i].Season < sortedItems[j].Season
			}
			return sortedItems[i].Episode < sortedItems[j].Episode
		}
		return false
	})

	content := make([]string, 0)
	i := 0
	for i < len(sortedItems) {
		item := sortedItems[i]

		if item.Type == "movie" {
			content = append(content, formatMovie(item, now, cfg, color, widthPerColumn))
			i++
			continue
		}

		showStart := i
		showEnd := i + 1
		for showEnd < len(sortedItems) &&
			sortedItems[showEnd].Type == "episode" &&
			sortedItems[showEnd].ShowTitle == item.ShowTitle {
			showEnd++
		}
		consecutiveCount := showEnd - showStart

		if consecutiveCount > maxDisplayPerShow {
			for j := showStart; j < showStart+maxDisplayPerShow; j++ {
				content = append(content, formatEpisode(sortedItems[j], now, cfg, color, widthPerColumn))
			}
			remaining := consecutiveCount - maxDisplayPerShow
			content = append(content, color(ColorCyan)+fmt.Sprintf("  + %d more episodes", remaining)+color(ColorReset))
		} else {
			for j := showStart; j < showEnd; j++ {
				content = append(content, formatEpisode(sortedItems[j], now, cfg, color, widthPerColumn))
			}
		}

		i = showEnd
	}

	return content
}

func formatEpisode(ep CalendarItem, now time.Time, cfg CalendarToolConfig, color func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(ep, now, cfg.NoColor)
	timeStr := ep.AirTime.Format("15:04")

	showTitle := ep.ShowTitle
	episodeTitle := ep.Title

	fixedCharsLine1 := len(timeStr) + 2 + 2 + 2 + 2 + 4 // "HH:MM  " + " - S##E##"
	fixedCharsLine1 = 16
	maxShowLen := widthPerColumn - fixedCharsLine1
	if maxShowLen < minShowTitleLength {
		maxShowLen = minShowTitleLength
	}

	if len(showTitle) > maxShowLen {
		showTitle = showTitle[:maxShowLen-3] + "..."
	}

	indentSpaces := strings.Repeat(" ", len(timeStr)+2)
	maxEpisodeLen := widthPerColumn - len(indentSpaces)
	if maxEpisodeLen < minEpisodeTitleLength {
		maxEpisodeLen = minEpisodeTitleLength
	}

	if len(episodeTitle) > maxEpisodeLen {
		episodeTitle = episodeTitle[:maxEpisodeLen-3] + "..."
	}

	line1 := fmt.Sprintf("%s %s%s - S%02dE%02d%s",
		timeStr, statusColor, showTitle, ep.Season, ep.Episode, color(ColorReset))
	line2 := fmt.Sprintf("%s%s%s%s",
		indentSpaces, statusColor, episodeTitle, color(ColorReset))

	return line1 + "\n" + line2
}

func formatMovie(movie CalendarItem, now time.Time, cfg CalendarToolConfig, color func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(movie, now, cfg.NoColor)
	timeStr := movie.AirTime.Format("15:04")

	title := movie.Title

	// Format: "HH:MM TITLE (YYYY)"
	// Fixed chars: time(5) + space(1) + " ("(2) + year(4) + ")"(1) = len(timeStr) + 8
	fixedChars := len(timeStr) + 8
	maxTitleLen := widthPerColumn - fixedChars

	if maxTitleLen < minMovieTitleLength {
		maxTitleLen = minMovieTitleLength
	}

	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	return fmt.Sprintf("%s %s%s (%d)%s",
		timeStr, statusColor, title, movie.Year, color(ColorReset))
}

func displayVertical(items []CalendarItem, cfg CalendarToolConfig, now time.Time, start time.Time) error {
	color := func(code string) string {
		if cfg.NoColor {
			return ""
		}
		return code
	}

	dayGroups := make(map[string][]CalendarItem)
	for _, item := range items {
		dayKey := item.AirTime.Format("2006-01-02")
		dayGroups[dayKey] = append(dayGroups[dayKey], item)
	}

	totalDaysToDisplay := cfg.Days + cfg.DaysPast
	for d := 0; d < totalDaysToDisplay; d++ {
		currentDay := start.AddDate(0, 0, d)
		dayKey := currentDay.Format("2006-01-02")
		dayItems := dayGroups[dayKey]

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

		showEpisodes := make(map[string][]CalendarItem)
		var movieItems []CalendarItem

		for _, item := range dayItems {
			if item.Type == "episode" {
				showEpisodes[item.ShowTitle] = append(showEpisodes[item.ShowTitle], item)
			} else {
				movieItems = append(movieItems, item)
			}
		}

		for show, episodes := range showEpisodes {
			for i, ep := range episodes {
				if i >= maxDisplayPerShowVertical {
					remaining := len(episodes) - maxDisplayPerShowVertical
					fmt.Printf("  %s+ %d more episodes%s\n",
						color(ColorCyan), remaining, color(ColorReset))
					break
				}

				statusColor := getStatusColor(ep, now, cfg.NoColor)
				timeStr := ep.AirTime.Format("15:04")

				fmt.Printf("  %s%s%s %s%s - S%02dE%02d%s\n",
					color(ColorBold), timeStr, color(ColorReset),
					statusColor, show, ep.Season, ep.Episode, color(ColorReset))
				fmt.Printf("         %s%s%s\n",
					statusColor, ep.Title, color(ColorReset))
			}
		}

		for _, movie := range movieItems {
			statusColor := getStatusColor(movie, now, cfg.NoColor)
			timeStr := movie.AirTime.Format("15:04")

			fmt.Printf("  %s%s%s %s%s (%d)%s\n",
				color(ColorBold), timeStr, color(ColorReset),
				statusColor, movie.Title, movie.Year, color(ColorReset))
		}
	}

	displaySummary(items, now, color)

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
