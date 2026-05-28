package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	httpclient "github.com/calmcacil/CalmsToolkit/internal/http"
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

	client := httpclient.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
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

	client := httpclient.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
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

	client := httpclient.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
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
}

func TestTruncateWithEllipsis(t *testing.T) {
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
			name:     "Long text, needs truncation with ellipsis",
			text:     "This is a very long text that needs truncation",
			maxLen:   20,
			expected: "This is a very long…",
		},
		{
			name:     "Exact length",
			text:     "Exactly",
			maxLen:   7,
			expected: "Exactly",
		},
		{
			name:     "Unicode text",
			text:     "Hello 世 Wörld",
			maxLen:   8,
			expected: "Hello 世…",
		},
		{
			name:     "Max length 1",
			text:     "Hi",
			maxLen:   1,
			expected: "H",
		},
		{
			name:     "Empty string",
			text:     "",
			maxLen:   5,
			expected: "",
		},
		{
			name:     "Zero max length",
			text:     "Hello",
			maxLen:   0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateWithEllipsis(tt.text, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateWithEllipsis(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestBuildToolConfig(t *testing.T) {
	tk := config.DefaultToolkitConfig()
	tk.Sonarr = []config.ArrInstance{
		{Name: "Sonarr HD", URL: "http://sonarr:8989", APIKey: "token1"},
	}
	tk.Radarr = []config.ArrInstance{
		{Name: "Radarr HD", URL: "http://radarr:7878", APIKey: "token2"},
	}
	tk.MediaCalendar.Days = 7
	tk.MediaCalendar.DaysPast = 1
	tk.MediaCalendar.WatchInterval = 600
	tk.General.Timeout = "30s"

	cfg := BuildToolConfig(tk)

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

func TestBuildToolConfigNil(t *testing.T) {
	cfg := BuildToolConfig(nil)

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

func TestAggregateCalendarPartialFailureDoesNotCancelSuccessfulSources(t *testing.T) {
	badHit := make(chan struct{})
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/calendar" {
			http.NotFound(w, r)
			return
		}

		close(badHit)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer badServer.Close()

	goodEpisode := []SonarrEpisode{
		{
			Title:         "Successful Episode",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AirDateUtc:    time.Now().Add(time.Hour),
			Monitored:     true,
			Series: &Series{
				Title: "Successful Show",
				Year:  2026,
			},
		},
	}
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/calendar":
			select {
			case <-badHit:
				time.Sleep(100 * time.Millisecond)
			case <-time.After(2 * time.Second):
				t.Error("timed out waiting for failing source")
				http.Error(w, "timeout", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(goodEpisode)
		case "/api/v3/queue":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(QueueResponse{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer goodServer.Close()

	cfg := ToolConfig{
		SonarrInstances: []config.ArrInstance{
			{Name: "Failing", URL: badServer.URL, APIKey: "bad-token"},
			{Name: "Successful", URL: goodServer.URL, APIKey: "good-token"},
		},
		Days:    1,
		Timeout: 2 * time.Second,
	}

	items, _, err := aggregateCalendar(context.Background(), cfg)
	if err != nil {
		t.Fatalf("aggregateCalendar returned error for partial failure: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("aggregateCalendar returned %d items, want 1", len(items))
	}
	if items[0].ShowTitle != "Successful Show" {
		t.Fatalf("aggregateCalendar item ShowTitle = %q, want Successful Show", items[0].ShowTitle)
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
		cfg      ToolConfig
		expected int
	}{
		{"no filter", ToolConfig{}, 4},
		{"monitored only", ToolConfig{MonitoredOnly: true}, 3},
		{"filter available", ToolConfig{Filter: "available"}, 1},
		{"filter missing", ToolConfig{Filter: "missing"}, 1},
		{"filter premieres", ToolConfig{Filter: "premieres"}, 1},
		{"filter monitored", ToolConfig{Filter: "monitored"}, 3},
		{"filter missing+premieres", ToolConfig{Filter: "missing,premieres"}, 2},
		{"monitored+filter available", ToolConfig{MonitoredOnly: true, Filter: "available"}, 1},
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

func TestGetStatusColor(t *testing.T) {
	now := time.Now()
	p := colors.GetPalette("default")

	tests := []struct {
		name string
		item CalendarItem
		now  time.Time
	}{
		{
			name: "Downloaded episode",
			item: CalendarItem{
				Type:    "episode",
				HasFile: true,
				AirTime: now.Add(-24 * time.Hour),
			},
			now: now,
		},
		{
			name: "Aired but not downloaded",
			item: CalendarItem{
				Type:    "episode",
				HasFile: false,
				AirTime: now.Add(-24 * time.Hour),
			},
			now: now,
		},
		{
			name: "Future episode",
			item: CalendarItem{
				Type:    "episode",
				HasFile: false,
				AirTime: now.Add(48 * time.Hour),
			},
			now: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = getStatusColor(tt.item, tt.now, p)
		})
	}
}
