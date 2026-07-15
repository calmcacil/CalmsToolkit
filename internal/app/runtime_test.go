package app

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/core"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{{nil, 0}, {errors.New("failure"), 1}, {Error(ExitUsage, errors.New("bad flags")), 2}, {&core.PartialError{Warnings: []string{"source"}}, 3}, {context.Canceled, 130}}
	for _, tt := range tests {
		if got := ExitCode(tt.err); got != tt.want {
			t.Errorf("ExitCode(%v)=%d want %d", tt.err, got, tt.want)
		}
	}
}

func TestNewRuntimeDefaults(t *testing.T) {
	rt := NewRuntime(context.Background(), nil, io.Discard, io.Discard)
	if rt.Context == nil || rt.Clock == nil || rt.HTTPClient == nil || rt.Timeout <= 0 {
		t.Fatalf("incomplete runtime: %+v", rt)
	}
}
