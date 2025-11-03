package config

import (
	"os"
	"strings"
	"time"
)

// Config holds application configuration
type Config struct {
	Overseerr OverseerrConfig `json:"overseerr"`
	Plex      PlexConfig      `json:"plex"`
	Jellyfin  JellyfinConfig  `json:"jellyfin"`
	Sonarr    []SonarrConfig  `json:"sonarr"`
	Radarr    []RadarrConfig  `json:"radarr"`
	NoColor   bool            `json:"noColor"`
	Timeout   time.Duration   `json:"timeout"`
	Debug     bool            `json:"debug"`
}

// OverseerrConfig holds Overseerr/Jellyseerr configuration
type OverseerrConfig struct {
	URL     string        `json:"url"`
	Token   string        `json:"token"`
	Timeout time.Duration `json:"timeout"`
}

// PlexConfig holds Plex configuration
type PlexConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// JellyfinConfig holds Jellyfin configuration
type JellyfinConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// SonarrConfig holds Sonarr configuration
type SonarrConfig struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

// RadarrConfig holds Radarr configuration
type RadarrConfig struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() *Config {
	config := &Config{
		Timeout: 30 * time.Second,
		NoColor: false,
		Debug:   false,
	}

	// Load from .env file
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, config)
	}

	// Environment variables override .env file
	if envURL := os.Getenv("OVERSEERR_URL"); envURL != "" {
		config.Overseerr.URL = envURL
	} else if envURL := os.Getenv("JELLYSEERR_URL"); envURL != "" {
		config.Overseerr.URL = envURL
	}

	if envToken := os.Getenv("OVERSEERR_TOKEN"); envToken != "" {
		config.Overseerr.Token = envToken
	} else if envToken := os.Getenv("JELLYSEERR_TOKEN"); envToken != "" {
		config.Overseerr.Token = envToken
	}

	// Global settings
	if os.Getenv("NO_COLOR") != "" {
		config.NoColor = true
	}

	if os.Getenv("DEBUG") != "" {
		config.Debug = true
	}

	// Clean up URLs
	config.Overseerr.URL = strings.TrimSuffix(config.Overseerr.URL, "/")
	config.Plex.URL = strings.TrimSuffix(config.Plex.URL, "/")
	config.Jellyfin.URL = strings.TrimSuffix(config.Jellyfin.URL, "/")

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
		case "OVERSEERR_URL", "JELLYSEERR_URL":
			config.Overseerr.URL = value
		case "OVERSEERR_TOKEN", "JELLYSEERR_TOKEN":
			config.Overseerr.Token = value
		case "PLEX_URL":
			config.Plex.URL = value
		case "PLEX_TOKEN":
			config.Plex.Token = value
		case "JELLYFIN_URL":
			config.Jellyfin.URL = value
		case "JELLYFIN_TOKEN":
			config.Jellyfin.Token = value
		}
	}
}
