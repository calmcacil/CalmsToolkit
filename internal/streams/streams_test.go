package streams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

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

func TestGetResolutionName(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		expected string
	}{
		{"4K resolution", 2160, "4K"},
		{"1080p resolution", 1080, "1080p"},
		{"720p resolution", 720, "720p"},
		{"480p resolution", 480, "480p"},
		{"Unknown low resolution", 360, "360p"},
		{"1440p resolution", 1440, "1440p"},
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

func TestGenerateSessionID(t *testing.T) {
	stream1 := StreamInfo{
		Server: "plex",
		User:   "alice",
		Title:  "Inception",
		Client: "Chrome",
	}

	stream2 := StreamInfo{
		Server: "plex",
		User:   "bob",
		Title:  "Inception",
		Client: "Chrome",
	}

	id1 := generateSessionID(stream1)
	id2 := generateSessionID(stream2)

	id1Again := generateSessionID(stream1)
	if id1 != id1Again {
		t.Errorf("Same stream generated different IDs: %v vs %v", id1, id1Again)
	}

	if id1 == id2 {
		t.Error("Different users generated same session ID")
	}
}

func TestBuildToolConfig(t *testing.T) {
	tk := config.DefaultToolkitConfig()
	tk.MediaStreams.ServerType = "plex"
	tk.MediaStreams.PlexURL = "http://plex.test.com/"
	tk.MediaStreams.PlexToken = "plex-test-token"
	tk.General.Timeout = "10s"

	cfg := BuildToolConfig(tk)

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

	nilCfg := BuildToolConfig(nil)
	if nilCfg.ServerType != "both" {
		t.Errorf("nil ServerType = %v, want 'both'", nilCfg.ServerType)
	}
	if nilCfg.Timeout != 10*time.Second {
		t.Errorf("nil Timeout = %v, want %v", nilCfg.Timeout, 10*time.Second)
	}
}

func TestBuildToolConfigDefaults(t *testing.T) {
	cfg := BuildToolConfig(config.DefaultToolkitConfig())
	if cfg.ServerType != "both" {
		t.Errorf("ServerType = %v, want 'both'", cfg.ServerType)
	}

	tk := config.DefaultToolkitConfig()
	tk.General.Timeout = "not-a-duration"
	cfg = BuildToolConfig(tk)
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 10*time.Second)
	}

	tk.MediaStreams.WatchInterval = 0
	cfg = BuildToolConfig(tk)
	if cfg.WatchSeconds != 10 {
		t.Errorf("WatchSeconds = %v, want 10", cfg.WatchSeconds)
	}

	tk.MediaStreams.HistoryDuration = "bad"
	cfg = BuildToolConfig(tk)
	if cfg.HistoryDuration != 15*time.Minute {
		t.Errorf("HistoryDuration = %v, want 15m", cfg.HistoryDuration)
	}
}

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
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Errorf("Expected X-Plex-Token header 'test-token', got %q", r.Header.Get("X-Plex-Token"))
		}
		if !strings.Contains(r.URL.Path, "/status/sessions") {
			t.Errorf("Expected /status/sessions in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(mockXML))
	}))
	defer server.Close()

	cfg := ToolConfig{
		CommonConfig: core.CommonConfig{Timeout: 5 * time.Second},
		PlexURL:      server.URL,
		PlexToken:    "test-token",
	}
	client := httputil.NewClient(cfg.Timeout)

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
				PositionTicks: 600000000,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-token" {
			t.Errorf("Expected X-API-Key header 'test-token', got %q", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept header 'application/json', got %q", r.Header.Get("Accept"))
		}
		if !strings.Contains(r.URL.Path, "/Sessions") {
			t.Errorf("Expected /Sessions in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := ToolConfig{
		CommonConfig:  core.CommonConfig{Timeout: 5 * time.Second},
		JellyfinURL:   server.URL,
		JellyfinToken: "test-token",
	}
	client := httputil.NewClient(cfg.Timeout)

	streams, err := fetchJellyfinStreams(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(streams) != 1 {
		t.Errorf("Got %d streams, want 2", len(streams))
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
			PositionTicks: 600000000,
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

func TestUpdateHistory(t *testing.T) {
	history := &SessionHistory{
		Records: make(map[string]*SessionRecord),
	}

	stream := StreamInfo{
		Server: "plex",
		User:   "alice",
		Title:  "Test Movie",
		Client: "Chrome",
	}

	updateHistory(history, []StreamInfo{stream}, 5*time.Minute)

	sessionID := generateSessionID(stream)
	if _, exists := history.Records[sessionID]; !exists {
		t.Error("Session not added to history")
	}

	record := history.Records[sessionID]
	if record.Stream.User != "alice" {
		t.Errorf("User = %v, want %v", record.Stream.User, "alice")
	}
	if record.EndTime != nil {
		t.Error("New session should still be active (EndTime should be nil)")
	}

	time.Sleep(10 * time.Millisecond)
	updateHistory(history, []StreamInfo{stream}, 5*time.Minute)

	record = history.Records[sessionID]
	if record.EndTime != nil {
		t.Error("Session should still be active")
	}

	updateHistory(history, []StreamInfo{}, 5*time.Minute)

	record = history.Records[sessionID]
	if record.EndTime == nil {
		t.Error("Session should be marked ended when stream stops")
	}
}

func TestGetActiveAndEndedSessions(t *testing.T) {
	now := time.Now()
	endTime := now.Add(-2 * time.Minute)
	history := &SessionHistory{
		Records: map[string]*SessionRecord{
			"active1": {
				Stream:    StreamInfo{User: "alice", Title: "Movie 1", Server: "plex", Client: "web"},
				StartTime: now,
				EndTime:   nil,
				SessionID: "active1",
			},
			"active2": {
				Stream:    StreamInfo{User: "bob", Title: "Movie 2", Server: "plex", Client: "web"},
				StartTime: now,
				EndTime:   nil,
				SessionID: "active2",
			},
			"ended1": {
				Stream:    StreamInfo{User: "charlie", Title: "Movie 3", Server: "plex", Client: "web"},
				StartTime: now.Add(-10 * time.Minute),
				EndTime:   &endTime,
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
