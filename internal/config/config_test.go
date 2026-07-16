package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultToolkitConfig(t *testing.T) {
	cfg := DefaultToolkitConfig()
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
	if cfg.General.Timeout != "10s" {
		t.Errorf("Timeout = %q, want %q", cfg.General.Timeout, "10s")
	}
	if cfg.MediaCalendar.Days != 1 {
		t.Errorf("Days = %d, want 1", cfg.MediaCalendar.Days)
	}
	if cfg.MediaStreams.ServerType != "" {
		t.Errorf("ServerType = %q, want empty until a service is configured", cfg.MediaStreams.ServerType)
	}
	if cfg.MediaRequests.OverseerrURL != "http://localhost:5055" {
		t.Errorf("OverseerrURL = %q, want %q", cfg.MediaRequests.OverseerrURL, "http://localhost:5055")
	}
	if cfg.ArrFeed.PollInterval != "5s" {
		t.Errorf("PollInterval = %q, want %q", cfg.ArrFeed.PollInterval, "5s")
	}
}

func TestValidateAggregatesProblems(t *testing.T) {
	cfg := DefaultToolkitConfig()
	cfg.General.Timeout = "bad"
	cfg.Sonarr = []ArrInstance{{Name: "Same", URL: "not-url"}, {Name: "same", URL: "also-bad"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{"general.timeout", "invalid url", "duplicate name", "api_key"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestApplyEnvironmentPrecedence(t *testing.T) {
	t.Setenv("PLEX_TOKEN", "legacy")
	t.Setenv("CALMSTOOLKIT_PLEX_TOKEN", "new")
	t.Setenv("CALMSTOOLKIT_SONARR_SONARR_HD_API_KEY", "instance")
	cfg := DefaultToolkitConfig()
	cfg.MediaStreams.PlexToken = "file"
	cfg.Sonarr = []ArrInstance{{Name: "Sonarr HD", URL: "http://localhost", APIKey: "file"}}
	ApplyEnvironment(cfg)
	if cfg.MediaStreams.PlexToken != "new" {
		t.Fatalf("token=%q", cfg.MediaStreams.PlexToken)
	}
	if cfg.Sonarr[0].APIKey != "instance" {
		t.Fatalf("instance token=%q", cfg.Sonarr[0].APIKey)
	}
}

func TestConfigSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	cfg := DefaultToolkitConfig()
	cfg.Sonarr = []ArrInstance{
		{Name: "Sonarr HD", URL: "http://sonarr:8989", ExternalURL: "https://sonarr.example.com", APIKey: "abc123"},
	}
	cfg.Radarr = []ArrInstance{
		{Name: "Radarr HD", URL: "http://radarr:7878", APIKey: "xyz789"},
	}
	cfg.MediaCalendar.Days = 7
	cfg.MediaCalendar.DaysPast = 1

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadToolkitConfig()
	if err != nil {
		t.Fatalf("LoadToolkitConfig() error: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if len(loaded.Sonarr) != 1 {
		t.Fatalf("Sonarr instances = %d, want 1", len(loaded.Sonarr))
	}
	if loaded.Sonarr[0].Name != "Sonarr HD" {
		t.Errorf("Sonarr[0].Name = %q, want %q", loaded.Sonarr[0].Name, "Sonarr HD")
	}
	if loaded.Sonarr[0].URL != "http://sonarr:8989" {
		t.Errorf("Sonarr[0].URL = %q, want %q", loaded.Sonarr[0].URL, "http://sonarr:8989")
	}
	if loaded.Sonarr[0].ExternalURL != "https://sonarr.example.com" {
		t.Errorf("Sonarr[0].ExternalURL = %q, want %q", loaded.Sonarr[0].ExternalURL, "https://sonarr.example.com")
	}
	if loaded.Sonarr[0].APIKey != "abc123" {
		t.Errorf("Sonarr[0].APIKey = %q, want %q", loaded.Sonarr[0].APIKey, "abc123")
	}
	if loaded.MediaCalendar.Days != 7 {
		t.Errorf("Days = %d, want 7", loaded.MediaCalendar.Days)
	}
	if loaded.MediaCalendar.DaysPast != 1 {
		t.Errorf("DaysPast = %d, want 1", loaded.MediaCalendar.DaysPast)
	}

	if loaded.Radarr[0].URL != "http://radarr:7878" {
		t.Errorf("URL should not have trailing slash, got %q", loaded.Radarr[0].URL)
	}
}

func TestLoadToolkitConfigFileNotFound(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	_, err := LoadToolkitConfig()
	if err == nil {
		t.Fatal("Expected error for missing config, got nil")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ToolkitConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultToolkitConfig(),
			wantErr: false,
		},
		{
			name:    "invalid version",
			cfg:     &ToolkitConfig{Version: 99},
			wantErr: true,
		},
		{
			name: "missing sonarr name",
			cfg: &ToolkitConfig{
				Version: 1,
				Sonarr:  []ArrInstance{{URL: "http://x", APIKey: "y"}},
			},
			wantErr: true,
		},
		{
			name: "missing sonarr url",
			cfg: &ToolkitConfig{
				Version: 1,
				Sonarr:  []ArrInstance{{Name: "x", APIKey: "y"}},
			},
			wantErr: true,
		},
		{
			name: "invalid sonarr external URL",
			cfg: &ToolkitConfig{
				Version: 1,
				Sonarr: []ArrInstance{{
					Name: "x", URL: "http://sonarr:8989", ExternalURL: "sonarr.example.com", APIKey: "y",
				}},
			},
			wantErr: true,
		},
		{
			name: "invalid radarr external URL",
			cfg: &ToolkitConfig{
				Version: 1,
				Radarr: []ArrInstance{{
					Name: "x", URL: "http://radarr:7878", ExternalURL: "radarr.example.com", APIKey: "y",
				}},
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			cfg: &ToolkitConfig{
				Version: 1,
				General: GeneralConfig{Timeout: "not-a-duration"},
			},
			wantErr: true,
		},
		{
			name: "invalid server type",
			cfg: &ToolkitConfig{
				Version:      1,
				MediaStreams: StreamsConfig{ServerType: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "max events out of range",
			cfg: &ToolkitConfig{
				Version: 1,
				ArrFeed: FeedConfig{MaxEvents: 200},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfigURLNormalization(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultToolkitConfig()
	cfg.Sonarr = []ArrInstance{
		{Name: "Test", URL: "http://example.com/", ExternalURL: "https://external.example.com/sonarr/", APIKey: "key"},
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadToolkitConfig()
	if err != nil {
		t.Fatalf("LoadToolkitConfig() error: %v", err)
	}

	if loaded.Sonarr[0].URL != "http://example.com" {
		t.Errorf("URL = %q, want %q", loaded.Sonarr[0].URL, "http://example.com")
	}
	if loaded.Sonarr[0].ExternalURL != "https://external.example.com/sonarr" {
		t.Errorf("ExternalURL = %q, want %q", loaded.Sonarr[0].ExternalURL, "https://external.example.com/sonarr")
	}
}

func TestArrInstanceBrowserURL(t *testing.T) {
	tests := []struct {
		name string
		inst ArrInstance
		want string
	}{
		{
			name: "external URL",
			inst: ArrInstance{URL: "http://sonarr:8989", ExternalURL: "https://media.example.com/sonarr/"},
			want: "https://media.example.com/sonarr",
		},
		{
			name: "API URL fallback",
			inst: ArrInstance{URL: "http://sonarr:8989/"},
			want: "http://sonarr:8989",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.inst.BrowserURL(); got != tt.want {
				t.Errorf("BrowserURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	path := ConfigPath()
	expected := filepath.Join(dir, ".config", "calmstoolkit", "config.json")
	if path != expected {
		t.Errorf("ConfigPath() = %q, want %q", path, expected)
	}
}

func TestTokenFromEnv(t *testing.T) {
	const envKey = "TEST_TOKEN_FROM_ENV"
	defer os.Unsetenv(envKey)

	got := TokenFromEnv(envKey, "fallback")
	if got != "fallback" {
		t.Errorf("TokenFromEnv with unset var = %q, want %q", got, "fallback")
	}

	os.Setenv(envKey, "env-value")
	got = TokenFromEnv(envKey, "fallback")
	if got != "env-value" {
		t.Errorf("TokenFromEnv with set var = %q, want %q", got, "env-value")
	}
}
