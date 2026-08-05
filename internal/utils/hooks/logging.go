package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

var (
	ErrLoggerClosed = errors.New("hooks: logger is closed")
)

// LogLevel represents the log severity level.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	names := [...]string{"debug", "info", "warn", strconst.StrError}
	if int(l) < len(names) {
		return names[l]
	}
	return strconst.StrUnknown
}

// HookLogEntry represents a single hook execution log entry.
type HookLogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     LogLevel          `json:"level"`
	HookID    string            `json:"hook_id"`
	HookType  HookType          `json:"hook_type"`
	Event     HookEvent         `json:"event"`
	Result    HookResult        `json:"result"`
	Duration  time.Duration     `json:"duration"`
	Attempt   int               `json:"attempt"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// HookMetrics holds aggregated hook execution metrics.
type HookMetrics struct {
	mu              sync.RWMutex
	totalExecutions uint64
	successCount    uint64
	failureCount    uint64
	totalDuration   time.Duration
	byType          map[HookType]*TypeMetrics
	byHook          map[string]*HookMetricsEntry
}

type TypeMetrics struct {
	Count    uint64
	Success  uint64
	Failure  uint64
	TotalDur time.Duration
}

type HookMetricsEntry struct {
	ID         string
	Type       HookType
	Count      uint64
	Success    uint64
	Failure    uint64
	TotalDur   time.Duration
	AvgDur     time.Duration
	LastExec   time.Time
	LastResult HookResult
}

// HookLogger handles structured logging and metrics for hook executions.
type HookLogger struct {
	mu         sync.Mutex
	output     io.Writer
	level      LogLevel
	metrics    *HookMetrics
	entries    []HookLogEntry
	maxEntries int
	closed     bool
	hooks      []func(HookLogEntry)
}

// NewHookLogger creates a new hook logger.
func NewHookLogger(output io.Writer, level LogLevel) *HookLogger {
	if output == nil {
		output = io.Discard
	}
	return &HookLogger{
		output:     output,
		level:      level,
		metrics:    &HookMetrics{byType: make(map[HookType]*TypeMetrics), byHook: make(map[string]*HookMetricsEntry)},
		maxEntries: 1000,
		hooks:      make([]func(HookLogEntry), 0),
	}
}

// SetLevel sets the minimum log level.
func (l *HookLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// AddHook adds a callback for log entries.
func (l *HookLogger) AddHook(fn func(HookLogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, fn)
}

// Log logs a hook execution entry.
func (l *HookLogger) Log(entry HookLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	if entry.Level < l.level {
		return
	}

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[1:]
	}

	l.updateMetrics(entry)
	l.writeEntry(entry)
	for _, h := range l.hooks {
		h(entry)
	}
}

func (l *HookLogger) writeEntry(entry HookLogEntry) {
	data, _ := json.Marshal(entry)
	_, _ = fmt.Fprintln(l.output, string(data))
}

func (l *HookLogger) updateMetrics(entry HookLogEntry) {
	atomic.AddUint64(&l.metrics.totalExecutions, 1)
	if entry.Result.Success {
		atomic.AddUint64(&l.metrics.successCount, 1)
	} else {
		atomic.AddUint64(&l.metrics.failureCount, 1)
	}

	l.metrics.mu.Lock()
	defer l.metrics.mu.Unlock()

	l.metrics.totalDuration += entry.Duration

	if _, ok := l.metrics.byType[entry.HookType]; !ok {
		l.metrics.byType[entry.HookType] = &TypeMetrics{}
	}
	tm := l.metrics.byType[entry.HookType]
	tm.Count++
	if entry.Result.Success {
		tm.Success++
	} else {
		tm.Failure++
	}
	tm.TotalDur += entry.Duration

	if _, ok := l.metrics.byHook[entry.HookID]; !ok {
		l.metrics.byHook[entry.HookID] = &HookMetricsEntry{ID: entry.HookID, Type: entry.HookType}
	}
	hme := l.metrics.byHook[entry.HookID]
	hme.Count++
	if entry.Result.Success {
		hme.Success++
	} else {
		hme.Failure++
	}
	hme.TotalDur += entry.Duration
	hme.AvgDur = hme.TotalDur / time.Duration(hme.Count)
	hme.LastExec = entry.Timestamp
	hme.LastResult = entry.Result
}

// Metrics returns a snapshot of current metrics.
func (l *HookLogger) Metrics() HookMetricsSnapshot {
	l.metrics.mu.RLock()
	defer l.metrics.mu.RUnlock()

	byType := make(map[HookType]TypeMetricsSnapshot)
	for k, v := range l.metrics.byType {
		byType[k] = TypeMetricsSnapshot{
			Count:    v.Count,
			Success:  v.Success,
			Failure:  v.Failure,
			TotalDur: v.TotalDur,
			AvgDur:   v.TotalDur / time.Duration(max(v.Count, 1)),
		}
	}

	byHook := make(map[string]HookMetricsEntrySnapshot)
	for k, v := range l.metrics.byHook {
		byHook[k] = HookMetricsEntrySnapshot{
			ID:       v.ID,
			Type:     v.Type,
			Count:    v.Count,
			Success:  v.Success,
			Failure:  v.Failure,
			TotalDur: v.TotalDur,
			AvgDur:   v.AvgDur,
			LastExec: v.LastExec,
		}
	}

	return HookMetricsSnapshot{
		TotalExecutions: atomic.LoadUint64(&l.metrics.totalExecutions),
		SuccessCount:    atomic.LoadUint64(&l.metrics.successCount),
		FailureCount:    atomic.LoadUint64(&l.metrics.failureCount),
		TotalDuration:   l.metrics.totalDuration,
		ByType:          byType,
		ByHook:          byHook,
	}
}

// HookMetricsSnapshot is a read-only snapshot of metrics.
type HookMetricsSnapshot struct {
	TotalExecutions uint64
	SuccessCount    uint64
	FailureCount    uint64
	TotalDuration   time.Duration
	ByType          map[HookType]TypeMetricsSnapshot
	ByHook          map[string]HookMetricsEntrySnapshot
}

type TypeMetricsSnapshot struct {
	Count    uint64
	Success  uint64
	Failure  uint64
	TotalDur time.Duration
	AvgDur   time.Duration
}

type HookMetricsEntrySnapshot struct {
	ID       string
	Type     HookType
	Count    uint64
	Success  uint64
	Failure  uint64
	TotalDur time.Duration
	AvgDur   time.Duration
	LastExec time.Time
}

// RecentEntries returns the most recent log entries.
func (l *HookLogger) RecentEntries(n int) []HookLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || n > len(l.entries) {
		n = len(l.entries)
	}
	start := len(l.entries) - n
	result := make([]HookLogEntry, n)
	copy(result, l.entries[start:])
	return result
}

// Close closes the logger.
func (l *HookLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	return nil
}

// LogHookExecution is a convenience function to log a hook execution.
func (l *HookLogger) LogHookExecution(ctx context.Context, config HookConfig, event HookEvent, result HookResult, attempt int) {
	level := LogLevelInfo
	if !result.Success {
		level = LogLevelError
	} else if result.Duration > 5*time.Second {
		level = LogLevelWarn
	}

	entry := HookLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		HookID:    config.ID,
		HookType:  config.Type,
		Event:     event,
		Result:    result,
		Duration:  result.Duration,
		Attempt:   attempt,
	}
	if result.Error != "" {
		entry.Error = result.Error
	}

	l.Log(entry)
}
