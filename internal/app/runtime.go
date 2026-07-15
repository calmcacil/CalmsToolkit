// Package app provides the shared command runtime and exit semantics.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/console"
)

const (
	ExitOK          = 0
	ExitOperational = 1
	ExitUsage       = 2
	ExitPartial     = 3
	ExitInterrupted = 130
)

// Clock permits deterministic timestamps and polling tests.
type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Runtime is the only command-level dependency container.
type Runtime struct {
	Context                       context.Context
	Stdin                         io.Reader
	Stdout, Stderr                io.Writer
	Logger                        *slog.Logger
	Clock                         Clock
	HTTPClient                    func(time.Duration) *http.Client
	Capabilities                  console.Capabilities
	Output                        console.OutputMode
	Config                        *config.ToolkitConfig
	ConfigPath                    string
	Theme                         string
	NoColor, Debug, Quiet, Strict bool
	Timeout                       time.Duration
}

// NewRuntime constructs a runtime with production defaults and injectable I/O.
func NewRuntime(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *Runtime {
	return &Runtime{Context: ctx, Stdin: stdin, Stdout: stdout, Stderr: stderr, Clock: realClock{}, HTTPClient: func(timeout time.Duration) *http.Client { return &http.Client{Timeout: timeout} }, Output: console.OutputAuto, Timeout: 10 * time.Second}
}

// ExitError carries an intentional process status without terminating a package.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return "command failed"
	}
	return e.Err.Error()
}
func (e *ExitError) Unwrap() error    { return e.Err }
func Error(code int, err error) error { return &ExitError{Code: code, Err: err} }
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	var partial interface{ PartialResult() bool }
	if errors.As(err, &partial) && partial.PartialResult() {
		return ExitPartial
	}
	return ExitOperational
}
