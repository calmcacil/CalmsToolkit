//go:build arrfeed

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestMapSonarrEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"grabbed", "Grabbed"},
		{"downloadFolderImported", "Imported"},
		{"downloadFailed", "Failed"},
		{"episodeFileDeleted", "Deleted"},
		{"episodeFileRenamed", "Renamed"},
		{"downloadIgnored", "Ignored"},
		{"seriesFolderImported", "Bulk Import"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapSonarrEventType(tt.input)
			if result != tt.expected {
				t.Errorf("mapSonarrEventType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapRadarrEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"grabbed", "Grabbed"},
		{"downloadFolderImported", "Imported"},
		{"downloadFailed", "Failed"},
		{"movieFileDeleted", "Deleted"},
		{"movieFileRenamed", "Renamed"},
		{"downloadIgnored", "Ignored"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapRadarrEventType(tt.input)
			if result != tt.expected {
				t.Errorf("mapRadarrEventType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatEpisode(t *testing.T) {
	tests := []struct {
		season   int
		episode  int
		expected string
	}{
		{1, 5, "S01E05"},
		{10, 23, "S10E23"},
		{2, 1, "S02E01"},
		{0, 0, "S00E00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatEpisode(tt.season, tt.episode)
			if result != tt.expected {
				t.Errorf("formatEpisode(%d, %d) = %q, want %q", tt.season, tt.episode, result, tt.expected)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"just now", now.Add(-30 * time.Second), "Just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"3 days ago", now.Add(-72 * time.Hour), "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatRelativeTime() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetActionColor(t *testing.T) {
	tests := []struct {
		action   string
		expected string
	}{
		{"Imported", ColorGreen},
		{"Bulk Import", ColorGreen},
		{"Grabbed", ColorCyan},
		{"Failed", ColorRed},
		{"Deleted", ColorYellow},
		{"Ignored", ColorGray},
		{"Renamed", ColorBlue},
		{"Unknown", ColorReset},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			result := getActionColor(tt.action)
			if result != tt.expected {
				t.Errorf("getActionColor(%q) = %q, want %q", tt.action, result, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a very long string", 10, "this is..."},
		{"truncate", 5, "tr..."},
		{"abc", 3, "abc"},
		{"ab", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestFilterEvents(t *testing.T) {
	events := []HistoryEvent{
		{Action: "Grabbed"},
		{Action: "Imported"},
		{Action: "Failed"},
		{Action: "Deleted"},
		{Action: "Ignored"},
		{Action: "Bulk Import"},
	}

	tests := []struct {
		name     string
		config   FeedToolConfig
		expected int
	}{
		{
			name: "all enabled",
			config: FeedToolConfig{
				ShowGrabbed:  true,
				ShowImported: true,
				ShowFailed:   true,
				ShowDeleted:  true,
				ShowIgnored:  true,
			},
			expected: 6,
		},
		{
			name: "no ignored",
			config: FeedToolConfig{
				ShowGrabbed:  true,
				ShowImported: true,
				ShowFailed:   true,
				ShowDeleted:  true,
				ShowIgnored:  false,
			},
			expected: 5,
		},
		{
			name: "only grabbed",
			config: FeedToolConfig{
				ShowGrabbed:  true,
				ShowImported: false,
				ShowFailed:   false,
				ShowDeleted:  false,
				ShowIgnored:  false,
			},
			expected: 1,
		},
		{
			name: "none enabled",
			config: FeedToolConfig{
				ShowGrabbed:  false,
				ShowImported: false,
				ShowFailed:   false,
				ShowDeleted:  false,
				ShowIgnored:  false,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterEvents(events, tt.config)
			if len(result) != tt.expected {
				t.Errorf("filterEvents() returned %d events, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    FeedToolConfig
		shouldErr bool
	}{
		{
			name: "valid sonarr only",
			config: FeedToolConfig{
				SonarrInstances: []ArrInstance{
					{Name: "sonarr1", URL: "http://localhost:8989", APIKey: "token"},
				},
			},
			shouldErr: false,
		},
		{
			name: "valid radarr only",
			config: FeedToolConfig{
				RadarrInstances: []ArrInstance{
					{Name: "radarr1", URL: "http://localhost:7878", APIKey: "token"},
				},
			},
			shouldErr: false,
		},
		{
			name: "valid both",
			config: FeedToolConfig{
				SonarrInstances: []ArrInstance{
					{Name: "sonarr1", URL: "http://localhost:8989", APIKey: "token1"},
				},
				RadarrInstances: []ArrInstance{
					{Name: "radarr1", URL: "http://localhost:7878", APIKey: "token2"},
				},
			},
			shouldErr: false,
		},
		{
			name:      "no instances",
			config:    FeedToolConfig{},
			shouldErr: true,
		},
		{
			name: "sonarr missing url",
			config: FeedToolConfig{
				SonarrInstances: []ArrInstance{
					{Name: "sonarr1", URL: "", APIKey: "token"},
				},
			},
			shouldErr: true,
		},
		{
			name: "sonarr missing api_key",
			config: FeedToolConfig{
				SonarrInstances: []ArrInstance{
					{Name: "sonarr1", URL: "http://localhost:8989", APIKey: ""},
				},
			},
			shouldErr: true,
		},
		{
			name: "radarr missing url",
			config: FeedToolConfig{
				RadarrInstances: []ArrInstance{
					{Name: "radarr1", URL: "", APIKey: "token"},
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateConfig() error = %v, shouldErr = %v", err, tt.shouldErr)
			}
		})
	}
}

func TestSonarrToHistoryEvent(t *testing.T) {
	sh := SonarrHistory{
		ID:          123,
		EventType:   "grabbed",
		Date:        "2025-10-17T10:30:00Z",
		SourceTitle: "Breaking.Bad.S01E05.720p.BluRay.x264-DEMAND",
		Quality: SonarrQuality{
			Quality: SonarrQualityItem{
				Name: "Bluray-720p",
			},
			CustomFormats: []CustomFormat{
				{ID: 1, Name: "AMZN"},
				{ID: 2, Name: "DV"},
			},
		},
		Series: &SonarrSeries{
			ID:    1,
			Title: "Breaking Bad",
		},
		Episode: &SonarrEpisode{
			ID:            1,
			SeasonNumber:  1,
			EpisodeNumber: 5,
			Title:         "Gray Matter",
		},
	}

	event := sonarrToHistoryEvent(sh)

	if event.Server != "sonarr" {
		t.Errorf("Server = %q, want %q", event.Server, "sonarr")
	}
	if event.Action != "Grabbed" {
		t.Errorf("Action = %q, want %q", event.Action, "Grabbed")
	}
	if event.Title != "Breaking Bad" {
		t.Errorf("Title = %q, want %q", event.Title, "Breaking Bad")
	}
	if event.Episode != "S01E05" {
		t.Errorf("Episode = %q, want %q", event.Episode, "S01E05")
	}
	if event.EpisodeTitle != "Gray Matter" {
		t.Errorf("EpisodeTitle = %q, want %q", event.EpisodeTitle, "Gray Matter")
	}
	if event.Quality != "Bluray-720p" {
		t.Errorf("Quality = %q, want %q", event.Quality, "Bluray-720p")
	}
	if len(event.Formats) != 2 {
		t.Errorf("len(Formats) = %d, want 2", len(event.Formats))
	}
	if event.ID != 123 {
		t.Errorf("ID = %d, want 123", event.ID)
	}
}

func TestRadarrToHistoryEvent(t *testing.T) {
	rh := RadarrHistory{
		ID:          456,
		EventType:   "downloadFolderImported",
		Date:        "2025-10-17T11:00:00Z",
		SourceTitle: "The.Matrix.1999.1080p.BluRay.x264-GROUP",
		Quality: RadarrQuality{
			Quality: RadarrQualityItem{
				Name: "Bluray-1080p",
			},
			CustomFormats: []CustomFormat{
				{ID: 3, Name: "IMAX"},
			},
		},
		Movie: &RadarrMovie{
			ID:    10,
			Title: "The Matrix",
			Year:  1999,
		},
	}

	event := radarrToHistoryEvent(rh)

	if event.Server != "radarr" {
		t.Errorf("Server = %q, want %q", event.Server, "radarr")
	}
	if event.Action != "Imported" {
		t.Errorf("Action = %q, want %q", event.Action, "Imported")
	}
	if event.Title != "The Matrix (1999)" {
		t.Errorf("Title = %q, want %q", event.Title, "The Matrix (1999)")
	}
	if event.Quality != "Bluray-1080p" {
		t.Errorf("Quality = %q, want %q", event.Quality, "Bluray-1080p")
	}
	if len(event.Formats) != 1 {
		t.Errorf("len(Formats) = %d, want 1", len(event.Formats))
	}
	if event.Formats[0] != "IMAX" {
		t.Errorf("Formats[0] = %q, want %q", event.Formats[0], "IMAX")
	}
	if event.ID != 456 {
		t.Errorf("ID = %d, want 456", event.ID)
	}
}

func TestFetchSonarrHistory(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErr    bool
		expectedCount  int
		validateEvents func(*testing.T, []HistoryEvent)
	}{
		{
			name:          "successful fetch",
			statusCode:    200,
			responseBody:  `[{"id":1,"eventType":"grabbed","date":"2025-10-17T10:00:00Z","sourceTitle":"Test.Show.S01E01.720p","quality":{"quality":{"name":"HDTV-720p"},"customFormats":[]},"episodeId":1,"seriesId":1,"series":{"id":1,"title":"Test Show"},"episode":{"id":1,"seasonNumber":1,"episodeNumber":1,"title":"Pilot"}}]`,
			expectedErr:   false,
			expectedCount: 1,
			validateEvents: func(t *testing.T, events []HistoryEvent) {
				if events[0].Server != "sonarr" {
					t.Errorf("Server = %q, want %q", events[0].Server, "sonarr")
				}
				if events[0].Action != "Grabbed" {
					t.Errorf("Action = %q, want %q", events[0].Action, "Grabbed")
				}
				if events[0].Title != "Test Show" {
					t.Errorf("Title = %q, want %q", events[0].Title, "Test Show")
				}
			},
		},
		{
			name:          "wrapped records shape",
			statusCode:    200,
			responseBody:  `{"records":[{"id":3,"eventType":"grabbed","date":"2025-10-17T10:05:00Z","sourceTitle":"Wrapped.Show.S01E02","quality":{"quality":{"name":"HDTV-720p"},"customFormats":[]},"episodeId":2,"seriesId":1,"series":{"id":1,"title":"Wrapped Show"},"episode":{"id":2,"seasonNumber":1,"episodeNumber":2,"title":"Second"}}]}`,
			expectedErr:   false,
			expectedCount: 1,
			validateEvents: func(t *testing.T, events []HistoryEvent) {
				if events[0].Title != "Wrapped Show" {
					t.Errorf("Title = %q, want %q", events[0].Title, "Wrapped Show")
				}
			},
		},
		{
			name:         "401 unauthorized",
			statusCode:   401,
			responseBody: `{"error":"Unauthorized"}`,
			expectedErr:  true,
		},
		{
			name:         "404 not found",
			statusCode:   404,
			responseBody: `{"error":"Not found"}`,
			expectedErr:  true,
		},
		{
			name:         "500 server error",
			statusCode:   500,
			responseBody: `{"error":"Internal server error"}`,
			expectedErr:  true,
		},
		{
			name:          "empty array",
			statusCode:    200,
			responseBody:  `[]`,
			expectedErr:   false,
			expectedCount: 0,
		},
		{
			name:         "invalid json",
			statusCode:   200,
			responseBody: `{invalid json}`,
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Invalid API key"}`))
					return
				}

				if !strings.Contains(r.URL.Path, "/api/v3/history/since") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if r.URL.Query().Get("includeEpisode") != "true" {
					t.Errorf("includeEpisode not set to true")
				}
				if r.URL.Query().Get("includeSeries") != "true" {
					t.Errorf("includeSeries not set to true")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			inst := ArrInstance{URL: server.URL, APIKey: "test-token"}
			since := time.Now().Add(-1 * time.Hour)

			events, err := fetchSonarrHistory(context.Background(), client, inst, since)

			if (err != nil) != tt.expectedErr {
				t.Errorf("fetchSonarrHistory() error = %v, expectedErr = %v", err, tt.expectedErr)
				return
			}

			if !tt.expectedErr {
				if len(events) != tt.expectedCount {
					t.Errorf("got %d events, want %d", len(events), tt.expectedCount)
				}

				if tt.validateEvents != nil {
					tt.validateEvents(t, events)
				}
			}
		})
	}
}

func TestFetchRadarrHistory(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErr    bool
		expectedCount  int
		validateEvents func(*testing.T, []HistoryEvent)
	}{
		{
			name:          "successful fetch",
			statusCode:    200,
			responseBody:  `[{"id":2,"eventType":"downloadFolderImported","date":"2025-10-17T11:00:00Z","sourceTitle":"Test.Movie.2024.1080p","quality":{"quality":{"name":"Bluray-1080p"},"customFormats":[{"id":1,"name":"DV"}]},"movieId":10,"movie":{"id":10,"title":"Test Movie","year":2024}}]`,
			expectedErr:   false,
			expectedCount: 1,
			validateEvents: func(t *testing.T, events []HistoryEvent) {
				if events[0].Server != "radarr" {
					t.Errorf("Server = %q, want %q", events[0].Server, "radarr")
				}
				if events[0].Action != "Imported" {
					t.Errorf("Action = %q, want %q", events[0].Action, "Imported")
				}
				if events[0].Title != "Test Movie (2024)" {
					t.Errorf("Title = %q, want %q", events[0].Title, "Test Movie (2024)")
				}
			},
		},
		{
			name:          "wrapped records shape",
			statusCode:    200,
			responseBody:  `{"records":[{"id":5,"eventType":"downloadFolderImported","date":"2025-10-17T11:05:00Z","sourceTitle":"Wrapped.Movie.2024","quality":{"quality":{"name":"Bluray-1080p"},"customFormats":[{"id":2,"name":"IMAX"}]},"movieId":11,"movie":{"id":11,"title":"Wrapped Movie","year":2024}}]}`,
			expectedErr:   false,
			expectedCount: 1,
			validateEvents: func(t *testing.T, events []HistoryEvent) {
				if events[0].Title != "Wrapped Movie (2024)" {
					t.Errorf("Title = %q, want %q", events[0].Title, "Wrapped Movie (2024)")
				}
			},
		},
		{
			name:         "401 unauthorized",
			statusCode:   401,
			responseBody: `{"error":"Unauthorized"}`,
			expectedErr:  true,
		},
		{
			name:         "404 not found",
			statusCode:   404,
			responseBody: `{"error":"Not found"}`,
			expectedErr:  true,
		},
		{
			name:         "500 server error",
			statusCode:   500,
			responseBody: `{"error":"Internal server error"}`,
			expectedErr:  true,
		},
		{
			name:          "empty array",
			statusCode:    200,
			responseBody:  `[]`,
			expectedErr:   false,
			expectedCount: 0,
		},
		{
			name:         "invalid json",
			statusCode:   200,
			responseBody: `{invalid json}`,
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Invalid API key"}`))
					return
				}

				if !strings.Contains(r.URL.Path, "/api/v3/history/since") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if r.URL.Query().Get("includeMovie") != "true" {
					t.Errorf("includeMovie not set to true")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			inst := ArrInstance{URL: server.URL, APIKey: "test-token"}
			since := time.Now().Add(-1 * time.Hour)

			events, err := fetchRadarrHistory(context.Background(), client, inst, since)

			if (err != nil) != tt.expectedErr {
				t.Errorf("fetchRadarrHistory() error = %v, expectedErr = %v", err, tt.expectedErr)
				return
			}

			if !tt.expectedErr {
				if len(events) != tt.expectedCount {
					t.Errorf("got %d events, want %d", len(events), tt.expectedCount)
				}

				if tt.validateEvents != nil {
					tt.validateEvents(t, events)
				}
			}
		})
	}
}

func TestFetchAllHistory(t *testing.T) {
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"eventType":"grabbed","date":"2025-10-17T10:00:00Z","sourceTitle":"Test.Show.S01E01","quality":{"quality":{"name":"HDTV-720p"},"customFormats":[]},"episodeId":1,"seriesId":1,"series":{"id":1,"title":"Test Show"},"episode":{"id":1,"seasonNumber":1,"episodeNumber":1,"title":"Pilot"}}]`))
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":2,"eventType":"downloadFolderImported","date":"2025-10-17T11:30:00Z","sourceTitle":"Test.Movie.2024","quality":{"quality":{"name":"Bluray-1080p"},"customFormats":[]},"movieId":10,"movie":{"id":10,"title":"Test Movie","year":2024}}]`))
	}))
	defer radarrServer.Close()

	cfg := FeedToolConfig{
		SonarrInstances: []ArrInstance{
			{Name: "sonarr1", URL: sonarrServer.URL, APIKey: "sonarr-token"},
		},
		RadarrInstances: []ArrInstance{
			{Name: "radarr1", URL: radarrServer.URL, APIKey: "radarr-token"},
		},
		Timeout: 5 * time.Second,
	}

	client := &http.Client{Timeout: cfg.Timeout}
	since := time.Now().Add(-1 * time.Hour)
	events, err := fetchAllHistory(context.Background(), client, cfg, since)

	if err != nil {
		t.Fatalf("fetchAllHistory() error = %v", err)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}

	if events[0].When.Before(events[1].When) {
		t.Errorf("events not sorted in descending order by timestamp")
	}

	foundSonarr := false
	foundRadarr := false
	for _, e := range events {
		if e.Server == "sonarr" {
			foundSonarr = true
		}
		if e.Server == "radarr" {
			foundRadarr = true
		}
	}

	if !foundSonarr {
		t.Errorf("no sonarr events found")
	}
	if !foundRadarr {
		t.Errorf("no radarr events found")
	}
}

func TestFetchAllHistoryPartialFailure(t *testing.T) {
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"eventType":"grabbed","date":"2025-10-17T10:00:00Z","sourceTitle":"Test.Show.S01E01","quality":{"quality":{"name":"HDTV-720p"},"customFormats":[]},"episodeId":1,"seriesId":1,"series":{"id":1,"title":"Test Show"},"episode":{"id":1,"seasonNumber":1,"episodeNumber":1,"title":"Pilot"}}]`))
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Server error"}`))
	}))
	defer radarrServer.Close()

	cfg := FeedToolConfig{
		SonarrInstances: []ArrInstance{
			{Name: "sonarr1", URL: sonarrServer.URL, APIKey: "sonarr-token"},
		},
		RadarrInstances: []ArrInstance{
			{Name: "radarr1", URL: radarrServer.URL, APIKey: "radarr-token"},
		},
		Timeout: 5 * time.Second,
	}

	client := &http.Client{Timeout: cfg.Timeout}
	since := time.Now().Add(-1 * time.Hour)
	events, err := fetchAllHistory(context.Background(), client, cfg, since)

	if err != nil {
		t.Fatalf("fetchAllHistory() should not error on partial failure, got: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("got %d events, want 1 (from successful sonarr fetch)", len(events))
	}

	if events[0].Server != "sonarr" {
		t.Errorf("event server = %q, want %q", events[0].Server, "sonarr")
	}
}

func TestFetchAllHistoryCompleteFailure(t *testing.T) {
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
	}))
	defer radarrServer.Close()

	cfg := FeedToolConfig{
		SonarrInstances: []ArrInstance{
			{Name: "sonarr1", URL: sonarrServer.URL, APIKey: "bad-token"},
		},
		RadarrInstances: []ArrInstance{
			{Name: "radarr1", URL: radarrServer.URL, APIKey: "bad-token"},
		},
		Timeout: 5 * time.Second,
	}

	client := &http.Client{Timeout: cfg.Timeout}
	since := time.Now().Add(-1 * time.Hour)
	events, err := fetchAllHistory(context.Background(), client, cfg, since)

	if err == nil {
		t.Errorf("fetchAllHistory() should error when all instances fail")
	}

	if events != nil {
		t.Errorf("events should be nil on complete failure")
	}
}

func TestBuildFeedToolConfig(t *testing.T) {
	// nil config returns defaults
	cfg := BuildFeedToolConfig(nil)
	if cfg.Timeout != 10*time.Second {
		t.Errorf("nil config: Timeout = %v, want 10s", cfg.Timeout)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("nil config: PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.HistoryWindow != 1*time.Hour {
		t.Errorf("nil config: HistoryWindow = %v, want 1h", cfg.HistoryWindow)
	}
	if cfg.MaxEvents != 50 {
		t.Errorf("nil config: MaxEvents = %d, want 50", cfg.MaxEvents)
	}
	if !cfg.ShowGrabbed {
		t.Errorf("nil config: ShowGrabbed = false, want true")
	}
	if !cfg.ShowImported {
		t.Errorf("nil config: ShowImported = false, want true")
	}
	if !cfg.ShowFailed {
		t.Errorf("nil config: ShowFailed = false, want true")
	}

	// full config
	tk := &ToolkitConfig{
		General: GeneralConfig{
			Timeout: "30s",
			NoColor: true,
		},
		Sonarr: []ArrInstance{
			{Name: "sonarr1", URL: "http://sonarr:8989", APIKey: "token1"},
		},
		Radarr: []ArrInstance{
			{Name: "radarr1", URL: "http://radarr:7878", APIKey: "token2"},
		},
		ArrFeed: FeedConfig{
			PollInterval:  "10s",
			HistoryWindow: "2h",
			ShowGrabbed:   false,
			ShowImported:  true,
			ShowFailed:    false,
			ShowDeleted:   true,
			ShowIgnored:   true,
			MaxEvents:     25,
		},
	}
	cfg = BuildFeedToolConfig(tk)
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if !cfg.NoColor {
		t.Errorf("NoColor = false, want true")
	}
	if len(cfg.SonarrInstances) != 1 {
		t.Errorf("len(SonarrInstances) = %d, want 1", len(cfg.SonarrInstances))
	}
	if cfg.SonarrInstances[0].Name != "sonarr1" {
		t.Errorf("SonarrInstances[0].Name = %q, want %q", cfg.SonarrInstances[0].Name, "sonarr1")
	}
	if len(cfg.RadarrInstances) != 1 {
		t.Errorf("len(RadarrInstances) = %d, want 1", len(cfg.RadarrInstances))
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %v, want 10s", cfg.PollInterval)
	}
	if cfg.HistoryWindow != 2*time.Hour {
		t.Errorf("HistoryWindow = %v, want 2h", cfg.HistoryWindow)
	}
	if cfg.ShowGrabbed {
		t.Errorf("ShowGrabbed = true, want false")
	}
	if !cfg.ShowImported {
		t.Errorf("ShowImported = false, want true")
	}
	if cfg.ShowFailed {
		t.Errorf("ShowFailed = true, want false")
	}
	if !cfg.ShowDeleted {
		t.Errorf("ShowDeleted = false, want true")
	}
	if !cfg.ShowIgnored {
		t.Errorf("ShowIgnored = false, want true")
	}
	if cfg.MaxEvents != 25 {
		t.Errorf("MaxEvents = %d, want 25", cfg.MaxEvents)
	}

	// max events clamped to 100
	tk.ArrFeed.MaxEvents = 150
	cfg = BuildFeedToolConfig(tk)
	if cfg.MaxEvents != 100 {
		t.Errorf("MaxEvents = %d, want 100 (clamped)", cfg.MaxEvents)
	}

	// invalid duration falls back to default
	tk.General.Timeout = "invalid"
	cfg = BuildFeedToolConfig(tk)
	if cfg.Timeout != 10*time.Second {
		t.Errorf("invalid Timeout = %v, want 10s (fallback)", cfg.Timeout)
	}
}

func TestEventSorting(t *testing.T) {
	now := time.Now()
	events := []HistoryEvent{
		{ID: 1, When: now.Add(-10 * time.Minute), Action: "Grabbed"},
		{ID: 2, When: now.Add(-5 * time.Minute), Action: "Imported"},
		{ID: 3, When: now.Add(-20 * time.Minute), Action: "Failed"},
		{ID: 4, When: now.Add(-1 * time.Minute), Action: "Deleted"},
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].When.After(events[j].When)
	})

	if events[0].ID != 4 {
		t.Errorf("first event ID = %d, want 4 (most recent)", events[0].ID)
	}
	if events[1].ID != 2 {
		t.Errorf("second event ID = %d, want 2", events[1].ID)
	}
	if events[2].ID != 1 {
		t.Errorf("third event ID = %d, want 1", events[2].ID)
	}
	if events[3].ID != 3 {
		t.Errorf("fourth event ID = %d, want 3 (oldest)", events[3].ID)
	}

	for i := 0; i < len(events)-1; i++ {
		if events[i].When.Before(events[i+1].When) {
			t.Errorf("events[%d].When (%v) is before events[%d].When (%v), want descending order",
				i, events[i].When, i+1, events[i+1].When)
		}
	}
}

func TestGetColorFunc(t *testing.T) {
	tests := []struct {
		name     string
		config   FeedToolConfig
		expected string
	}{
		{
			name:     "color enabled",
			config:   FeedToolConfig{NoColor: false, JSON: false},
			expected: ColorRed,
		},
		{
			name:     "no-color flag",
			config:   FeedToolConfig{NoColor: true, JSON: false},
			expected: "",
		},
		{
			name:     "json mode",
			config:   FeedToolConfig{NoColor: false, JSON: true},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colorFunc := getColorFunc(tt.config)
			result := colorFunc(ColorRed)
			if result != tt.expected {
				t.Errorf("colorFunc(ColorRed) = %q, want %q", result, tt.expected)
			}
		})
	}
}
