package requests

import (
	"context"
	"fmt"
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
func Run(ctx context.Context, cfg ToolConfig) error {
	if cfg.APIKey == "" {
		return fmt.Errorf("API key is not set")
	}

	if cfg.ServerURL == "" {
		return fmt.Errorf("server URL is not set")
	}

	if err := testConnection(ctx, cfg); err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	runInteractiveMenu(ctx, cfg)
	return nil
}
