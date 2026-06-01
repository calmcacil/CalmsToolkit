package cmdutil

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/buildinfo"
	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/slogx"
)

type Options struct {
	IncludeWatch bool
	IncludeDebug bool
	IncludeQuiet bool
}

type Common struct {
	Timeout time.Duration
	NoColor bool
	Theme   string
	Debug   bool
	Quiet   bool
	Palette *colors.Palette
	Logger  *slog.Logger

	Watch        bool
	WatchSeconds int

	version  bool
	noColor  bool
	theme    string
	json     bool
	watch    bool
	interval int
	debug    bool
	quiet    bool
	timeout  time.Duration
}

func RegisterCommonFlags(fs *flag.FlagSet, tk *config.ToolkitConfig, opt Options) *Common {
	c := &Common{
		timeout:  10 * time.Second,
		interval: 10,
		theme:    "default",
	}
	if tk != nil {
		if d, err := time.ParseDuration(tk.General.Timeout); err == nil && d > 0 {
			c.timeout = d
		}
		c.theme = tk.General.Theme
		if c.theme == "" {
			c.theme = "default"
		}
	}

	fs.BoolVar(&c.version, "version", false, "Print version and exit")
	fs.BoolVar(&c.noColor, "no-color", false, "Disable colored output")
	fs.StringVar(&c.theme, "theme", c.theme, "Color theme (default, catppuccin-mocha, catppuccin-latte)")
	fs.DurationVar(&c.timeout, "timeout", c.timeout, "HTTP connection timeout")
	fs.BoolVar(&c.json, "json", false, "Output in JSON format")

	if opt.IncludeWatch {
		fs.BoolVar(&c.watch, "watch", false, "Continuously monitor")
		fs.IntVar(&c.interval, "interval", c.interval, "Watch mode refresh interval in seconds")
	}
	if opt.IncludeDebug {
		fs.BoolVar(&c.debug, "debug", false, "Enable debug logging")
	}
	if opt.IncludeQuiet {
		fs.BoolVar(&c.quiet, "quiet", false, "Suppress non-error output")
	}

	return c
}

func (c *Common) Apply() {
	if c.version {
		buildinfo.Print(os.Stdout)
		os.Exit(0)
	}

	c.NoColor = c.noColor || c.json
	c.Theme = c.theme
	if c.Theme != "" && !colors.ValidateTheme(c.Theme) {
		valid := strings.Join(colors.ValidThemes(), ", ")
		fmt.Fprintf(os.Stderr, "Warning: unknown theme %q, falling back to default (valid: %s)\n",
			c.Theme, valid)
		c.Theme = "default"
	}
	c.Palette = colors.GetPalette(c.Theme)

	c.Timeout = c.timeout
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}

	c.Watch = c.watch
	c.WatchSeconds = c.interval
	c.Debug = c.debug
	c.Quiet = c.quiet

	c.Logger = slogx.New(slogx.Options{
		Level: slog.LevelInfo,
		JSON:  c.json,
	})
	if c.debug {
		c.Logger = slogx.New(slogx.Options{
			Level: slog.LevelDebug,
			JSON:  c.json,
		})
	}
	slogx.SetDefault(c.Logger)
}

func (c *Common) JSONFlag() bool {
	return c.json
}

func LoadAndValidate() *config.ToolkitConfig {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: config not loaded, using defaults: %v\n", err)
	}
	if tk != nil {
		if err := tk.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: config validation: %v\n", err)
		}
	}
	return tk
}
