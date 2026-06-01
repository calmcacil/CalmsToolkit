package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/cmdutil"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/requests"
)

func main() {
	tk := cmdutil.LoadAndValidate()
	cfg := requests.BuildToolConfig(tk)

	url := flag.String("url", cfg.ServerURL, "Overseerr/Jellyseerr server URL")
	token := flag.String("token", config.TokenFromEnv("OVERSEERR_API_KEY", cfg.APIKey), "API key/token")
	verbose := flag.Bool("verbose", cfg.Verbose, "Enable verbose diagnostic output")

	cu := cmdutil.RegisterCommonFlags(flag.CommandLine, tk, cmdutil.Options{
		IncludeQuiet: true,
	})
	flag.Parse()
	cu.Apply()

	cfg.ServerURL = strings.TrimSuffix(*url, "/")
	cfg.APIKey = *token
	cfg.Timeout = cu.Timeout
	cfg.NoColor = cu.NoColor
	cfg.Theme = cu.Theme
	cfg.JSONOutput = cu.JSONFlag()
	cfg.Verbose = *verbose
	cfg.Quiet = cu.Quiet

	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: server URL is required (use -url flag or set overseerr_url in config)\n")
		os.Exit(1)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	requests.Run(cfg)
}
