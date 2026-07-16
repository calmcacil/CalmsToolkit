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
	for _, name := range []string{"streams", "calendar", "requests", "airtime", "feed", "anime", "config", "completion", "doctor", "version"} {
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

func TestConfigSetupEditsSectionAndKeepsPromptsOffStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	input := strings.Join([]string{
		"1", "25s", "catppuccin-mocha", "yes",
		"s",
	}, "\n") + "\n"
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(input), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(out.String(), "Choose a section") || !strings.Contains(out.String(), "Saved configuration") {
		t.Fatalf("stdout contains prompts or lacks result: %q", out.String())
	}
	if !strings.Contains(stderr.String(), "Configuration sections") {
		t.Fatalf("stderr missing setup menu: %q", stderr.String())
	}
	cfg, err := config.LoadToolkitConfigAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.General.Timeout != "25s" || cfg.General.Theme != "catppuccin-mocha" || !cfg.General.NoColor {
		t.Fatalf("general config not updated: %+v", cfg.General)
	}
}

func TestConfigSetupGuidedFlowCoversEverySection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	answers := []string{"a"}
	answers = append(answers, "", "", "") // General.
	answers = append(answers, "d", "d")   // Sonarr and Radarr.
	answers = append(answers, "", "", "", "", "", "", "")
	answers = append(answers, "", "", "")     // Requests.
	answers = append(answers, "", "", "", "") // Calendar.
	answers = append(answers, "", "", "", "") // Airtime.
	answers = append(answers, "", "", "", "", "", "", "", "", "")
	answers = append(answers, "", "", "") // AniSearch.
	answers = append(answers, "s")
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(strings.Join(answers, "\n")+"\n"), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, section := range []string{"[General]", "[Sonarr Instances]", "[Radarr Instances]", "[Media Streams]", "[Media Requests]", "[Media Calendar]", "[Media Airtime]", "[Arr Feed]", "[AniSearch]"} {
		if !strings.Contains(stderr.String(), section) {
			t.Errorf("guided setup missing %s", section)
		}
	}
}

func TestConfigSetupCanCancelWithoutSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader("q\n"), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("configuration was saved after cancellation: %v", err)
	}
}

func TestConfigSetupRejectsMachineOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, mode := range []string{"json", "ndjson"} {
		t.Run(mode, func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--config", path, "--output", mode, "config", "setup", "--defaults"})
			if code != app.ExitUsage || out.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
			}
		})
	}
}

func TestConfigSetupHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, stderr bytes.Buffer
	code := Execute(ctx, strings.NewReader("1\n"), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != app.ExitInterrupted {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("configuration was saved after cancellation: %v", err)
	}
}

func TestConfigSetupRedactsExistingSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultToolkitConfig()
	cfg.MediaStreams.PlexToken = "super-secret"
	if err := cfg.SaveAt(path); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	input := "4\n\n\n\n\n\n\n\nq\n"
	code := Execute(context.Background(), strings.NewReader(input), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "super-secret") || !strings.Contains(stderr.String(), "Plex token (optional when Plex is disabled) [configured]") {
		t.Fatalf("secret was not redacted: %q", stderr.String())
	}
}

func TestConfigSetupDoesNotPersistEnvironmentSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultToolkitConfig()
	cfg.MediaStreams.PlexToken = "file-secret"
	if err := cfg.SaveAt(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALMSTOOLKIT_PLEX_TOKEN", "environment-secret")
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader("s\n"), &out, &stderr, []string{"--config", path, "config", "setup"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	persisted, err := config.LoadPersistedToolkitConfigAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.MediaStreams.PlexToken != "file-secret" {
		t.Fatalf("setup persisted environment token: %q", persisted.MediaStreams.PlexToken)
	}
}

func TestCompletionScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"--config", filepath.Join(t.TempDir(), "missing.json"), "completion", shell})
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
			if out.Len() < 100 || !strings.Contains(strings.ToLower(out.String()), "calmstoolkit") {
				t.Fatalf("unexpected completion output: %q", out.String())
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"completion", "tcsh"})
	if code != app.ExitUsage || !strings.Contains(stderr.String(), "unsupported shell") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestCompletionSuggestsFlagValues(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, []string{"__complete", "streams", "--server", "j"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(out.String(), "jellyfin") {
		t.Fatalf("server value completion missing: %q", out.String())
	}
}

func TestCompletionDoesNotRequireValidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--config", path, "completion", "bash"},
		{"--config", path, "__complete", "streams", "--server", "p"},
	} {
		var out, stderr bytes.Buffer
		if code := Execute(context.Background(), strings.NewReader(""), &out, &stderr, args); code != 0 {
			t.Fatalf("args=%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
}
