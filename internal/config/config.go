package config

import (
	"os"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Sonarr instances
	SonarrURLs   []string
	SonarrTokens []string

	// Radarr instances
	RadarrURLs   []string
	RadarrTokens []string

	// Global settings
	Timeout time.Duration
	NoColor bool
	Debug   bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := &Config{
		Timeout: 10 * time.Second,
		NoColor: false,
		Debug:   os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
	}

	// Try to load from .env file first
	envPaths := []string{".env", "/opt/apps/compose/.env"}
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			loadEnvFile(envPath, config)
			break // Use first found .env file
		}
	}

	// Environment variables override .env file
	// Support both plural (SONARR_URLS) and singular (SONARR_URL) variants
	if envURLs := os.Getenv("SONARR_URLS"); envURLs != "" {
		config.SonarrURLs = parseCommaSeparated(envURLs)
	} else if envURL := os.Getenv("SONARR_URL"); envURL != "" {
		// Fallback to singular SONARR_URL
		config.SonarrURLs = []string{envURL}
	}

	if envTokens := os.Getenv("SONARR_TOKENS"); envTokens != "" {
		config.SonarrTokens = parseCommaSeparated(envTokens)
	} else if envToken := os.Getenv("SONARR_API_TOKEN"); envToken != "" {
		// Fallback to singular SONARR_API_TOKEN
		config.SonarrTokens = []string{envToken}
	}

	if envURLs := os.Getenv("RADARR_URLS"); envURLs != "" {
		config.RadarrURLs = parseCommaSeparated(envURLs)
	} else if envURL := os.Getenv("RADARR_URL"); envURL != "" {
		// Fallback to singular RADARR_URL
		config.RadarrURLs = []string{envURL}
	}

	if envTokens := os.Getenv("RADARR_TOKENS"); envTokens != "" {
		config.RadarrTokens = parseCommaSeparated(envTokens)
	} else if envToken := os.Getenv("RADARR_API_TOKEN"); envToken != "" {
		// Fallback to singular RADARR_API_TOKEN
		config.RadarrTokens = []string{envToken}
	}

	// Check for NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		config.NoColor = true
	}

	// Clean URLs
	for i := range config.SonarrURLs {
		config.SonarrURLs[i] = strings.TrimSuffix(config.SonarrURLs[i], "/")
	}
	for i := range config.RadarrURLs {
		config.RadarrURLs[i] = strings.TrimSuffix(config.RadarrURLs[i], "/")
	}

	return config
}

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
		case "SONARR_URLS":
			config.SonarrURLs = parseCommaSeparated(value)
		case "SONARR_TOKENS":
			config.SonarrTokens = parseCommaSeparated(value)
		case "RADARR_URLS":
			config.RadarrURLs = parseCommaSeparated(value)
		case "RADARR_TOKENS":
			config.RadarrTokens = parseCommaSeparated(value)
		}
	}
}

func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
