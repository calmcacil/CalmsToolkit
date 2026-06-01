package main

import (
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	cfg := config.DefaultToolkitConfig()
	if cfg == nil {
		t.Fatal("DefaultToolkitConfig() returned nil")
	}
	if cfg.MediaStreams.ServerType != "both" {
		t.Errorf("ServerType = %q, want %q", cfg.MediaStreams.ServerType, "both")
	}
}
