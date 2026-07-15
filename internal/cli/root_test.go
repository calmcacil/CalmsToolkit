package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/app"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func TestRootCommandSurface(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--help"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, name := range []string{"streams", "calendar", "requests", "airtime", "feed", "anime", "config", "doctor", "version"} {
		if !strings.Contains(out.String(), name) {
			t.Errorf("help missing %q", name)
		}
	}
}

func TestInvalidOutputIsUsageError(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--output=xml", "version"})
	if code != app.ExitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestMissingArgumentsIsUsageError(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"airtime"}); code != app.ExitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestRequestsRejectsMachineMode(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--output=json", "requests"})
	if code != app.ExitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestConfigAndFlagPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.DefaultToolkitConfig()
	cfg.General.NoColor = true
	cfg.General.Theme = "catppuccin-mocha"
	cfg.General.Timeout = "22s"
	cfg.MediaStreams.WatchInterval = 77
	if err := cfg.SaveAt(path); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	rt := app.NewRuntime(context.Background(), strings.NewReader(""), &out, &stderr)
	root := NewRootCommand(rt)
	root.SetArgs([]string{"--config", path, "--output=terminal", "--no-color=false", "--timeout=3s", "version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if rt.NoColor {
		t.Fatal("explicit flag did not override configured no_color")
	}
	if rt.Timeout.String() != "3s" {
		t.Fatalf("timeout=%v", rt.Timeout)
	}
	if rt.Theme != "catppuccin-mocha" {
		t.Fatalf("theme=%q", rt.Theme)
	}
	if rt.Config.MediaStreams.WatchInterval != 77 {
		t.Fatal("configured interval was overwritten")
	}
}

func TestVersionJSONEnvelope(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--config", filepath.Join(t.TempDir(), "missing.json"), "--output=json", "version"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatal("ANSI in JSON")
	}
	if !strings.Contains(out.String(), `"schema_version":"1"`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestConfigSetupPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--config", path, "config", "setup", "--defaults"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestConfigSetupEOFDoesNotSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != app.ExitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("configuration was saved after EOF: %v", err)
	}
}
