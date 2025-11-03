package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Sonarr    []InstanceConfig `json:"sonarr"`
	Radarr    []InstanceConfig `json:"radarr"`
	Overseerr OverseerrConfig  `json:"overseerr"`
	Plex      PlexConfig       `json:"plex"`
	Jellyfin  JellyfinConfig   `json:"jellyfin"`
	Global    GlobalConfig     `json:"global"`
}

// InstanceConfig represents a Sonarr/Radarr instance configuration
type InstanceConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Name  string `json:"name,omitempty"`
}

// OverseerrConfig represents Overseerr configuration
type OverseerrConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// PlexConfig represents Plex configuration
type PlexConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// JellyfinConfig represents Jellyfin configuration
type JellyfinConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// GlobalConfig represents global application settings
type GlobalConfig struct {
	NoColor bool          `json:"noColor"`
	Timeout time.Duration `json:"timeout"`
	Debug   bool          `json:"debug"`
}

// Load loads configuration from environment variables and .env file
func Load() (*Config, error) {
	config := &Config{
		Global: GlobalConfig{
			NoColor: os.Getenv("NO_COLOR") != "",
			Timeout: 30 * time.Second,
			Debug:   os.Getenv("DEBUG") != "",
		},
	}

	// Try to load from .env file first
	if envPath := os.Getenv("ENV_PATH"); envPath != "" {
		loadEnvFile(envPath, config)
	} else if _, err := os.Stat(".env"); err == nil {
		loadEnvFile(".env", config)
	} else if _, err := os.Stat("/opt/apps/compose/.env"); err == nil {
		loadEnvFile("/opt/apps/compose/.env", config)
	}

	// Override with environment variables
	loadFromEnv(config)

	return config, nil
}

// loadEnvFile loads configuration from a .env file
func loadEnvFile(path string, config *Config) {
	file, err := os.Open(path)
	if err != nil {
		return
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
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = strings.Trim(value, `"`)
		}

		os.Setenv(key, value)
	}
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) {
	// Sonarr configuration
	if sonarrURLs := os.Getenv("SONARR_URLS"); sonarrURLs != "" {
		urls := strings.Split(sonarrURLs, ",")
		tokens := strings.Split(os.Getenv("SONARR_TOKENS"), ",")

		for i, url := range urls {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}

			instance := InstanceConfig{URL: url}
			if i < len(tokens) {
				instance.Token = strings.TrimSpace(tokens[i])
			}

			config.Sonarr = append(config.Sonarr, instance)
		}
	}

	// Radarr configuration
	if radarrURLs := os.Getenv("RADARR_URLS"); radarrURLs != "" {
		urls := strings.Split(radarrURLs, ",")
		tokens := strings.Split(os.Getenv("RADARR_TOKENS"), ",")

		for i, url := range urls {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}

			instance := InstanceConfig{URL: url}
			if i < len(tokens) {
				instance.Token = strings.TrimSpace(tokens[i])
			}

			config.Radarr = append(config.Radarr, instance)
		}
	}

	// Overseerr configuration
	config.Overseerr = OverseerrConfig{
		URL:   os.Getenv("OVERSEERR_URL"),
		Token: os.Getenv("OVERSEERR_TOKEN"),
	}

	// Plex configuration
	config.Plex = PlexConfig{
		URL:   os.Getenv("PLEX_URL"),
		Token: os.Getenv("PLEX_TOKEN"),
	}

	// Jellyfin configuration
	config.Jellyfin = JellyfinConfig{
		URL:   os.Getenv("JELLYFIN_URL"),
		Token: os.Getenv("JELLYFIN_TOKEN"),
	}
}

// ValidateQueueConfig validates the queue configuration
func (c *Config) ValidateQueueConfig() error {
	sonarrCount := len(c.Sonarr)
	radarrCount := len(c.Radarr)

	if sonarrCount == 0 && radarrCount == 0 {
		return fmt.Errorf("at least one Sonarr or Radarr instance must be configured")
	}

	for i, instance := range c.Sonarr {
		if instance.URL == "" {
			return fmt.Errorf("Sonarr instance %d has no URL", i+1)
		}
		if instance.Token == "" {
			return fmt.Errorf("Sonarr instance %d has no token", i+1)
		}
	}

	for i, instance := range c.Radarr {
		if instance.URL == "" {
			return fmt.Errorf("Radarr instance %d has no URL", i+1)
		}
		if instance.Token == "" {
			return fmt.Errorf("Radarr instance %d has no token", i+1)
		}
	}

	return nil
}

// GetInstanceName returns a human-readable name for an instance
func (c *Config) GetInstanceName(instanceURL, instanceType string) string {
	instances := c.Sonarr
	if instanceType == "radarr" {
		instances = c.Radarr
	}

	for i, instance := range instances {
		if instance.URL == instanceURL {
			if instance.Name != "" {
				return instance.Name
			}
			return fmt.Sprintf("%s%d", strings.Title(instanceType), i+1)
		}
	}

	return instanceType
}
