// SPDX-License-Identifier: MIT
package observe

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	prefix string
}

func NewLogger(prefix string, level Level) *Logger {
	return &Logger{
		level:  level,
		output: os.Stderr,
		prefix: prefix,
	}
}

func DefaultLogger() *Logger {
	return NewLogger("dxrk", LevelInfo)
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	l.output = w
	l.mu.Unlock()
}

func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format(time.RFC3339)

	line := fmt.Sprintf("%s [%s] [%s] %s\n", timestamp, level.String(), l.prefix, msg)
	_, _ = l.output.Write([]byte(line))

	if level >= LevelError {
		_ = log.Output(3, msg)
	}
}

type LogFields map[string]any

func (l *Logger) WithFields(fields LogFields) *Logger {
	parts := make([]string, 0, len(fields))
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	suffix := " " + strings.Join(parts, " ")
	return NewLogger(l.prefix+suffix, l.level)
}

var Log = DefaultLogger()
