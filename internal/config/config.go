package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the unified configuration for all tools
type Config struct {
	// Global settings
	Global GlobalConfig `json:"global" yaml:"global"`

	// Tool-specific settings
	MediaRequests MediaRequestsConfig `json:"media_requests" yaml:"media_requests"`
	Streams       StreamsConfig       `json:"streams" yaml:"streams"`
	Calendar      CalendarConfig      `json:"calendar" yaml:"calendar"`
	Queue         QueueConfig         `json:"queue" yaml:"queue"`
	ArrFeed       ArrFeedConfig       `json:"arr_feed" yaml:"arr_feed"`
}

// GlobalConfig contains global application settings
type GlobalConfig struct {
	NoColor bool          `json:"no_color" yaml:"no_color"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	Debug   bool          `json:"debug" yaml:"debug"`
}

// MediaRequestsConfig contains settings for the media requests tool
type MediaRequestsConfig struct {
	Overseerr  OverseerrConfig  `json:"overseerr" yaml:"overseerr"`
	Jellyseerr JellyseerrConfig `json:"jellyseerr" yaml:"jellyseerr"`
}

// StreamsConfig contains settings for the media streams tool
type StreamsConfig struct {
	Plex     PlexConfig     `json:"plex" yaml:"plex"`
	Jellyfin JellyfinConfig `json:"jellyfin" yaml:"jellyfin"`
}

// CalendarConfig contains settings for the media calendar tool
type CalendarConfig struct {
	Sonarr []SonarrConfig `json:"sonarr" yaml:"sonarr"`
	Radarr []RadarrConfig `json:"radarr" yaml:"radarr"`
}

// QueueConfig contains settings for the queue remediation tool
type QueueConfig struct {
	Sonarr []SonarrConfig `json:"sonarr" yaml:"sonarr"`
	Radarr []RadarrConfig `json:"radarr" yaml:"radarr"`
}

// ArrFeedConfig contains settings for the ARR feed tool
type ArrFeedConfig struct {
	Sonarr []SonarrConfig `json:"sonarr" yaml:"sonarr"`
	Radarr []RadarrConfig `json:"radarr" yaml:"radarr"`
}

// OverseerrConfig represents Overseerr/Jellyseerr configuration
type OverseerrConfig struct {
	URL     string        `json:"url" yaml:"url"`
	Token   string        `json:"token" yaml:"token"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// JellyseerrConfig represents Jellyseerr-specific configuration
type JellyseerrConfig struct {
	URL     string        `json:"url" yaml:"url"`
	Token   string        `json:"token" yaml:"token"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// PlexConfig represents Plex server configuration
type PlexConfig struct {
	URL     string        `json:"url" yaml:"url"`
	Token   string        `json:"token" yaml:"token"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// JellyfinConfig represents Jellyfin server configuration
type JellyfinConfig struct {
	URL     string        `json:"url" yaml:"url"`
	Token   string        `json:"token" yaml:"token"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// SonarrConfig represents Sonarr instance configuration
type SonarrConfig struct {
	Name    string        `json:"name" yaml:"name"`
	URL     string        `json:"url" yaml:"url"`
	APIKey  string        `json:"api_key" yaml:"api_key"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// RadarrConfig represents Radarr instance configuration
type RadarrConfig struct {
	Name    string        `json:"name" yaml:"name"`
	URL     string        `json:"url" yaml:"url"`
	APIKey  string        `json:"api_key" yaml:"api_key"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// LoadConfig loads configuration from multiple sources with priority:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration file (~/.config/calms-toolkit/config.yaml)
// 4. .env file (legacy support)
// 5. Default values (lowest priority)
func LoadConfig() (*Config, error) {
	config := &Config{
		Global: GlobalConfig{
			NoColor: false,
			Timeout: 30 * time.Second,
			Debug:   false,
		},
	}

	// Load from configuration file
	if err := loadConfigFile(config); err != nil {
		// Don't fail if config file doesn't exist, just continue with defaults
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}

	// Load from .env file (legacy support)
	loadEnvFile(config)

	// Load from environment variables
	loadEnvironmentVariables(config)

	return config, nil
}

// loadConfigFile loads configuration from the config file
func loadConfigFile(config *Config) error {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "calms-toolkit", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Config file doesn't exist, that's OK
	}

	// TODO: Implement YAML parsing when needed
	// For now, we'll rely on environment variables and .env file
	return nil
}

// loadEnvFile loads configuration from a .env file
func loadEnvFile(config *Config) {
	envPaths := []string{
		"/opt/apps/compose/.env", // Default path from existing tools
		".env",                   // Local .env file
		".env.local",             // Local override
	}

	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err != nil {
			continue
		}

		file, err := os.Open(envPath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

			// Map .env variables to config
			switch key {
			// Media Requests / Overseerr
			case "OVERSEERR_URL", "JELLYSEERR_URL":
				config.MediaRequests.Overseerr.URL = value
			case "OVERSEERR_TOKEN", "JELLYSEERR_TOKEN":
				config.MediaRequests.Overseerr.Token = value

			// Streams / Plex
			case "PLEX_URL":
				config.Streams.Plex.URL = value
			case "PLEX_TOKEN":
				config.Streams.Plex.Token = value

			// Streams / Jellyfin
			case "JELLYFIN_URL":
				config.Streams.Jellyfin.URL = value
			case "JELLYFIN_TOKEN":
				config.Streams.Jellyfin.Token = value

			// Calendar / Sonarr
			case "SONARR_URL":
				if len(config.Calendar.Sonarr) == 0 {
					config.Calendar.Sonarr = []SonarrConfig{{}}
				}
				config.Calendar.Sonarr[0].URL = value
			case "SONARR_API_KEY":
				if len(config.Calendar.Sonarr) == 0 {
					config.Calendar.Sonarr = []SonarrConfig{{}}
				}
				config.Calendar.Sonarr[0].APIKey = value

			// Calendar / Radarr
			case "RADARR_URL":
				if len(config.Calendar.Radarr) == 0 {
					config.Calendar.Radarr = []RadarrConfig{{}}
				}
				config.Calendar.Radarr[0].URL = value
			case "RADARR_API_KEY":
				if len(config.Calendar.Radarr) == 0 {
					config.Calendar.Radarr = []RadarrConfig{{}}
				}
				config.Calendar.Radarr[0].APIKey = value

			// Global settings
			case "NO_COLOR":
				config.Global.NoColor = strings.ToLower(value) == "true" || value == "1"
			}
		}
	}
}

// loadEnvironmentVariables loads configuration from environment variables
func loadEnvironmentVariables(config *Config) {
	// Media Requests / Overseerr
	if url := os.Getenv("OVERSEERR_URL"); url != "" {
		config.MediaRequests.Overseerr.URL = url
	}
	if url := os.Getenv("JELLYSEERR_URL"); url != "" {
		config.MediaRequests.Overseerr.URL = url
	}
	if token := os.Getenv("OVERSEERR_TOKEN"); token != "" {
		config.MediaRequests.Overseerr.Token = token
	}
	if token := os.Getenv("JELLYSEERR_TOKEN"); token != "" {
		config.MediaRequests.Overseerr.Token = token
	}

	// Streams / Plex
	if url := os.Getenv("PLEX_URL"); url != "" {
		config.Streams.Plex.URL = url
	}
	if token := os.Getenv("PLEX_TOKEN"); token != "" {
		config.Streams.Plex.Token = token
	}

	// Streams / Jellyfin
	if url := os.Getenv("JELLYFIN_URL"); url != "" {
		config.Streams.Jellyfin.URL = url
	}
	if token := os.Getenv("JELLYFIN_TOKEN"); token != "" {
		config.Streams.Jellyfin.Token = token
	}

	// Calendar / Sonarr
	if url := os.Getenv("SONARR_URL"); url != "" {
		if len(config.Calendar.Sonarr) == 0 {
			config.Calendar.Sonarr = []SonarrConfig{{}}
		}
		config.Calendar.Sonarr[0].URL = url
	}
	if apiKey := os.Getenv("SONARR_API_KEY"); apiKey != "" {
		if len(config.Calendar.Sonarr) == 0 {
			config.Calendar.Sonarr = []SonarrConfig{{}}
		}
		config.Calendar.Sonarr[0].APIKey = apiKey
	}

	// Calendar / Radarr
	if url := os.Getenv("RADARR_URL"); url != "" {
		if len(config.Calendar.Radarr) == 0 {
			config.Calendar.Radarr = []RadarrConfig{{}}
		}
		config.Calendar.Radarr[0].URL = url
	}
	if apiKey := os.Getenv("RADARR_API_KEY"); apiKey != "" {
		if len(config.Calendar.Radarr) == 0 {
			config.Calendar.Radarr = []RadarrConfig{{}}
		}
		config.Calendar.Radarr[0].APIKey = apiKey
	}

	// Global settings
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		config.Global.NoColor = true
	}
}

// ApplyFlags applies command-line flag overrides to the configuration
func (c *Config) ApplyFlags(flags map[string]interface{}) {
	for key, value := range flags {
		switch key {
		case "no-color":
			if noColor, ok := value.(bool); ok {
				c.Global.NoColor = noColor
			}
		case "debug":
			if debug, ok := value.(bool); ok {
				c.Global.Debug = debug
			}
		case "timeout":
			if timeout, ok := value.(time.Duration); ok {
				c.Global.Timeout = timeout
			}
		case "overseerr-url":
			if url, ok := value.(string); ok {
				c.MediaRequests.Overseerr.URL = url
			}
		case "overseerr-token":
			if token, ok := value.(string); ok {
				c.MediaRequests.Overseerr.Token = token
			}
		case "plex-url":
			if url, ok := value.(string); ok {
				c.Streams.Plex.URL = url
			}
		case "plex-token":
			if token, ok := value.(string); ok {
				c.Streams.Plex.Token = token
			}
		case "jellyfin-url":
			if url, ok := value.(string); ok {
				c.Streams.Jellyfin.URL = url
			}
		case "jellyfin-token":
			if token, ok := value.(string); ok {
				c.Streams.Jellyfin.Token = token
			}
		}
	}
}

// Sanitize cleans up configuration values
func (c *Config) Sanitize() {
	// Trim trailing slashes from URLs
	c.MediaRequests.Overseerr.URL = strings.TrimSuffix(c.MediaRequests.Overseerr.URL, "/")
	c.Streams.Plex.URL = strings.TrimSuffix(c.Streams.Plex.URL, "/")
	c.Streams.Jellyfin.URL = strings.TrimSuffix(c.Streams.Jellyfin.URL, "/")

	for i := range c.Calendar.Sonarr {
		c.Calendar.Sonarr[i].URL = strings.TrimSuffix(c.Calendar.Sonarr[i].URL, "/")
	}
	for i := range c.Calendar.Radarr {
		c.Calendar.Radarr[i].URL = strings.TrimSuffix(c.Calendar.Radarr[i].URL, "/")
	}
	for i := range c.Queue.Sonarr {
		c.Queue.Sonarr[i].URL = strings.TrimSuffix(c.Queue.Sonarr[i].URL, "/")
	}
	for i := range c.Queue.Radarr {
		c.Queue.Radarr[i].URL = strings.TrimSuffix(c.Queue.Radarr[i].URL, "/")
	}
	for i := range c.ArrFeed.Sonarr {
		c.ArrFeed.Sonarr[i].URL = strings.TrimSuffix(c.ArrFeed.Sonarr[i].URL, "/")
	}
	for i := range c.ArrFeed.Radarr {
		c.ArrFeed.Radarr[i].URL = strings.TrimSuffix(c.ArrFeed.Radarr[i].URL, "/")
	}

	// Set default timeouts if not specified
	if c.MediaRequests.Overseerr.Timeout == 0 {
		c.MediaRequests.Overseerr.Timeout = c.Global.Timeout
	}
	if c.Streams.Plex.Timeout == 0 {
		c.Streams.Plex.Timeout = c.Global.Timeout
	}
	if c.Streams.Jellyfin.Timeout == 0 {
		c.Streams.Jellyfin.Timeout = c.Global.Timeout
	}
	for i := range c.Calendar.Sonarr {
		if c.Calendar.Sonarr[i].Timeout == 0 {
			c.Calendar.Sonarr[i].Timeout = c.Global.Timeout
		}
	}
	for i := range c.Calendar.Radarr {
		if c.Calendar.Radarr[i].Timeout == 0 {
			c.Calendar.Radarr[i].Timeout = c.Global.Timeout
		}
	}
}
