//go:build queueremediation && manual

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	var (
		sonarrURLs   = flag.String("sonarr-urls", "", "Comma-separated Sonarr URLs")
		sonarrTokens = flag.String("sonarr-tokens", "", "Comma-separated Sonarr API tokens")
		radarrURLs   = flag.String("radarr-urls", "", "Comma-separated Radarr URLs")
		radarrTokens = flag.String("radarr-tokens", "", "Comma-separated Radarr API tokens")
		timeout      = flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
		useRestAPI   = flag.Bool("use-rest-api", false, "Use REST API for manual imports instead of Command API")
		verbose      = flag.Bool("verbose", false, "Show verbose logging (API calls, filtering decisions)")
		debug        = flag.Bool("debug", false, "Show debug logging (full request/response payloads, implies -verbose)")
	)
	flag.Parse()

	// Debug implies verbose
	verboseMode := *verbose || *debug

	// Configure log output to stderr with no prefix (we add our own [LEVEL] tags)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags)

	config := loadConfig(*sonarrURLs, *sonarrTokens, *radarrURLs, *radarrTokens, *timeout, *useRestAPI, verboseMode, *debug)

	if err := validateQueueConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please set SONARR_URLS/SONARR_TOKENS or RADARR_URLS/RADARR_TOKENS environment variables\n")
		fmt.Fprintf(os.Stderr, "Or use -sonarr-urls/-sonarr-tokens or -radarr-urls/-radarr-tokens flags\n")
		os.Exit(1)
	}

	if err := RunTUI(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
