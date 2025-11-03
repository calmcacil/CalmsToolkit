package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Global        GlobalConfig        `json:"global" yaml:"global"`
	MediaRequests MediaRequestsConfig `json:"media_requests" yaml:"media_requests"`
	Streams       StreamsConfig       `json:"streams" yaml:"streams"`
	Calendar      CalendarConfig      `json:"calendar" yaml:"calendar"`
	Queue         QueueConfig         `json:"queue" yaml:"queue"`
	ArrFeed       ArrFeedConfig       `json:"arr_feed" yaml:"arr_feed"`
}

type GlobalConfig struct {
	NoColor bool          `json:"no_color" yaml:"no_color"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	Debug   bool          `json:"debug" yaml:"debug"`
}

type MediaRequestsConfig struct {
	OverseerrURL  string        `json:"overseerr_url" yaml:"overseerr_url"`
	OverseerrKey  string        `json:"overseerr_key" yaml:"overseerr_key"`
	JellyseerrURL string        `json:"jellyseerr_url" yaml:"jellyseerr_url"`
	JellyseerrKey string        `json:"jellyseerr_key" yaml:"jellyseerr_key"`
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
}

type StreamsConfig struct {
	PlexURL       string        `json:"plex_url" yaml:"plex_url"`
	PlexToken     string        `json:"plex_token" yaml:"plex_token"`
	JellyfinURL   string        `json:"jellyfin_url" yaml:"jellyfin_url"`
	JellyfinToken string        `json:"jellyfin_token" yaml:"jellyfin_token"`
	ServerType    string        `json:"server_type" yaml:"server_type"`
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
}

type CalendarConfig struct {
	OverseerrURL string        `json:"overseerr_url" yaml:"overseerr_url"`
	OverseerrKey string        `json:"overseerr_key" yaml:"overseerr_key"`
	Days         int           `json:"days" yaml:"days"`
	Timeout      time.Duration `json:"timeout" yaml:"timeout"`
}

type QueueConfig struct {
	SonarrURLs []string      `json:"sonarr_urls" yaml:"sonarr_urls"`
	SonarrKeys []string      `json:"sonarr_keys" yaml:"sonarr_keys"`
	RadarrURLs []string      `json:"radarr_urls" yaml:"radarr_urls"`
	RadarrKeys []string      `json:"radarr_keys" yaml:"radarr_keys"`
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
}

type ArrFeedConfig struct {
	SonarrURLs []string      `json:"sonarr_urls" yaml:"sonarr_urls"`
	SonarrKeys []string      `json:"sonarr_keys" yaml:"sonarr_keys"`
	RadarrURLs []string      `json:"radarr_urls" yaml:"radarr_urls"`
	RadarrKeys []string      `json:"radarr_keys" yaml:"radarr_keys"`
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
}

func LoadConfig() (*Config, error) {
	config := &Config{
		Global: GlobalConfig{
			NoColor: false,
			Timeout: 30 * time.Second,
			Debug:   false,
		},
		MediaRequests: MediaRequestsConfig{
			Timeout: 30 * time.Second,
		},
		Streams: StreamsConfig{
			ServerType: "both",
			Timeout:    30 * time.Second,
		},
		Calendar: CalendarConfig{
			Days:    7,
			Timeout: 30 * time.Second,
		},
		Queue: QueueConfig{
			Timeout: 30 * time.Second,
		},
		ArrFeed: ArrFeedConfig{
			Timeout: 30 * time.Second,
		},
	}

	// Load from .env file
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, config)
	}

	// Environment variables override .env file
	loadEnvironmentVariables(config)

	// Apply global settings
	if config.Global.Timeout > 0 {
		config.MediaRequests.Timeout = config.Global.Timeout
		config.Streams.Timeout = config.Global.Timeout
		config.Calendar.Timeout = config.Global.Timeout
		config.Queue.Timeout = config.Global.Timeout
		config.ArrFeed.Timeout = config.Global.Timeout
	}

	// Trim URL suffixes
	config.MediaRequests.OverseerrURL = strings.TrimSuffix(config.MediaRequests.OverseerrURL, "/")
	config.MediaRequests.JellyseerrURL = strings.TrimSuffix(config.MediaRequests.JellyseerrURL, "/")
	config.Streams.PlexURL = strings.TrimSuffix(config.Streams.PlexURL, "/")
	config.Streams.JellyfinURL = strings.TrimSuffix(config.Streams.JellyfinURL, "/")
	config.Calendar.OverseerrURL = strings.TrimSuffix(config.Calendar.OverseerrURL, "/")

	for i := range config.Queue.SonarrURLs {
		config.Queue.SonarrURLs[i] = strings.TrimSuffix(config.Queue.SonarrURLs[i], "/")
	}
	for i := range config.Queue.RadarrURLs {
		config.Queue.RadarrURLs[i] = strings.TrimSuffix(config.Queue.RadarrURLs[i], "/")
	}
	for i := range config.ArrFeed.SonarrURLs {
		config.ArrFeed.SonarrURLs[i] = strings.TrimSuffix(config.ArrFeed.SonarrURLs[i], "/")
	}
	for i := range config.ArrFeed.RadarrURLs {
		config.ArrFeed.RadarrURLs[i] = strings.TrimSuffix(config.ArrFeed.RadarrURLs[i], "/")
	}

	return config, nil
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
		case "OVERSEERR_URL", "JELLYSEERR_URL":
			config.MediaRequests.OverseerrURL = value
			config.Calendar.OverseerrURL = value
		case "OVERSEERR_TOKEN", "JELLYSEERR_TOKEN":
			config.MediaRequests.OverseerrKey = value
			config.Calendar.OverseerrKey = value
		case "PLEX_URL":
			config.Streams.PlexURL = value
		case "PLEX_TOKEN":
			config.Streams.PlexToken = value
		case "JELLYFIN_URL":
			config.Streams.JellyfinURL = value
		case "JELLYFIN_TOKEN":
			config.Streams.JellyfinToken = value
		case "SONARR_URLS":
			config.Queue.SonarrURLs = strings.Split(value, ",")
			config.ArrFeed.SonarrURLs = strings.Split(value, ",")
		case "SONARR_KEYS":
			config.Queue.SonarrKeys = strings.Split(value, ",")
			config.ArrFeed.SonarrKeys = strings.Split(value, ",")
		case "RADARR_URLS":
			config.Queue.RadarrURLs = strings.Split(value, ",")
			config.ArrFeed.RadarrURLs = strings.Split(value, ",")
		case "RADARR_KEYS":
			config.Queue.RadarrKeys = strings.Split(value, ",")
			config.ArrFeed.RadarrKeys = strings.Split(value, ",")
		}
	}
}

func loadEnvironmentVariables(config *Config) {
	if url := os.Getenv("OVERSEERR_URL"); url != "" {
		config.MediaRequests.OverseerrURL = url
		config.Calendar.OverseerrURL = url
	}
	if url := os.Getenv("JELLYSEERR_URL"); url != "" {
		config.MediaRequests.JellyseerrURL = url
	}
	if token := os.Getenv("OVERSEERR_TOKEN"); token != "" {
		config.MediaRequests.OverseerrKey = token
		config.Calendar.OverseerrKey = token
	}
	if token := os.Getenv("JELLYSEERR_TOKEN"); token != "" {
		config.MediaRequests.JellyseerrKey = token
	}
	if url := os.Getenv("PLEX_URL"); url != "" {
		config.Streams.PlexURL = url
	}
	if token := os.Getenv("PLEX_TOKEN"); token != "" {
		config.Streams.PlexToken = token
	}
	if url := os.Getenv("JELLYFIN_URL"); url != "" {
		config.Streams.JellyfinURL = url
	}
	if token := os.Getenv("JELLYFIN_TOKEN"); token != "" {
		config.Streams.JellyfinToken = token
	}
	if noColor := os.Getenv("NO_COLOR"); noColor != "" {
		config.Global.NoColor = true
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		config.Global.Debug = true
	}
}
