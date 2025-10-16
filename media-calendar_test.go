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
