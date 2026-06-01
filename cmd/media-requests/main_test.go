package main

import (
	"os"
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func TestConfigPath(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	dir := t.TempDir()
	os.Setenv("HOME", dir)

	path := config.ConfigPath()
	if path == "" {
		t.Fatal("ConfigPath() returned empty")
	}
}
