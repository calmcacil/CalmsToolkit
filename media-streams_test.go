//go:build mediastreams
// +build mediastreams

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

// TestFormatTimeSince verifies time formatting
func TestFormatTimeSince(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "Just now",
			time:     now,
			expected: "1 second ago",
		},
		{
			name:     "5 minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "2 hours 30 minutes ago",
			time:     now.Add(-2*time.Hour - 30*time.Minute),
			expected: "2 hours ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-24 * time.Hour),
			expected: "24 hours ago",
		},
		{
			name:     "1 day 5 hours ago",
			time:     now.Add(-29 * time.Hour),
			expected: "29 hours ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeSince(tt.time)
			if got != tt.expected {
				t.Errorf("formatTimeSince() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestFormatDuration verifies duration formatting
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "Zero duration",
			duration: 0,
			expected: "0 seconds",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			expected: "30 seconds",
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "5 minutes 30 seconds",
			duration: 5*time.Minute + 30*time.Second,
			expected: "5m 30s",
		},
		{
			name:     "1 hour 30 minutes",
			duration: 90 * time.Minute,
			expected: "1h 30m",
		},
		{
			name:     "2 hours 15 minutes 45 seconds",
			duration: 2*time.Hour + 15*time.Minute + 45*time.Second,
			expected: "2h 15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("formatDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGetResolutionName verifies resolution naming
func TestGetResolutionName(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		expected string
	}{
		{"4K resolution", 2160, "4K"},
		{"1080p resolution", 1080, "1080p"},
		{"720p resolution", 720, "720p"},
		{"576p resolution", 576, "480p"}, // Falls through to 480p case
		{"480p resolution", 480, "480p"},
		{"Unknown low resolution", 360, "360p"},
		{"Unknown high resolution", 1440, "1440p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getResolutionName(tt.height)
			if got != tt.expected {
				t.Errorf("getResolutionName(%d) = %v, want %v", tt.height, got, tt.expected)
			}
		})
	}
}

// TestGenerateSessionID verifies session ID generation
func TestGenerateSessionID(t *testing.T) {
	stream1 := StreamInfo{
		User:  "alice",
		Title: "Inception",
		Year:  "2010",
	}

	stream2 := StreamInfo{
		User:  "bob",
		Title: "Inception",
		Year:  "2010",
	}

	stream3 := StreamInfo{
		User:  "alice",
		Title: "The Matrix",
		Year:  "1999",
	}

	id1 := generateSessionID(stream1)
	id2 := generateSessionID(stream2)
	id3 := generateSessionID(stream3)

	// Same session should generate same ID
	id1Again := generateSessionID(stream1)
	if id1 != id1Again {
		t.Errorf("Same stream generated different IDs: %v vs %v", id1, id1Again)
	}

	// Different users should generate different IDs
	if id1 == id2 {
		t.Error("Different users generated same session ID")
	}

	// Different titles should generate different IDs
	if id1 == id3 {
		t.Error("Different titles generated same session ID")
	}
}

// TestBuildStreamsToolConfig verifies config building from ToolkitConfig
func TestBuildStreamsToolConfig(t *testing.T) {
	tk := DefaultToolkitConfig()
	tk.MediaStreams.ServerType = "plex"
	tk.MediaStreams.PlexURL = "http://plex.test.com/"
	tk.MediaStreams.PlexToken = "plex-test-token"
	tk.General.Timeout = "10s"

	cfg := BuildStreamsToolConfig(tk)

	if cfg.ServerType != "plex" {
		t.Errorf("ServerType = %v, want %v", cfg.ServerType, "plex")
	}
	if cfg.PlexURL != "http://plex.test.com" {
		t.Errorf("PlexURL = %v, want %v", cfg.PlexURL, "http://plex.test.com")
	}
	if cfg.PlexToken != "plex-test-token" {
		t.Errorf("PlexToken = %v, want %v", cfg.PlexToken, "plex-test-token")
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 10*time.Second)
	}

	// Test nil config
	nilCfg := BuildStreamsToolConfig(nil)
	if nilCfg.ServerType != "both" {
		t.Errorf("nil ServerType = %v, want 'both'", nilCfg.ServerType)
	}
	if nilCfg.Timeout != 10*time.Second {
		t.Errorf("nil Timeout = %v, want %v", nilCfg.Timeout, 10*time.Second)
	}
}

// TestBuildStreamsToolConfigDefaults verifies defaults and edge cases
func TestBuildStreamsToolConfigDefaults(t *testing.T) {
	// Test empty ServerType defaults to "both"
	cfg := BuildStreamsToolConfig(DefaultToolkitConfig())
	if cfg.ServerType != "both" {
		t.Errorf("ServerType = %v, want 'both'", cfg.ServerType)
	}

	// Test invalid timeout defaults
	tk := DefaultToolkitConfig()
	tk.General.Timeout = "not-a-duration"
	cfg = BuildStreamsToolConfig(tk)
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 10*time.Second)
	}

	// Test WatchInterval <= 0 defaults
	tk.MediaStreams.WatchInterval = 0
	cfg = BuildStreamsToolConfig(tk)
	if cfg.WatchSeconds != 10 {
		t.Errorf("WatchSeconds = %v, want 10", cfg.WatchSeconds)
	}

	// Test invalid HistoryDuration defaults
	tk.MediaStreams.HistoryDuration = "bad"
	cfg = BuildStreamsToolConfig(tk)
	if cfg.HistoryDuration != 15*time.Minute {
		t.Errorf("HistoryDuration = %v, want 15m", cfg.HistoryDuration)
	}
}

// TestFetchPlexStreams verifies Plex stream fetching
func TestFetchPlexStreams(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video title="Test Movie" year="2024" type="movie">
    <User title="alice"/>
    <Player title="Chrome" device="Desktop"/>
    <Session bandwidth="5000"/>
    <Media videoResolution="1080" videoCodec="h264" audioCodec="aac"/>
  </Video>
</MediaContainer>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		// Plex uses token as query parameter
		if !strings.Contains(r.URL.RawQuery, "X-Plex-Token=test-token") {
			t.Errorf("Expected X-Plex-Token in query string, got %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.Path, "/status/sessions") {
			t.Errorf("Expected /status/sessions in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(mockXML))
	}))
	defer server.Close()

	cfg := StreamsToolConfig{
		PlexURL:   server.URL,
		PlexToken: "test-token",
		Timeout:   5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.Timeout}

	streams, err := fetchPlexStreams(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(streams) != 1 {
		t.Errorf("Got %d streams, want 1", len(streams))
	}

	if len(streams) > 0 {
		if streams[0].Title != "Test Movie" {
			t.Errorf("Title = %v, want %v", streams[0].Title, "Test Movie")
		}
		if streams[0].User != "alice" {
			t.Errorf("User = %v, want %v", streams[0].User, "alice")
		}
		if streams[0].Server != "plex" {
			t.Errorf("Server = %v, want %v", streams[0].Server, "plex")
		}
	}
}

// TestFetchJellyfinStreams verifies Jellyfin stream fetching
func TestFetchJellyfinStreams(t *testing.T) {
	mockResponse := []JellyfinSession{
		{
			UserName: "bob",
			Client:   "Web Player",
			NowPlayingItem: &JellyfinNowPlayingItem{
				Name:              "Test Show",
				ProductionYear:    2023,
				Type:              "Episode",
				SeriesName:        "Test Series",
				ParentIndexNumber: 1,
				IndexNumber:       5,
			},
			PlayState: JellyfinPlayState{
				PositionTicks: 600000000, // 1 minute in ticks
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		// Jellyfin uses api_key as query parameter
		if !strings.Contains(r.URL.RawQuery, "api_key=test-token") {
			t.Errorf("Expected api_key in query string, got %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.Path, "/Sessions") {
			t.Errorf("Expected /Sessions in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := StreamsToolConfig{
		JellyfinURL:   server.URL,
		JellyfinToken: "test-token",
		Timeout:       5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.Timeout}

	streams, err := fetchJellyfinStreams(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(streams) != 1 {
		t.Errorf("Got %d streams, want 1", len(streams))
	}

	if len(streams) > 0 {
		if streams[0].User != "bob" {
			t.Errorf("User = %v, want %v", streams[0].User, "bob")
		}
		if streams[0].Server != "jellyfin" {
			t.Errorf("Server = %v, want %v", streams[0].Server, "jellyfin")
		}
	}
}

// TestPlexVideoToStream verifies Plex video to stream conversion
func TestPlexVideoToStream(t *testing.T) {
	video := PlexVideo{
		Title:            "Inception",
		Year:             "2010",
		Type:             "movie",
		GrandparentTitle: "",
		User: PlexUser{
			Title: "alice",
		},
		Player: PlexPlayer{
			Title:  "Chrome",
			Device: "Desktop",
		},
		Session: PlexSession{
			Bandwidth: 5000,
		},
		Media: []PlexMedia{
			{
				VideoResolution: "1080",
				VideoCodec:      "h264",
				AudioCodec:      "aac",
			},
		},
	}

	stream := plexVideoToStream(video)

	if stream.Title != "Inception" {
		t.Errorf("Title = %v, want %v", stream.Title, "Inception")
	}
	if stream.Year != "2010" {
		t.Errorf("Year = %v, want %v", stream.Year, "2010")
	}
	if stream.User != "alice" {
		t.Errorf("User = %v, want %v", stream.User, "alice")
	}
	if stream.Server != "plex" {
		t.Errorf("Server = %v, want %v", stream.Server, "plex")
	}
	if stream.Client != "Chrome" {
		t.Errorf("Client = %v, want %v", stream.Client, "Chrome")
	}
}

// TestJellyfinSessionToStream verifies Jellyfin session to stream conversion
func TestJellyfinSessionToStream(t *testing.T) {
	session := JellyfinSession{
		UserName: "bob",
		Client:   "Web Player",
		NowPlayingItem: &JellyfinNowPlayingItem{
			Name:           "The Matrix",
			ProductionYear: 1999,
			Type:           "Movie",
		},
		PlayState: JellyfinPlayState{
			PositionTicks: 600000000, // 1 minute
			IsPaused:      false,
		},
		TranscodingInfo: &JellyfinTranscodingInfo{
			VideoCodec:       "h264",
			AudioCodec:       "aac",
			IsVideoDirect:    true,
			IsAudioDirect:    true,
			Height:           1080,
			TranscodeReasons: []string{},
		},
	}

	stream := jellyfinSessionToStream(session)

	if stream.Title != "The Matrix" {
		t.Errorf("Title = %v, want %v", stream.Title, "The Matrix")
	}
	if stream.Year != "1999" {
		t.Errorf("Year = %v, want %v", stream.Year, "1999")
	}
	if stream.User != "bob" {
		t.Errorf("User = %v, want %v", stream.User, "bob")
	}
	if stream.Server != "jellyfin" {
		t.Errorf("Server = %v, want %v", stream.Server, "jellyfin")
	}
	if stream.Client != "Web Player" {
		t.Errorf("Client = %v, want %v", stream.Client, "Web Player")
	}
}

// TestUpdateHistory verifies session history tracking
func TestUpdateHistory(t *testing.T) {
	history := &SessionHistory{
		Records: make(map[string]*SessionRecord),
	}

	stream := StreamInfo{
		User:   "alice",
		Title:  "Test Movie",
		Year:   "2024",
		Server: "plex",
	}

	// First update - should create new session
	updateHistory(history, []StreamInfo{stream}, 5*time.Minute)

	sessionID := generateSessionID(stream)
	if _, exists := history.Records[sessionID]; !exists {
		t.Error("Session not added to history")
	}

	// Verify session data
	record := history.Records[sessionID]
	if record.Stream.User != "alice" {
		t.Errorf("User = %v, want %v", record.Stream.User, "alice")
	}
	if record.EndTime != nil {
		t.Error("New session should still be active (EndTime should be nil)")
	}

	// Second update with same stream - should keep existing session active
	time.Sleep(10 * time.Millisecond)
	updateHistory(history, []StreamInfo{stream}, 5*time.Minute)

	record = history.Records[sessionID]
	if record.EndTime != nil {
		t.Error("Session should still be active")
	}

	// Update without stream - should mark as ended
	updateHistory(history, []StreamInfo{}, 5*time.Minute)

	record = history.Records[sessionID]
	if record.EndTime == nil {
		t.Error("Session should be marked ended when stream stops")
	}
}

// TestGetActiveAndEndedSessions verifies session filtering
func TestGetActiveAndEndedSessions(t *testing.T) {
	now := time.Now()
	endTime := now.Add(-2 * time.Minute)
	history := &SessionHistory{
		Records: map[string]*SessionRecord{
			"active1": {
				Stream:    StreamInfo{User: "alice", Title: "Movie 1"},
				StartTime: now,
				EndTime:   nil, // Active session
				SessionID: "active1",
			},
			"active2": {
				Stream:    StreamInfo{User: "bob", Title: "Movie 2"},
				StartTime: now,
				EndTime:   nil, // Active session
				SessionID: "active2",
			},
			"ended1": {
				Stream:    StreamInfo{User: "charlie", Title: "Movie 3"},
				StartTime: now.Add(-10 * time.Minute),
				EndTime:   &endTime, // Ended session
				SessionID: "ended1",
			},
		},
	}

	active, ended := getActiveAndEndedSessions(history)

	if len(active) != 2 {
		t.Errorf("Got %d active sessions, want 2", len(active))
	}
	if len(ended) != 1 {
		t.Errorf("Got %d ended sessions, want 1", len(ended))
	}
}
