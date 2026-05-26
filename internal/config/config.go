package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ToolkitConfig struct {
	Version       int            `json:"version"`
	General       GeneralConfig  `json:"general"`
	Sonarr        []ArrInstance  `json:"sonarr_instances"`
	Radarr        []ArrInstance  `json:"radarr_instances"`
	MediaCalendar CalendarConfig `json:"media_calendar"`
	MediaStreams  StreamsConfig  `json:"media_streams"`
	MediaRequests RequestsConfig `json:"media_requests"`
	ArrFeed       FeedConfig     `json:"arr_feed"`
}

type ArrInstance struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type GeneralConfig struct {
	Timeout string `json:"timeout"`
	NoColor bool   `json:"no_color"`
}

type CalendarConfig struct {
	Days          int  `json:"days"`
	DaysPast      int  `json:"days_past"`
	WatchInterval int  `json:"watch_interval"`
	Debug         bool `json:"debug"`
}

type StreamsConfig struct {
	PlexURL         string `json:"plex_url"`
	PlexToken       string `json:"plex_token"`
	JellyfinURL     string `json:"jellyfin_url"`
	JellyfinToken   string `json:"jellyfin_token"`
	ServerType      string `json:"server_type"`
	WatchInterval   int    `json:"watch_interval"`
	HistoryDuration string `json:"history_duration"`
}

type RequestsConfig struct {
	OverseerrURL string `json:"overseerr_url"`
	APIKey       string `json:"api_key"`
	Verbose      bool   `json:"verbose"`
}

type FeedConfig struct {
	PollInterval  string `json:"poll_interval"`
	HistoryWindow string `json:"history_window"`
	ShowGrabbed   bool   `json:"show_grabbed"`
	ShowImported  bool   `json:"show_imported"`
	ShowFailed    bool   `json:"show_failed"`
	ShowDeleted   bool   `json:"show_deleted"`
	ShowIgnored   bool   `json:"show_ignored"`
	MaxEvents     int    `json:"max_events"`
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "calmstoolkit", "config.json")
}

func DefaultToolkitConfig() *ToolkitConfig {
	return &ToolkitConfig{
		Version: 1,
		General: GeneralConfig{
			Timeout: "10s",
			NoColor: false,
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
		ArrFeed: FeedConfig{
			PollInterval:  "5s",
			HistoryWindow: "1h",
			ShowGrabbed:   true,
			ShowImported:  true,
			ShowFailed:    true,
			ShowDeleted:   false,
			ShowIgnored:   false,
			MaxEvents:     50,
		},
	}
}

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

	if cfg.Version != 1 {
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

func (c *ToolkitConfig) Validate() error {
	if c.Version != 1 {
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

	serverType := c.MediaStreams.ServerType
	if serverType != "" && serverType != "plex" && serverType != "jellyfin" && serverType != "both" {
		return fmt.Errorf("media_streams.server_type: must be 'plex', 'jellyfin', or 'both'")
	}

	return nil
}

func (c *ToolkitConfig) Save() error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
