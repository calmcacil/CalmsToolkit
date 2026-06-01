package slogx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var defaultLogger = New(Options{})

func Default() *slog.Logger { return defaultLogger }

func SetDefault(l *slog.Logger) { defaultLogger = l; slog.SetDefault(l) }

func InitDefault(opts Options) { l := New(opts); SetDefault(l) }

func Fmt(l *slog.Logger, msg string, err error) {
	if err != nil && l != nil {
		l.Error(msg, "err", err)
	}
}

type Options struct {
	Level  slog.Leveler
	JSON   bool
	Writer io.Writer
}

func New(opts Options) *slog.Logger {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}
	hOpts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(w, hOpts)
	} else {
		h = &prefixedHandler{
			w:     w,
			mu:    new(sync.Mutex),
			level: level,
		}
	}
	return slog.New(h)
}

type prefixedHandler struct {
	w      io.Writer
	mu     *sync.Mutex
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func (h *prefixedHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *prefixedHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var buf []byte
	buf = append(buf, levelPrefix(r.Level)...)
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)
	if len(h.attrs) > 0 {
		for _, a := range h.attrs {
			buf = append(buf, ' ')
			buf = append(buf, a.Key...)
			buf = append(buf, '=')
			buf = append(buf, a.Value.String()...)
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
		return true
	})
	buf = append(buf, '\n')
	_, err := h.w.Write(buf)
	return err
}

func (h *prefixedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(n.attrs, attrs...)
	return &n
}

func (h *prefixedHandler) WithGroup(name string) slog.Handler {
	n := *h
	n.groups = append(n.groups, name)
	return &n
}

func levelPrefix(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
