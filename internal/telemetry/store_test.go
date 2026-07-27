// SPDX-License-Identifier: MIT
package telemetry

import (
	"os"
	"testing"
	"time"
)

func TestNewStore_Disabled(t *testing.T) {
	s := NewStore(Config{Enabled: false, Dir: t.TempDir()})
	s.Record("tool_call", "test_tool", true, time.Second)
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

func TestStore_RecordAndFlush(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(Config{Enabled: true, Dir: dir})

	s.Record("tool_call", "greet", true, 100*time.Millisecond)
	s.Record("tool_call", "search", false, 500*time.Millisecond)

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected telemetry files, got none")
	}

	hasEventFile := false
	for _, e := range entries {
		if len(e.Name()) > 7 && e.Name()[:7] == "events_" {
			hasEventFile = true
			break
		}
	}
	if !hasEventFile {
		t.Fatal("expected events_*.json file, got none")
	}
}

func TestStore_EnableDisable(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(DefaultConfig(dir))

	if s.IsEnabled() {
		t.Fatal("telemetry should start disabled")
	}

	if err := s.Enable(); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if !s.IsEnabled() {
		t.Fatal("telemetry should be enabled after Enable()")
	}

	s.Disable()
	if s.IsEnabled() {
		t.Fatal("telemetry should be disabled after Disable()")
	}
}

func TestStore_FlushEmpty(t *testing.T) {
	s := NewStore(Config{Enabled: true, Dir: t.TempDir()})
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush() on empty store should not error, got: %v", err)
	}
}

func TestToolCallCounter(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(Config{Enabled: true, Dir: dir})
	counter := NewToolCallCounter(store)

	counter.RecordCall("test_tool", true, 50*time.Millisecond)
	counter.RecordCall("slow_tool", false, 2*time.Second)

	if err := counter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) < 1 {
		t.Fatal("expected at least one telemetry file")
	}
}
