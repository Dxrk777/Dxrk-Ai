// SPDX-License-Identifier: MIT
package permissions

import (
	"fmt"
	"sync"
	"time"
)

// Entry is a single audit log entry.
type Entry struct {
	Time    time.Time
	Tool    string
	Action  Action
	Target  string
	Allowed bool
	Reason  string
}

// Audit maintains an append-only log of permission checks.
type Audit struct {
	mu      sync.RWMutex
	entries []Entry
	maxSize int
}

// NewAudit creates an Audit. maxSize limits entries; 0 means unlimited.
func NewAudit(maxSize int) *Audit {
	return &Audit{maxSize: maxSize}
}

// Record appends a permission check result to the log.
func (a *Audit) Record(tool string, action Action, target string, result Result) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := Entry{
		Time:    time.Now(),
		Tool:    tool,
		Action:  action,
		Target:  target,
		Allowed: result.Allowed,
		Reason:  result.Reason,
	}

	if a.maxSize > 0 && len(a.entries) >= a.maxSize {
		a.entries = a.entries[1:]
	}
	a.entries = append(a.entries, entry)
}

// Recent returns the most recent n entries.
func (a *Audit) Recent(n int) []Entry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if n <= 0 || n > len(a.entries) {
		n = len(a.entries)
	}
	result := make([]Entry, n)
	copy(result, a.entries[len(a.entries)-n:])
	return result
}

// All returns all entries.
func (a *Audit) All() []Entry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]Entry, len(a.entries))
	copy(result, a.entries)
	return result
}

// Denied returns entries where permission was denied.
func (a *Audit) Denied() []Entry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var denied []Entry
	for _, e := range a.entries {
		if !e.Allowed {
			denied = append(denied, e)
		}
	}
	return denied
}

// Clear removes all entries.
func (a *Audit) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = nil
}

// Summary returns a human-readable summary of the audit log.
func (a *Audit) Summary() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	total := len(a.entries)
	allowed := 0
	denied := 0
	for _, e := range a.entries {
		if e.Allowed {
			allowed++
		} else {
			denied++
		}
	}

	return fmt.Sprintf("total=%d allowed=%d denied=%d", total, allowed, denied)
}
