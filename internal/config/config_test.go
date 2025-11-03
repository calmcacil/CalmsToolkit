package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Test default configuration
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Test default values
	if config.Global.Timeout != 30*time.Second {
		t.Errorf("Global.Timeout = %v, want %v", config.Global.Timeout, 30*time.Second)
	}

	if config.Global.NoColor != false {
		t.Errorf("Global.NoColor = %v, want %v", config.Global.NoColor, false)
	}

	if config.Calendar.Days != 7 {
		t.Errorf("Calendar.Days = %v, want %v", config.Calendar.Days, 7)
	}

	if config.Streams.ServerType != "both" {
		t.Errorf("Streams.ServerType = %v, want %v", config.Streams.ServerType, "both")
	}
}

func TestLoadConfigWithEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("NO_COLOR", "1")
	os.Setenv("DEBUG", "1")
	os.Setenv("OVERSEERR_URL", "http://test:5055")
	os.Setenv("OVERSEERR_TOKEN", "test-token")
	defer func() {
		os.Unsetenv("NO_COLOR")
		os.Unsetenv("DEBUG")
		os.Unsetenv("OVERSEERR_URL")
		os.Unsetenv("OVERSEERR_TOKEN")
	}()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if !config.Global.NoColor {
		t.Errorf("Global.NoColor = %v, want %v", config.Global.NoColor, true)
	}

	if !config.Global.Debug {
		t.Errorf("Global.Debug = %v, want %v", config.Global.Debug, true)
	}

	if config.MediaRequests.OverseerrURL != "http://test:5055" {
		t.Errorf("MediaRequests.OverseerrURL = %v, want %v", config.MediaRequests.OverseerrURL, "http://test:5055")
	}

	if config.MediaRequests.OverseerrKey != "test-token" {
		t.Errorf("MediaRequests.OverseerrKey = %v, want %v", config.MediaRequests.OverseerrKey, "test-token")
	}
}

func TestURLTrimming(t *testing.T) {
	os.Setenv("OVERSEERR_URL", "http://test:5055/")
	os.Setenv("PLEX_URL", "http://plex:32400/")
	defer func() {
		os.Unsetenv("OVERSEERR_URL")
		os.Unsetenv("PLEX_URL")
	}()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// URLs should be trimmed of trailing slashes
	if config.MediaRequests.OverseerrURL != "http://test:5055" {
		t.Errorf("MediaRequests.OverseerrURL = %v, want %v", config.MediaRequests.OverseerrURL, "http://test:5055")
	}

	if config.Streams.PlexURL != "http://plex:32400" {
		t.Errorf("Streams.PlexURL = %v, want %v", config.Streams.PlexURL, "http://plex:32400")
	}
}
