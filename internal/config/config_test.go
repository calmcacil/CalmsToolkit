package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Test loading default configuration
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	// Test default values
	if config.Global.NoColor != false {
		t.Errorf("Expected NoColor to be false, got %v", config.Global.NoColor)
	}
	if config.Global.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout to be 30s, got %v", config.Global.Timeout)
	}
	if config.Global.Debug != false {
		t.Errorf("Expected Debug to be false, got %v", config.Global.Debug)
	}
}

func TestLoadEnvFile(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `# Test configuration
OVERSEERR_URL=http://test-overseerr:5055
OVERSEERR_TOKEN=test-token
PLEX_URL=http://test-plex:32400
PLEX_TOKEN=test-plex-token
NO_COLOR=true
`

	err := os.WriteFile(envPath, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	config := &Config{
		Global: GlobalConfig{
			NoColor: false,
			Timeout: 30 * time.Second,
			Debug:   false,
		},
	}

	loadEnvFile(config)

	// Test that values were loaded
	if config.MediaRequests.Overseerr.URL != "http://test-overseerr:5055" {
		t.Errorf("Expected Overseerr URL to be loaded, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.MediaRequests.Overseerr.Token != "test-token" {
		t.Errorf("Expected Overseerr token to be loaded, got %q", config.MediaRequests.Overseerr.Token)
	}
	if config.Streams.Plex.URL != "http://test-plex:32400" {
		t.Errorf("Expected Plex URL to be loaded, got %q", config.Streams.Plex.URL)
	}
	if config.Streams.Plex.Token != "test-plex-token" {
		t.Errorf("Expected Plex token to be loaded, got %q", config.Streams.Plex.Token)
	}
	if !config.Global.NoColor {
		t.Errorf("Expected NO_COLOR to be true, got %v", config.Global.NoColor)
	}
}

func TestLoadEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("OVERSEERR_URL", "http://env-overseerr:5055")
	os.Setenv("OVERSEERR_TOKEN", "env-token")
	os.Setenv("PLEX_URL", "http://env-plex:32400")
	os.Setenv("PLEX_TOKEN", "env-plex-token")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		os.Unsetenv("OVERSEERR_URL")
		os.Unsetenv("OVERSEERR_TOKEN")
		os.Unsetenv("PLEX_URL")
		os.Unsetenv("PLEX_TOKEN")
		os.Unsetenv("NO_COLOR")
	}()

	config := &Config{
		Global: GlobalConfig{
			NoColor: false,
			Timeout: 30 * time.Second,
			Debug:   false,
		},
	}

	loadEnvironmentVariables(config)

	// Test that values were loaded
	if config.MediaRequests.Overseerr.URL != "http://env-overseerr:5055" {
		t.Errorf("Expected Overseerr URL to be loaded from env, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.MediaRequests.Overseerr.Token != "env-token" {
		t.Errorf("Expected Overseerr token to be loaded from env, got %q", config.MediaRequests.Overseerr.Token)
	}
	if config.Streams.Plex.URL != "http://env-plex:32400" {
		t.Errorf("Expected Plex URL to be loaded from env, got %q", config.Streams.Plex.URL)
	}
	if config.Streams.Plex.Token != "env-plex-token" {
		t.Errorf("Expected Plex token to be loaded from env, got %q", config.Streams.Plex.Token)
	}
	if !config.Global.NoColor {
		t.Errorf("Expected NO_COLOR to be true from env, got %v", config.Global.NoColor)
	}
}

func TestApplyFlags(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{
			NoColor: false,
			Timeout: 30 * time.Second,
			Debug:   false,
		},
		MediaRequests: MediaRequestsConfig{
			Overseerr: OverseerrConfig{
				URL:   "http://original:5055",
				Token: "original-token",
			},
		},
	}

	flags := map[string]interface{}{
		"no-color":        true,
		"debug":           true,
		"timeout":         60 * time.Second,
		"overseerr-url":   "http://flag-overseerr:5055",
		"overseerr-token": "flag-token",
	}

	config.ApplyFlags(flags)

	// Test that flags were applied
	if !config.Global.NoColor {
		t.Errorf("Expected NoColor to be true from flag, got %v", config.Global.NoColor)
	}
	if !config.Global.Debug {
		t.Errorf("Expected Debug to be true from flag, got %v", config.Global.Debug)
	}
	if config.Global.Timeout != 60*time.Second {
		t.Errorf("Expected Timeout to be 60s from flag, got %v", config.Global.Timeout)
	}
	if config.MediaRequests.Overseerr.URL != "http://flag-overseerr:5055" {
		t.Errorf("Expected Overseerr URL to be from flag, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.MediaRequests.Overseerr.Token != "flag-token" {
		t.Errorf("Expected Overseerr token to be from flag, got %q", config.MediaRequests.Overseerr.Token)
	}
}

func TestSanitize(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{
			Timeout: 30 * time.Second,
		},
		MediaRequests: MediaRequestsConfig{
			Overseerr: OverseerrConfig{
				URL: "http://test-overseerr:5055/",
			},
		},
		Streams: StreamsConfig{
			Plex: PlexConfig{
				URL: "http://test-plex:32400/",
			},
			Jellyfin: JellyfinConfig{
				URL: "http://test-jellyfin:8096/",
			},
		},
		Calendar: CalendarConfig{
			Sonarr: []SonarrConfig{{
				URL: "http://test-sonarr:8989/",
			}},
			Radarr: []RadarrConfig{{
				URL: "http://test-radarr:7878/",
			}},
		},
	}

	config.Sanitize()

	// Test that trailing slashes were removed
	if config.MediaRequests.Overseerr.URL != "http://test-overseerr:5055" {
		t.Errorf("Expected trailing slash to be removed from Overseerr URL, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.Streams.Plex.URL != "http://test-plex:32400" {
		t.Errorf("Expected trailing slash to be removed from Plex URL, got %q", config.Streams.Plex.URL)
	}
	if config.Streams.Jellyfin.URL != "http://test-jellyfin:8096" {
		t.Errorf("Expected trailing slash to be removed from Jellyfin URL, got %q", config.Streams.Jellyfin.URL)
	}
	if config.Calendar.Sonarr[0].URL != "http://test-sonarr:8989" {
		t.Errorf("Expected trailing slash to be removed from Sonarr URL, got %q", config.Calendar.Sonarr[0].URL)
	}
	if config.Calendar.Radarr[0].URL != "http://test-radarr:7878" {
		t.Errorf("Expected trailing slash to be removed from Radarr URL, got %q", config.Calendar.Radarr[0].URL)
	}

	// Test that default timeouts were applied
	if config.MediaRequests.Overseerr.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be applied to Overseerr, got %v", config.MediaRequests.Overseerr.Timeout)
	}
	if config.Streams.Plex.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be applied to Plex, got %v", config.Streams.Plex.Timeout)
	}
	if config.Streams.Jellyfin.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be applied to Jellyfin, got %v", config.Streams.Jellyfin.Timeout)
	}
	if config.Calendar.Sonarr[0].Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be applied to Sonarr, got %v", config.Calendar.Sonarr[0].Timeout)
	}
	if config.Calendar.Radarr[0].Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be applied to Radarr, got %v", config.Calendar.Radarr[0].Timeout)
	}
}

func TestConfigPriority(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `OVERSEERR_URL=http://env-overseerr:5055
OVERSEERR_TOKEN=env-token
`

	err := os.WriteFile(envPath, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	// Set environment variables
	os.Setenv("OVERSEERR_URL", "http://env-var-overseerr:5055")
	os.Setenv("OVERSEERR_TOKEN", "env-var-token")
	defer func() {
		os.Unsetenv("OVERSEERR_URL")
		os.Unsetenv("OVERSEERR_TOKEN")
	}()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Environment variables should override .env file
	if config.MediaRequests.Overseerr.URL != "http://env-var-overseerr:5055" {
		t.Errorf("Expected environment variable to override .env file, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.MediaRequests.Overseerr.Token != "env-var-token" {
		t.Errorf("Expected environment variable to override .env file, got %q", config.MediaRequests.Overseerr.Token)
	}

	// Apply flags to test highest priority
	flags := map[string]interface{}{
		"overseerr-url":   "http://flag-overseerr:5055",
		"overseerr-token": "flag-token",
	}

	config.ApplyFlags(flags)

	// Flags should override everything
	if config.MediaRequests.Overseerr.URL != "http://flag-overseerr:5055" {
		t.Errorf("Expected flag to override environment variable, got %q", config.MediaRequests.Overseerr.URL)
	}
	if config.MediaRequests.Overseerr.Token != "flag-token" {
		t.Errorf("Expected flag to override environment variable, got %q", config.MediaRequests.Overseerr.Token)
	}
}
