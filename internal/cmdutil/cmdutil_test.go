package cmdutil

import (
	"flag"
	"os"
	"os/exec"
	"testing"
)

func TestLoadAndValidate_MissingConfigIsWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tk := LoadAndValidate()
	if tk != nil {
		t.Fatal("expected nil config when no config file exists")
	}
}

func TestRegisterCommonFlags_DefaultTimeout(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := RegisterCommonFlags(fs, nil, Options{})
	if err := fs.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	c.Apply()
	if c.Timeout != 10e9 {
		t.Errorf("default timeout = %v, want 10s", c.Timeout)
	}
}

func TestVersionFlagExits(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		os.Args = []string{"test", "--version"}
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		c := RegisterCommonFlags(flag.CommandLine, nil, Options{})
		flag.Parse()
		c.Apply()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestVersionFlagExits")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--version should exit 0, got: %v", err)
	}
}

func TestNoColorImpliedByJSON(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := RegisterCommonFlags(fs, nil, Options{})
	if err := fs.Parse([]string{"-json"}); err != nil {
		t.Fatal(err)
	}
	c.Apply()
	if !c.NoColor {
		t.Error("NoColor should be true when JSON is enabled")
	}
}

func TestInvalidThemeFallsBack(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := RegisterCommonFlags(fs, nil, Options{})
	if err := fs.Parse([]string{"-theme", "nonexistent"}); err != nil {
		t.Fatal(err)
	}
	c.Apply()
	if c.Theme != "default" {
		t.Errorf("invalid theme should fall back to default, got %q", c.Theme)
	}
	if c.Palette == nil {
		t.Error("Palette should not be nil after Apply")
	}
}

func TestWatchFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := RegisterCommonFlags(fs, nil, Options{IncludeWatch: true})
	if err := fs.Parse([]string{"-watch", "-interval", "5"}); err != nil {
		t.Fatal(err)
	}
	c.Apply()
	if !c.Watch {
		t.Error("Watch should be true")
	}
	if c.WatchSeconds != 5 {
		t.Errorf("WatchSeconds = %d, want 5", c.WatchSeconds)
	}
}
