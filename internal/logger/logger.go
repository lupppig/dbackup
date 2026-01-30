package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

type Logger struct {
	inner *slog.Logger
	color bool
}

type Config struct {
	Writer  io.Writer
	JSON    bool
	NoColor bool
	Level   slog.Level
}

func New(cfg Config) *Logger {
	if cfg.Writer == nil {
		cfg.Writer = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   a.Key,
					Value: slog.StringValue(a.Value.Time().Format("2006/01/02 15:04:05")),
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(cfg.Writer, opts)
	} else {
		handler = &colorHandler{
			Handler: slog.NewTextHandler(cfg.Writer, opts),
			Writer:  cfg.Writer,
			noColor: cfg.NoColor,
		}
	}

	return &Logger{
		inner: slog.New(handler),
		color: !cfg.NoColor && !cfg.JSON,
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		inner: l.inner.With(args...),
		color: l.color,
	}
}

type colorHandler struct {
	slog.Handler
	Writer  io.Writer
	noColor bool
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	cLevel := level
	cKey := ""
	cReset := ""
	cTime := ""

	if !h.noColor {
		cReset = colorReset
		cKey = colorCyan
		switch r.Level {
		case slog.LevelInfo:
			cLevel = colorGreen + level + colorReset
		case slog.LevelWarn:
			cLevel = colorYellow + level + colorReset
		case slog.LevelError:
			cLevel = colorRed + level + colorReset
		case slog.LevelDebug:
			cLevel = colorBlue + level + colorReset
		}
		cTime = "\033[2m" // Faint for time
	}

	attrs := ""
	r.Attrs(func(a slog.Attr) bool {
		attrs += fmt.Sprintf(" %s%s%s=%v", cKey, a.Key, cReset, a.Value.Any())
		return true
	})

	fmt.Fprintf(h.Writer, "%s%s%s [%s] %s%s\n",
		cTime, r.Time.Format("2006/01/02 15:04:05"), cReset,
		cLevel,
		r.Message,
		attrs,
	)

	return nil
}
