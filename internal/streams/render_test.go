package streams

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/console"
)

func captureStdout(t *testing.T, render func() error) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = originalStdout
		_ = r.Close()
		_ = w.Close()
	})

	err = render()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close output pipe: %v", closeErr)
	}
	os.Stdout = originalStdout
	if err != nil {
		t.Fatalf("render error = %v", err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(output)
}

func assertEmptyStreamsBox(t *testing.T, output string) {
	t.Helper()

	output = console.StripANSI(output)
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5:\n%s", len(lines), output)
	}
	if !strings.HasPrefix(lines[2], "├") || !strings.HasSuffix(lines[2], "┤") {
		t.Errorf("empty-state separator is malformed: %q", lines[2])
	}
	if !strings.Contains(lines[3], "No active streams") {
		t.Errorf("empty-state message missing: %q", lines[3])
	}
	if !strings.HasPrefix(lines[4], "└") || !strings.HasSuffix(lines[4], "┘") {
		t.Errorf("empty-state bottom border is malformed: %q", lines[4])
	}
	for i, line := range lines {
		if got := colors.VisibleLen(line); got != 80 {
			t.Errorf("line %d width = %d, want 80: %q", i+1, got, line)
		}
	}
}

func TestDisplayTerminalOutputEmptyStateKeepsBoxOpen(t *testing.T) {
	p := colors.GetPalette("")
	for _, tc := range []struct {
		name    string
		noColor bool
	}{
		{name: "terminal", noColor: false},
		{name: "plain", noColor: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := captureStdout(t, func() error {
				return displayTerminalOutput(nil, 0, 0, tc.noColor, p)
			})
			assertEmptyStreamsBox(t, output)
			if tc.noColor && output != console.StripANSI(output) {
				t.Fatal("plain output contains ANSI sequences")
			}
		})
	}
}

func TestDisplayTerminalOutputWithHistoryEmptyStateKeepsBoxOpen(t *testing.T) {
	p := colors.GetPalette("")
	for _, tc := range []struct {
		name  string
		plain bool
	}{
		{name: "watch terminal", plain: false},
		{name: "watch plain", plain: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := &SessionHistory{Records: make(map[string]*SessionRecord)}
			output := captureStdout(t, func() error {
				return displayTerminalOutputWithHistory(nil, history, 0, 0, true, tc.plain, p)
			})
			assertEmptyStreamsBox(t, output)
			if tc.plain && output != console.StripANSI(output) {
				t.Fatal("plain watch output contains ANSI sequences")
			}
		})
	}
}

func TestDisplayJSONOutputEmptyStateIsMachineSafe(t *testing.T) {
	output := captureStdout(t, func() error {
		return displayJSONOutput(nil, 0, 0, false, nil)
	})
	if output != console.StripANSI(output) {
		t.Fatal("machine output contains ANSI sequences")
	}

	var envelope struct {
		SchemaVersion string  `json:"schema_version"`
		Command       string  `json:"command"`
		Data          Summary `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("invalid machine output: %v\n%s", err, output)
	}
	if envelope.SchemaVersion != "1" || envelope.Command != "streams" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if envelope.Data.TotalStreams != 0 || len(envelope.Data.Streams) != 0 {
		t.Fatalf("unexpected empty summary: %+v", envelope.Data)
	}
}
