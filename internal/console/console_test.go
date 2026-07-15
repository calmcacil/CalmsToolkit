package console

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWriteEnvelopeAndNoANSI(t *testing.T) {
	var out bytes.Buffer
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
	if err := WriteEnvelope(&out, "streams", map[string]int{"active": 2}, false, nil, now); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatal("machine output contains ANSI")
	}
	var got Envelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != "1" || got.Command != "streams" || got.GeneratedAt.Location() != time.UTC {
		t.Fatalf("unexpected envelope: %+v", got)
	}
}

func TestStripTruncateAndPadUnicode(t *testing.T) {
	if got := StripANSI("\x1b[31mhello\x1b[0m"); got != "hello" {
		t.Fatalf("StripANSI = %q", got)
	}
	if got := Truncate("界界界", 4); got != "界界" {
		t.Fatalf("Truncate = %q", got)
	}
	if got := PadRight("界", 4); got != "界  " {
		t.Fatalf("PadRight = %q", got)
	}
}

func TestResolveOutput(t *testing.T) {
	if got := ResolveOutput(OutputAuto, Capabilities{}); got != OutputPlain {
		t.Fatalf("got %q", got)
	}
	if got := ResolveOutput(OutputAuto, Capabilities{TTY: true, Unicode: true}); got != OutputTerminal {
		t.Fatalf("got %q", got)
	}
}

func TestResponsiveLayoutAndPrimitives(t *testing.T) {
	for _, tt := range []struct {
		width int
		want  Layout
	}{{40, LayoutStacked}, {59, LayoutStacked}, {60, LayoutCompact}, {99, LayoutCompact}, {100, LayoutFull}, {120, LayoutFull}} {
		if got := LayoutForWidth(tt.width); got != tt.want {
			t.Errorf("width %d layout=%v", tt.width, got)
		}
	}
	if got := Progress(50, 6, false); got != "###---" {
		t.Fatalf("progress=%q", got)
	}
	if got := Borders(false).TopLeft; got != "+" {
		t.Fatalf("ASCII border=%q", got)
	}
	for _, line := range Wrap("界界界", 4) {
		if len([]rune(line)) > 2 {
			t.Fatalf("wide wrap=%q", line)
		}
	}
}
