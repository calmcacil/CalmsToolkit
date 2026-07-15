package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	Version       int             `json:"version"`
	General       GeneralConfig   `json:"general"`
	Sonarr        []ArrInstance   `json:"sonarr_instances"`
	Radarr        []ArrInstance   `json:"radarr_instances"`
	MediaCalendar CalendarConfig  `json:"media_calendar"`
	MediaStreams  StreamsConfig   `json:"media_streams"`
	MediaRequests RequestsConfig  `json:"media_requests"`
	MediaAirtime  AirtimeConfig   `json:"media_airtime"`
	ArrFeed       FeedConfig      `json:"arr_feed"`
	AniSearch     AniSearchConfig `json:"anisearch"`
}

// CurrentVersion is unchanged until a configuration shape migration is needed.
const CurrentVersion = 1

type migration func(*ToolkitConfig) error

var migrations = map[int]migration{}

// Migrate applies registered one-version-at-a-time migrations in memory.
func Migrate(cfg *ToolkitConfig) error {
	if cfg.Version > CurrentVersion {
		return fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	for cfg.Version < CurrentVersion {
		migrate, ok := migrations[cfg.Version]
		if !ok {
			return fmt.Errorf("no migration registered from config version %d", cfg.Version)
		}
		if err := migrate(cfg); err != nil {
			return fmt.Errorf("migrating config version %d: %w", cfg.Version, err)
		}
		cfg.Version++
	}
	return nil
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

// AniSearchConfig holds anisearch tool settings.
type AniSearchConfig struct {
	MappingURL  string `json:"mapping_url"`
	MappingPath string `json:"mapping_path"`
	Limit       int    `json:"limit"`
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
	if path := strings.TrimSpace(os.Getenv("CALMSTOOLKIT_CONFIG")); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "calmstoolkit", "config.json")
}

// ResolvePath applies configuration path precedence: explicit flag,
// CALMSTOOLKIT_CONFIG, then the standard user configuration path.
func ResolvePath(explicit string) string {
	if path := strings.TrimSpace(explicit); path != "" {
		return path
	}
	return ConfigPath()
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
			ServerType:      "",
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
		AniSearch: AniSearchConfig{
			MappingURL:  "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst",
			MappingPath: "",
			Limit:       5,
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
	return LoadToolkitConfigAt("")
}

// LoadToolkitConfigAt loads a configuration file. An empty path uses
// CALMSTOOLKIT_CONFIG or the standard user path.
func LoadToolkitConfigAt(explicitPath string) (*ToolkitConfig, error) {
	path := ResolvePath(explicitPath)
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

	if err := Migrate(cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Sonarr {
		cfg.Sonarr[i].URL = strings.TrimSuffix(cfg.Sonarr[i].URL, "/")
	}
	for i := range cfg.Radarr {
		cfg.Radarr[i].URL = strings.TrimSuffix(cfg.Radarr[i].URL, "/")
	}

	ApplyEnvironment(cfg)
	return cfg, nil
}

var nonEnvName = regexp.MustCompile(`[^A-Z0-9]+`)

// NormalizeInstanceName converts an instance name into its environment suffix.
func NormalizeInstanceName(name string) string {
	return strings.Trim(nonEnvName.ReplaceAllString(strings.ToUpper(strings.TrimSpace(name)), "_"), "_")
}

// ApplyEnvironment overlays supported environment variables on cfg. New
// CALMSTOOLKIT names take precedence over legacy names.
func ApplyEnvironment(cfg *ToolkitConfig) {
	if cfg == nil {
		return
	}
	cfg.MediaStreams.PlexToken = firstEnvironment("CALMSTOOLKIT_PLEX_TOKEN", "PLEX_TOKEN", cfg.MediaStreams.PlexToken)
	cfg.MediaStreams.JellyfinToken = firstEnvironment("CALMSTOOLKIT_JELLYFIN_TOKEN", "JELLYFIN_TOKEN", cfg.MediaStreams.JellyfinToken)
	cfg.MediaRequests.APIKey = firstEnvironment("CALMSTOOLKIT_REQUESTS_API_KEY", "OVERSEERR_API_KEY", cfg.MediaRequests.APIKey)
	for i := range cfg.Sonarr {
		cfg.Sonarr[i].APIKey = firstEnvironment("CALMSTOOLKIT_SONARR_"+NormalizeInstanceName(cfg.Sonarr[i].Name)+"_API_KEY", "", cfg.Sonarr[i].APIKey)
	}
	for i := range cfg.Radarr {
		cfg.Radarr[i].APIKey = firstEnvironment("CALMSTOOLKIT_RADARR_"+NormalizeInstanceName(cfg.Radarr[i].Name)+"_API_KEY", "", cfg.Radarr[i].APIKey)
	}
}

func firstEnvironment(primary, legacy, fallback string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	if legacy != "" {
		if value := os.Getenv(legacy); value != "" {
			return value
		}
	}
	return fallback
}

// Validate checks the configuration for required fields and valid values.
func (c *ToolkitConfig) Validate() error {
	var problems []error
	add := func(format string, args ...any) { problems = append(problems, fmt.Errorf(format, args...)) }
	if c.Version < 1 {
		add("unsupported version: %d", c.Version)
	}

	seenSonarr := make(map[string]bool)
	for i, inst := range c.Sonarr {
		if inst.URL == "" {
			add("sonarr_instances[%d]: url is required", i)
		} else if !validHTTPURL(inst.URL) {
			add("sonarr_instances[%d]: invalid url %q", i, inst.URL)
		}
		if inst.APIKey == "" {
			add("sonarr_instances[%d]: api_key is required", i)
		}
		if inst.Name == "" {
			add("sonarr_instances[%d]: name is required", i)
		} else {
			key := strings.ToLower(strings.TrimSpace(inst.Name))
			if seenSonarr[key] {
				add("sonarr_instances[%d]: duplicate name %q", i, inst.Name)
			}
			seenSonarr[key] = true
		}
	}
	seenRadarr := make(map[string]bool)
	for i, inst := range c.Radarr {
		if inst.URL == "" {
			add("radarr_instances[%d]: url is required", i)
		} else if !validHTTPURL(inst.URL) {
			add("radarr_instances[%d]: invalid url %q", i, inst.URL)
		}
		if inst.APIKey == "" {
			add("radarr_instances[%d]: api_key is required", i)
		}
		if inst.Name == "" {
			add("radarr_instances[%d]: name is required", i)
		} else {
			key := strings.ToLower(strings.TrimSpace(inst.Name))
			if seenRadarr[key] {
				add("radarr_instances[%d]: duplicate name %q", i, inst.Name)
			}
			seenRadarr[key] = true
		}
	}

	if d, err := time.ParseDuration(c.General.Timeout); err != nil || d <= 0 {
		add("general.timeout: invalid positive duration %q", c.General.Timeout)
	}
	if dt := c.MediaStreams.HistoryDuration; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			add("media_streams.history_duration: invalid duration %q", dt)
		}
	}
	if dt := c.ArrFeed.PollInterval; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			add("arr_feed.poll_interval: invalid duration %q", dt)
		}
	}
	if dt := c.ArrFeed.HistoryWindow; dt != "" {
		if _, err := time.ParseDuration(dt); err != nil {
			add("arr_feed.history_window: invalid duration %q", dt)
		}
	}

	if c.ArrFeed.MaxEvents < 0 || c.ArrFeed.MaxEvents > 100 {
		add("arr_feed.max_events: must be between 0 and 100")
	}
	if c.MediaCalendar.Days < 0 {
		add("media_calendar.days: must be >= 0")
	}
	if c.MediaCalendar.WatchInterval < 1 {
		add("media_calendar.watch_interval: must be >= 1")
	}
	if c.MediaStreams.WatchInterval < 1 {
		add("media_streams.watch_interval: must be >= 1")
	}

	if c.MediaAirtime.Limit < 1 || c.MediaAirtime.Limit > 50 {
		add("media_airtime.limit: must be between 1 and 50")
	}
	if c.MediaAirtime.PastDays < 0 {
		add("media_airtime.past_days: must be >= 0")
	}
	if c.MediaAirtime.FutureDays < 0 {
		add("media_airtime.future_days: must be >= 0")
	}
	if c.AniSearch.Limit < 1 || c.AniSearch.Limit > 50 {
		add("anisearch.limit: must be between 1 and 50")
	}
	if c.MediaStreams.PlexURL != "" && !validHTTPURL(c.MediaStreams.PlexURL) {
		add("media_streams.plex_url: invalid url %q", c.MediaStreams.PlexURL)
	}
	if c.MediaStreams.JellyfinURL != "" && !validHTTPURL(c.MediaStreams.JellyfinURL) {
		add("media_streams.jellyfin_url: invalid url %q", c.MediaStreams.JellyfinURL)
	}
	if c.MediaRequests.OverseerrURL != "" && !validHTTPURL(c.MediaRequests.OverseerrURL) {
		add("media_requests.overseerr_url: invalid url %q", c.MediaRequests.OverseerrURL)
	}

	serverType := c.MediaStreams.ServerType
	if serverType != "" && serverType != "plex" && serverType != "jellyfin" && serverType != "both" {
		add("media_streams.server_type: must be 'plex', 'jellyfin', or 'both'")
	}
	if (serverType == "plex" || serverType == "both") && c.MediaStreams.PlexToken == "" {
		add("media_streams.plex_token: credential is required")
	}
	if (serverType == "jellyfin" || serverType == "both") && c.MediaStreams.JellyfinToken == "" {
		add("media_streams.jellyfin_token: credential is required")
	}

	return errors.Join(problems...)
}

func validHTTPURL(value string) bool {
	u, err := url.ParseRequestURI(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Save writes the configuration to the default config file path.
func (c *ToolkitConfig) Save() error {
	return c.SaveAt("")
}

// SaveAt atomically writes the configuration with owner-only permissions.
func (c *ToolkitConfig) SaveAt(explicitPath string) error {
	path := ResolvePath(explicitPath)
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

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("setting temporary config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replacing config: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("setting config file permissions: %w", err)
	}
	return nil
}
