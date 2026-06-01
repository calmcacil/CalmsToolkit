package core

import (
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func TestFromToolkitNil(t *testing.T) {
	c := FromToolkit(nil)
	if c.Timeout != 10*time.Second {
		t.Errorf("nil timeout = %v, want 10s", c.Timeout)
	}
}

func TestFromToolkitWithConfig(t *testing.T) {
	tk := config.DefaultToolkitConfig()
	tk.General.Timeout = "30s"
	tk.General.NoColor = true
	tk.General.Theme = "catppuccin-mocha"

	c := FromToolkit(tk)
	if c.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", c.Timeout)
	}
	if !c.NoColor {
		t.Error("NoColor should be true")
	}
	if c.Theme != "catppuccin-mocha" {
		t.Errorf("Theme = %q, want catppuccin-mocha", c.Theme)
	}
}

func TestFromToolkitInvalidTimeout(t *testing.T) {
	tk := config.DefaultToolkitConfig()
	tk.General.Timeout = "not-a-duration"
	c := FromToolkit(tk)
	if c.Timeout != 10*time.Second {
		t.Errorf("invalid timeout should fall back to 10s, got %v", c.Timeout)
	}
}

func TestPickPositive(t *testing.T) {
	if got := pickPositive(5, 1); got != 5 {
		t.Errorf("pickPositive(5, 1) = %d, want 5", got)
	}
	if got := pickPositive(0, 10); got != 10 {
		t.Errorf("pickPositive(0, 10) = %d, want 10", got)
	}
	if got := pickPositive(-1, 10); got != 10 {
		t.Errorf("pickPositive(-1, 10) = %d, want 10", got)
	}
}
