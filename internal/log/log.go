// SPDX-License-Identifier: MIT
package log

import (
	"context"
	"log/slog"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger is the universal logging interface for Dxrk.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	Level() Level
}

type slogAdapter struct {
	logger *slog.Logger
	level  Level
}

func (a *slogAdapter) Debug(msg string, args ...any) { a.logger.Debug(msg, args...) }
func (a *slogAdapter) Info(msg string, args ...any)  { a.logger.Info(msg, args...) }
func (a *slogAdapter) Warn(msg string, args ...any)  { a.logger.Warn(msg, args...) }
func (a *slogAdapter) Error(msg string, args ...any) { a.logger.Error(msg, args...) }
func (a *slogAdapter) With(args ...any) Logger {
	return &slogAdapter{logger: a.logger.With(args...), level: a.level}
}
func (a *slogAdapter) Level() Level { return a.level }

func detectLevel(logger *slog.Logger) Level {
	ctx := context.Background()
	if logger.Enabled(ctx, slog.LevelDebug) {
		return LevelDebug
	}
	if logger.Enabled(ctx, slog.LevelInfo) {
		return LevelInfo
	}
	if logger.Enabled(ctx, slog.LevelWarn) {
		return LevelWarn
	}
	return LevelError
}

// NewSlog creates a Logger backed by log/slog.
func NewSlog(s *slog.Logger) Logger {
	return &slogAdapter{logger: s, level: detectLevel(s)}
}

type nopLogger struct{}

func (nopLogger) Debug(_ string, _ ...any) {}
func (nopLogger) Info(_ string, _ ...any)  {}
func (nopLogger) Warn(_ string, _ ...any)  {}
func (nopLogger) Error(_ string, _ ...any) {}
func (nopLogger) With(_ ...any) Logger     { return nopLogger{} }
func (nopLogger) Level() Level             { return LevelInfo }

// NewNop creates a no-op logger.
func NewNop() Logger { return nopLogger{} }
