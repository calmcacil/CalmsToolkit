package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
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

var scanner = bufio.NewScanner(os.Stdin)

func prompt(label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal
	}
	return input
}

func promptYesNo(label string, defaultVal bool) bool {
	suffix := " [y/N]"
	if defaultVal {
		suffix = " [Y/n]"
	}
	fmt.Printf("  %s%s: ", label, suffix)
	scanner.Scan()
	input := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}

func promptInt(label string, defaultVal int) int {
	for {
		input := prompt(label, strconv.Itoa(defaultVal))
		val, err := strconv.Atoi(input)
		if err == nil {
			return val
		}
		fmt.Printf("  Invalid number. Please enter a number.\n")
	}
}

func promptInstances(kind string) []ArrInstance {
	var instances []ArrInstance
	for {
		fmt.Printf("\n--- %s Instance ---\n", kind)
		name := prompt("Friendly name", kind+" HD")
		url := prompt("URL", "http://localhost:8989")
		token := prompt("API Key", "")
		if token == "" {
			fmt.Println("  API Key is required. Skipping this instance.")
		} else {
			instances = append(instances, ArrInstance{
				Name:   name,
				URL:    strings.TrimSuffix(url, "/"),
				APIKey: token,
			})
		}
		if !promptYesNo("Add another "+kind+" instance?", false) {
			break
		}
	}
	return instances
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "calmstoolkit", "config.json")
}

func main() {
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println("  CalmsToolkit Configuration Setup")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	path := configPath()
	if path == "" {
		fmt.Println("ERROR: Cannot determine home directory")
		os.Exit(1)
	}

	fmt.Printf("Config file: %s\n", path)

	cfg := defaultConfig()

	// Check if config already exists
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err == nil {
			fmt.Println("\nExisting configuration found.")
			if !promptYesNo("Overwrite existing config?", false) {
				fmt.Println("Setup cancelled.")
				return
			}
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create config directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Leave blank to accept defaults shown in [brackets].")
	fmt.Println()

	// Section 1: General
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 1/6: General Settings")
	fmt.Println(strings.Repeat("─", 50))
	cfg.General.Timeout = prompt("HTTP timeout (e.g. 10s, 30s)", cfg.General.Timeout)
	cfg.General.NoColor = promptYesNo("Disable colors in terminal output", cfg.General.NoColor)
	fmt.Println()

	// Section 2: Sonarr instances
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 2/6: Sonarr Instances")
	fmt.Println(strings.Repeat("─", 50))
	sonarr := promptInstances("Sonarr")
	cfg.Sonarr = sonarr
	fmt.Println()

	// Section 3: Radarr instances
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 3/6: Radarr Instances")
	fmt.Println(strings.Repeat("─", 50))
	radarr := promptInstances("Radarr")
	cfg.Radarr = radarr
	fmt.Println()

	// Section 4: Plex & Jellyfin
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 4/6: Plex & Jellyfin Streams")
	fmt.Println(strings.Repeat("─", 50))
	if promptYesNo("Configure Plex?", true) {
		cfg.MediaStreams.PlexURL = prompt("Plex URL", cfg.MediaStreams.PlexURL)
		cfg.MediaStreams.PlexToken = prompt("Plex Token", cfg.MediaStreams.PlexToken)
	}
	if promptYesNo("Configure Jellyfin?", true) {
		cfg.MediaStreams.JellyfinURL = prompt("Jellyfin URL", cfg.MediaStreams.JellyfinURL)
		cfg.MediaStreams.JellyfinToken = prompt("Jellyfin Token", cfg.MediaStreams.JellyfinToken)
	}
	serverType := prompt("Server type (plex/jellyfin/both)", cfg.MediaStreams.ServerType)
	cfg.MediaStreams.ServerType = serverType
	cfg.MediaStreams.WatchInterval = promptInt("Watch refresh interval (seconds)", cfg.MediaStreams.WatchInterval)
	cfg.MediaStreams.HistoryDuration = prompt("Session history duration (e.g. 15m, 1h)", cfg.MediaStreams.HistoryDuration)
	fmt.Println()

	// Section 5: Overseerr / Jellyseerr
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 5/6: Media Requests (Overseerr / Jellyseerr)")
	fmt.Println(strings.Repeat("─", 50))
	if promptYesNo("Configure media request server?", true) {
		cfg.MediaRequests.OverseerrURL = prompt("Server URL", cfg.MediaRequests.OverseerrURL)
		cfg.MediaRequests.APIKey = prompt("API Key", cfg.MediaRequests.APIKey)
		cfg.MediaRequests.Verbose = promptYesNo("Verbose output", cfg.MediaRequests.Verbose)
	}
	fmt.Println()

	// Section 6: Calendar & Feed
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 6/6: Calendar & Feed Settings")
	fmt.Println(strings.Repeat("─", 50))

	fmt.Println("\n  -- Media Calendar --")
	cfg.MediaCalendar.Days = promptInt("Days to show", cfg.MediaCalendar.Days)
	cfg.MediaCalendar.DaysPast = promptInt("Past days to include", cfg.MediaCalendar.DaysPast)
	cfg.MediaCalendar.WatchInterval = promptInt("Watch refresh interval (seconds)", cfg.MediaCalendar.WatchInterval)
	cfg.MediaCalendar.Debug = promptYesNo("Debug mode (show API URLs)", cfg.MediaCalendar.Debug)

	fmt.Println("\n  -- ARR Feed --")
	cfg.ArrFeed.PollInterval = prompt("Poll interval (e.g. 5s, 30s)", cfg.ArrFeed.PollInterval)
	cfg.ArrFeed.HistoryWindow = prompt("History window (e.g. 1h, 24h)", cfg.ArrFeed.HistoryWindow)
	cfg.ArrFeed.MaxEvents = promptInt("Max events to display (0-100)", cfg.ArrFeed.MaxEvents)
	cfg.ArrFeed.ShowGrabbed = promptYesNo("Show grabbed events", cfg.ArrFeed.ShowGrabbed)
	cfg.ArrFeed.ShowImported = promptYesNo("Show imported events", cfg.ArrFeed.ShowImported)
	cfg.ArrFeed.ShowFailed = promptYesNo("Show failed events", cfg.ArrFeed.ShowFailed)
	cfg.ArrFeed.ShowDeleted = promptYesNo("Show deleted events", cfg.ArrFeed.ShowDeleted)
	cfg.ArrFeed.ShowIgnored = promptYesNo("Show ignored events", cfg.ArrFeed.ShowIgnored)
	fmt.Println()

	// Summary
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println("  Configuration Summary")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Printf("  Sonarr instances:  %d\n", len(cfg.Sonarr))
	fmt.Printf("  Radarr instances:  %d\n", len(cfg.Radarr))
	fmt.Printf("  Plex configured:   %v\n", cfg.MediaStreams.PlexToken != "")
	fmt.Printf("  Jellyfin configured: %v\n", cfg.MediaStreams.JellyfinToken != "")
	fmt.Printf("  Requests server:   %v\n", cfg.MediaRequests.APIKey != "")
	fmt.Printf("  Calendar days:     %d (+%d past)\n", cfg.MediaCalendar.Days, cfg.MediaCalendar.DaysPast)
	fmt.Printf("  Feed max events:   %d\n", cfg.ArrFeed.MaxEvents)
	fmt.Println()

	cfg.Version = 1

	if !promptYesNo("Write configuration to "+path+"?", true) {
		fmt.Println("Setup cancelled.")
		return
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot encode config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot write config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Configuration written to %s\n", path)
	fmt.Println("  Run 'make build' to compile all tools.")
	fmt.Println("  Run 'make test' to verify everything works.")
	fmt.Println()
}

func defaultConfig() *Config {
	return &Config{
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
			JellyfinURL:     "http://localhost:8096",
			ServerType:      "both",
			WatchInterval:   10,
			HistoryDuration: "15m",
		},
		MediaRequests: RequestsConfig{
			OverseerrURL: "http://localhost:5055",
			Verbose:      false,
		},
		ArrFeed: FeedConfig{
			PollInterval:  "5s",
			HistoryWindow: "1h",
			ShowGrabbed:   true,
			ShowImported:  true,
			ShowFailed:    true,
			MaxEvents:     50,
		},
	}
}
