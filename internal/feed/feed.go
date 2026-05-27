package feed

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	httpclient "github.com/calmcacil/CalmsToolkit/internal/http"
)

// ToolConfig holds configuration for the Arr event feed tool.
type ToolConfig struct {
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
	PollInterval    time.Duration
	HistoryWindow   time.Duration
	Timeout         time.Duration
	NoColor         bool
	Theme           string
	JSON            bool
	Watch           bool
	ShowGrabbed     bool
	ShowImported    bool
	ShowFailed      bool
	ShowDeleted     bool
	ShowIgnored     bool
	ShowSubtitles   bool
	MaxEvents       int
	Quiet           bool
}

// HistoryEvent represents a normalized Sonarr/Radarr history event.
type HistoryEvent struct {
	Server       string    `json:"server"`
	When         time.Time `json:"when"`
	Action       string    `json:"action"`
	Title        string    `json:"title"`
	Episode      string    `json:"episode,omitempty"`
	EpisodeTitle string    `json:"episodeTitle,omitempty"`
	Quality      string    `json:"quality"`
	Formats      []string  `json:"formats,omitempty"`
	SourceTitle  string    `json:"sourceTitle,omitempty"`
	EventType    string    `json:"eventType"`
	ID           int       `json:"id"`
	FileID       int       `json:"fileId,omitempty"`
	Subtitles    string    `json:"subtitles,omitempty"`
}

// SonarrHistory is the raw Sonarr history API response entry.
type SonarrHistory struct {
	EpisodeID     int                    `json:"episodeId"`
	SeriesID      int                    `json:"seriesId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       SonarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Episode       *SonarrEpisode         `json:"episode,omitempty"`
	Series        *SonarrSeries          `json:"series,omitempty"`
	ID            int                    `json:"id"`
}

// SonarrQuality wraps the quality info in Sonarr history.
type SonarrQuality struct {
	Quality       SonarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

// SonarrQualityItem is a single quality definition.
type SonarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

// CustomFormat represents a Sonarr/Radarr custom format.
type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// QualityRevision represents a quality revision entry.
type QualityRevision struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

// SonarrEpisodeFileResponse is the Sonarr episode file API response.
type SonarrEpisodeFileResponse struct {
	ID        int              `json:"id"`
	MediaInfo *SonarrMediaInfo `json:"mediaInfo,omitempty"`
}

// RadarrMovieFileResponse is the Radarr movie file API response.
type RadarrMovieFileResponse struct {
	ID        int              `json:"id"`
	MediaInfo *RadarrMediaInfo `json:"mediaInfo,omitempty"`
}

// SonarrMediaInfo holds media info including subtitle languages (V3 API).
type SonarrMediaInfo struct {
	Subtitles string `json:"subtitles"`
}

// RadarrMediaInfo holds media info including subtitle languages (V3 API).
type RadarrMediaInfo struct {
	Subtitles string `json:"subtitles"`
}

// SonarrEpisode is an episode reference in Sonarr history.
type SonarrEpisode struct {
	ID            int    `json:"id"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

// SonarrSeries is a series reference in Sonarr history.
type SonarrSeries struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// RadarrHistory is the raw Radarr history API response entry.
type RadarrHistory struct {
	MovieID       int                    `json:"movieId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       RadarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Movie         *RadarrMovie           `json:"movie,omitempty"`
	ID            int                    `json:"id"`
}

// RadarrQuality wraps the quality info in Radarr history.
type RadarrQuality struct {
	Quality       RadarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

// RadarrQualityItem is a single quality definition.
type RadarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

// RadarrMovie is a movie reference in Radarr history.
type RadarrMovie struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.PollInterval = 5 * time.Second
		cfg.HistoryWindow = 1 * time.Hour
		cfg.MaxEvents = 50
		cfg.ShowGrabbed = true
		cfg.ShowImported = true
		cfg.ShowFailed = true
		return cfg
	}

	dur, _ := time.ParseDuration(tk.General.Timeout)
	if dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor
	cfg.Theme = tk.General.Theme
	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)

	dur, _ = time.ParseDuration(tk.ArrFeed.PollInterval)
	if dur > 0 {
		cfg.PollInterval = dur
	} else {
		cfg.PollInterval = 5 * time.Second
	}
	dur, _ = time.ParseDuration(tk.ArrFeed.HistoryWindow)
	if dur > 0 {
		cfg.HistoryWindow = dur
	} else {
		cfg.HistoryWindow = 1 * time.Hour
	}

	cfg.MaxEvents = tk.ArrFeed.MaxEvents
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 50
	}
	if cfg.MaxEvents > 100 {
		cfg.MaxEvents = 100
	}
	cfg.ShowGrabbed = tk.ArrFeed.ShowGrabbed
	cfg.ShowImported = tk.ArrFeed.ShowImported
	cfg.ShowFailed = tk.ArrFeed.ShowFailed
	cfg.ShowDeleted = tk.ArrFeed.ShowDeleted
	cfg.ShowIgnored = tk.ArrFeed.ShowIgnored
	cfg.ShowSubtitles = tk.ArrFeed.ShowSubtitles
	return cfg
}

// Run executes the Arr event feed tool.
func Run(cfg ToolConfig) {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	client := httpclient.NewClient(cfg.Timeout)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := colors.GetPalette(cfg.Theme)

	if cfg.Watch {
		runWatchMode(ctx, cfg, client, p)
	} else {
		runSingleMode(ctx, cfg, client, p)
	}
}

func runSingleMode(ctx context.Context, cfg ToolConfig, client *httpclient.Client, p *colors.Palette) {
	since := time.Now().Add(-cfg.HistoryWindow)
	events, err := fetchAllHistory(ctx, client, cfg, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	events = filterEvents(events, cfg)

	if cfg.JSON {
		renderJSON(events)
	} else {
		renderTable(events, cfg, p)
	}
}

func runWatchMode(ctx context.Context, cfg ToolConfig, client *httpclient.Client, p *colors.Palette) {
	if !cfg.JSON {
		fmt.Print(colors.HideCursor)
		defer fmt.Print(colors.ShowCursor)
	}

	eventCache := make([]HistoryEvent, 0, 100)
	lastFetch := time.Now().Add(-cfg.HistoryWindow)

	for {
		newEvents, err := fetchAllHistory(ctx, client, cfg, lastFetch)
		if err != nil {
			if !cfg.JSON {
				fmt.Print(colors.HomeCursor)
				clr := getColorFunc(cfg, p)
				fmt.Printf("%sERROR: %v%s\n", clr(p.Error), err, clr(p.Reset))
				fmt.Printf("Retrying in %v...\n", cfg.PollInterval)
				fmt.Print(colors.EraseDown)
			}
		} else {
			for _, event := range newEvents {
				eventCache = append(eventCache, event)
			}

			if len(eventCache) > 100 {
				eventCache = eventCache[len(eventCache)-100:]
			}

			filteredEvents := filterEvents(eventCache, cfg)

			if len(filteredEvents) > cfg.MaxEvents {
				filteredEvents = filteredEvents[:cfg.MaxEvents]
			}

			if cfg.JSON {
				renderJSON(filteredEvents)
			} else {
				fmt.Print(colors.HomeCursor)
				renderTable(filteredEvents, cfg, p)
				fmt.Print(colors.EraseDown)
			}

			if len(newEvents) > 0 {
				lastFetch = time.Now()
			}
		}

		select {
		case <-ctx.Done():
			if !cfg.JSON {
				fmt.Print(colors.ShowCursor)
			}
			return
		case <-time.After(cfg.PollInterval):
		}
	}
}

func fetchAllHistory(ctx context.Context, client *httpclient.Client, cfg ToolConfig, since time.Time) ([]HistoryEvent, error) {
	var wg sync.WaitGroup
	eventsChan := make(chan []HistoryEvent, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))
	errorsChan := make(chan error, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))

	for _, inst := range cfg.SonarrInstances {
		wg.Add(1)
		go func(inst config.ArrInstance) {
			defer wg.Done()
			events, err := fetchSonarrHistory(ctx, client, inst, since, cfg.ShowSubtitles)
			if err != nil {
				errorsChan <- fmt.Errorf("Sonarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	for _, inst := range cfg.RadarrInstances {
		wg.Add(1)
		go func(inst config.ArrInstance) {
			defer wg.Done()
			events, err := fetchRadarrHistory(ctx, client, inst, since, cfg.ShowSubtitles)
			if err != nil {
				errorsChan <- fmt.Errorf("Radarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	wg.Wait()
	close(eventsChan)
	close(errorsChan)

	var allEvents []HistoryEvent
	for events := range eventsChan {
		allEvents = append(allEvents, events...)
	}

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(allEvents) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all instances failed: %v", errors)
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].When.After(allEvents[j].When)
	})

	return allEvents, nil
}

func fetchSonarrHistory(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, since time.Time, showSubtitles bool) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeEpisode=true&includeSeries=true", inst.URL, sinceStr)

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", status, string(body))
	}

	var wrapper struct {
		Records []SonarrHistory `json:"records"`
	}
	var history []SonarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, sonarrToHistoryEvent(h))
	}

	if showSubtitles {
		enrichSonarrSubtitles(ctx, client, inst, events)
	}

	return events, nil
}

func enrichSonarrSubtitles(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, events []HistoryEvent) {
	var ids []int
	seen := make(map[int]bool)
	for _, ev := range events {
		if ev.FileID > 0 && !seen[ev.FileID] {
			seen[ev.FileID] = true
			ids = append(ids, ev.FileID)
		}
	}
	if len(ids) == 0 {
		return
	}

	endpoint := fmt.Sprintf("%s/api/v3/episodefile?", inst.URL)
	for i, fid := range ids {
		if i > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("episodeFileIds=%d", fid)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil || status != http.StatusOK {
		return
	}

	var files []SonarrEpisodeFileResponse
	if err := json.Unmarshal(body, &files); err != nil {
		return
	}

	subMap := make(map[int]string)
	for _, f := range files {
		if f.MediaInfo != nil && f.MediaInfo.Subtitles != "" {
			subMap[f.ID] = f.MediaInfo.Subtitles
		}
	}

	for i := range events {
		if subs, ok := subMap[events[i].FileID]; ok {
			events[i].Subtitles = subs
		}
	}
}

func fetchRadarrHistory(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, since time.Time, showSubtitles bool) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeMovie=true", inst.URL, sinceStr)

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", status, string(body))
	}

	var wrapper struct {
		Records []RadarrHistory `json:"records"`
	}
	var history []RadarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, radarrToHistoryEvent(h))
	}

	if showSubtitles {
		enrichRadarrSubtitles(ctx, client, inst, events)
	}

	return events, nil
}

func enrichRadarrSubtitles(ctx context.Context, client *httpclient.Client, inst config.ArrInstance, events []HistoryEvent) {
	var ids []int
	seen := make(map[int]bool)
	for _, ev := range events {
		if ev.FileID > 0 && !seen[ev.FileID] {
			seen[ev.FileID] = true
			ids = append(ids, ev.FileID)
		}
	}
	if len(ids) == 0 {
		return
	}

	endpoint := fmt.Sprintf("%s/api/v3/moviefile?", inst.URL)
	for i, fid := range ids {
		if i > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("movieFileIds=%d", fid)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil || status != http.StatusOK {
		return
	}

	var files []RadarrMovieFileResponse
	if err := json.Unmarshal(body, &files); err != nil {
		return
	}

	subMap := make(map[int]string)
	for _, f := range files {
		if f.MediaInfo != nil && f.MediaInfo.Subtitles != "" {
			subMap[f.ID] = f.MediaInfo.Subtitles
		}
	}

	for i := range events {
		if subs, ok := subMap[events[i].FileID]; ok {
			events[i].Subtitles = subs
		}
	}
}

func sonarrToHistoryEvent(sh SonarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, sh.Date)

	event := HistoryEvent{
		Server:      "sonarr",
		When:        when,
		Action:      mapSonarrEventType(sh.EventType),
		Quality:     sh.Quality.Quality.Name,
		SourceTitle: sh.SourceTitle,
		EventType:   sh.EventType,
		ID:          sh.ID,
	}

	if sh.Data != nil {
		for _, key := range []string{"fileId", "FileId"} {
			if fileIDVal, ok := sh.Data[key]; ok {
				if fid, err := parseInt(fmt.Sprintf("%v", fileIDVal)); err == nil {
					event.FileID = fid
				}
				break
			}
		}
	}

	if sh.Series != nil {
		event.Title = sh.Series.Title
	}

	if sh.Episode != nil {
		event.Episode = formatEpisode(sh.Episode.SeasonNumber, sh.Episode.EpisodeNumber)
		event.EpisodeTitle = sh.Episode.Title
	}

	var cf []CustomFormat
	if len(sh.CustomFormats) > 0 {
		cf = sh.CustomFormats
	} else {
		cf = sh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}

func radarrToHistoryEvent(rh RadarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, rh.Date)

	event := HistoryEvent{
		Server:      "radarr",
		When:        when,
		Action:      mapRadarrEventType(rh.EventType),
		Quality:     rh.Quality.Quality.Name,
		SourceTitle: rh.SourceTitle,
		EventType:   rh.EventType,
		ID:          rh.ID,
	}

	if rh.Data != nil {
		for _, key := range []string{"fileId", "FileId"} {
			if fileIDVal, ok := rh.Data[key]; ok {
				if fid, err := parseInt(fmt.Sprintf("%v", fileIDVal)); err == nil {
					event.FileID = fid
				}
				break
			}
		}
	}

	if rh.Movie != nil {
		event.Title = rh.Movie.Title
		if rh.Movie.Year > 0 {
			event.Title = fmt.Sprintf("%s (%d)", rh.Movie.Title, rh.Movie.Year)
		}
	}

	var cf []CustomFormat
	if len(rh.CustomFormats) > 0 {
		cf = rh.CustomFormats
	} else {
		cf = rh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}

func mapSonarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "episodeFileDeleted":
		return "Deleted"
	case "episodeFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	case "seriesFolderImported":
		return "Bulk Import"
	default:
		return eventType
	}
}

func mapRadarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "movieFileDeleted":
		return "Deleted"
	case "movieFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	default:
		return eventType
	}
}

func formatEpisode(season, episode int) string {
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	return t.Format("2006-01-02 15:04")
}

func filterEvents(events []HistoryEvent, cfg ToolConfig) []HistoryEvent {
	filtered := make([]HistoryEvent, 0, len(events))

	for _, event := range events {
		switch event.Action {
		case "Grabbed":
			if !cfg.ShowGrabbed {
				continue
			}
		case "Imported", "Bulk Import":
			if !cfg.ShowImported {
				continue
			}
		case "Failed":
			if !cfg.ShowFailed {
				continue
			}
		case "Deleted":
			if !cfg.ShowDeleted {
				continue
			}
		case "Ignored":
			if !cfg.ShowIgnored {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func visibleLen(s string) int {
	return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
}

func padRight(s string, width int) string {
	v := visibleLen(s)
	if v >= width {
		return s
	}
	return s + strings.Repeat(" ", width-v)
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func boxTop(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "┌")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┬")
		}
	}
	fmt.Fprint(bw, "┐\n")
}

func boxSep(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "├")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┼")
		}
	}
	fmt.Fprint(bw, "┤\n")
}

func boxBottom(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "└")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┴")
		}
	}
	fmt.Fprint(bw, "┘\n")
}

func renderTable(events []HistoryEvent, cfg ToolConfig, p *colors.Palette) {
	color := getColorFunc(cfg, p)

	termWidth := 120
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	props := []int{12, 8, 28, 8, 20, 12, 15}
	headers := []string{"When", "Action", "Series/Movie", "Episode", "Episode Title", "Quality", "Formats"}
	if cfg.ShowSubtitles {
		props = append(props, 10)
		headers = append(headers, "Subtitles")
	}

	totalCols := len(props)
	sumProp := 0
	for _, p := range props {
		sumProp += p
	}
	availWidth := termWidth - totalCols - 1

	colWidths := make([]int, totalCols)
	for i, p := range props {
		cw := availWidth * p / sumProp
		if cw < 5 {
			cw = 5
		}
		colWidths[i] = cw
	}

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	if len(events) == 0 {
		boxTop(bw, colWidths, cfg.ShowSubtitles)
		fmt.Fprint(bw, color(p.Bold))
		fmt.Fprint(bw, "│")
		totalW := 0
		for _, w := range colWidths {
			totalW += w
		}
		mid := (totalW - len("No events found")) / 2
		if mid < 0 {
			mid = 0
		}
		fmt.Fprint(bw, strings.Repeat(" ", mid))
		fmt.Fprint(bw, "No events found")
		fmt.Fprint(bw, padRight("", totalW-mid-len("No events found")))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, color(p.Reset))
		fmt.Fprintln(bw)
		boxBottom(bw, colWidths, cfg.ShowSubtitles)
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return
	}

	boxTop(bw, colWidths, cfg.ShowSubtitles)
	fmt.Fprint(bw, "│")
	for i, h := range headers {
		fmt.Fprint(bw, color(p.Bold))
		fmt.Fprint(bw, center(h, colWidths[i]))
		fmt.Fprint(bw, color(p.Reset))
		fmt.Fprint(bw, "│")
	}
	fmt.Fprintln(bw)

	boxSep(bw, colWidths, cfg.ShowSubtitles)

	for _, event := range events {
		actionColor := getActionColor(event.Action, p)
		timeStr := formatRelativeTime(event.When)
		title := truncateWithEllipsis(event.Title, colWidths[2])
		epiTitle := truncateWithEllipsis(event.EpisodeTitle, colWidths[4])
		quality := truncateWithEllipsis(event.Quality, colWidths[5])
		formats := truncateWithEllipsis(strings.Join(event.Formats, ", "), colWidths[6])

		vals := []string{
			center(timeStr, colWidths[0]),
			center(event.Action, colWidths[1]),
			center(title, colWidths[2]),
			center(event.Episode, colWidths[3]),
			center(epiTitle, colWidths[4]),
			center(quality, colWidths[5]),
			center(formats, colWidths[6]),
		}
		if cfg.ShowSubtitles {
			subs := truncateWithEllipsis(subtitlesDisplay(event.Subtitles), colWidths[7])
			vals = append(vals, center(subs, colWidths[7]))
		}

		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, vals[0])
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, color(actionColor))
		fmt.Fprint(bw, vals[1])
		fmt.Fprint(bw, color(p.Reset))
		fmt.Fprint(bw, "│")
		for i := 2; i < len(vals); i++ {
			fmt.Fprint(bw, vals[i])
			fmt.Fprint(bw, "│")
		}
		fmt.Fprintln(bw)
	}

	boxBottom(bw, colWidths, cfg.ShowSubtitles)

	fmt.Fprintf(bw, "\n%sTotal events: %d%s\n", color(p.Bold), len(events), color(p.Reset))

	bw.Flush()
	os.Stdout.Write(buf.Bytes())
}

func renderJSON(events []HistoryEvent) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(events)
}

func getActionColor(action string, p *colors.Palette) string {
	switch action {
	case "Imported", "Bulk Import":
		return p.Success
	case "Grabbed":
		return p.Grabbed
	case "Failed":
		return p.Error
	case "Deleted":
		return p.Warning
	case "Ignored":
		return p.Subdued
	case "Renamed":
		return p.Renamed
	default:
		return p.Reset
	}
}

func getColorFunc(cfg ToolConfig, p *colors.Palette) func(string) string {
	if cfg.NoColor || cfg.JSON {
		return func(s string) string { return "" }
	}
	return func(s string) string { return s }
}

func subtitlesDisplay(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func center(s string, width int) string {
	v := visibleLen(s)
	if v >= width {
		runes := []rune(s)
		if len(runes) > width {
			return string(runes[:width])
		}
		return s
	}
	padding := width - v
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
