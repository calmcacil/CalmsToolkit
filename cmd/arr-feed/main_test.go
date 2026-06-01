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
	if cfg.ArrFeed.MaxEvents != 50 {
		t.Errorf("MaxEvents = %d, want %d", cfg.ArrFeed.MaxEvents, 50)
	}
}
