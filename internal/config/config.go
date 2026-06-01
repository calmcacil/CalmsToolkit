package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TokenFromEnv returns the environment variable value if set, otherwise fallback.
func TokenFromEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ToolkitConfig is the top-level configuration structure for CalmsToolkit.
type ToolkitConfig struct {
	Version       int            `json:"version"`
	General       GeneralConfig  `json:"general"`
	Sonarr        []ArrInstance  `json:"sonarr_instances"`
	Radarr        []ArrInstance  `json:"radarr_instances"`
	MediaCalendar CalendarConfig `json:"media_calendar"`
	MediaStreams  StreamsConfig  `json:"media_streams"`
	MediaRequests RequestsConfig `json:"media_requests"`
	MediaAirtime  AirtimeConfig  `json:"media_airtime"`
	ArrFeed       FeedConfig     `json:"arr_feed"`
}

// ArrInstance represents a Sonarr or Radarr server instance.
type ArrInstance struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// GeneralConfig holds general toolkit settings.
type GeneralConfig struct {
	Timeout string `json:"timeout"`
	NoColor bool   `json:"no_color"`
	Theme   string `json:"theme"`
}

// CalendarConfig holds media calendar tool settings.
type CalendarConfig struct {
	Days          int  `json:"days"`
	DaysPast      int  `json:"days_past"`
	WatchInterval int  `json:"watch_interval"`
	Debug         bool `json:"debug"`
}

// StreamsConfig holds media streams monitoring configuration.
type StreamsConfig struct {
	PlexURL         string `json:"plex_url"`
	PlexToken       string `json:"plex_token"`
	JellyfinURL     string `json:"jellyfin_url"`
	JellyfinToken   string `json:"jellyfin_token"`
	ServerType      string `json:"server_type"`
	WatchInterval   int    `json:"watch_interval"`
	HistoryDuration string `json:"history_duration"`
}

// RequestsConfig holds media requests (Overseerr) configuration.
type RequestsConfig struct {
	OverseerrURL string `json:"overseerr_url"`
	APIKey       string `json:"api_key"`
	Verbose      bool   `json:"verbose"`
}

// AirtimeConfig holds media airtime search tool settings.
type AirtimeConfig struct {
	Limit      int  `json:"limit"`
	PastDays   int  `json:"past_days"`
	FutureDays int  `json:"future_days"`
	Debug      bool `json:"debug"`
}

// FeedConfig holds Arr event feed tool settings.
type FeedConfig struct {
	PollInterval  string `json:"poll_interval"`
	HistoryWindow string `json:"history_window"`
	ShowGrabbed   bool   `json:"show_grabbed"`
	ShowImported  bool   `json:"show_imported"`
	ShowFailed    bool   `json:"show_failed"`
	ShowDeleted   bool   `json:"show_deleted"`
	ShowIgnored   bool   `json:"show_ignored"`
	ShowSubtitles bool   `json:"show_subtitles"`
	MaxEvents     int    `json:"max_events"`
}

// ConfigPath returns the default config file path (~/.config/calmstoolkit/config.json).
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "calmstoolkit", "config.json")
}

// DefaultToolkitConfig returns a ToolkitConfig with sensible defaults.
func DefaultToolkitConfig() *ToolkitConfig {
	return &ToolkitConfig{
		Version: 1,
		General: GeneralConfig{
			Timeout: "10s",
			NoColor: false,
			Theme:   "default",
		},
		MediaCalendar: CalendarConfig{
			Days:          1,
			DaysPast:      0,
			WatchInterval: 300,
			Debug:         false,
		},
		MediaStreams: StreamsConfig{
			PlexURL:         "http://localhost:32400",
			PlexToken:       "",
			JellyfinURL:     "http://localhost:8096",
			JellyfinToken:   "",
			ServerType:      "both",
			WatchInterval:   10,
			HistoryDuration: "15m",
		},
		MediaRequests: RequestsConfig{
			OverseerrURL: "http://localhost:5055",
			APIKey:       "",
			Verbose:      false,
		},
		MediaAirtime: AirtimeConfig{
			Limit:      10,
			PastDays:   7,
			FutureDays: 30,
			Debug:      false,
		},
		ArrFeed: FeedConfig{
			PollInterval:  "5s",
			HistoryWindow: "1h",
			ShowGrabbed:   true,
			ShowImported:  true,
			ShowFailed:    true,
			ShowDeleted:   false,
			ShowIgnored:   false,
			ShowSubtitles: false,
			MaxEvents:     50,
		},
	}
}

// LoadToolkitConfig reads and parses the config file from the default path.
func LoadToolkitConfig() (*ToolkitConfig, error) {
	path := ConfigPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found at %s (run 'make setup')", path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultToolkitConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Version < 1 {
		return nil, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}

	for i := range cfg.Sonarr {
		cfg.Sonarr[i].URL = strings.TrimSuffix(cfg.Sonarr[i].URL, "/")
	}
	for i := range cfg.Radarr {
		cfg.Radarr[i].URL = strings.TrimSuffix(cfg.Radarr[i].URL, "/")
	}

	return cfg, nil
}

// Validate checks the configuration for required fields and valid values.
func (c *ToolkitConfig) Validate() error {
	if c.Version < 1 {
		return fmt.Errorf("unsupported version: %d", c.Version)
	}

	for i, inst := range c.Sonarr {
		if inst.URL == "" {
			return fmt.Errorf("sonarr_instances[%d]: url is required", i)
		}
		if inst.APIKey == "" {
			return fmt.Errorf("sonarr_instances[%d]: api_key is required", i)
		}
		if inst.Name == "" {
			return fmt.Errorf("sonarr_instances[%d]: name is required", i)
		}
	}
	for i, inst := range c.Radarr {
		if inst.URL == "" {
			return fmt.Errorf("radarr_instances[%d]: url is required", i)
		}
		if inst.APIKey == "" {
			return fmt.Errorf("radarr_instances[%d]: api_key is required", i)
		}
		if inst.Name == "" {
			return fmt.Errorf("radarr_instances[%d]: name is required", i)
		}
	}

	if _, err := time.ParseDuration(c.General.Timeout); err != nil {
		return fmt.Errorf("general.timeout: invalid duration %q: %w", c.General.Timeout, err)
	}
	if dt := c.MediaStreams.HistoryDuration; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			return fmt.Errorf("media_streams.history_duration: invalid duration %q: %w", dt, err)
		}
	}
	if dt := c.ArrFeed.PollInterval; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			return fmt.Errorf("arr_feed.poll_interval: invalid duration %q: %w", dt, err)
		}
	}
	if dt := c.ArrFeed.HistoryWindow; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			return fmt.Errorf("arr_feed.history_window: invalid duration %q: %w", dt, err)
		}
	}

	if c.ArrFeed.MaxEvents < 0 || c.ArrFeed.MaxEvents > 100 {
		return fmt.Errorf("arr_feed.max_events: must be between 0 and 100")
	}
	if c.MediaCalendar.Days < 0 {
		return fmt.Errorf("media_calendar.days: must be >= 0")
	}
	if c.MediaCalendar.WatchInterval < 1 {
		return fmt.Errorf("media_calendar.watch_interval: must be >= 1")
	}
	if c.MediaStreams.WatchInterval < 1 {
		return fmt.Errorf("media_streams.watch_interval: must be >= 1")
	}

	if c.MediaAirtime.Limit < 1 || c.MediaAirtime.Limit > 50 {
		return fmt.Errorf("media_airtime.limit: must be between 1 and 50")
	}
	if c.MediaAirtime.PastDays < 0 {
		return fmt.Errorf("media_airtime.past_days: must be >= 0")
	}
	if c.MediaAirtime.FutureDays < 0 {
		return fmt.Errorf("media_airtime.future_days: must be >= 0")
	}

	serverType := c.MediaStreams.ServerType
	if serverType != "" && serverType != "plex" && serverType != "jellyfin" && serverType != "both" {
		return fmt.Errorf("media_streams.server_type: must be 'plex', 'jellyfin', or 'both'")
	}

	return nil
}

// Save writes the configuration to the default config file path.
func (c *ToolkitConfig) Save() error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("setting config directory permissions: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("setting config file permissions: %w", err)
	}

	return nil
}
