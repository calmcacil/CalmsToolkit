package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func main() {
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println("  CalmsToolkit Configuration Setup")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	path := ConfigPath()
	if path == "" {
		fmt.Fprintln(os.Stderr, "ERROR: Cannot determine home directory")
		os.Exit(1)
	}

	fmt.Printf("Config file: %s\n", path)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create config directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot set config directory permissions: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultToolkitConfig()

	existing := false
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err == nil {
			existing = true
			fmt.Println("\nExisting configuration loaded. New options (if any) have been added with defaults.")
			fmt.Println("Press Enter to keep the current value shown in [brackets].")
		}
	}
	if !existing {
		fmt.Println("\nNo existing config found. Defaults shown in [brackets].")
		fmt.Println("Press Enter to accept a default.")
	}

	fmt.Println()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 1/6: General Settings")
	fmt.Println(strings.Repeat("─", 50))
	cfg.General.Timeout = prompt("HTTP timeout (e.g. 10s, 30s)", cfg.General.Timeout)
	cfg.General.NoColor = promptYesNo("Disable colors in terminal output", cfg.General.NoColor)
	fmt.Println()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 2/6: Sonarr Instances")
	fmt.Println(strings.Repeat("─", 50))
	cfg.Sonarr = promptInstances("Sonarr", cfg.Sonarr)
	fmt.Println()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 3/6: Radarr Instances")
	fmt.Println(strings.Repeat("─", 50))
	cfg.Radarr = promptInstances("Radarr", cfg.Radarr)
	fmt.Println()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 4/6: Plex & Jellyfin Streams")
	fmt.Println(strings.Repeat("─", 50))
	if promptYesNo("Configure Plex?", cfg.MediaStreams.PlexToken != "") {
		cfg.MediaStreams.PlexURL = prompt("Plex URL", cfg.MediaStreams.PlexURL)
		cfg.MediaStreams.PlexToken = promptSecret("Plex Token", cfg.MediaStreams.PlexToken)
	} else {
		cfg.MediaStreams.PlexURL = ""
		cfg.MediaStreams.PlexToken = ""
	}
	if promptYesNo("Configure Jellyfin?", cfg.MediaStreams.JellyfinToken != "") {
		cfg.MediaStreams.JellyfinURL = prompt("Jellyfin URL", cfg.MediaStreams.JellyfinURL)
		cfg.MediaStreams.JellyfinToken = promptSecret("Jellyfin Token", cfg.MediaStreams.JellyfinToken)
	} else {
		cfg.MediaStreams.JellyfinURL = ""
		cfg.MediaStreams.JellyfinToken = ""
	}
	cfg.MediaStreams.ServerType = prompt("Server type (plex/jellyfin/both)", cfg.MediaStreams.ServerType)
	cfg.MediaStreams.WatchInterval = promptInt("Watch refresh interval (seconds)", cfg.MediaStreams.WatchInterval)
	cfg.MediaStreams.HistoryDuration = prompt("Session history duration (e.g. 15m, 1h)", cfg.MediaStreams.HistoryDuration)
	fmt.Println()

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("  Step 5/6: Media Requests (Overseerr / Jellyseerr)")
	fmt.Println(strings.Repeat("─", 50))
	if promptYesNo("Configure media request server?", cfg.MediaRequests.APIKey != "") {
		cfg.MediaRequests.OverseerrURL = prompt("Server URL", cfg.MediaRequests.OverseerrURL)
		cfg.MediaRequests.APIKey = promptSecret("API Key", cfg.MediaRequests.APIKey)
		cfg.MediaRequests.Verbose = promptYesNo("Verbose output", cfg.MediaRequests.Verbose)
	} else {
		cfg.MediaRequests.OverseerrURL = ""
		cfg.MediaRequests.APIKey = ""
		cfg.MediaRequests.Verbose = false
	}
	fmt.Println()

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
	cfg.ArrFeed.ShowSubtitles = promptYesNo("Show subtitle info for imported events", cfg.ArrFeed.ShowSubtitles)
	fmt.Println()

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
	fmt.Printf("  Feed subtitles:    %v\n", cfg.ArrFeed.ShowSubtitles)
	fmt.Println()

	cfg.Version = 1

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Configuration has validation errors: %v\n", err)
		fmt.Fprintf(os.Stderr, "You can still write it, but some tools may not work correctly.\n")
	}

	if !promptYesNo("Write configuration to "+path+"?", true) {
		fmt.Println("Setup cancelled.")
		return
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot encode config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot write config: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chmod(path, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot set config permissions: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Configuration written to %s\n", path)
	fmt.Println("  Run 'make build' to compile all tools.")
	fmt.Println("  Run 'make test' to verify everything works.")
	fmt.Println()
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

func promptSecret(label, defaultVal string) string {
	masked := ""
	if defaultVal != "" {
		masked = "****"
	}
	if masked != "" {
		fmt.Printf("  %s [%s]: ", label, masked)
	} else {
		fmt.Printf("  %s: ", label)
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		byteInput, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return defaultVal
		}
		input := strings.TrimSpace(string(byteInput))
		if input == "" {
			return defaultVal
		}
		return input
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

func promptInstances(kind string, existing []config.ArrInstance) []config.ArrInstance {
	instances := make([]config.ArrInstance, len(existing))
	copy(instances, existing)

	if len(instances) > 0 {
		fmt.Printf("\n  You have %d existing %s instance(s):\n", len(instances), kind)
		for i, inst := range instances {
			fmt.Printf("    %d. %s (%s)\n", i+1, inst.Name, inst.URL)
		}
		if !promptYesNo("Review and update instances?", false) {
			return instances
		}
		for i := range instances {
			fmt.Printf("\n  --- %s Instance %d ---\n", kind, i+1)
			instances[i].Name = prompt("Friendly name", instances[i].Name)
			instances[i].URL = prompt("URL", instances[i].URL)
			token := promptSecret("API Key (enter blank to keep current)", instances[i].APIKey)
			if token != "" {
				instances[i].APIKey = token
			}
			instances[i].URL = strings.TrimSuffix(instances[i].URL, "/")
		}
	}

	if promptYesNo("Add another "+kind+" instance?", false) {
		for {
			fmt.Printf("\n--- New %s Instance ---\n", kind)
			name := prompt("Friendly name", kind+" HD")
			url := prompt("URL", "http://localhost:8989")
			token := promptSecret("API Key", "")
			if token == "" {
				fmt.Println("  API Key is required. Skipping this instance.")
			} else {
				instances = append(instances, config.ArrInstance{
					Name:   name,
					URL:    strings.TrimSuffix(url, "/"),
					APIKey: token,
				})
			}
			if !promptYesNo("Add another "+kind+" instance?", false) {
				break
			}
		}
	}

	return instances
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "calmstoolkit", "config.json")
}
