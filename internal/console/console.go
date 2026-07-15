// Package console owns terminal capability detection and machine rendering.
package console

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// OutputMode is a stable output contract shared by every command.
type OutputMode string

const (
	OutputAuto     OutputMode = "auto"
	OutputTerminal OutputMode = "terminal"
	OutputPlain    OutputMode = "plain"
	OutputJSON     OutputMode = "json"
	OutputNDJSON   OutputMode = "ndjson"
)

// Capabilities describes the output terminal without coupling views to os.Stdout.
type Capabilities struct {
	TTY, UTF8, Color, Unicode bool
	Width                     int
}

// Layout selects the responsive information density.
type Layout int

const (
	LayoutStacked Layout = iota
	LayoutCompact
	LayoutFull
)

func LayoutForWidth(width int) Layout {
	if width < 60 {
		return LayoutStacked
	}
	if width < 100 {
		return LayoutCompact
	}
	return LayoutFull
}

// BorderSet contains the shared box glyphs.
type BorderSet struct{ TopLeft, TopRight, BottomLeft, BottomRight, Horizontal, Vertical string }

func Borders(unicode bool) BorderSet {
	if unicode {
		return BorderSet{"┌", "┐", "└", "┘", "─", "│"}
	}
	return BorderSet{"+", "+", "+", "+", "-", "|"}
}

// Detect inspects an output file and relevant terminal environment variables.
func Detect(file *os.File, noColor bool) Capabilities {
	c := Capabilities{Width: 80}
	if file != nil {
		c.TTY = term.IsTerminal(int(file.Fd()))
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			c.Width = width
		}
	}
	lang := strings.ToLower(os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG"))
	c.UTF8 = strings.Contains(lang, "utf-8") || strings.Contains(lang, "utf8")
	c.Unicode = c.UTF8 && os.Getenv("TERM") != "dumb"
	c.Color = c.TTY && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == "" && !noColor
	return c
}

// ResolveOutput resolves auto based on terminal capabilities.
func ResolveOutput(mode OutputMode, caps Capabilities) OutputMode {
	if mode != OutputAuto {
		return mode
	}
	if caps.TTY && caps.Unicode {
		return OutputTerminal
	}
	return OutputPlain
}

// Envelope is the versioned machine-output contract.
type Envelope struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Command       string    `json:"command"`
	Partial       bool      `json:"partial"`
	Warnings      []string  `json:"warnings"`
	Data          any       `json:"data"`
}

// WriteEnvelope writes exactly one JSON value (also one NDJSON record).
func WriteEnvelope(w io.Writer, command string, data any, partial bool, warnings []string, now time.Time) error {
	if warnings == nil {
		warnings = []string{}
	}
	return json.NewEncoder(w).Encode(Envelope{SchemaVersion: "1", GeneratedAt: now.UTC(), Command: command, Partial: partial, Warnings: warnings, Data: data})
}

var ansiPattern = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

// StripANSI removes terminal control sequences from machine and plain output.
func StripANSI(value string) string { return ansiPattern.ReplaceAllString(value, "") }

// Truncate shortens text by display cells and never counts ANSI sequences.
func Truncate(value string, width int) string {
	value = StripANSI(value)
	if width <= 0 {
		return ""
	}
	return runewidth.Truncate(value, width, "")
}

// PadRight pads or truncates a string to exactly width display cells.
func PadRight(value string, width int) string {
	value = Truncate(value, width)
	return value + strings.Repeat(" ", max(0, width-runewidth.StringWidth(value)))
}

// Wrap splits text into display-width-bounded lines.
func Wrap(value string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	wrapped := runewidth.Wrap(StripANSI(value), width)
	return strings.Split(wrapped, "\n")
}

// Progress renders a bounded semantic progress bar without color.
func Progress(percent float64, width int, unicode bool) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if width < 0 {
		width = 0
	}
	filled := int(percent*float64(width)/100 + .5)
	full, empty := "#", "-"
	if unicode {
		full, empty = "█", "░"
	}
	return strings.Repeat(full, filled) + strings.Repeat(empty, width-filled)
}
