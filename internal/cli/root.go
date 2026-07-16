// Package cli defines the single CalmsToolkit command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/airtime"
	"github.com/calmcacil/CalmsToolkit/internal/anisearch"
	"github.com/calmcacil/CalmsToolkit/internal/app"
	"github.com/calmcacil/CalmsToolkit/internal/buildinfo"
	"github.com/calmcacil/CalmsToolkit/internal/calendar"
	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/console"
	"github.com/calmcacil/CalmsToolkit/internal/feed"
	"github.com/calmcacil/CalmsToolkit/internal/requests"
	"github.com/calmcacil/CalmsToolkit/internal/streams"
	"github.com/spf13/cobra"
)

type globalOptions struct {
	configPath, output, theme     string
	noColor, debug, quiet, strict bool
	timeout                       time.Duration
}

// NewRootCommand returns the complete command tree with no process termination.
func NewRootCommand(rt *app.Runtime) *cobra.Command {
	var global globalOptions
	root := &cobra.Command{
		Use:           "calmstoolkit",
		Short:         "A unified SSH-friendly media toolkit",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "completion" || strings.HasPrefix(cmd.Name(), "__complete") {
				return nil
			}
			return configureRuntime(cmd, rt, global)
		},
	}
	root.SetIn(rt.Stdin)
	root.SetOut(rt.Stdout)
	root.SetErr(rt.Stderr)
	f := root.PersistentFlags()
	f.StringVar(&global.configPath, "config", "", "configuration file (or CALMSTOOLKIT_CONFIG)")
	f.StringVar(&global.output, "output", "auto", "output mode: auto, terminal, plain, json, ndjson")
	f.StringVar(&global.theme, "theme", "", "color theme (default, catppuccin-mocha, catppuccin-latte)")
	f.BoolVar(&global.noColor, "no-color", false, "disable colored output")
	f.DurationVar(&global.timeout, "timeout", 0, "HTTP timeout")
	f.BoolVar(&global.debug, "debug", false, "enable redacted diagnostics")
	f.BoolVar(&global.quiet, "quiet", false, "suppress informational diagnostics")
	f.BoolVar(&global.strict, "strict", false, "fail with status 3 on partial results")
	_ = root.RegisterFlagCompletionFunc("output", fixedCompletions("auto", "terminal", "plain", "json", "ndjson"))
	_ = root.RegisterFlagCompletionFunc("theme", fixedCompletions("default", "catppuccin-mocha", "catppuccin-latte"))
	root.AddCommand(newStreamsCommand(rt), newCalendarCommand(rt), newRequestsCommand(rt), newAirtimeCommand(rt), newFeedCommand(rt), newAnimeCommand(rt), newConfigCommand(rt), newCompletionCommand(), newDoctorCommand(rt), newVersionCommand(rt))
	return root
}

func configureRuntime(cmd *cobra.Command, rt *app.Runtime, opt globalOptions) error {
	path := config.ResolvePath(opt.configPath)
	cfg, err := config.LoadToolkitConfigAt(path)
	if err != nil {
		if !os.IsNotExist(rootCause(err)) && !strings.Contains(err.Error(), "config not found") {
			return app.Error(app.ExitUsage, err)
		}
		cfg = config.DefaultToolkitConfig()
		config.ApplyEnvironment(cfg)
	}
	rt.Config, rt.ConfigPath = cfg, path
	timeout, parseErr := time.ParseDuration(cfg.General.Timeout)
	if parseErr != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}
	rt.Timeout, rt.Theme, rt.NoColor = timeout, cfg.General.Theme, cfg.General.NoColor
	flags := cmd.Flags()
	if flags.Changed("timeout") {
		if opt.timeout <= 0 {
			return app.Error(app.ExitUsage, errors.New("--timeout must be positive"))
		}
		rt.Timeout = opt.timeout
	}
	if flags.Changed("theme") {
		rt.Theme = opt.theme
	}
	if rt.Theme == "" {
		rt.Theme = "default"
	}
	if !colors.ValidateTheme(rt.Theme) {
		return app.Error(app.ExitUsage, fmt.Errorf("unknown theme %q", rt.Theme))
	}
	if flags.Changed("no-color") {
		rt.NoColor = opt.noColor
	}
	mode := console.OutputMode(opt.output)
	switch mode {
	case console.OutputAuto, console.OutputTerminal, console.OutputPlain, console.OutputJSON, console.OutputNDJSON:
	default:
		return app.Error(app.ExitUsage, fmt.Errorf("invalid --output %q", opt.output))
	}
	var outputFile *os.File
	if file, ok := rt.Stdout.(*os.File); ok {
		outputFile = file
	}
	rt.Capabilities = console.Detect(outputFile, rt.NoColor)
	rt.Output = console.ResolveOutput(mode, rt.Capabilities)
	if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON || rt.Output == console.OutputPlain {
		rt.NoColor = true
	}
	rt.Debug, rt.Quiet, rt.Strict = opt.debug, opt.quiet, opt.strict
	level := slog.LevelInfo
	if rt.Debug {
		level = slog.LevelDebug
	}
	if rt.Quiet {
		level = slog.LevelWarn
	}
	rt.Logger = slog.New(slog.NewTextHandler(rt.Stderr, &slog.HandlerOptions{Level: level}))
	return nil
}

func rootCause(err error) error {
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}
	return err
}
func applyCommon(rt *app.Runtime, cfg *appConfig) {
	cfg.Timeout = rt.Timeout
	cfg.NoColor = rt.NoColor
	cfg.Theme = rt.Theme
	cfg.JSONOutput = rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON
	cfg.PlainOutput = rt.Output == console.OutputPlain
	cfg.Debug = rt.Debug
	cfg.Quiet = rt.Quiet
	cfg.Strict = rt.Strict
}

type appConfig struct {
	Timeout                                       time.Duration
	NoColor                                       bool
	Theme                                         string
	JSONOutput, PlainOutput, Debug, Quiet, Strict bool
}

func newStreamsCommand(rt *app.Runtime) *cobra.Command {
	var server, plexURL, plexToken, jellyURL, jellyToken string
	var watch bool
	var interval int
	var history time.Duration
	cmd := &cobra.Command{Use: "streams", Short: "Show Plex and Jellyfin sessions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := streams.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.JSONOutput, cfg.PlainOutput, cfg.Quiet, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.JSONOutput, common.PlainOutput, common.Quiet, common.Strict
		if cmd.Flags().Changed("server") {
			cfg.ServerType = server
		}
		if cmd.Flags().Changed("plex-url") {
			cfg.PlexURL = strings.TrimSuffix(plexURL, "/")
		}
		if cmd.Flags().Changed("plex-token") {
			cfg.PlexToken = plexToken
		}
		if cmd.Flags().Changed("jellyfin-url") {
			cfg.JellyfinURL = strings.TrimSuffix(jellyURL, "/")
		}
		if cmd.Flags().Changed("jellyfin-token") {
			cfg.JellyfinToken = jellyToken
		}
		if cmd.Flags().Changed("history-duration") {
			cfg.HistoryDuration = history
		}
		cfg.Watch = watch
		if cmd.Flags().Changed("interval") {
			cfg.WatchSeconds = interval
		}
		if cfg.WatchSeconds < 1 {
			return app.Error(app.ExitUsage, errors.New("--interval must be at least 1"))
		}
		if watch && rt.Output == console.OutputJSON {
			return app.Error(app.ExitUsage, errors.New("watch mode requires --output=ndjson for machine output"))
		}
		if cfg.ServerType != "plex" && cfg.ServerType != "jellyfin" && cfg.ServerType != "both" {
			return app.Error(app.ExitUsage, fmt.Errorf("invalid --server %q", cfg.ServerType))
		}
		if (cfg.ServerType == "plex" || cfg.ServerType == "both") && cfg.PlexToken == "" {
			return app.Error(app.ExitUsage, errors.New("plex token is required"))
		}
		if (cfg.ServerType == "jellyfin" || cfg.ServerType == "both") && cfg.JellyfinToken == "" {
			return app.Error(app.ExitUsage, errors.New("jellyfin token is required"))
		}
		return streams.Run(rt.Context, cfg)
	}}
	f := cmd.Flags()
	f.StringVar(&server, "server", "", "server: plex, jellyfin, or both")
	f.StringVar(&plexURL, "plex-url", "", "Plex URL")
	f.StringVar(&plexToken, "plex-token", "", "Plex token")
	f.StringVar(&jellyURL, "jellyfin-url", "", "Jellyfin URL")
	f.StringVar(&jellyToken, "jellyfin-token", "", "Jellyfin token")
	f.BoolVar(&watch, "watch", false, "continuously monitor")
	f.IntVar(&interval, "interval", 0, "watch interval in seconds")
	f.DurationVar(&history, "history-duration", 0, "session history duration")
	_ = cmd.RegisterFlagCompletionFunc("server", fixedCompletions("plex", "jellyfin", "both"))
	return cmd
}

func newCalendarCommand(rt *app.Runtime) *cobra.Command {
	var days, past, interval int
	var watch, noBanner, monitored bool
	var filter string
	cmd := &cobra.Command{Use: "calendar", Short: "Show the Sonarr and Radarr calendar", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := calendar.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.JSONOutput, cfg.PlainOutput, cfg.Debug, cfg.Quiet, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.JSONOutput, common.PlainOutput, common.Debug, common.Quiet, common.Strict
		if cmd.Flags().Changed("days") {
			cfg.Days = days
		}
		if cmd.Flags().Changed("days-past") {
			cfg.DaysPast = past
		}
		if cmd.Flags().Changed("interval") {
			cfg.WatchSeconds = interval
		}
		cfg.WatchMode = watch
		cfg.NoBanner = noBanner
		cfg.MonitoredOnly = monitored
		cfg.Filter = filter
		if cfg.Days < 0 || cfg.DaysPast < 0 || cfg.WatchSeconds < 1 {
			return app.Error(app.ExitUsage, errors.New("days must be non-negative and interval must be at least 1"))
		}
		if watch && rt.Output == console.OutputJSON {
			return app.Error(app.ExitUsage, errors.New("watch mode requires --output=ndjson for machine output"))
		}
		return calendar.Run(rt.Context, cfg)
	}}
	f := cmd.Flags()
	f.IntVar(&days, "days", 0, "future days")
	f.IntVar(&past, "days-past", 0, "past days")
	f.BoolVar(&watch, "watch", false, "continuously monitor")
	f.IntVar(&interval, "interval", 0, "watch interval in seconds")
	f.BoolVar(&noBanner, "no-banner", false, "suppress banner")
	f.StringVar(&filter, "filter", "", "comma-separated filters")
	f.BoolVar(&monitored, "monitored-only", false, "only monitored items")
	return cmd
}

func newRequestsCommand(rt *app.Runtime) *cobra.Command {
	var serverURL, token string
	var verbose bool
	cmd := &cobra.Command{Use: "requests", Short: "Open the interactive Overseerr/Jellyseerr requester", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON {
			return app.Error(app.ExitUsage, errors.New("requests is interactive and does not support JSON or NDJSON"))
		}
		cfg := requests.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.PlainOutput, cfg.Quiet, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.PlainOutput, common.Quiet, common.Strict
		if cmd.Flags().Changed("url") {
			cfg.ServerURL = strings.TrimSuffix(serverURL, "/")
		}
		if cmd.Flags().Changed("token") {
			cfg.APIKey = token
		}
		cfg.Verbose = cfg.Verbose || verbose
		if cfg.ServerURL == "" || cfg.APIKey == "" {
			return app.Error(app.ExitUsage, errors.New("requests URL and API key are required"))
		}
		return requests.Run(rt.Context, cfg)
	}}
	f := cmd.Flags()
	f.StringVar(&serverURL, "url", "", "Overseerr/Jellyseerr URL")
	f.StringVar(&token, "token", "", "API key")
	f.BoolVar(&verbose, "verbose", false, "verbose diagnostics")
	return cmd
}

func newAirtimeCommand(rt *app.Runtime) *cobra.Command {
	var typ string
	var limit, season, past, future int
	var exact, noBanner, full bool
	cmd := &cobra.Command{Use: "airtime <query>", Short: "Find previous and upcoming airtimes", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg := airtime.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.JSONOutput, cfg.PlainOutput, cfg.Debug, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.JSONOutput, common.PlainOutput, common.Debug, common.Strict
		if cmd.Flags().Changed("type") {
			cfg.SearchType = typ
		}
		if cmd.Flags().Changed("limit") {
			cfg.Limit = limit
		}
		if cmd.Flags().Changed("season") {
			cfg.Season = season
		}
		if cmd.Flags().Changed("past") {
			cfg.PastDays = past
		}
		if cmd.Flags().Changed("future") {
			cfg.FutureDays = future
		}
		cfg.Exact = exact
		cfg.NoBanner = noBanner
		cfg.FullSeason = full
		if cfg.SearchType != "auto" && cfg.SearchType != "series" && cfg.SearchType != "movie" {
			return app.Error(app.ExitUsage, errors.New("--type must be auto, series, or movie"))
		}
		if cfg.Limit < 1 || cfg.Limit > 50 {
			return app.Error(app.ExitUsage, errors.New("--limit must be between 1 and 50"))
		}
		return airtime.Run(rt.Context, strings.Join(args, " "), cfg)
	}}
	f := cmd.Flags()
	f.StringVar(&typ, "type", "", "auto, series, or movie")
	f.IntVar(&limit, "limit", 0, "maximum matches")
	f.BoolVar(&exact, "exact", false, "require exact title")
	f.IntVar(&season, "season", 0, "season override")
	f.IntVar(&past, "past", 0, "past days")
	f.IntVar(&future, "future", 0, "future days")
	f.BoolVar(&noBanner, "no-banner", false, "suppress banner")
	f.BoolVar(&full, "full-season", false, "show full season")
	_ = cmd.RegisterFlagCompletionFunc("type", fixedCompletions("auto", "series", "movie"))
	return cmd
}

func newFeedCommand(rt *app.Runtime) *cobra.Command {
	var poll, window time.Duration
	var watch, grabbed, imported, failed, deleted, ignored, subtitles bool
	var events int
	cmd := &cobra.Command{Use: "feed", Short: "Show Sonarr and Radarr activity", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := feed.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.JSONOutput, cfg.PlainOutput, cfg.Quiet, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.JSONOutput, common.PlainOutput, common.Quiet, common.Strict
		if cmd.Flags().Changed("interval") {
			cfg.PollInterval = poll
		}
		if cmd.Flags().Changed("duration") {
			cfg.HistoryWindow = window
		}
		if cmd.Flags().Changed("events") {
			cfg.MaxEvents = events
		}
		cfg.Watch = watch
		values := map[string]bool{"show-grabbed": grabbed, "show-imported": imported, "show-failed": failed, "show-deleted": deleted, "show-ignored": ignored, "show-subtitles": subtitles}
		for name, dst := range map[string]*bool{"show-grabbed": &cfg.ShowGrabbed, "show-imported": &cfg.ShowImported, "show-failed": &cfg.ShowFailed, "show-deleted": &cfg.ShowDeleted, "show-ignored": &cfg.ShowIgnored, "show-subtitles": &cfg.ShowSubtitles} {
			if cmd.Flags().Changed(name) {
				*dst = values[name]
			}
		}
		if cfg.PollInterval <= 0 || cfg.MaxEvents < 0 || cfg.MaxEvents > 100 {
			return app.Error(app.ExitUsage, errors.New("interval must be positive and events between 0 and 100"))
		}
		if watch && rt.Output == console.OutputJSON {
			return app.Error(app.ExitUsage, errors.New("watch mode requires --output=ndjson for machine output"))
		}
		return feed.Run(rt.Context, cfg)
	}}
	f := cmd.Flags()
	f.DurationVar(&poll, "interval", 0, "poll interval")
	f.DurationVar(&window, "duration", 0, "history window")
	f.IntVar(&events, "events", 0, "maximum events")
	f.BoolVar(&watch, "watch", false, "continuously monitor")
	f.BoolVar(&grabbed, "show-grabbed", false, "show grabbed")
	f.BoolVar(&imported, "show-imported", false, "show imported")
	f.BoolVar(&failed, "show-failed", false, "show failed")
	f.BoolVar(&deleted, "show-deleted", false, "show deleted")
	f.BoolVar(&ignored, "show-ignored", false, "show ignored")
	f.BoolVar(&subtitles, "show-subtitles", false, "show subtitles")
	return cmd
}

func newAnimeCommand(rt *app.Runtime) *cobra.Command {
	var limit int
	var mappingURL, mappingPath string
	var force, noTVDB bool
	cmd := &cobra.Command{Use: "anime <query>", Aliases: []string{"anisearch"}, Short: "Search AniList with TVDB mappings", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg := anisearch.BuildToolConfig(rt.Config)
		common := appConfig{}
		applyCommon(rt, &common)
		cfg.Timeout, cfg.NoColor, cfg.Theme, cfg.JSONOutput, cfg.PlainOutput, cfg.Debug, cfg.Quiet, cfg.Strict = common.Timeout, common.NoColor, common.Theme, common.JSONOutput, common.PlainOutput, common.Debug, common.Quiet, common.Strict
		if cmd.Flags().Changed("limit") {
			cfg.Limit = limit
		}
		if cmd.Flags().Changed("mapping-url") {
			cfg.MappingURL = mappingURL
		}
		if cmd.Flags().Changed("mapping-path") {
			cfg.MappingPath = mappingPath
		}
		cfg.ForceRefresh = force
		cfg.NoTVDB = noTVDB
		if cfg.Limit < 1 || cfg.Limit > 50 {
			return app.Error(app.ExitUsage, errors.New("--limit must be between 1 and 50"))
		}
		return anisearch.Run(rt.Context, strings.Join(args, " "), cfg)
	}}
	f := cmd.Flags()
	f.IntVar(&limit, "limit", 0, "results per page")
	f.StringVar(&mappingURL, "mapping-url", "", "mapping URL")
	f.StringVar(&mappingPath, "mapping-path", "", "mapping cache path")
	f.BoolVar(&force, "force-refresh", false, "refresh mapping")
	f.BoolVar(&noTVDB, "no-tvdb", false, "skip TVDB mapping")
	return cmd
}

func newConfigCommand(rt *app.Runtime) *cobra.Command {
	parent := &cobra.Command{Use: "config", Short: "Manage configuration"}
	validate := &cobra.Command{Use: "validate", Args: cobra.NoArgs, Short: "Validate all configuration fields", RunE: func(*cobra.Command, []string) error {
		if err := rt.Config.Validate(); err != nil {
			return app.Error(app.ExitUsage, err)
		}
		if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON {
			return console.WriteEnvelope(rt.Stdout, "config validate", map[string]any{"path": rt.ConfigPath, "valid": true}, false, nil, rt.Clock.Now())
		}
		fmt.Fprintf(rt.Stdout, "Configuration is valid: %s\n", rt.ConfigPath)
		return nil
	}}
	var force, defaults bool
	setup := &cobra.Command{Use: "setup", Args: cobra.NoArgs, Short: "Interactively configure every CalmsToolkit feature", Long: "Guide the user through general settings, services, feature defaults, and paths, then securely save the complete configuration.", RunE: func(*cobra.Command, []string) error {
		if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON {
			return app.Error(app.ExitUsage, errors.New("config setup is interactive and does not support JSON or NDJSON output"))
		}
		_, statErr := os.Stat(rt.ConfigPath)
		if statErr == nil && defaults && !force {
			return app.Error(app.ExitUsage, fmt.Errorf("configuration already exists at %s (use --force to replace)", rt.ConfigPath))
		}
		var cfg *config.ToolkitConfig
		if force || statErr != nil {
			cfg = config.DefaultToolkitConfig()
		} else {
			var err error
			cfg, err = config.LoadPersistedToolkitConfigAt(rt.ConfigPath)
			if err != nil {
				return err
			}
		}
		if !defaults {
			if err := promptSetup(rt, cfg); err != nil {
				if errors.Is(err, errSetupCancelled) {
					fmt.Fprintln(rt.Stderr, "Setup cancelled; no changes were saved.")
					return nil
				}
				return app.Error(app.ExitUsage, err)
			}
		}
		if err := cfg.Validate(); err != nil {
			return app.Error(app.ExitUsage, err)
		}
		if err := cfg.SaveAt(rt.ConfigPath); err != nil {
			return err
		}
		fmt.Fprintf(rt.Stdout, "Saved configuration to %s with mode 0600. Run 'calmstoolkit config validate' or 'calmstoolkit doctor' to verify it.\n", rt.ConfigPath)
		return nil
	}}
	setup.Flags().BoolVar(&force, "force", false, "replace existing configuration")
	setup.Flags().BoolVar(&defaults, "defaults", false, "write all defaults without prompting")
	parent.AddCommand(setup, validate)
	return parent
}

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func newDoctorCommand(rt *app.Runtime) *cobra.Command {
	return &cobra.Command{Use: "doctor", Args: cobra.NoArgs, Short: "Check config, permissions, terminal, and services", RunE: func(*cobra.Command, []string) error {
		checks := runDoctor(rt)
		failed := false
		for _, c := range checks {
			failed = failed || !c.OK
		}
		if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON {
			if err := console.WriteEnvelope(rt.Stdout, "doctor", checks, failed, nil, rt.Clock.Now()); err != nil {
				return err
			}
		} else {
			for _, c := range checks {
				status := "OK"
				if !c.OK {
					status = "FAIL"
				}
				fmt.Fprintf(rt.Stdout, "%-4s  %-18s %s\n", status, c.Name, c.Detail)
			}
		}
		if failed {
			return app.Error(app.ExitOperational, errors.New("one or more doctor checks failed"))
		}
		return nil
	}}
}
func runDoctor(rt *app.Runtime) []doctorCheck {
	checks := []doctorCheck{{Name: "configuration", OK: rt.Config.Validate() == nil, Detail: rt.ConfigPath}, {Name: "terminal", OK: true, Detail: fmt.Sprintf("tty=%t utf8=%t color=%t width=%d", rt.Capabilities.TTY, rt.Capabilities.UTF8, rt.Capabilities.Color, rt.Capabilities.Width)}}
	if info, err := os.Stat(rt.ConfigPath); err == nil {
		checks = append(checks, doctorCheck{Name: "permissions", OK: info.Mode().Perm() == 0600, Detail: info.Mode().Perm().String()})
	} else {
		checks = append(checks, doctorCheck{Name: "permissions", OK: false, Detail: "configuration file unavailable"})
	}
	type endpoint struct{ name, url, token, header string }
	eps := []endpoint{}
	for _, i := range rt.Config.Sonarr {
		eps = append(eps, endpoint{"sonarr " + i.Name, i.URL + "/api/v3/system/status", i.APIKey, "X-Api-Key"})
	}
	for _, i := range rt.Config.Radarr {
		eps = append(eps, endpoint{"radarr " + i.Name, i.URL + "/api/v3/system/status", i.APIKey, "X-Api-Key"})
	}
	if rt.Config.MediaStreams.PlexToken != "" {
		eps = append(eps, endpoint{"plex", strings.TrimSuffix(rt.Config.MediaStreams.PlexURL, "/") + "/identity", rt.Config.MediaStreams.PlexToken, "X-Plex-Token"})
	}
	if rt.Config.MediaStreams.JellyfinToken != "" {
		eps = append(eps, endpoint{"jellyfin", strings.TrimSuffix(rt.Config.MediaStreams.JellyfinURL, "/") + "/System/Info", rt.Config.MediaStreams.JellyfinToken, "X-Emby-Token"})
	}
	if rt.Config.MediaRequests.APIKey != "" {
		eps = append(eps, endpoint{"requests", strings.TrimSuffix(rt.Config.MediaRequests.OverseerrURL, "/") + "/api/v1/status", rt.Config.MediaRequests.APIKey, "X-Api-Key"})
	}
	for _, e := range eps {
		ctx, cancel := context.WithTimeout(rt.Context, rt.Timeout)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.url, nil)
		req.Header.Set(e.header, e.token)
		resp, err := rt.HTTPClient(rt.Timeout).Do(req)
		cancel()
		ok := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300
		var detail string
		if err != nil {
			detail = err.Error()
		} else {
			detail = resp.Status
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		checks = append(checks, doctorCheck{Name: e.name, OK: ok, Detail: detail})
	}
	return checks
}

func newVersionCommand(rt *app.Runtime) *cobra.Command {
	return &cobra.Command{Use: "version", Args: cobra.NoArgs, Short: "Print build version", RunE: func(*cobra.Command, []string) error {
		if rt.Output == console.OutputJSON || rt.Output == console.OutputNDJSON {
			return console.WriteEnvelope(rt.Stdout, "version", map[string]string{"version": buildinfo.Version, "commit": buildinfo.Commit, "built_at": buildinfo.Date}, false, nil, rt.Clock.Now())
		}
		buildinfo.Print(rt.Stdout)
		return nil
	}}
}

// Execute runs root and maps its returned error to a process status.
func Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	rt := app.NewRuntime(ctx, stdin, stdout, stderr)
	root := NewRootCommand(rt)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil && app.ExitCode(err) == app.ExitOperational && cobraUsageError(err) {
		err = app.Error(app.ExitUsage, err)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return app.ExitCode(err)
}

func cobraUsageError(err error) bool {
	message := err.Error()
	for _, marker := range []string{"unknown command", "unknown flag", "requires at least", "accepts ", "required flag", "invalid argument"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
