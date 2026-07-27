// SPDX-License-Identifier: MIT
package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlog_Levels(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	s := slog.New(h)
	l := NewSlog(s)

	l.Debug("d msg", "k1", "v1")
	l.Info("i msg", "k2", "v2")
	l.Warn("w msg", "k3", "v3")
	l.Error("e msg", "k4", "v4")

	out := buf.String()
	for _, want := range []string{"d msg", "i msg", "w msg", "e msg"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestNop_Noop(t *testing.T) {
	l := NewNop()
	// must not panic
	l.Debug("x")
	l.Info("x")
	l.Warn("x")
	l.Error("x")
	l.With("key", "val")
	_ = l.Level()
}

func TestSlog_With(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	s := slog.New(h)
	l := NewSlog(s)

	child := l.With("trace", "abc123")
	child.Info("test")

	out := buf.String()
	if !strings.Contains(out, "abc123") {
		t.Errorf("child logger output missing With field: %s", out)
	}
}

func TestSlog_Level(t *testing.T) {
	t.Run("debug", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		l := NewSlog(slog.New(h))
		if l.Level() != LevelDebug {
			t.Errorf("Level() = %d, want %d", l.Level(), LevelDebug)
		}
	})
	t.Run("info", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
		l := NewSlog(slog.New(h))
		if l.Level() != LevelInfo {
			t.Errorf("Level() = %d, want %d", l.Level(), LevelInfo)
		}
	})
	t.Run("warn", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
		l := NewSlog(slog.New(h))
		if l.Level() != LevelWarn {
			t.Errorf("Level() = %d, want %d", l.Level(), LevelWarn)
		}
	})
}
