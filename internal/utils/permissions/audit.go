// SPDX-License-Identifier: MIT
package permissions

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// ---- Audit Entry ----

// AuditEntry records a single permission decision.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`
	Resource  string    `json:"resource"`
	Action    Action    `json:"action"`
	RuleID    string    `json:"rule_id,omitempty"`
	Layer     string    `json:"layer,omitempty"`
	User      string    `json:"user,omitempty"`
	RiskLevel string    `json:"risk_level,omitempty"`
	Details   string    `json:"details,omitempty"`
}

// ---- Audit Filter ----

// AuditFilter defines query criteria for audit entries.
type AuditFilter struct {
	From         time.Time `json:"from,omitempty"`
	To           time.Time `json:"to,omitempty"`
	Tool         string    `json:"tool,omitempty"`
	Action       *Action   `json:"action,omitempty"`
	MinRiskLevel RiskLevel `json:"min_risk_level,omitempty"`
}

// ---- Audit Log ----

// AuditLog is a thread-safe ring-buffer audit trail.
type AuditLog struct {
	mu         sync.RWMutex
	entries    []AuditEntry
	maxEntries int
	head       int
	full       bool
}

// NewAuditLog creates an audit log with a fixed maximum entry count.
func NewAuditLog(maxEntries int) *AuditLog {
	if maxEntries <= 0 {
		maxEntries = 4096
	}
	return &AuditLog{
		entries:    make([]AuditEntry, maxEntries),
		maxEntries: maxEntries,
	}
}

// Log appends an audit entry to the ring buffer.
func (al *AuditLog) Log(entry AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	al.entries[al.head] = entry
	al.head = (al.head + 1) % al.maxEntries
	if al.head == 0 {
		al.full = true
	}
}

// Query returns entries matching the filter, ordered oldest to newest.
func (al *AuditLog) Query(filter AuditFilter) []AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	result := make([]AuditEntry, 0, 64)
	count := al.maxEntries
	if !al.full {
		count = al.head
	}

	for i := 0; i < count; i++ {
		idx := i
		if al.full {
			idx = (al.head + i) % al.maxEntries
		}
		entry := al.entries[idx]
		if al.matchesFilter(entry, filter) {
			result = append(result, entry)
		}
	}
	return result
}

func (al *AuditLog) matchesFilter(entry AuditEntry, f AuditFilter) bool {
	if !f.From.IsZero() && entry.Timestamp.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && entry.Timestamp.After(f.To) {
		return false
	}
	if f.Tool != "" && entry.Tool != f.Tool {
		return false
	}
	if f.Action != nil && entry.Action != *f.Action {
		return false
	}
	if f.MinRiskLevel > 0 && entry.RiskLevel != "" {
		level := parseRiskLevel(entry.RiskLevel)
		if level < f.MinRiskLevel {
			return false
		}
	}
	return true
}

func parseRiskLevel(s string) RiskLevel {
	switch s {
	case "low":
		return Low
	case strconst.StrMedium2:
		return Medium
	case "high":
		return High
	case strconst.StrCritical:
		return Critical
	default:
		return Low
	}
}

// Len returns the number of stored entries.
func (al *AuditLog) Len() int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	if al.full {
		return al.maxEntries
	}
	return al.head
}

// ---- Export ----

// ExportJSON writes all entries as a JSON array to w.
func (al *AuditLog) ExportJSON(w io.Writer) error {
	al.mu.RLock()
	entries := al.orderedEntries()
	al.mu.RUnlock()

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

// ExportCSV writes all entries as CSV to w.
func (al *AuditLog) ExportCSV(w io.Writer) error {
	al.mu.RLock()
	entries := al.orderedEntries()
	al.mu.RUnlock()

	wr := csv.NewWriter(w)
	defer wr.Flush()

	if err := wr.Write([]string{
		"timestamp", "tool", "resource", "action", "rule_id",
		"layer", "user", "risk_level", "details",
	}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, e := range entries {
		row := []string{
			e.Timestamp.Format(time.RFC3339),
			e.Tool,
			e.Resource,
			e.Action.String(),
			e.RuleID,
			e.Layer,
			e.User,
			e.RiskLevel,
			e.Details,
		}
		if err := wr.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	return nil
}

func (al *AuditLog) orderedEntries() []AuditEntry {
	count := al.maxEntries
	if !al.full {
		count = al.head
	}
	result := make([]AuditEntry, count)
	for i := 0; i < count; i++ {
		idx := i
		if al.full {
			idx = (al.head + i) % al.maxEntries
		}
		result[i] = al.entries[idx]
	}
	return result
}

// ---- Streaming ----

// AuditStreamer sends entries to a channel as they are logged.
type AuditStreamer struct {
	ch      chan AuditEntry
	dropped int
}

// NewAuditStreamer creates a streamer with a buffered channel.
func NewAuditStreamer(bufSize int) *AuditStreamer {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &AuditStreamer{
		ch: make(chan AuditEntry, bufSize),
	}
}

// Channel returns the receive-only channel of streamed entries.
func (as *AuditStreamer) Channel() <-chan AuditEntry {
	return as.ch
}

// Dropped returns the count of dropped entries (channel full).
func (as *AuditStreamer) Dropped() int {
	return as.dropped
}

// Send pushes an entry to the streamer. Drops if the buffer is full.
func (as *AuditStreamer) Send(entry AuditEntry) {
	select {
	case as.ch <- entry:
	default:
		as.dropped++
	}
}

// Close closes the streaming channel.
func (as *AuditStreamer) Close() {
	close(as.ch)
}

// ---- Combined Log with Streamers ----

// StreamingAuditLog wraps AuditLog and fans out to registered streamers.
type StreamingAuditLog struct {
	log       *AuditLog
	streamers []*AuditStreamer
	mu        sync.Mutex
}

// NewStreamingAuditLog creates a streaming audit log.
func NewStreamingAuditLog(maxEntries int) *StreamingAuditLog {
	return &StreamingAuditLog{
		log: NewAuditLog(maxEntries),
	}
}

// Log appends an entry and broadcasts to all streamers.
func (sal *StreamingAuditLog) Log(entry AuditEntry) {
	sal.log.Log(entry)
	sal.mu.Lock()
	defer sal.mu.Unlock()
	for _, s := range sal.streamers {
		s.Send(entry)
	}
}

// AddStreamer registers a streamer for future entries.
func (sal *StreamingAuditLog) AddStreamer(s *AuditStreamer) {
	sal.mu.Lock()
	defer sal.mu.Unlock()
	sal.streamers = append(sal.streamers, s)
}

// Query delegates to the underlying audit log.
func (sal *StreamingAuditLog) Query(filter AuditFilter) []AuditEntry {
	return sal.log.Query(filter)
}

// ExportJSON delegates to the underlying audit log.
func (sal *StreamingAuditLog) ExportJSON(w io.Writer) error {
	return sal.log.ExportJSON(w)
}

// ExportCSV delegates to the underlying audit log.
func (sal *StreamingAuditLog) ExportCSV(w io.Writer) error {
	return sal.log.ExportCSV(w)
}
