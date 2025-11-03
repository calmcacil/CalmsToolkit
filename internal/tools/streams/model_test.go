package streams

import (
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
)

func TestModel_NewModel(t *testing.T) {
	cfg := &config.Config{
		RefreshInterval: 5 * time.Second,
		HistoryDuration: 10 * time.Minute,
		NoColor:         true,
	}

	model := NewModel(cfg)

	if model.config != cfg {
		t.Errorf("Expected config to be set, got nil")
	}

	if model.api == nil {
		t.Errorf("Expected API to be initialized, got nil")
	}

	if model.history == nil {
		t.Errorf("Expected history to be initialized, got nil")
	}

	if !model.loading {
		t.Errorf("Expected loading to be true initially, got false")
	}
}

func TestSessionHistory_NewSessionHistory(t *testing.T) {
	history := NewSessionHistory()

	if history.Records == nil {
		t.Errorf("Expected Records map to be initialized, got nil")
	}

	if len(history.Records) != 0 {
		t.Errorf("Expected empty Records map, got %d records", len(history.Records))
	}
}

func TestColors_New(t *testing.T) {
	// Test with colors enabled
	colorsEnabled := colors.New(false)
	_ = colorsEnabled // Just test that it doesn't panic

	// Test with colors disabled
	colorsDisabled := colors.New(true)
	_ = colorsDisabled // Just test that it doesn't panic
}

func TestColors_ServerColor(t *testing.T) {
	c := colors.New(false)

	plexColor := c.ServerColor("plex")
	if plexColor != colors.Yellow {
		t.Errorf("Expected plex color to be %s, got %s", colors.Yellow, plexColor)
	}

	jellyfinColor := c.ServerColor("jellyfin")
	if jellyfinColor != colors.Magenta {
		t.Errorf("Expected jellyfin color to be %s, got %s", colors.Magenta, jellyfinColor)
	}

	defaultColor := c.ServerColor("unknown")
	if defaultColor != colors.Cyan {
		t.Errorf("Expected default color to be %s, got %s", colors.Cyan, defaultColor)
	}
}

func TestColors_StatusColor(t *testing.T) {
	c := colors.New(false)

	transcodingColor := c.StatusColor(true)
	if transcodingColor != colors.Red {
		t.Errorf("Expected transcoding color to be %s, got %s", colors.Red, transcodingColor)
	}

	directColor := c.StatusColor(false)
	if directColor != colors.Green {
		t.Errorf("Expected direct color to be %s, got %s", colors.Green, directColor)
	}
}

func TestStreamInfo_GenerateSessionID(t *testing.T) {
	stream := StreamInfo{
		Server: "plex",
		User:   "testuser",
		Title:  "Test Movie",
		Client: "Plex Web",
	}

	expected := "plex:testuser:Test Movie:Plex Web"
	actual := generateSessionID(stream)

	if actual != expected {
		t.Errorf("Expected session ID %s, got %s", expected, actual)
	}
}

func TestSessionRecord_Duration(t *testing.T) {
	start := time.Now()
	end := start.Add(5*time.Minute + 30*time.Second)

	record := SessionRecord{
		StartTime: start,
		EndTime:   &end,
	}

	duration := record.EndTime.Sub(record.StartTime)
	expected := 5*time.Minute + 30*time.Second

	if duration != expected {
		t.Errorf("Expected duration %v, got %v", expected, duration)
	}
}

func TestModel_UpdateHistory(t *testing.T) {
	cfg := &config.Config{
		HistoryDuration: 5 * time.Minute,
	}

	model := NewModel(cfg)

	// Add some test streams
	streams := []StreamInfo{
		{
			Server: "plex",
			User:   "user1",
			Title:  "Movie 1",
			Client: "Client 1",
		},
		{
			Server: "jellyfin",
			User:   "user2",
			Title:  "Movie 2",
			Client: "Client 2",
		},
	}

	model.updateHistory(streams)

	// Check that sessions were added
	if len(model.history.Records) != 2 {
		t.Errorf("Expected 2 records in history, got %d", len(model.history.Records))
	}

	// Simulate one stream ending
	streams = []StreamInfo{
		streams[0], // Only first stream remains
	}

	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamp
	model.updateHistory(streams)

	// Should still have 2 records, but one ended
	if len(model.history.Records) != 2 {
		t.Errorf("Expected 2 records in history after one ended, got %d", len(model.history.Records))
	}

	// Check that one session is marked as ended
	endedCount := 0
	for _, record := range model.history.Records {
		if record.EndTime != nil {
			endedCount++
		}
	}

	if endedCount != 1 {
		t.Errorf("Expected 1 ended session, got %d", endedCount)
	}
}
