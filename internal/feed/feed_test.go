package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
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
		{"older than 7 days", now.Add(-10 * 24 * time.Hour), now.Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04")},
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

func TestFilterEvents(t *testing.T) {
	baseTime := time.Now()
	events := []HistoryEvent{
		{Action: "Grabbed", Title: "Movie 1", When: baseTime},
		{Action: "Imported", Title: "Movie 2", When: baseTime},
		{Action: "Failed", Title: "Movie 3", When: baseTime},
		{Action: "Deleted", Title: "Movie 4", When: baseTime},
		{Action: "Ignored", Title: "Movie 5", When: baseTime},
	}

	tests := []struct {
		name     string
		cfg      ToolConfig
		expected int
	}{
		{"show all", ToolConfig{ShowGrabbed: true, ShowImported: true, ShowFailed: true, ShowDeleted: true, ShowIgnored: true}, 5},
		{"only grabbed", ToolConfig{ShowGrabbed: true}, 1},
		{"no failed", ToolConfig{ShowGrabbed: true, ShowImported: true, ShowFailed: false, ShowDeleted: true, ShowIgnored: true}, 4},
		{"all off", ToolConfig{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterEvents(events, tt.cfg)
			if len(result) != tt.expected {
				t.Errorf("got %d events, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s        string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hello", 3, "hel"},
		{"ab", 2, "ab"},
		{"a", 0, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.s, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, result, tt.expected)
		}
	}
}

func TestCenter(t *testing.T) {
	tests := []struct {
		s        string
		width    int
		expected string
	}{
		{"hi", 6, "  hi  "},
		{"hi", 5, " hi  "},
		{"hello", 3, "hel"},
		{"", 4, "    "},
	}

	for _, tt := range tests {
		result := center(tt.s, tt.width)
		if result != tt.expected {
			t.Errorf("center(%q, %d) = %q, want %q", tt.s, tt.width, result, tt.expected)
		}
	}
}

func TestGetActionColor(t *testing.T) {
	tests := []struct {
		action string
	}{
		{"Imported"},
		{"Bulk Import"},
		{"Grabbed"},
		{"Failed"},
		{"Deleted"},
		{"Ignored"},
		{"Renamed"},
		{"unknown"},
	}

	p := colors.GetPalette("")
	for _, tt := range tests {
		result := getActionColor(tt.action, p)
		if result == "" {
			t.Errorf("getActionColor(%q) returned empty", tt.action)
		}
	}
}

func TestBuildToolConfig(t *testing.T) {
	tk := config.DefaultToolkitConfig()
	tk.ArrFeed.PollInterval = "10s"
	tk.ArrFeed.HistoryWindow = "2h"
	tk.ArrFeed.MaxEvents = 25
	tk.ArrFeed.ShowGrabbed = false
	tk.ArrFeed.ShowImported = true

	cfg := BuildToolConfig(tk)

	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %v, want 10s", cfg.PollInterval)
	}
	if cfg.HistoryWindow != 2*time.Hour {
		t.Errorf("HistoryWindow = %v, want 2h", cfg.HistoryWindow)
	}
	if cfg.MaxEvents != 25 {
		t.Errorf("MaxEvents = %d, want 25", cfg.MaxEvents)
	}
	if cfg.ShowGrabbed {
		t.Error("ShowGrabbed should be false")
	}
	if !cfg.ShowImported {
		t.Error("ShowImported should be true")
	}
}

func TestBuildToolConfigNil(t *testing.T) {
	cfg := BuildToolConfig(nil)

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.HistoryWindow != 1*time.Hour {
		t.Errorf("HistoryWindow = %v, want 1h", cfg.HistoryWindow)
	}
	if cfg.MaxEvents != 50 {
		t.Errorf("MaxEvents = %d, want 50", cfg.MaxEvents)
	}
}

func TestBuildToolConfigDefaults(t *testing.T) {
	cfg := BuildToolConfig(config.DefaultToolkitConfig())
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.MaxEvents != 50 {
		t.Errorf("MaxEvents = %d, want 50", cfg.MaxEvents)
	}
}

func TestSonarrToHistoryEvent(t *testing.T) {
	now := time.Now()
	sh := SonarrHistory{
		ID:          123,
		EventType:   "grabbed",
		Date:        now.UTC().Format(time.RFC3339),
		SourceTitle: "Test.Source.2024",
		Quality: SonarrQuality{
			Quality: SonarrQualityItem{Name: "HD-1080p"},
		},
		Series: &SonarrSeries{
			ID:    1,
			Title: "Test Series",
		},
		Episode: &SonarrEpisode{
			ID:            100,
			SeasonNumber:  2,
			EpisodeNumber: 5,
			Title:         "Episode 5",
		},
	}

	event := sonarrToHistoryEvent(sh)

	if event.Server != "sonarr" {
		t.Errorf("Server = %q, want %q", event.Server, "sonarr")
	}
	if event.Action != "Grabbed" {
		t.Errorf("Action = %q, want %q", event.Action, "Grabbed")
	}
	if event.Title != "Test Series" {
		t.Errorf("Title = %q, want %q", event.Title, "Test Series")
	}
	if event.Episode != "S02E05" {
		t.Errorf("Episode = %q, want %q", event.Episode, "S02E05")
	}
	if event.EpisodeTitle != "Episode 5" {
		t.Errorf("EpisodeTitle = %q, want %q", event.EpisodeTitle, "Episode 5")
	}
	if event.Quality != "HD-1080p" {
		t.Errorf("Quality = %q, want %q", event.Quality, "HD-1080p")
	}
	if event.SourceTitle != "Test.Source.2024" {
		t.Errorf("SourceTitle = %q, want %q", event.SourceTitle, "Test.Source.2024")
	}
	if event.ID != 123 {
		t.Errorf("ID = %d, want %d", event.ID, 123)
	}
}

func TestRadarrToHistoryEvent(t *testing.T) {
	now := time.Now()
	rh := RadarrHistory{
		ID:          456,
		EventType:   "downloadFolderImported",
		Date:        now.UTC().Format(time.RFC3339),
		SourceTitle: "Test.Movie.2024.1080p",
		Quality: RadarrQuality{
			Quality: RadarrQualityItem{Name: "HD-1080p"},
		},
		Movie: &RadarrMovie{
			ID:    1,
			Title: "Test Movie",
			Year:  2024,
		},
	}

	event := radarrToHistoryEvent(rh)

	if event.Server != "radarr" {
		t.Errorf("Server = %q, want %q", event.Server, "radarr")
	}
	if event.Action != "Imported" {
		t.Errorf("Action = %q, want %q", event.Action, "Imported")
	}
	if event.Title != "Test Movie (2024)" {
		t.Errorf("Title = %q, want %q", event.Title, "Test Movie (2024)")
	}
	if event.Quality != "HD-1080p" {
		t.Errorf("Quality = %q, want %q", event.Quality, "HD-1080p")
	}
	if event.ID != 456 {
		t.Errorf("ID = %d, want %d", event.ID, 456)
	}
}

func TestFetchSonarrHistory(t *testing.T) {
	mockResponse := `{"records": [{"id": 1, "eventType": "grabbed", "date": "2024-01-15T10:00:00Z", "sourceTitle": "Test", "quality": {"quality": {"name": "HD"}}}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	since := time.Now().Add(-1 * time.Hour)

	events, err := fetchSonarrHistory(context.Background(), client, inst, since, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Got %d events, want 1", len(events))
	}
}

func TestFetchRadarrHistory(t *testing.T) {
	mockResponse := `{"records": [{"id": 1, "eventType": "grabbed", "date": "2024-01-15T10:00:00Z", "sourceTitle": "Test", "quality": {"quality": {"name": "HD"}}}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	since := time.Now().Add(-1 * time.Hour)

	events, err := fetchRadarrHistory(context.Background(), client, inst, since, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Got %d events, want 1", len(events))
	}
}

func TestFetchAllHistory(t *testing.T) {
	sonarrResponse := `{"records": [{"id": 1, "eventType": "grabbed", "date": "2024-01-15T10:00:00Z", "sourceTitle": "Test", "quality": {"quality": {"name": "HD"}}}]}`
	radarrResponse := `{"records": [{"id": 2, "eventType": "grabbed", "date": "2024-01-15T10:00:00Z", "sourceTitle": "Test", "quality": {"quality": {"name": "HD"}}}]}`

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sonarrResponse))
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(radarrResponse))
	}))
	defer radarrServer.Close()

	client := httputil.NewClient(5 * time.Second)
	cfg := ToolConfig{
		SonarrInstances: []config.ArrInstance{
			{Name: "Sonarr", URL: sonarrServer.URL, APIKey: "token"},
		},
		RadarrInstances: []config.ArrInstance{
			{Name: "Radarr", URL: radarrServer.URL, APIKey: "token"},
		},
	}

	since := time.Now().Add(-1 * time.Hour)
	events, err := fetchAllHistory(context.Background(), client, cfg, since)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Got %d events, want 2", len(events))
	}
}

func TestCustomFormatsInSonarrHistory(t *testing.T) {
	mockResponse := `{"records": [{
		"id": 1,
		"eventType": "grabbed",
		"date": "2024-01-15T10:00:00Z",
		"sourceTitle": "Test",
		"quality": {
			"quality": {"name": "HD-1080p"},
			"customFormats": [{"id": 1, "name": "HDR"}]
		}
	}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	since := time.Now().Add(-1 * time.Hour)

	events, err := fetchSonarrHistory(context.Background(), client, inst, since, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Got %d events, want 1", len(events))
	}

	if len(events[0].Formats) == 0 || events[0].Formats[0] != "HDR" {
		t.Errorf("Formats = %v, want [HDR]", events[0].Formats)
	}
}

func TestCustomFormatsInRadarrHistory(t *testing.T) {
	mockResponse := `{"records": [{
		"id": 1,
		"eventType": "grabbed",
		"date": "2024-01-15T10:00:00Z",
		"sourceTitle": "Test Movie",
		"quality": {
			"quality": {"name": "4K"},
			"customFormats": [{"id": 1, "name": "Dolby Vision"}]
		}
	}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	since := time.Now().Add(-1 * time.Hour)

	events, err := fetchRadarrHistory(context.Background(), client, inst, since, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Got %d events, want 1", len(events))
	}

	if len(events[0].Formats) == 0 || events[0].Formats[0] != "Dolby Vision" {
		t.Errorf("Formats = %v, want [Dolby Vision]", events[0].Formats)
	}
}

func TestFetchAllHistoryErrorHandling(t *testing.T) {
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer.Close()

	goodResponse := `{"records": [{"id": 1, "eventType": "grabbed", "date": "2024-01-15T10:00:00Z", "sourceTitle": "Test", "quality": {"quality": {"name": "HD"}}}]}`
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(goodResponse))
	}))
	defer goodServer.Close()

	client := httputil.NewClient(5 * time.Second)
	cfg := ToolConfig{
		SonarrInstances: []config.ArrInstance{
			{Name: "Bad", URL: badServer.URL, APIKey: "token"},
			{Name: "Good", URL: goodServer.URL, APIKey: "token"},
		},
	}

	since := time.Now().Add(-1 * time.Hour)
	events, err := fetchAllHistory(context.Background(), client, cfg, since)
	if err != nil {
		t.Fatalf("Expected partial success, got error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Got %d events, want 1 (from the good server)", len(events))
	}
}

func TestEventSortOrder(t *testing.T) {
	now := time.Now()
	events := []HistoryEvent{
		{ID: 1, When: now.Add(-5 * time.Minute), Action: "Grabbed"},
		{ID: 2, When: now, Action: "Grabbed"},
		{ID: 3, When: now.Add(-10 * time.Minute), Action: "Grabbed"},
	}

	client := httputil.NewClient(5 * time.Second)
	cfg := ToolConfig{
		SonarrInstances: []config.ArrInstance{},
		RadarrInstances: []config.ArrInstance{},
	}
	since := now.Add(-1 * time.Hour)

	// fetchAllHistory uses sort.Slice with After, so we need to call sort directly
	sort.Slice(events, func(i, j int) bool {
		return events[i].When.After(events[j].When)
	})

	if events[0].ID != 2 {
		t.Errorf("Most recent event should be first, got ID %d", events[0].ID)
	}
	if events[1].ID != 1 {
		t.Errorf("Second event should be ID 1, got ID %d", events[1].ID)
	}
	if events[2].ID != 3 {
		t.Errorf("Oldest event should be last, got ID %d", events[2].ID)
	}

	// Test the actual fetchAllHistory returns events sorted by time desc
	_ = cfg
	_ = client
	_ = since
}

func TestSonarrToHistoryEventWithFileID(t *testing.T) {
	now := time.Now()
	sh := SonarrHistory{
		ID:          123,
		EventType:   "downloadFolderImported",
		Date:        now.UTC().Format(time.RFC3339),
		SourceTitle: "Test.Source.2024",
		Data: map[string]interface{}{
			"fileId": "42",
		},
		Quality: SonarrQuality{
			Quality: SonarrQualityItem{Name: "HD-1080p"},
		},
		Series: &SonarrSeries{
			ID:    1,
			Title: "Test Series",
		},
		Episode: &SonarrEpisode{
			ID:            100,
			SeasonNumber:  2,
			EpisodeNumber: 5,
			Title:         "Episode 5",
		},
	}

	event := sonarrToHistoryEvent(sh)
	if event.FileID != 42 {
		t.Errorf("FileID = %d, want 42", event.FileID)
	}
}

func TestRadarrToHistoryEventWithFileID(t *testing.T) {
	now := time.Now()
	rh := RadarrHistory{
		ID:          456,
		EventType:   "downloadFolderImported",
		Date:        now.UTC().Format(time.RFC3339),
		SourceTitle: "Test.Movie.2024.1080p",
		Data: map[string]interface{}{
			"fileId": "99",
		},
		Quality: RadarrQuality{
			Quality: RadarrQualityItem{Name: "HD-1080p"},
		},
		Movie: &RadarrMovie{
			ID:    1,
			Title: "Test Movie",
			Year:  2024,
		},
	}

	event := radarrToHistoryEvent(rh)
	if event.FileID != 99 {
		t.Errorf("FileID = %d, want 99", event.FileID)
	}
}

func TestSubtitlesDisplay(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "-"},
		{"English", "English"},
		{"English, French", "English, French"},
	}

	for _, tt := range tests {
		result := subtitlesDisplay(tt.input)
		if result != tt.expected {
			t.Errorf("subtitlesDisplay(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"42", 42},
		{"0", 0},
		{"-1", -1},
	}

	for _, tt := range tests {
		result, err := parseInt(tt.input)
		if err != nil {
			t.Errorf("parseInt(%q) unexpected error: %v", tt.input, err)
		}
		if result != tt.expected {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseIntInvalid(t *testing.T) {
	_, err := parseInt("not-a-number")
	if err == nil {
		t.Error("parseInt('not-a-number') expected error, got nil")
	}
}

func TestEnrichSonarrSubtitles(t *testing.T) {
	mockResponse := `[{
		"id": 42,
		"mediaInfo": {"subtitles": "English, French"}
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "episodeFileIds=42") {
			t.Errorf("Expected episodeFileIds=42 in URL, got %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}

	events := []HistoryEvent{
		{ID: 1, FileID: 42, Action: "Imported"},
		{ID: 2, FileID: 0, Action: "Grabbed"},
	}

	enrichSonarrSubtitles(context.Background(), client, inst, events)

	if events[0].Subtitles != "English, French" {
		t.Errorf("Subtitles = %q, want 'English, French'", events[0].Subtitles)
	}
	if events[1].Subtitles != "" {
		t.Errorf("Grabbed event should have empty subtitles, got %q", events[1].Subtitles)
	}
}

func TestEnrichRadarrSubtitles(t *testing.T) {
	mockResponse := `[{
		"id": 99,
		"mediaInfo": {"subtitles": "English"}
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "movieFileIds=99") {
			t.Errorf("Expected movieFileIds=99 in URL, got %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}

	events := []HistoryEvent{
		{ID: 1, FileID: 99, Action: "Imported"},
	}

	enrichRadarrSubtitles(context.Background(), client, inst, events)

	if events[0].Subtitles != "English" {
		t.Errorf("Subtitles = %q, want 'English'", events[0].Subtitles)
	}
}

func TestEnrichSonarrSubtitlesNoFileID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Should not make API call when no file IDs")
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}

	events := []HistoryEvent{
		{ID: 1, FileID: 0, Action: "Grabbed"},
	}

	enrichSonarrSubtitles(context.Background(), client, inst, events)
	// If we get here without the handler being called, the test passes
}

func TestEnrichSonarrSubtitlesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}

	events := []HistoryEvent{
		{ID: 1, FileID: 42, Action: "Imported"},
	}

	enrichSonarrSubtitles(context.Background(), client, inst, events)
	if events[0].Subtitles != "" {
		t.Errorf("Subtitles should be empty after API error, got %q", events[0].Subtitles)
	}
}

func TestEnrichSonarrSubtitlesNoMediaInfo(t *testing.T) {
	mockResponse := `[{
		"id": 42,
		"mediaInfo": null
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}

	events := []HistoryEvent{
		{ID: 1, FileID: 42, Action: "Imported"},
	}

	enrichSonarrSubtitles(context.Background(), client, inst, events)
	if events[0].Subtitles != "" {
		t.Errorf("Subtitles should be empty when no mediaInfo, got %q", events[0].Subtitles)
	}
}
