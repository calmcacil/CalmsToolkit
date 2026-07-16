package cli

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/app"
	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

type setupPrompter struct {
	rt      *app.Runtime
	scanner *bufio.Scanner
}

var errSetupCancelled = errors.New("configuration setup cancelled")

func promptSetup(rt *app.Runtime, cfg *config.ToolkitConfig) error {
	p := setupPrompter{rt: rt, scanner: bufio.NewScanner(rt.Stdin)}
	fmt.Fprintln(rt.Stderr, "CalmsToolkit configuration setup")
	fmt.Fprintln(rt.Stderr, "Press Enter to keep the value in brackets. Enter - to clear an optional value.")

	for {
		p.printMenu(cfg)
		choice, err := p.read("Choose a section", "", false, false, func(value string) error {
			return oneOf(false, "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "s", "q")(strings.ToLower(value))
		})
		if err != nil {
			return err
		}
		switch strings.ToLower(choice) {
		case "1":
			err = p.general(cfg)
		case "2":
			cfg.Sonarr, err = p.instances("Sonarr", cfg.Sonarr)
		case "3":
			cfg.Radarr, err = p.instances("Radarr", cfg.Radarr)
		case "4":
			err = p.streams(cfg)
		case "5":
			err = p.requests(cfg)
		case "6":
			err = p.calendar(cfg)
		case "7":
			err = p.airtime(cfg)
		case "8":
			err = p.feed(cfg)
		case "9":
			err = p.anime(cfg)
		case "a":
			err = p.configureAll(cfg)
		case "s":
			if err := cfg.Validate(); err != nil {
				fmt.Fprintf(rt.Stderr, "\nConfiguration is not valid yet:\n%v\n", err)
				continue
			}
			return nil
		case "q":
			return errSetupCancelled
		}
		if err != nil {
			return err
		}
	}
}

func (p *setupPrompter) printMenu(cfg *config.ToolkitConfig) {
	streamStatus := cfg.MediaStreams.ServerType
	if streamStatus == "" {
		streamStatus = "disabled"
	}
	requestStatus := "URL only"
	if cfg.MediaRequests.APIKey != "" {
		requestStatus = "configured"
	}
	fmt.Fprintf(p.rt.Stderr, `
Configuration sections
  1. General             theme=%s, timeout=%s
  2. Sonarr instances    %d configured
  3. Radarr instances    %d configured
  4. Media streams       %s
  5. Media requests      %s
  6. Media calendar      %d future / %d past days
  7. Media airtime       limit=%d
  8. Arr feed            interval=%s
  9. AniSearch           limit=%d

  A. Configure all sections
  S. Validate and save
  Q. Quit without saving
`, cfg.General.Theme, cfg.General.Timeout, len(cfg.Sonarr), len(cfg.Radarr), streamStatus,
		requestStatus, cfg.MediaCalendar.Days, cfg.MediaCalendar.DaysPast,
		cfg.MediaAirtime.Limit, cfg.ArrFeed.PollInterval, cfg.AniSearch.Limit)
}

func (p *setupPrompter) configureAll(cfg *config.ToolkitConfig) error {
	steps := []func() error{
		func() error { return p.general(cfg) },
		func() (err error) { cfg.Sonarr, err = p.instances("Sonarr", cfg.Sonarr); return err },
		func() (err error) { cfg.Radarr, err = p.instances("Radarr", cfg.Radarr); return err },
		func() error { return p.streams(cfg) },
		func() error { return p.requests(cfg) },
		func() error { return p.calendar(cfg) },
		func() error { return p.airtime(cfg) },
		func() error { return p.feed(cfg) },
		func() error { return p.anime(cfg) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (p *setupPrompter) section(title, description string) {
	fmt.Fprintf(p.rt.Stderr, "\n[%s]\n%s\n", title, description)
}

func (p *setupPrompter) read(label, current string, secret, optional bool, validate func(string) error) (string, error) {
	for {
		if err := p.rt.Context.Err(); err != nil {
			return "", err
		}
		display := current
		if secret && current != "" {
			display = "configured"
		}
		if display == "" {
			fmt.Fprintf(p.rt.Stderr, "%s: ", label)
		} else {
			fmt.Fprintf(p.rt.Stderr, "%s [%s]: ", label, display)
		}
		if !p.scanner.Scan() {
			if err := p.scanner.Err(); err != nil {
				return "", err
			}
			return "", fmt.Errorf("input ended while reading %s", label)
		}
		value := strings.TrimSpace(p.scanner.Text())
		if value == "" {
			value = current
		} else if value == "-" && optional {
			value = ""
		}
		if validate != nil {
			if err := validate(value); err != nil {
				fmt.Fprintf(p.rt.Stderr, "  Invalid value: %v. Try again.\n", err)
				continue
			}
		}
		return value, nil
	}
}

func (p *setupPrompter) text(label, current string, optional bool, validate func(string) error) (string, error) {
	return p.read(label, current, false, optional, validate)
}

func (p *setupPrompter) secret(label, current string, optional bool) (string, error) {
	return p.read(label, current, true, optional, nil)
}

func (p *setupPrompter) integer(label string, current, min, max int) (int, error) {
	value, err := p.read(label, strconv.Itoa(current), false, false, func(value string) error {
		n, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("enter a whole number")
		}
		if n < min || (max > 0 && n > max) {
			if max > 0 {
				return fmt.Errorf("must be between %d and %d", min, max)
			}
			return fmt.Errorf("must be at least %d", min)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

func (p *setupPrompter) boolean(label string, current bool) (bool, error) {
	currentValue := "no"
	if current {
		currentValue = "yes"
	}
	value, err := p.read(label+" (yes/no)", currentValue, false, false, func(value string) error {
		if _, ok := parseBool(value); !ok {
			return errors.New("enter yes or no")
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	result, _ := parseBool(value)
	return result, nil
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(value) {
	case "y", "yes", "true", "1":
		return true, true
	case "n", "no", "false", "0":
		return false, true
	default:
		return false, false
	}
}

func oneOf(optional bool, values ...string) func(string) error {
	return func(value string) error {
		if optional && value == "" {
			return nil
		}
		for _, candidate := range values {
			if value == candidate {
				return nil
			}
		}
		return fmt.Errorf("choose %s", strings.Join(values, ", "))
	}
}

func httpURL(optional bool) func(string) error {
	return func(value string) error {
		if optional && value == "" {
			return nil
		}
		u, err := url.ParseRequestURI(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("enter a complete http:// or https:// URL")
		}
		return nil
	}
}

func duration(positive bool) func(string) error {
	return func(value string) error {
		d, err := time.ParseDuration(value)
		if err != nil {
			return errors.New("enter a Go duration such as 10s, 5m, or 1h")
		}
		if positive && d <= 0 {
			return errors.New("duration must be positive")
		}
		return nil
	}
}

func required(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is required")
	}
	return nil
}

func (p *setupPrompter) general(cfg *config.ToolkitConfig) (err error) {
	p.section("General", "Shared HTTP and terminal presentation defaults.")
	if cfg.General.Timeout, err = p.text("HTTP timeout (for example 10s)", cfg.General.Timeout, false, duration(true)); err != nil {
		return err
	}
	if cfg.General.Theme, err = p.text("Theme (default, catppuccin-mocha, catppuccin-latte)", cfg.General.Theme, false, func(value string) error {
		if !colors.ValidateTheme(value) {
			return errors.New("choose default, catppuccin-mocha, or catppuccin-latte")
		}
		return nil
	}); err != nil {
		return err
	}
	cfg.General.NoColor, err = p.boolean("Disable color by default", cfg.General.NoColor)
	return err
}

func (p *setupPrompter) instances(service string, current []config.ArrInstance) ([]config.ArrInstance, error) {
	p.section(service+" Instances", "Add, edit, or remove API endpoints. The external URL is optional and is only used for browser links.")
	instances := append([]config.ArrInstance(nil), current...)
	for {
		fmt.Fprintln(p.rt.Stderr)
		for i, instance := range instances {
			fmt.Fprintf(p.rt.Stderr, "  %d. %s (%s)\n", i+1, instance.Name, instance.URL)
		}
		fmt.Fprintln(p.rt.Stderr, "  A. Add instance")
		if len(instances) > 0 {
			fmt.Fprintln(p.rt.Stderr, "  R. Remove instance")
		}
		fmt.Fprintln(p.rt.Stderr, "  D. Done")
		choice, err := p.text("Choose an instance to edit", "", false, func(value string) error {
			lower := strings.ToLower(value)
			if lower == "a" || lower == "d" || (lower == "r" && len(instances) > 0) {
				return nil
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > len(instances) {
				return errors.New("choose an instance number, A, R, or D")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(choice) {
		case "a":
			instance, err := p.instance(service, len(instances)+1, config.ArrInstance{})
			if err != nil {
				return nil, err
			}
			instances = append(instances, instance)
		case "r":
			n, err := p.integer("Instance number to remove", 1, 1, len(instances))
			if err != nil {
				return nil, err
			}
			instances = append(instances[:n-1], instances[n:]...)
		case "d":
			return instances, nil
		default:
			n, _ := strconv.Atoi(choice)
			instance, err := p.instance(service, n, instances[n-1])
			if err != nil {
				return nil, err
			}
			instances[n-1] = instance
		}
	}
}

func (p *setupPrompter) instance(service string, number int, instance config.ArrInstance) (config.ArrInstance, error) {
	prefix := fmt.Sprintf("%s %d", service, number)
	var err error
	if instance.Name, err = p.text(prefix+" name (for example HD)", instance.Name, false, required); err != nil {
		return instance, err
	}
	if instance.URL, err = p.text(prefix+" API URL", instance.URL, false, httpURL(false)); err != nil {
		return instance, err
	}
	instance.URL = strings.TrimSuffix(instance.URL, "/")
	if instance.ExternalURL, err = p.text(prefix+" external URL (optional)", instance.ExternalURL, true, httpURL(true)); err != nil {
		return instance, err
	}
	instance.ExternalURL = strings.TrimSuffix(instance.ExternalURL, "/")
	instance.APIKey, err = p.read(prefix+" API key", instance.APIKey, true, false, required)
	return instance, err
}

func (p *setupPrompter) streams(cfg *config.ToolkitConfig) (err error) {
	p.section("Media Streams", "Configure Plex, Jellyfin, or both for session monitoring. Choose none to leave integrations disabled.")
	if cfg.MediaStreams.ServerType, err = p.text("Enabled servers (plex, jellyfin, both; - disables)", cfg.MediaStreams.ServerType, true, oneOf(true, "plex", "jellyfin", "both")); err != nil {
		return err
	}
	if cfg.MediaStreams.PlexURL, err = p.text("Plex URL", cfg.MediaStreams.PlexURL, true, httpURL(true)); err != nil {
		return err
	}
	if cfg.MediaStreams.PlexToken, err = p.secret("Plex token (optional when Plex is disabled)", cfg.MediaStreams.PlexToken, true); err != nil {
		return err
	}
	if cfg.MediaStreams.JellyfinURL, err = p.text("Jellyfin URL", cfg.MediaStreams.JellyfinURL, true, httpURL(true)); err != nil {
		return err
	}
	if cfg.MediaStreams.JellyfinToken, err = p.secret("Jellyfin token (optional when Jellyfin is disabled)", cfg.MediaStreams.JellyfinToken, true); err != nil {
		return err
	}
	if cfg.MediaStreams.WatchInterval, err = p.integer("Watch interval in seconds", cfg.MediaStreams.WatchInterval, 1, 0); err != nil {
		return err
	}
	cfg.MediaStreams.HistoryDuration, err = p.text("Session history duration (for example 15m)", cfg.MediaStreams.HistoryDuration, false, duration(true))
	return err
}

func (p *setupPrompter) requests(cfg *config.ToolkitConfig) (err error) {
	p.section("Media Requests", "Configure the interactive Overseerr or Jellyseerr requester. The credential may be supplied later through CALMSTOOLKIT_REQUESTS_API_KEY.")
	if cfg.MediaRequests.OverseerrURL, err = p.text("Overseerr/Jellyseerr URL", cfg.MediaRequests.OverseerrURL, true, httpURL(true)); err != nil {
		return err
	}
	if cfg.MediaRequests.APIKey, err = p.secret("Requests API key (optional)", cfg.MediaRequests.APIKey, true); err != nil {
		return err
	}
	cfg.MediaRequests.Verbose, err = p.boolean("Verbose request diagnostics", cfg.MediaRequests.Verbose)
	return err
}

func (p *setupPrompter) calendar(cfg *config.ToolkitConfig) (err error) {
	p.section("Media Calendar", "Defaults for release range and dashboard refresh behavior.")
	if cfg.MediaCalendar.Days, err = p.integer("Future days", cfg.MediaCalendar.Days, 0, 0); err != nil {
		return err
	}
	if cfg.MediaCalendar.DaysPast, err = p.integer("Past days", cfg.MediaCalendar.DaysPast, 0, 0); err != nil {
		return err
	}
	if cfg.MediaCalendar.WatchInterval, err = p.integer("Watch interval in seconds", cfg.MediaCalendar.WatchInterval, 1, 0); err != nil {
		return err
	}
	cfg.MediaCalendar.Debug, err = p.boolean("Calendar debug diagnostics", cfg.MediaCalendar.Debug)
	return err
}

func (p *setupPrompter) airtime(cfg *config.ToolkitConfig) (err error) {
	p.section("Media Airtime", "Defaults for Sonarr and Radarr library searches.")
	if cfg.MediaAirtime.Limit, err = p.integer("Maximum matches", cfg.MediaAirtime.Limit, 1, 50); err != nil {
		return err
	}
	if cfg.MediaAirtime.PastDays, err = p.integer("Past-day search window", cfg.MediaAirtime.PastDays, 0, 0); err != nil {
		return err
	}
	if cfg.MediaAirtime.FutureDays, err = p.integer("Future-day search window", cfg.MediaAirtime.FutureDays, 0, 0); err != nil {
		return err
	}
	cfg.MediaAirtime.Debug, err = p.boolean("Airtime debug diagnostics", cfg.MediaAirtime.Debug)
	return err
}

func (p *setupPrompter) feed(cfg *config.ToolkitConfig) (err error) {
	p.section("Arr Feed", "Defaults for Sonarr and Radarr activity polling and event visibility.")
	if cfg.ArrFeed.PollInterval, err = p.text("Poll interval (for example 5s)", cfg.ArrFeed.PollInterval, false, duration(true)); err != nil {
		return err
	}
	if cfg.ArrFeed.HistoryWindow, err = p.text("History window (for example 1h)", cfg.ArrFeed.HistoryWindow, false, duration(true)); err != nil {
		return err
	}
	if cfg.ArrFeed.MaxEvents, err = p.integer("Maximum events (0 means unlimited)", cfg.ArrFeed.MaxEvents, 0, 100); err != nil {
		return err
	}
	fields := []struct {
		label string
		value *bool
	}{
		{"Show grabbed events", &cfg.ArrFeed.ShowGrabbed},
		{"Show imported events", &cfg.ArrFeed.ShowImported},
		{"Show failed events", &cfg.ArrFeed.ShowFailed},
		{"Show deleted events", &cfg.ArrFeed.ShowDeleted},
		{"Show ignored events", &cfg.ArrFeed.ShowIgnored},
		{"Show subtitle details", &cfg.ArrFeed.ShowSubtitles},
	}
	for _, field := range fields {
		if *field.value, err = p.boolean(field.label, *field.value); err != nil {
			return err
		}
	}
	return nil
}

func (p *setupPrompter) anime(cfg *config.ToolkitConfig) (err error) {
	p.section("AniSearch", "Configure the AniList-to-TVDB mapping source, local cache, and result count.")
	if cfg.AniSearch.MappingURL, err = p.text("Mapping download URL", cfg.AniSearch.MappingURL, false, httpURL(false)); err != nil {
		return err
	}
	if cfg.AniSearch.MappingPath, err = p.text("Mapping cache path (optional; default uses the user cache)", cfg.AniSearch.MappingPath, true, nil); err != nil {
		return err
	}
	cfg.AniSearch.Limit, err = p.integer("Results per page", cfg.AniSearch.Limit, 1, 50)
	return err
}
