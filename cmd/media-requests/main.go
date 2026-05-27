package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/requests"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := requests.BuildToolConfig(tk)

	url := flag.String("url", cfg.ServerURL, "Overseerr/Jellyseerr server URL")
	token := flag.String("token", cfg.APIKey, "API key/token")
	timeout := flag.Duration("timeout", cfg.Timeout, "Connection timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	verbose := flag.Bool("verbose", cfg.Verbose, "Enable verbose diagnostic output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	quiet := flag.Bool("quiet", false, "Suppress warnings")
	flag.Parse()

	cfg.ServerURL = *url
	cfg.APIKey = *token
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *jsonOutput
	cfg.Verbose = *verbose
	cfg.JSONOutput = *jsonOutput
	cfg.Quiet = *quiet

	if tk != nil {
		if err := tk.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config validation: %v\n", err)
		}
	}
	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: server URL is required (use -url flag or set overseerr_url in config)\n")
		os.Exit(1)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10
	}

	requests.Run(cfg)
}
