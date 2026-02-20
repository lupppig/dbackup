package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogger_JSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Writer: &buf,
		JSON:   true,
		Level:  slog.LevelInfo,
	})

	l.Info("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, `"level":"INFO"`)
	assert.Contains(t, output, `"msg":"test message"`)
	assert.Contains(t, output, `"key":"value"`)
}

func TestLogger_Text(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Writer:  &buf,
		JSON:    false,
		NoColor: true, // test without color codes for easier matching
		Level:   slog.LevelDebug,
	})

	l.Debug("debug msg", "foo", "bar")
	l.Warn("warn msg")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "[DEBUG]")
	assert.Contains(t, lines[0], "debug msg")
	assert.Contains(t, lines[0], "foo=bar")

	assert.Contains(t, lines[1], "[WARN]")
	assert.Contains(t, lines[1], "warn msg")
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Writer:  &buf,
		JSON:    false,
		NoColor: true,
		Level:   slog.LevelWarn, // Should ignore Info and Debug
	})

	l.Debug("debug")
	l.Info("info")
	l.Warn("warn")
	l.Error("error")

	output := buf.String()
	assert.NotContains(t, output, "debug")
	assert.NotContains(t, output, "info")
	assert.Contains(t, output, "warn")
	assert.Contains(t, output, "error")
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Writer: &buf,
		JSON:   true,
		Level:  slog.LevelInfo,
	})

	l2 := l.With("context", "request123")
	l2.Info("processing")

	output := buf.String()
	assert.Contains(t, output, `"context":"request123"`)
}
