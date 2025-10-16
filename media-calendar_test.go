//go:build mediacalendar
// +build mediacalendar

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseCommaSeparated verifies comma-separated string parsing
func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single item",
			input:    "http://sonarr1",
			expected: []string{"http://sonarr1"},
		},
		{
			name:     "Multiple items",
			input:    "http://sonarr1,http://sonarr2,http://sonarr3",
			expected: []string{"http://sonarr1", "http://sonarr2", "http://sonarr3"},
		},
		{
			name:     "Empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "Items with spaces",
			input:    " http://sonarr1 , http://sonarr2 ",
			expected: []string{"http://sonarr1", "http://sonarr2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparated(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseCommaSeparated() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseCommaSeparated()[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// TestLoadEnvFile verifies environment file loading for media-calendar
func TestLoadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	content := `SONARR_URLS=http://sonarr1.example.com,http://sonarr2.example.com
SONARR_TOKENS=token1,token2
RADARR_URLS=http://radarr1.example.com
RADARR_TOKENS=token3
# Comment line
IGNORED_VAR=value
`
	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	config := &Config{}
	loadEnvFile(envFile, config)

	if len(config.SonarrURLs) != 2 {
		t.Errorf("SonarrURLs count = %d, want 2", len(config.SonarrURLs))
	}
	if len(config.SonarrTokens) != 2 {
		t.Errorf("SonarrTokens count = %d, want 2", len(config.SonarrTokens))
	}
	if len(config.RadarrURLs) != 1 {
		t.Errorf("RadarrURLs count = %d, want 1", len(config.RadarrURLs))
	}
	if len(config.RadarrTokens) != 1 {
		t.Errorf("RadarrTokens count = %d, want 1", len(config.RadarrTokens))
	}
}

// TestLoadConfig verifies configuration loading
func TestLoadConfig(t *testing.T) {
	config := loadConfig(
		"http://sonarr1,http://sonarr2",
		"token1,token2",
		"http://radarr1",
		"token3",
		10*time.Second,
		7,
		0,
		false,
		false,
		false,
		30,
		false,
	)

	if len(config.SonarrURLs) != 2 {
		t.Errorf("SonarrURLs count = %d, want 2", len(config.SonarrURLs))
	}
	if config.SonarrURLs[0] != "http://sonarr1" {
		t.Errorf("SonarrURLs[0] = %v, want %v", config.SonarrURLs[0], "http://sonarr1")
	}
	if len(config.SonarrTokens) != 2 {
		t.Errorf("SonarrTokens count = %d, want 2", len(config.SonarrTokens))
	}
	if config.Days != 7 {
		t.Errorf("Days = %d, want 7", config.Days)
	}
	if config.WatchMode != false {
		t.Errorf("WatchMode = %v, want false", config.WatchMode)
	}
}

// TestFetchSonarrCalendar verifies Sonarr calendar fetching
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

	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	episodes, err := fetchSonarrCalendar(server.URL, "test-token", start, end, false)
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

// TestFetchRadarrCalendar verifies Radarr calendar fetching
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

	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	movies, err := fetchRadarrCalendar(server.URL, "test-token", start, end, false)
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

// TestFetchQueue verifies queue fetching
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

	queue, err := fetchQueue(server.URL, "test-token", false)
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

// TestCalculateColumnLayout verifies column layout calculation
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
			expectedColumns:     3, // minComfortableColumnWidth=45, so 3 cols @ 50 width
			expectedWidthPerCol: 50,
		},
		{
			name:                "Narrow terminal, 7 days",
			termWidth:           80,
			totalDays:           7,
			expectedColumns:     1,  // Only 1 column fits with minComfortableColumnWidth=45
			expectedWidthPerCol: 76, // (80 - 1 - 3) / 1 = 76
		},
		{
			name:                "Very narrow terminal",
			termWidth:           40,
			totalDays:           7,
			expectedColumns:     1,  // Only 1 column fits
			expectedWidthPerCol: 36, // (40 - 1 - 3) / 1 = 36
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

// TestTruncateText verifies text truncation
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

// TestAggregateCalendar verifies calendar aggregation and deduplication
func TestAggregateCalendar(t *testing.T) {
	// Create mock servers for Sonarr and Radarr
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
		if r.URL.Path == "/api/v3/calendar" {
			json.NewEncoder(w).Encode(sonarrResponse)
		} else if r.URL.Path == "/api/v3/queue" {
			json.NewEncoder(w).Encode(queueResponse)
		}
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/calendar" {
			json.NewEncoder(w).Encode(radarrResponse)
		} else if r.URL.Path == "/api/v3/queue" {
			json.NewEncoder(w).Encode(QueueResponse{TotalRecords: 0})
		}
	}))
	defer radarrServer.Close()

	config := Config{
		SonarrURLs:   []string{sonarrServer.URL},
		SonarrTokens: []string{"test-token"},
		RadarrURLs:   []string{radarrServer.URL},
		RadarrTokens: []string{"test-token"},
		Days:         7,
		DaysPast:     0,
		Timeout:      5 * time.Second,
		Debug:        false,
	}

	items, issues, err := aggregateCalendar(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(items) < 2 {
		t.Errorf("Expected at least 2 calendar items, got %d", len(items))
	}

	if len(issues) < 1 {
		t.Errorf("Expected at least 1 queue issue, got %d", len(issues))
	}

	// Verify Sonarr episode is present
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

	// Verify Radarr movie is present
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

// TestGetStatusColor verifies status color selection
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
			// Just verify the function doesn't panic
			_ = getStatusColor(tt.item, tt.now, tt.noColor)
		})
	}
}

// TestBuildDayContentSorting verifies that episodes are sorted by air time, not alphabetically by show name
func TestBuildDayContentSorting(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	config := Config{
		NoColor: true,
	}

	// Create items with different air times and show names
	// Shows are alphabetically: Alpha, Beta, Zulu
	// But air times should be: 14:00 (Zulu), 16:00 (Alpha), 18:00 (Beta)
	dayItems := []CalendarItem{
		{
			Type:      "episode",
			ShowTitle: "Alpha Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(16 * time.Hour), // 16:00
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Zulu Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(14 * time.Hour), // 14:00 - should be first
			HasFile:   false,
		},
		{
			Type:      "episode",
			ShowTitle: "Beta Show",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(18 * time.Hour), // 18:00
			HasFile:   false,
		},
	}

	colorFunc := func(s string) string { return "" }
	content := buildDayContent(dayItems, now, config, colorFunc, 80)

	// Verify we got content (3 episodes)
	if len(content) < 3 {
		t.Fatalf("Expected at least 3 content lines, got %d", len(content))
	}

	// The content should be in time order: Zulu (14:00), Alpha (16:00), Beta (18:00)
	// Check that the time strings appear in the correct order
	foundZulu := false
	foundAlpha := false
	foundBeta := false

	for i, line := range content {
		if !foundZulu && (contains(line, "Zulu Show") || contains(line, "14:00")) {
			foundZulu = true
			// Alpha and Beta should not have been found yet
			if foundAlpha || foundBeta {
				t.Errorf("Found Zulu Show at position %d, but Alpha or Beta was already found", i)
			}
		}
		if !foundAlpha && (contains(line, "Alpha Show") || contains(line, "16:00")) {
			foundAlpha = true
			// Zulu should have been found, Beta should not
			if !foundZulu {
				t.Errorf("Found Alpha Show at position %d before Zulu Show", i)
			}
			if foundBeta {
				t.Errorf("Found Alpha Show at position %d after Beta Show", i)
			}
		}
		if !foundBeta && (contains(line, "Beta Show") || contains(line, "18:00")) {
			foundBeta = true
			// Both Zulu and Alpha should have been found
			if !foundZulu || !foundAlpha {
				t.Errorf("Found Beta Show at position %d, but Zulu or Alpha was not found yet", i)
			}
		}
	}

	if !foundZulu || !foundAlpha || !foundBeta {
		t.Errorf("Not all shows were found in content. Zulu: %v, Alpha: %v, Beta: %v", foundZulu, foundAlpha, foundBeta)
	}
}

// TestBuildDayContentTruncation verifies truncation works with time-sorted episodes
func TestBuildDayContentTruncation(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	config := Config{
		NoColor: true,
	}

	// Create 4 consecutive episodes from the same show
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
	content := buildDayContent(dayItems, now, config, colorFunc, 80)

	// Should show 2 episodes + 1 truncation line = 3 lines
	// (First 2 episodes shown, "+ 2 more episodes" for the rest)
	foundTruncation := false
	episodeCount := 0

	for _, line := range content {
		if contains(line, "+ 2 more episodes") {
			foundTruncation = true
		}
		if contains(line, "Episode") {
			episodeCount++
		}
	}

	if !foundTruncation {
		t.Error("Expected truncation message not found")
	}
}

// TestBuildDayContentMixedTypes verifies movies and episodes are sorted by time
func TestBuildDayContentMixedTypes(t *testing.T) {
	now := time.Date(2025, 10, 16, 12, 0, 0, 0, time.UTC)
	baseTime := now.Truncate(24 * time.Hour)

	config := Config{
		NoColor: true,
	}

	// Mix movies and episodes with different air times
	dayItems := []CalendarItem{
		{
			Type:    "movie",
			Title:   "Movie B",
			AirTime: baseTime.Add(17 * time.Hour), // 17:00
			HasFile: false,
		},
		{
			Type:      "episode",
			ShowTitle: "Show A",
			Title:     "Episode 1",
			Season:    1,
			Episode:   1,
			AirTime:   baseTime.Add(15 * time.Hour), // 15:00 - should be first
			HasFile:   false,
		},
		{
			Type:    "movie",
			Title:   "Movie A",
			AirTime: baseTime.Add(19 * time.Hour), // 19:00
			HasFile: false,
		},
	}

	colorFunc := func(s string) string { return "" }
	content := buildDayContent(dayItems, now, config, colorFunc, 80)

	// Should be in order: Episode (15:00), Movie B (17:00), Movie A (19:00)
	foundEpisode := false
	foundMovieB := false
	foundMovieA := false

	for i, line := range content {
		if !foundEpisode && contains(line, "Show A") {
			foundEpisode = true
			if foundMovieB || foundMovieA {
				t.Errorf("Found episode at position %d after a movie", i)
			}
		}
		if !foundMovieB && contains(line, "Movie B") {
			foundMovieB = true
			if !foundEpisode {
				t.Errorf("Found Movie B at position %d before episode", i)
			}
			if foundMovieA {
				t.Errorf("Found Movie B at position %d after Movie A", i)
			}
		}
		if !foundMovieA && contains(line, "Movie A") {
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
