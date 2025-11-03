package config

import (
	"os"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Server settings
	ServerType    string
	PlexURL       string
	PlexToken     string
	JellyfinURL   string
	JellyfinToken string

	// General settings
	Timeout         time.Duration
	NoColor         bool
	HistoryDuration time.Duration

	// TUI specific settings
	RefreshInterval time.Duration
}

// LoadConfig loads configuration from environment variables and defaults
func LoadConfig() *Config {
	config := &Config{
		ServerType:      "both",
		PlexURL:         "http://localhost:32400",
		PlexToken:       "",
		JellyfinURL:     "http://localhost:8096",
		JellyfinToken:   "",
		Timeout:         10 * time.Second,
		NoColor:         false,
		HistoryDuration: 5 * time.Minute,
		RefreshInterval: 5 * time.Second,
	}

	// Load from .env file if it exists
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, config)
	}

	// Environment variables override .env file
	if envURL := os.Getenv("PLEX_URL"); envURL != "" {
		config.PlexURL = envURL
	}
	if envToken := os.Getenv("PLEX_TOKEN"); envToken != "" {
		config.PlexToken = envToken
	}
	if envURL := os.Getenv("JELLYFIN_URL"); envURL != "" {
		config.JellyfinURL = envURL
	}
	if envToken := os.Getenv("JELLYFIN_TOKEN"); envToken != "" {
		config.JellyfinToken = envToken
	}
	if envNoColor := os.Getenv("NO_COLOR"); envNoColor != "" {
		config.NoColor = true
	}

	// Clean up URLs
	config.PlexURL = strings.TrimSuffix(config.PlexURL, "/")
	config.JellyfinURL = strings.TrimSuffix(config.JellyfinURL, "/")

	return config
}

// loadEnvFile loads configuration from a .env file
func loadEnvFile(path string, config *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "PLEX_URL":
			config.PlexURL = value
		case "PLEX_TOKEN":
			config.PlexToken = value
		case "JELLYFIN_URL":
			config.JellyfinURL = value
		case "JELLYFIN_TOKEN":
			config.JellyfinToken = value
		}
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ServerType == "plex" || c.ServerType == "both" {
		if c.PlexToken == "" {
			return ErrMissingPlexToken
		}
	}
	if c.ServerType == "jellyfin" || c.ServerType == "both" {
		if c.JellyfinToken == "" {
			return ErrMissingJellyfinToken
		}
	}
	return nil
}

// Configuration errors
var (
	ErrMissingPlexToken     = &ConfigError{"PLEX_TOKEN is required for Plex server"}
	ErrMissingJellyfinToken = &ConfigError{"JELLYFIN_TOKEN is required for Jellyfin server"}
)

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
