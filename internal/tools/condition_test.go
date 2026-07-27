// SPDX-License-Identifier: MIT
package tools

import (
	"testing"
)

func TestPathCondition_MatchInclude(t *testing.T) {
	cond := PathCondition{
		Patterns: []string{"*.go"},
		Include:  true,
	}
	if !cond.Match(map[string]any{"path": "main.go"}) {
		t.Fatal("expected match for *.go on main.go")
	}
	if cond.Match(map[string]any{"path": "main.ts"}) {
		t.Fatal("expected no match for *.go on main.ts")
	}
}

func TestPathCondition_MatchPathsField(t *testing.T) {
	cond := PathCondition{
		Patterns: []string{"*.go"},
		Include:  true,
	}
	if !cond.Match(map[string]any{"paths": []string{"a.go", "b.ts"}}) {
		t.Fatal("expected match when one path matches")
	}
	if cond.Match(map[string]any{"paths": []string{"a.ts", "b.ts"}}) {
		t.Fatal("expected no match when no path matches")
	}
}

func TestPathCondition_MatchExclude(t *testing.T) {
	cond := PathCondition{
		Patterns: []string{".env*"},
		Include:  false,
	}
	if !cond.Match(map[string]any{"path": "main.go"}) {
		t.Fatal("expected active when path does NOT match .env")
	}
	if cond.Match(map[string]any{"path": ".env"}) {
		t.Fatal("expected inactive when path matches .env")
	}
}

func TestPathCondition_NilInput(t *testing.T) {
	cond := PathCondition{
		Patterns: []string{"*.go"},
		Include:  true,
	}
	if cond.Match(nil) {
		t.Fatal("expected no match for nil input (include = false)")
	}
}

func TestKeyValueCondition(t *testing.T) {
	cond := KeyValueCondition{Key: "mode", Value: "dry-run"}
	if !cond.Match(map[string]any{"mode": "dry-run"}) {
		t.Fatal("expected match for mode=dry-run")
	}
	if cond.Match(map[string]any{"mode": "live"}) {
		t.Fatal("expected no match for mode=live")
	}
	if cond.Match(nil) {
		t.Fatal("expected no match for nil input")
	}
}

func TestAlwaysCondition(t *testing.T) {
	cond := AlwaysCondition{}
	if !cond.Match(nil) {
		t.Fatal("AlwaysCondition should match nil")
	}
}

func TestNeverCondition(t *testing.T) {
	cond := NeverCondition{}
	if cond.Match(nil) {
		t.Fatal("NeverCondition should not match")
	}
}

func TestAndCondition(t *testing.T) {
	trueCond := AlwaysCondition{}
	falseCond := NeverCondition{}

	andAllTrue := AndCondition{Conditions: []Condition{trueCond, trueCond}}
	if !andAllTrue.Match(nil) {
		t.Fatal("AndCondition all true should match")
	}

	andOneFalse := AndCondition{Conditions: []Condition{trueCond, falseCond}}
	if andOneFalse.Match(nil) {
		t.Fatal("AndCondition with false should not match")
	}
}

func TestOrCondition(t *testing.T) {
	trueCond := AlwaysCondition{}
	falseCond := NeverCondition{}

	orOneTrue := OrCondition{Conditions: []Condition{falseCond, trueCond}}
	if !orOneTrue.Match(nil) {
		t.Fatal("OrCondition with one true should match")
	}

	orAllFalse := OrCondition{Conditions: []Condition{falseCond, falseCond}}
	if orAllFalse.Match(nil) {
		t.Fatal("OrCondition all false should not match")
	}
}

func TestNewConditionalTool(t *testing.T) {
	tool := newTestTool(t, "test-tool")
	cond := PathCondition{
		Patterns: []string{"*.go"},
		Include:  true,
	}
	ct := NewConditionalTool(tool, cond)

	if !ct.IsActive(map[string]any{"path": "main.go"}) {
		t.Fatal("expected active for main.go")
	}
	if ct.IsActive(map[string]any{"path": "main.ts"}) {
		t.Fatal("expected inactive for main.ts")
	}
}

func TestConditionalTool_Description(t *testing.T) {
	tool := newTestTool(t, "test-tool")
	cond := KeyValueCondition{Key: "mode", Value: "dev"}
	ct := NewConditionalTool(tool, cond)
	desc := ct.Description()
	if !contains(desc, "test-tool") && !contains(desc, "condition") {
		t.Fatalf("unexpected description: %q", desc)
	}
}

func TestFilterActive(t *testing.T) {
	r := New()
	t1 := newTestTool(t, "enabled-1")
	disabled := false
	t2, _ := Build(ToolDef{
		Name:      "disabled",
		IsEnabled: &disabled,
		Execute:   func(ctx Context, input map[string]any) (any, error) { return nil, nil },
	})
	_ = r.Register(t1)
	_ = r.Register(t2)

	active := FilterActive(r.List(), nil)
	if len(active) != 1 {
		t.Fatalf("FilterActive = %d, want 1", len(active))
	}
}

func TestFilterConditionalActive(t *testing.T) {
	t1 := newTestTool(t, "go-tool")
	t2 := newTestTool(t, "ts-tool")

	ct1 := NewConditionalTool(t1, PathCondition{Patterns: []string{"*.go"}, Include: true})
	ct2 := NewConditionalTool(t2, PathCondition{Patterns: []string{"*.ts"}, Include: true})

	active := FilterConditionalActive([]ConditionalTool{ct1, ct2}, map[string]any{"path": "main.go"})
	if len(active) != 1 {
		t.Fatalf("FilterConditionalActive = %d, want 1", len(active))
	}
	if active[0].Name() != "go-tool" {
		t.Fatalf("expected go-tool active, got %q", active[0].Name())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringsContains(s, substr)
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
