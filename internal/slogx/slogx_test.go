package slogx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_TextDefault(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{Writer: &buf})
	l.Info("hello world")
	output := buf.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected message in output, got %q", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level in output, got %q", output)
	}
}

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{JSON: true, Writer: &buf})
	l.Info("hello")
	output := buf.String()
	if !strings.Contains(output, `"msg":"hello"`) {
		t.Errorf("expected JSON message in output, got %q", output)
	}
}

func TestNew_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{Level: slog.LevelDebug, Writer: &buf})
	l.Debug("debug message")
	output := buf.String()
	if !strings.Contains(output, "DEBUG") {
		t.Errorf("expected DEBUG level in output, got %q", output)
	}
}

func TestNew_DebugHiddenByDefault(t *testing.T) {
	var buf bytes.Buffer
	l := New(Options{Writer: &buf})
	l.Debug("should be hidden")
	if buf.Len() > 0 {
		t.Error("debug message should be hidden by default")
	}
}

func TestDefault(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestInitDefault(t *testing.T) {
	var buf bytes.Buffer
	InitDefault(Options{Writer: &buf, JSON: true})
	l := Default()
	l.Info("test")
	output := buf.String()
	if !strings.Contains(output, `"msg":"test"`) {
		t.Errorf("expected JSON after InitDefault, got %q", output)
	}
}
