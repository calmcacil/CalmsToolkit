//go:build mediacalendar

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchSonarrCalendar(t *testing.T) {
	mockResponse := []SonarrEpisode{
		{
			Title:         "Test Episode",
			SeasonNumber:  1,
			EpisodeNumber: 5,
			AirDateUtc:    time.Now(),
			HasFile:       true,
			Series: &Series{
				Title: "Test Show",
				Year:  2024,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header with test-token, got %s", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	inst := ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	ctx := context.Background()
	episodes, err := fetchSonarrCalendar(ctx, client, inst, start, end, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(episodes) != 1 {
		t.Errorf("Got %d episodes, want 1", len(episodes))
	}

	if len(episodes) > 0 {
		if episodes[0].Series.Title != "Test Show" {
			t.Errorf("Series.Title = %v, want %v", episodes[0].Series.Title, "Test Show")
		}
		if episodes[0].EpisodeNumber != 5 {
			t.Errorf("EpisodeNumber = %v, want %v", episodes[0].EpisodeNumber, 5)
		}
		if !episodes[0].HasFile {
			t.Error("Expected HasFile to be true")
		}
	}
}

func TestFetchRadarrCalendar(t *testing.T) {
	mockResponse := []RadarrMovie{
		{
			Title:     "Test Movie",
			Year:      2024,
			InCinemas: "2024-10-16T00:00:00Z",
			HasFile:   false,
			Monitored: true,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header with test-token, got %s", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	inst := ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	ctx := context.Background()
	movies, err := fetchRadarrCalendar(ctx, client, inst, start, end, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(movies) != 1 {
		t.Errorf("Got %d movies, want 1", len(movies))
	}

	if len(movies) > 0 {
		if movies[0].Title != "Test Movie" {
			t.Errorf("Title = %v, want %v", movies[0].Title, "Test Movie")
		}
		if movies[0].Year != 2024 {
			t.Errorf("Year = %v, want %v", movies[0].Year, 2024)
		}
		if movies[0].HasFile {
			t.Error("Expected HasFile to be false")
		}
	}
}

func TestFetchQueue(t *testing.T) {
	mockResponse := QueueResponse{
		TotalRecords: 2,
		Records: []QueueItem{
			{
				Status:       "downloading",
				TrackedState: "ok",
			},
			{
				Status:       "warning",
				TrackedState: "warning",
				StatusMessages: []StatusMessage{
					{
						Title:    "Download warning",
						Messages: []string{"Download client unavailable"},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header with test-token, got %s", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	inst := ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	ctx := context.Background()

	queue, err := fetchQueue(ctx, client, inst, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if queue.TotalRecords != 2 {
		t.Errorf("TotalRecords = %d, want 2", queue.TotalRecords)
	}

	if len(queue.Records) != 2 {
		t.Errorf("Records count = %d, want 2", len(queue.Records))
	}

	if len(queue.Records) > 1 {
		if queue.Records[1].Status != "warning" {
			t.Errorf("Record[1].Status = %v, want %v", queue.Records[1].Status, "warning")
		}
	}
}

func TestCalculateColumnLayout(t *testing.T) {
	tests := []struct {
		name                string
		termWidth           int
		totalDays           int
		expectedColumns     int
		expectedWidthPerCol int
	}{
		{
			name:                "Wide terminal, 7 days",
			termWidth:           160,
			totalDays:           7,
			expectedColumns:     3,
			expectedWidthPerCol: 50,
		},
		{
			name:                "Narrow terminal, 7 days",
			termWidth:           80,
			totalDays:           7,
			expectedColumns:     1,
			expectedWidthPerCol: 76,
		},
		{
			name:                "Very narrow terminal",
			termWidth:           40,
			totalDays:           7,
			expectedColumns:     1,
			expectedWidthPerCol: 36,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, width := calculateColumnLayout(tt.termWidth, tt.totalDays)
			if cols != tt.expectedColumns {
				t.Errorf("columns = %d, want %d", cols, tt.expectedColumns)
			}
			if width != tt.expectedWidthPerCol {
				t.Errorf("widthPerColumn = %d, want %d", width, tt.expectedWidthPerCol)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "Short text, no truncation",
			text:     "Short",
			maxLen:   10,
			expected: "Short",
		},
		{
			name:     "Long text, needs truncation",
			text:     "This is a very long text that needs truncation",
			maxLen:   20,
			expected: "This is a very lo...",
		},
		{
			name:     "Exact length",
			text:     "Exactly",
			maxLen:   7,
			expected: "Exactly",
		},
		{
			name:     "Very short max length",
			text:     "Hello World",
			maxLen:   5,
			expected: "He...",
		},
		{
			name:     "Max length less than 3",
			text:     "Hello World",
			maxLen:   2,
			expected: "He",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateText(tt.text, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestAggregateCalendar(t *testing.T) {
	sonarrResponse := []SonarrEpisode{
		{
			Title:         "Episode 5",
			SeasonNumber:  1,
			EpisodeNumber: 5,
			AirDateUtc:    time.Now().Add(24 * time.Hour),
			HasFile:       false,
			Series: &Series{
				Title: "Test Show",
				Year:  2024,
			},
		},
	}

	radarrResponse := []RadarrMovie{
		{
			Title:     "Test Movie",
			Year:      2024,
			InCinemas: time.Now().Add(48 * time.Hour).UTC().Format("2006-01-02T15:04:05Z"),
			HasFile:   true,
			Monitored: true,
		},
	}

	queueResponse := QueueResponse{
		TotalRecords: 1,
		Records: []QueueItem{
			{
				Status:       "warning",
				TrackedState: "warning",
				StatusMessages: []StatusMessage{
					{
						Title:    "Test error",
						Messages: []string{"Download failed"},
					},
				},
			},
		},
	}

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/calendar") {
			json.NewEncoder(w).Encode(sonarrResponse)
		} else if strings.HasSuffix(r.URL.Path, "/queue") {
			json.NewEncoder(w).Encode(queueResponse)
		}
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/calendar") {
			json.NewEncoder(w).Encode(radarrResponse)
		} else if strings.HasSuffix(r.URL.Path, "/queue") {
			json.NewEncoder(w).Encode(QueueResponse{TotalRecords: 0})
		}
	}))
	defer radarrServer.Close()

	cfg := CalendarToolConfig{
		SonarrInstances: []ArrInstance{
			{Name: "Sonarr", URL: sonarrServer.URL, APIKey: "test-token"},
		},
		RadarrInstances: []ArrInstance{
			{Name: "Radarr", URL: radarrServer.URL, APIKey: "test-token"},
		},
		Days:       7,
		DaysPast:   0,
		Timeout:    5 * time.Second,
		Debug:      false,
	}

	ctx := context.Background()
	items, issues, err := aggregateCalendar(ctx, cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(items) < 2 {
		t.Errorf("Expected at least 2 calendar items, got %d", len(items))
	}

	if len(issues) < 1 {
		t.Errorf("Expected at least 1 queue issue, got %d", len(issues))
	}

	foundEpisode := false
	for _, item := range items {
		if item.ShowTitle == "Test Show" && item.Type == "episode" {
			foundEpisode = true
			break
		}
	}
	if !foundEpisode {
		t.Error("Expected to find Sonarr episode in calendar items")
	}

	foundMovie := false
	for _, item := range items {
		if item.Title == "Test Movie" && item.Type == "movie" {
			foundMovie = true
			break
		}
	}
	if !foundMovie {
		t.Error("Expected to find Radarr movie in calendar items")
	}
}

func TestGetStatusColor(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		item    CalendarItem
		now     time.Time
		noColor bool
	}{
		{
			name: "Downloaded episode",
			item: CalendarItem{
				Type:    "episode",
				HasFile: true,
				AirTime: now.Add(-24 * time.Hour),
			},
			now:     now,
			noColor: true,
		},
		{
			name: "Aired but not downloaded",
			item: CalendarItem{
				Type:    "episode",
				HasFile: false,
				AirTime: now.Add(-24 * time.Hour),
			},
			now:     now,
			noColor: true,
		},
		{
			name: "Future episode",
			item: CalendarItem{
				Type:    "episode",
				HasFile: false,
				AirTime: now.Add(48 * time.Hour),
			},
			now:     now,
			noColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = getStatusColor(tt.item, tt.now, tt.noColor)
		})
	}
}

func TestBuildDayContentSorting(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	cfg := CalendarToolConfig{NoColor: true}

	dayItems := []CalendarItem{
		{
			Type:      "episode",
			ShowTitle: "Alpha Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(16 * time.Hour),
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Zulu Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(14 * time.Hour),
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Beta Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(18 * time.Hour),
			HasFile:   false,
		},
	}

	colorFunc := func(s string) string { return "" }
	content := buildDayContent(dayItems, now, cfg, colorFunc, 80)

	if len(content) < 3 {
		t.Fatalf("Expected at least 3 content lines, got %d", len(content))
	}

	foundZulu := false
	foundAlpha := false
	foundBeta := false

	for i, line := range content {
		if !foundZulu && (strings.Contains(line, "Zulu Show") || strings.Contains(line, "14:00")) {
			foundZulu = true
			if foundAlpha || foundBeta {
				t.Errorf("Found Zulu Show at position %d, but Alpha or Beta was already found", i)
			}
		}
		if !foundAlpha && (strings.Contains(line, "Alpha Show") || strings.Contains(line, "16:00")) {
			foundAlpha = true
			if !foundZulu {
				t.Errorf("Found Alpha Show at position %d before Zulu Show", i)
			}
			if foundBeta {
				t.Errorf("Found Alpha Show at position %d after Beta Show", i)
			}
		}
		if !foundBeta && (strings.Contains(line, "Beta Show") || strings.Contains(line, "18:00")) {
			foundBeta = true
			if !foundZulu || !foundAlpha {
				t.Errorf("Found Beta Show at position %d, but Zulu or Alpha was not found yet", i)
			}
		}
	}

	if !foundZulu || !foundAlpha || !foundBeta {
		t.Errorf("Not all shows were found in content. Zulu: %v, Alpha: %v, Beta: %v", foundZulu, foundAlpha, foundBeta)
	}
}

func TestBuildDayContentTruncation(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	cfg := CalendarToolConfig{NoColor: true}

	dayItems := []CalendarItem{
		{
			Type:      "episode",
			ShowTitle: "Test Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(14 * time.Hour),
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Test Show",
			Title:     "Episode 2",
			Season:    1,
			Episode:   2,
			AirTime:   baseTime.Add(14*time.Hour + 30*time.Minute),
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Test Show",
			Title:     "Episode 3",
			Season:    1,
			Episode:   3,
			AirTime:   baseTime.Add(15 * time.Hour),
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Test Show",
			Title:     "Episode 4",
			Season:    1,
			Episode:   4,
			AirTime:   baseTime.Add(15*time.Hour + 30*time.Minute),
			HasFile:   false,
		},
	}

	colorFunc := func(s string) string { return "" }
	content := buildDayContent(dayItems, now, cfg, colorFunc, 80)

	foundTruncation := false
	episodeCount := 0

	for _, line := range content {
		if strings.Contains(line, "+ 2 more episodes") {
			foundTruncation = true
		}
		if strings.Contains(line, "Episode") {
			episodeCount++
		}
	}

	if !foundTruncation {
		t.Error("Expected truncation message not found")
	}
}

func TestBuildDayContentMixedTypes(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	cfg := CalendarToolConfig{NoColor: true}

	dayItems := []CalendarItem{
		{
			Type:    "movie",
			Title:   "Movie B",
			AirTime: baseTime.Add(17 * time.Hour),
			HasFile: false,
		},
		{
			Type:      "episode",
			ShowTitle: "Show A",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(15 * time.Hour),
			HasFile:   false,
		},
		{
			Type:    "movie",
			Title:   "Movie A",
			AirTime: baseTime.Add(19 * time.Hour),
			HasFile: false,
		},
	}

	colorFunc := func(s string) string { return "" }
	content := buildDayContent(dayItems, now, cfg, colorFunc, 80)

	foundEpisode := false
	foundMovieB := false
	foundMovieA := false

	for i, line := range content {
		if !foundEpisode && strings.Contains(line, "Show A") {
			foundEpisode = true
			if foundMovieB || foundMovieA {
				t.Errorf("Found episode at position %d after a movie", i)
			}
		}
		if !foundMovieB && strings.Contains(line, "Movie B") {
			foundMovieB = true
			if !foundEpisode {
				t.Errorf("Found Movie B at position %d before episode", i)
			}
			if foundMovieA {
				t.Errorf("Found Movie B at position %d after Movie A", i)
			}
		}
		if !foundMovieA && strings.Contains(line, "Movie A") {
			foundMovieA = true
			if !foundEpisode || !foundMovieB {
				t.Errorf("Found Movie A at position %d before episode or Movie B", i)
			}
		}
	}

	if !foundEpisode || !foundMovieB || !foundMovieA {
		t.Errorf("Not all items found. Episode: %v, Movie B: %v, Movie A: %v", foundEpisode, foundMovieB, foundMovieA)
	}
}

func TestBuildCalendarToolConfig(t *testing.T) {
	tk := DefaultToolkitConfig()
	tk.Sonarr = []ArrInstance{
		{Name: "Sonarr HD", URL: "http://sonarr:8989", APIKey: "token1"},
	}
	tk.Radarr = []ArrInstance{
		{Name: "Radarr HD", URL: "http://radarr:7878", APIKey: "token2"},
	}
	tk.MediaCalendar.Days = 7
	tk.MediaCalendar.DaysPast = 1
	tk.MediaCalendar.WatchInterval = 600
	tk.General.Timeout = "30s"

	cfg := BuildCalendarToolConfig(tk)

	if len(cfg.SonarrInstances) != 1 {
		t.Errorf("SonarrInstances = %d, want 1", len(cfg.SonarrInstances))
	}
	if cfg.SonarrInstances[0].Name != "Sonarr HD" {
		t.Errorf("SonarrInstances[0].Name = %q, want %q", cfg.SonarrInstances[0].Name, "Sonarr HD")
	}
	if cfg.Days != 7 {
		t.Errorf("Days = %d, want 7", cfg.Days)
	}
	if cfg.DaysPast != 1 {
		t.Errorf("DaysPast = %d, want 1", cfg.DaysPast)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.WatchSeconds != 600 {
		t.Errorf("WatchSeconds = %d, want 600", cfg.WatchSeconds)
	}
}

func TestBuildCalendarToolConfigNil(t *testing.T) {
	cfg := BuildCalendarToolConfig(nil)

	if cfg.Days != 1 {
		t.Errorf("Days = %d, want 1", cfg.Days)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.Timeout)
	}
	if len(cfg.SonarrInstances) != 0 {
		t.Errorf("Expected no Sonarr instances, got %d", len(cfg.SonarrInstances))
	}
}

func TestCalculateDateRange(t *testing.T) {
	start, end := calculateDateRange(1, 0)
	if end.Sub(start) != 24*time.Hour {
		t.Errorf("Expected 1 day range, got %v", end.Sub(start))
	}

	start, end = calculateDateRange(7, 1)
	if end.Sub(start) != 8*24*time.Hour {
		t.Errorf("Expected 8 day range (7+1), got %v", end.Sub(start))
	}
}

func TestApplyFilters(t *testing.T) {
	now := time.Now()
	items := []CalendarItem{
		{Title: "Available Movie", HasFile: true, Monitored: true, AirTime: now.Add(-24 * time.Hour)},
		{Title: "Missing Movie", HasFile: false, Monitored: true, AirTime: now.Add(-24 * time.Hour)},
		{Title: "Premiere Episode", HasFile: false, IsPremiere: true, Monitored: true, AirTime: now.Add(24 * time.Hour)},
		{Title: "Unmonitored Movie", HasFile: false, Monitored: false, AirTime: now.Add(24 * time.Hour)},
	}

	tests := []struct {
		name     string
		cfg      CalendarToolConfig
		expected int
	}{
		{"no filter", CalendarToolConfig{}, 4},
		{"monitored only", CalendarToolConfig{MonitoredOnly: true}, 3},
		{"filter available", CalendarToolConfig{Filter: "available"}, 1},
		{"filter missing", CalendarToolConfig{Filter: "missing"}, 1},
		{"filter premieres", CalendarToolConfig{Filter: "premieres"}, 1},
		{"filter monitored", CalendarToolConfig{Filter: "monitored"}, 3},
		{"filter missing+premieres", CalendarToolConfig{Filter: "missing,premieres"}, 2},
		{"monitored+filter available", CalendarToolConfig{MonitoredOnly: true, Filter: "available"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyFilters(items, tt.cfg)
			if len(result) != tt.expected {
				t.Errorf("got %d items, want %d", len(result), tt.expected)
			}
		})
	}
}
