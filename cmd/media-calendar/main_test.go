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
	if cfg.MediaCalendar.Days != 1 {
		t.Errorf("Days = %d, want %d", cfg.MediaCalendar.Days, 1)
	}
}
