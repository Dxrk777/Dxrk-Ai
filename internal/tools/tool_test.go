// SPDX-License-Identifier: MIT
package tools

import (
	"context"
	"errors"
	"testing"
)

func TestBuild_RequiresName(t *testing.T) {
	_, err := Build(ToolDef{Execute: func(ctx Context, input map[string]any) (any, error) {
		return nil, nil
	}})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestBuild_RequiresExecute(t *testing.T) {
	_, err := Build(ToolDef{Name: "test"})
	if err == nil {
		t.Fatal("expected error for nil execute")
	}
}

func TestBuild_Defaults(t *testing.T) {
	tool, err := Build(ToolDef{
		Name:        "test",
		Description: "a test tool",
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if tool.Name() != "test" {
		t.Fatalf("Name() = %q, want %q", tool.Name(), "test")
	}
	if tool.Description() != "a test tool" {
		t.Fatalf("Description() = %q, want %q", tool.Description(), "a test tool")
	}
	if !tool.IsEnabled() {
		t.Fatal("IsEnabled() = false, want true (default)")
	}
	if tool.IsReadOnly() {
		t.Fatal("IsReadOnly() = true, want false (default)")
	}
	if tool.IsConcurrentSafe() {
		t.Fatal("IsConcurrentSafe() = true, want false (default)")
	}
}

func TestBuild_Overrides(t *testing.T) {
	disabled := false
	readOnly := true
	concurrent := true

	tool, err := Build(ToolDef{
		Name:             "custom",
		Description:      "custom description",
		Execute:          func(ctx Context, input map[string]any) (any, error) { return nil, nil },
		IsEnabled:        &disabled,
		IsReadOnly:       &readOnly,
		IsConcurrentSafe: &concurrent,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if tool.IsEnabled() {
		t.Fatal("IsEnabled() = true, want false")
	}
	if !tool.IsReadOnly() {
		t.Fatal("IsReadOnly() = false, want true")
	}
	if !tool.IsConcurrentSafe() {
		t.Fatal("IsConcurrentSafe() = false, want true")
	}
}

func TestTool_Validate(t *testing.T) {
	tool, err := Build(ToolDef{
		Name: "validate-test",
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return nil, nil
		},
		Validate: func(input map[string]any) error {
			if input == nil {
				return errors.New("input is nil")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if err := tool.Validate(nil); err == nil {
		t.Fatal("expected validation error for nil input")
	}
	if err := tool.Validate(map[string]any{}); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestTool_Execute_Disabled(t *testing.T) {
	disabled := false
	tool, err := Build(ToolDef{
		Name:      "disabled-tool",
		IsEnabled: &disabled,
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return "ran", nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	_, err = tool.Execute(Context{Context: context.Background()}, nil)
	if err == nil {
		t.Fatal("expected error for disabled tool")
	}
}

func TestTool_Execute_Success(t *testing.T) {
	tool, err := Build(ToolDef{
		Name: "echo",
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return input["msg"], nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result, err := tool.Execute(Context{Context: context.Background()}, map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != "hello" {
		t.Fatalf("Execute() = %v, want %v", result, "hello")
	}
}

func TestDefaultEnabled(t *testing.T) {
	b := DefaultEnabled()
	if b == nil {
		t.Fatal("DefaultEnabled() returned nil")
	}
	if !*b {
		t.Fatal("DefaultEnabled() = false, want true")
	}
}

func TestDefaultDisabled(t *testing.T) {
	b := DefaultDisabled()
	if b == nil {
		t.Fatal("DefaultDisabled() returned nil")
	}
	if *b {
		t.Fatal("DefaultDisabled() = true, want false")
	}
}
