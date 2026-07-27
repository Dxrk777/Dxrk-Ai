// SPDX-License-Identifier: MIT
package permissions

import (
	"testing"
)

func TestAudit_Record(t *testing.T) {
	a := NewAudit(100)
	a.Record("test-tool", ActionRead, "/file.txt", Result{Allowed: true, Reason: "allowed"})
	entries := a.All()
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Tool != "test-tool" {
		t.Fatalf("Tool = %q, want %q", entries[0].Tool, "test-tool")
	}
	if !entries[0].Allowed {
		t.Fatal("Allowed = false, want true")
	}
}

func TestAudit_MaxSize(t *testing.T) {
	a := NewAudit(3)
	for i := 0; i < 5; i++ {
		a.Record("tool", ActionRead, "/file", Result{Allowed: true, Reason: "ok"})
	}
	entries := a.All()
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
}

func TestAudit_Denied(t *testing.T) {
	a := NewAudit(0)
	a.Record("tool-1", ActionRead, "/ok", Result{Allowed: true, Reason: "ok"})
	a.Record("tool-2", ActionRead, "/bad", Result{Allowed: false, Reason: "denied"})
	a.Record("tool-3", ActionRead, "/ok2", Result{Allowed: true, Reason: "ok"})

	denied := a.Denied()
	if len(denied) != 1 {
		t.Fatalf("Denied() = %d, want 1", len(denied))
	}
	if denied[0].Tool != "tool-2" {
		t.Fatalf("denied[0].Tool = %q, want %q", denied[0].Tool, "tool-2")
	}
}

func TestAudit_Recent(t *testing.T) {
	a := NewAudit(0)
	for i := 0; i < 10; i++ {
		a.Record("tool", ActionRead, "/file", Result{Allowed: true, Reason: "ok"})
	}

	recent := a.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("Recent(3) = %d, want 3", len(recent))
	}
}

func TestAudit_Clear(t *testing.T) {
	a := NewAudit(0)
	a.Record("tool", ActionRead, "/file", Result{Allowed: true, Reason: "ok"})
	a.Clear()
	if len(a.All()) != 0 {
		t.Fatal("entries remain after Clear")
	}
}

func TestAudit_Summary(t *testing.T) {
	a := NewAudit(0)
	a.Record("t1", ActionRead, "/ok", Result{Allowed: true, Reason: "ok"})
	a.Record("t2", ActionExec, "/bad", Result{Allowed: false, Reason: "denied"})
	a.Record("t3", ActionWrite, "/ok2", Result{Allowed: true, Reason: "ok"})

	summary := a.Summary()
	if summary != "total=3 allowed=2 denied=1" {
		t.Fatalf("Summary = %q, want %q", summary, "total=3 allowed=2 denied=1")
	}
}

func TestAudit_Empty(t *testing.T) {
	a := NewAudit(0)
	if len(a.All()) != 0 {
		t.Fatal("new audit should be empty")
	}
	if len(a.Denied()) != 0 {
		t.Fatal("new audit should have no denied entries")
	}
	if len(a.Recent(10)) != 0 {
		t.Fatal("Recent on empty audit should be empty")
	}
}
