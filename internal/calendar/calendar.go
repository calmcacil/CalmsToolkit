package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	httpclient "github.com/calmcacil/CalmsToolkit/internal/http"
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

// SonarrEpisode represents a Sonarr calendar episode.
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

// Series represents a TV series from Sonarr.
type Series struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SeasonCount int    `json:"seasonCount"`
}

// RadarrMovie represents a movie from the Radarr calendar.
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

// QueueResponse is the wrapper for download queue responses from Sonarr/Radarr.
type QueueResponse struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// QueueItem represents a single item in the download queue.
type QueueItem struct {
	ID             int             `json:"id"`
	Status         string          `json:"status"`
	TrackedState   string          `json:"trackedDownloadState"`
	StatusMessages []StatusMessage `json:"statusMessages"`
}

// StatusMessage describes a queue item status message.
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// CalendarItem is a unified display item representing an episode or movie.
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

// ToolConfig holds configuration for the media calendar tool.
type ToolConfig struct {
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
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

// QueueIssue represents a warning about a queue item needing intervention.
type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

// Summary provides an overview of calendar items.
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

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
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

// Run executes the media calendar tool.
func Run(cfg ToolConfig) {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	if cfg.WatchMode {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for {
			fmt.Print(colors.ClearScreen + colors.HomeCursor)
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

func displayCalendar(ctx context.Context, cfg ToolConfig) error {
	items, queueIssues, err := aggregateCalendar(ctx, cfg)
	if err != nil {
		return err
	}

	if cfg.JSONOutput {
		return displayJSON(items, queueIssues, cfg)
	}

	return displayTerminal(items, queueIssues, cfg)
}

func aggregateCalendar(ctx context.Context, cfg ToolConfig) ([]CalendarItem, []QueueIssue, error) {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)

	client := httpclient.NewTransportClient(cfg.Timeout)

	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)
	var mu sync.Mutex
	var qMu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)

	for _, inst := range cfg.SonarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchSonarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenEpisodes)
		})
	}

	for _, inst := range cfg.RadarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchRadarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenMovies)
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AirTime.Before(items[j].AirTime)
	})

	return items, queueIssues, nil
}

func fetchSonarrInstance(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
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

func fetchRadarrInstance(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
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

func fetchSonarrCalendar(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var episodes []SonarrEpisode
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &episodes); err != nil {
		return nil, err
	}
	return episodes, nil
}

func fetchRadarrCalendar(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var movies []RadarrMovie
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func fetchQueue(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, debug bool) (*QueueResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v3/queue?pageSize=1", inst.URL)

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Queue %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var queue QueueResponse
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

func displayJSON(items []CalendarItem, queueIssues []QueueIssue, cfg ToolConfig) error {
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

func displayTerminal(items []CalendarItem, queueIssues []QueueIssue, cfg ToolConfig) error {
	clr := func(code string) string {
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
			clr(colors.Bold), clr(colors.Red), totalIssues, clr(colors.Reset))
		for _, issue := range queueIssues {
			fmt.Printf("%s→ %s: %s%s\n",
				clr(colors.Yellow), issue.ServiceName, issue.URL, clr(colors.Reset))
		}
		fmt.Println()
	}

	if !cfg.NoBanner {
		fmt.Printf("%s%s=== Media Calendar ===%s\n\n",
			clr(colors.Bold), clr(colors.Cyan), clr(colors.Reset))
	}

	items = applyFilters(items, cfg)

	if len(items) == 0 {
		fmt.Printf("%sNo items match the current filters.%s\n",
			clr(colors.Green), clr(colors.Reset))
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
		color:          clr,
	}
	if err := renderHorizontalCalendar(dayGroups, cfg, now, start, layout); err != nil {
		return err
	}

	displaySummary(items, now, clr)

	return nil
}

func applyFilters(items []CalendarItem, cfg ToolConfig) []CalendarItem {
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

func displaySummary(items []CalendarItem, now time.Time, clr func(string) string) {
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
		clr(colors.Bold), clr(colors.Cyan), len(items), clr(colors.Reset), episodes, movies)
	fmt.Printf("%s%d available%s, %s%d missing%s, %d upcoming\n",
		clr(colors.Green), available, clr(colors.Reset),
		clr(colors.Red), missing, clr(colors.Reset),
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

func renderHorizontalCalendar(dayGroups map[string][]CalendarItem, cfg ToolConfig, now time.Time, start time.Time, layout calendarLayout) error {
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

func buildDayContent(dayItems []CalendarItem, now time.Time, cfg ToolConfig, clr func(string) string, widthPerColumn int) []string {
	if len(dayItems) == 0 {
		return []string{clr(colors.Green) + "No releases" + clr(colors.Reset)}
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
			content = append(content, formatMovie(item, now, cfg, clr, widthPerColumn))
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
				content = append(content, formatEpisode(sortedItems[j], now, cfg, clr, widthPerColumn))
			}
			remaining := consecutiveCount - maxDisplayPerShow
			content = append(content, clr(colors.Cyan)+fmt.Sprintf("  + %d more episodes", remaining)+clr(colors.Reset))
		} else {
			for j := showStart; j < showEnd; j++ {
				content = append(content, formatEpisode(sortedItems[j], now, cfg, clr, widthPerColumn))
			}
		}

		i = showEnd
	}

	return content
}

func formatEpisode(ep CalendarItem, now time.Time, cfg ToolConfig, clr func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(ep, now, cfg.NoColor)
	timeStr := ep.AirTime.Format("15:04")

	showTitle := ep.ShowTitle
	episodeTitle := ep.Title

	fixedCharsLine1 := len(timeStr) + 2 + 2 + 2 + 2 + 4
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
		timeStr, statusColor, showTitle, ep.Season, ep.Episode, clr(colors.Reset))
	line2 := fmt.Sprintf("%s%s%s%s",
		indentSpaces, statusColor, episodeTitle, clr(colors.Reset))

	return line1 + "\n" + line2
}

func formatMovie(movie CalendarItem, now time.Time, cfg ToolConfig, clr func(string) string, widthPerColumn int) string {
	statusColor := getStatusColor(movie, now, cfg.NoColor)
	timeStr := movie.AirTime.Format("15:04")

	title := movie.Title

	fixedChars := len(timeStr) + 8
	maxTitleLen := widthPerColumn - fixedChars

	if maxTitleLen < minMovieTitleLength {
		maxTitleLen = minMovieTitleLength
	}

	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	return fmt.Sprintf("%s %s%s (%d)%s",
		timeStr, statusColor, title, movie.Year, clr(colors.Reset))
}

func displayVertical(items []CalendarItem, cfg ToolConfig, now time.Time, start time.Time) error {
	clr := func(code string) string {
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
			clr(colors.Bold), clr(colors.Cyan),
			currentDay.Format("Mon 01/02"),
			clr(colors.Reset))
		fmt.Printf("%s%s%s%s\n",
			clr(colors.Bold), clr(colors.Cyan),
			strings.Repeat("━", 40),
			clr(colors.Reset))

		if len(dayItems) == 0 {
			fmt.Printf("%sNo scheduled releases%s\n",
				clr(colors.Green), clr(colors.Reset))
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

		showNames := make([]string, 0, len(showEpisodes))
			for name := range showEpisodes {
				showNames = append(showNames, name)
			}
			sort.Strings(showNames)

			for _, show := range showNames {
				episodes := showEpisodes[show]
				for i, ep := range episodes {
				if i >= maxDisplayPerShowVertical {
					remaining := len(episodes) - maxDisplayPerShowVertical
					fmt.Printf("  %s+ %d more episodes%s\n",
						clr(colors.Cyan), remaining, clr(colors.Reset))
					break
				}

				statusColor := getStatusColor(ep, now, cfg.NoColor)
				timeStr := ep.AirTime.Format("15:04")

				fmt.Printf("  %s%s%s %s%s - S%02dE%02d%s\n",
					clr(colors.Bold), timeStr, clr(colors.Reset),
					statusColor, show, ep.Season, ep.Episode, clr(colors.Reset))
				fmt.Printf("         %s%s%s\n",
					statusColor, ep.Title, clr(colors.Reset))
			}
		}

		for _, movie := range movieItems {
			statusColor := getStatusColor(movie, now, cfg.NoColor)
			timeStr := movie.AirTime.Format("15:04")

			fmt.Printf("  %s%s%s %s%s (%d)%s\n",
				clr(colors.Bold), timeStr, clr(colors.Reset),
				statusColor, movie.Title, movie.Year, clr(colors.Reset))
		}
	}

	displaySummary(items, now, clr)

	return nil
}

func getStatusColor(item CalendarItem, now time.Time, noColor bool) string {
	if noColor {
		return ""
	}

	if item.HasFile {
		return colors.Green
	}

	if item.AirTime.Before(now) {
		return colors.Red
	}

	if item.IsPremiere {
		return colors.Orange
	}

	return colors.Blue
}
