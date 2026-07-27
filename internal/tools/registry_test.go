// SPDX-License-Identifier: MIT
package tools

import (
	"context"
	"errors"
	"testing"
)

func newTestTool(t *testing.T, name string) Tool {
	t.Helper()
	tool, err := Build(ToolDef{
		Name: name,
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return name, nil
		},
	})
	if err != nil {
		t.Fatalf("Build(%q) error = %v", name, err)
	}
	return tool
}

func TestNew_Empty(t *testing.T) {
	r := New()
	if r.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", r.Len())
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := New()
	tool := newTestTool(t, "test-tool")
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("test-tool")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got.Name() != "test-tool" {
		t.Fatalf("Name() = %q, want %q", got.Name(), "test-tool")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := New()
	tool := newTestTool(t, "dup")
	if err := r.Register(tool); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	err := r.Register(tool)
	if !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("second Register() error = %v, want ErrDuplicateTool", err)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := New()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("Get() returned true for missing tool")
	}
}

func TestRegistry_List_Sorted(t *testing.T) {
	r := New()
	for _, name := range []string{"z-tool", "a-tool", "m-tool"} {
		if err := r.Register(newTestTool(t, name)); err != nil {
			t.Fatalf("Register(%q) error = %v", name, err)
		}
	}

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("Len() = %d, want 3", len(list))
	}
	if list[0].Name() != "a-tool" {
		t.Fatalf("list[0].Name() = %q, want %q", list[0].Name(), "a-tool")
	}
	if list[1].Name() != "m-tool" {
		t.Fatalf("list[1].Name() = %q, want %q", list[1].Name(), "m-tool")
	}
	if list[2].Name() != "z-tool" {
		t.Fatalf("list[2].Name() = %q, want %q", list[2].Name(), "z-tool")
	}
}

func TestRegistry_ListEnabled(t *testing.T) {
	r := New()
	disabled := false
	tool1 := newTestTool(t, "enabled-1")
	tool2, err := Build(ToolDef{
		Name:      "disabled",
		IsEnabled: &disabled,
		Execute:   func(ctx Context, input map[string]any) (any, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	tool3 := newTestTool(t, "enabled-2")

	for _, tool := range []Tool{tool1, tool2, tool3} {
		if err := r.Register(tool); err != nil {
			t.Fatalf("Register(%q) error = %v", tool.Name(), err)
		}
	}

	enabled := r.ListEnabled()
	if len(enabled) != 2 {
		t.Fatalf("ListEnabled() = %d, want 2", len(enabled))
	}
	if enabled[0].Name() != "enabled-1" {
		t.Fatalf("enabled[0].Name() = %q, want %q", enabled[0].Name(), "enabled-1")
	}
	if enabled[1].Name() != "enabled-2" {
		t.Fatalf("enabled[1].Name() = %q, want %q", enabled[1].Name(), "enabled-2")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := New()
	tool := newTestTool(t, "remove-me")
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if r.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", r.Len())
	}

	if err := r.Remove("remove-me"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("Len() after Remove() = %d, want 0", r.Len())
	}

	err := r.Remove("nonexistent")
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("Remove(nonexistent) error = %v, want ErrToolNotFound", err)
	}
}

func TestRegistry_ExecuteViaGet(t *testing.T) {
	r := New()
	tool := newTestTool(t, "greet")
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("greet")
	if !ok {
		t.Fatal("Get() returned false")
	}

	result, err := got.Execute(Context{Context: context.Background()}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != "greet" {
		t.Fatalf("Execute() = %v, want %q", result, "greet")
	}
}
