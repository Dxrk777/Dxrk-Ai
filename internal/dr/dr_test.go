// SPDX-License-Identifier: MIT
package dr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestIncidentLifecycle(t *testing.T) {
	t.Parallel()

	m := NewManager()
	inc := Incident{
		ID:          "INC-001",
		Title:       "Database connection failed",
		Description: "Primary DB unreachable",
		Severity:    SeverityHigh,
		DetectedAt:  time.Now(),
		Affected:    []string{"api", "web"},
	}

	m.RecordIncident(inc)

	open := m.ListOpenIncidents()
	if len(open) != 1 {
		t.Fatalf("expected 1 open incident, got %d", len(open))
	}

	all := m.ListIncidents()
	if len(all) != 1 {
		t.Fatalf("expected 1 incident total, got %d", len(all))
	}
	if all[0].ID != "INC-001" {
		t.Errorf("expected ID INC-001, got %s", all[0].ID)
	}
	if all[0].Severity != SeverityHigh {
		t.Errorf("expected SeverityHigh, got %v", all[0].Severity)
	}

	if err := m.ResolveIncident("INC-001", "admin"); err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}

	open = m.ListOpenIncidents()
	if len(open) != 0 {
		t.Fatalf("expected 0 open incidents after resolve, got %d", len(open))
	}

	all = m.ListIncidents()
	if len(all) != 1 {
		t.Fatalf("expected 1 incident total after resolve, got %d", len(all))
	}
	if all[0].ResolvedBy != "admin" {
		t.Errorf("expected ResolvedBy admin, got %s", all[0].ResolvedBy)
	}
	if all[0].ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestResolveUnknownIncident(t *testing.T) {
	t.Parallel()

	m := NewManager()
	err := m.ResolveIncident("DOES-NOT-EXIST", "admin")
	if err == nil {
		t.Error("expected error for unknown incident")
	}
}

func TestResolveAlreadyResolved(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.RecordIncident(Incident{ID: "INC-001", DetectedAt: time.Now()})
	m.ResolveIncident("INC-001", "admin")

	err := m.ResolveIncident("INC-001", "admin2")
	if err == nil {
		t.Error("expected error for already resolved incident")
	}
}

func TestPlanExecution(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{
		ID:   "PLAN-001",
		Name: "Restart Services",
		Steps: []RecoveryStep{
			{Order: 1, Name: "Echo test", Command: "echo hello", Critical: false},
			{Order: 2, Name: "Second echo", Command: "echo world", Critical: false},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	results, err := m.ExecutePlan(context.Background(), "PLAN-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Success {
			t.Errorf("step %d should succeed: %s", i, r.Error)
		}
		if r.Duration <= 0 {
			t.Errorf("step %d duration should be positive", i)
		}
	}
}

func TestPlanExecutionWithTimeout(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{
		ID: "PLAN-TIMEOUT",
		Steps: []RecoveryStep{
			{Order: 1, Name: "Sleep", Command: "sleep 5"},
		},
	})

	results, err := m.ExecutePlan(context.Background(), "PLAN-TIMEOUT", WithTimeout(10*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected timeout failure")
	}
}

func TestPlanExecutionUnknownPlan(t *testing.T) {
	t.Parallel()

	m := NewManager()
	results, err := m.ExecutePlan(context.Background(), "NONEXISTENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for unknown plan, got %v", results)
	}
}

func TestDryRunMode(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{
		ID: "PLAN-DRY",
		Steps: []RecoveryStep{
			{Order: 1, Name: "Failing cmd", Command: "exit 1"},
		},
	})

	results, err := m.ExecutePlan(context.Background(), "PLAN-DRY", WithDryRun())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Error("dry-run step should report success")
	}
	if results[0].Output != "[dry-run] would execute: exit 1" {
		t.Errorf("unexpected dry-run output: %s", results[0].Output)
	}
}

func TestPlanCriticalStepStopsExecution(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{
		ID: "PLAN-CRIT",
		Steps: []RecoveryStep{
			{Order: 1, Name: "First", Command: "echo first", Critical: false},
			{Order: 2, Name: "Second", Command: "exit 1", Critical: true},
			{Order: 3, Name: "Third", Command: "echo third", Critical: false},
		},
	})

	results, err := m.ExecutePlan(context.Background(), "PLAN-CRIT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (stopped at critical), got %d", len(results))
	}
	if !results[0].Success {
		t.Error("first step should succeed")
	}
	if results[1].Success {
		t.Error("critical step should fail")
	}
}

func TestBackupRestore(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "test.txt")
	content := []byte("hello dr backup")
	if err := os.WriteFile(srcFile, content, 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	dstFile := filepath.Join(dstDir, "backup.bak")
	result, err := Backup(context.Background(), srcFile, dstFile)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if result.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), result.Size)
	}
	if result.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if result.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	restDir := t.TempDir()
	restFile := filepath.Join(restDir, "restored.txt")
	if err := Restore(context.Background(), dstFile, restFile); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	restored, err := os.ReadFile(restFile)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(restored) != string(content) {
		t.Errorf("restored content mismatch: got %q, want %q", string(restored), string(content))
	}
}

func TestBackupNonexistentSource(t *testing.T) {
	t.Parallel()

	_, err := Backup(context.Background(), "/nonexistent/path", "/tmp/out")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestRestoreNonexistentBackup(t *testing.T) {
	t.Parallel()

	err := Restore(context.Background(), "/nonexistent/backup", "/tmp/out")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestCleanupBackups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	oldFile := filepath.Join(dir, "old_backup")
	if err := os.WriteFile(oldFile, []byte("old"), 0o640); err != nil {
		t.Fatalf("write old: %v", err)
	}

	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	newFile := filepath.Join(dir, "new_backup")
	if err := os.WriteFile(newFile, []byte("new"), 0o640); err != nil {
		t.Fatalf("write new: %v", err)
	}

	if err := CleanupBackups(context.Background(), dir, 24*time.Hour); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 remaining file, got %d", len(entries))
	}
	if entries[0].Name() != "new_backup" {
		t.Errorf("expected new_backup to remain, got %s", entries[0].Name())
	}
}

func TestCleanupBackupsNonexistentDir(t *testing.T) {
	t.Parallel()

	err := CleanupBackups(context.Background(), "/nonexistent/dir", time.Hour)
	if err != nil {
		t.Errorf("expected no error for nonexistent dir, got %v", err)
	}
}

func TestConcurrentSafety(t *testing.T) {
	t.Parallel()

	m := NewManager()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.RecordIncident(Incident{
				ID:         fmt.Sprintf("INC-%d", n),
				Title:      "Concurrent incident",
				Severity:   SeverityMedium,
				DetectedAt: time.Now(),
			})
		}(i)
	}

	wg.Wait()

	all := m.ListIncidents()
	if len(all) != 100 {
		t.Errorf("expected 100 incidents, got %d", len(all))
	}
}

func TestCreateBackupPlan(t *testing.T) {
	t.Parallel()

	m := NewManager()
	steps := []RecoveryStep{
		{Order: 1, Name: "Backup DB", Command: "pg_dump", Timeout: 5 * time.Minute},
	}
	plan := m.CreateBackupPlan("db-backup", steps)
	if plan.ID != "db-backup" {
		t.Errorf("expected ID db-backup, got %s", plan.ID)
	}
	if len(plan.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestMultiplePlans(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{ID: "A", Name: "Plan A"})
	m.AddPlan(RecoveryPlan{ID: "B", Name: "Plan B"})

	results, err := m.ExecutePlan(context.Background(), "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results == nil {
		t.Error("expected results for existing plan")
	}
}

func TestPlanWithOutput(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.AddPlan(RecoveryPlan{
		ID: "OUTPUT",
		Steps: []RecoveryStep{
			{Order: 1, Name: "Output", Command: "echo 'hello world'"},
		},
	})

	results, err := m.ExecutePlan(context.Background(), "OUTPUT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", results[0].Output)
	}
}

func TestSeverityValues(t *testing.T) {
	t.Parallel()

	if SeverityLow != 0 {
		t.Errorf("expected SeverityLow=0, got %d", SeverityLow)
	}
	if SeverityMedium != 1 {
		t.Errorf("expected SeverityMedium=1, got %d", SeverityMedium)
	}
	if SeverityHigh != 2 {
		t.Errorf("expected SeverityHigh=2, got %d", SeverityHigh)
	}
	if SeverityCritical != 3 {
		t.Errorf("expected SeverityCritical=3, got %d", SeverityCritical)
	}
}

func TestBackupChecksumConsistency(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "data.bin")
	content := []byte("deterministic content for checksum test")
	if err := os.WriteFile(srcFile, content, 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	dstFile := filepath.Join(dstDir, "backup.bak")
	r1, err := Backup(context.Background(), srcFile, dstFile)
	if err != nil {
		t.Fatalf("first backup: %v", err)
	}

	dstFile2 := filepath.Join(dstDir, "backup2.bak")
	r2, err := Backup(context.Background(), srcFile, dstFile2)
	if err != nil {
		t.Fatalf("second backup: %v", err)
	}

	if r1.Checksum != r2.Checksum {
		t.Errorf("checksums should match: %s vs %s", r1.Checksum, r2.Checksum)
	}
	if r1.Size != r2.Size {
		t.Errorf("sizes should match: %d vs %d", r1.Size, r2.Size)
	}
}
