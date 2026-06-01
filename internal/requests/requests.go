package requests

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
)

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{
		CommonConfig: core.FromToolkit(tk),
	}
	if tk == nil {
		cfg.ServerURL = "http://localhost:5055"
		return cfg
	}
	cfg.ServerURL = strings.TrimSuffix(tk.MediaRequests.OverseerrURL, "/")
	cfg.APIKey = tk.MediaRequests.APIKey
	cfg.Verbose = tk.MediaRequests.Verbose
	return cfg
}

// Run executes the media requests interactive tool.
func Run(cfg ToolConfig) {
	if cfg.APIKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: API key is not set\n")
		fmt.Fprintf(os.Stderr, "Set api_key in ~/.config/calmstoolkit/config.json or use -token flag\n")
		os.Exit(1)
	}

	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Server URL is not set\n")
		fmt.Fprintf(os.Stderr, "Set overseerr_url in ~/.config/calmstoolkit/config.json or use -url flag\n")
		os.Exit(1)
	}

	ctx := context.Background()
	if err := testConnection(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to server: %v\n", err)
		os.Exit(1)
	}

	runInteractiveMenu(ctx, cfg)
}
