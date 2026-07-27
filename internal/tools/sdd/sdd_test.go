// SPDX-License-Identifier: MIT
package sdd

import (
	"context"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func defaultCtx() tools.Context {
	return tools.Context{Context: context.Background()}
}

func TestRegisterAll_RegistersAllTools(t *testing.T) {
	reg := tools.New()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	expected := []string{
		"sdd_init", "sdd_explore", "sdd_propose", "sdd_spec",
		"sdd_design", "sdd_tasks", "sdd_apply", "sdd_verify",
		"sdd_archive", "sdd_onboard",
	}
	for _, name := range expected {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
	if reg.Len() != len(expected) {
		t.Errorf("Len() = %d, want %d", reg.Len(), len(expected))
	}
}

func TestTool_NamesAndDescriptions(t *testing.T) {
	reg := tools.New()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, tool := range reg.List() {
		if tool.Name() == "" {
			t.Error("tool has empty name")
		}
		if tool.Description() == "" {
			t.Errorf("tool %q has empty description", tool.Name())
		}
	}
}

func TestTool_ReadOnly(t *testing.T) {
	reg := tools.New()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, tool := range reg.List() {
		if tool.IsReadOnly() {
			t.Errorf("tool %q should not be read-only", tool.Name())
		}
	}
}

func TestTool_Validate(t *testing.T) {
	reg := tools.New()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, tool := range reg.List() {
		if err := tool.Validate(nil); err == nil {
			t.Errorf("tool %q: expected validation error for nil input", tool.Name())
		}
		if err := tool.Validate(map[string]any{}); err == nil {
			t.Errorf("tool %q: expected validation error for missing project_dir", tool.Name())
		}
		if err := tool.Validate(map[string]any{"project_dir": "/tmp/test"}); err != nil {
			t.Errorf("tool %q: Validate() error = %v, want nil", tool.Name(), err)
		}
		if err := tool.Validate(map[string]any{
			"project_dir": "/tmp/test",
			"phase_data":  "some data",
		}); err != nil {
			t.Errorf("tool %q: Validate() with phase_data error = %v, want nil", tool.Name(), err)
		}
	}
}

func TestSDDTool_Execute(t *testing.T) {
	reg := tools.New()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, tool := range reg.List() {
		result, err := tool.Execute(defaultCtx(), map[string]any{
			"project_dir": "/tmp/test-project",
			"phase_data":  "some context",
		})
		if err != nil {
			t.Errorf("tool %q: Execute() error = %v, want nil", tool.Name(), err)
			continue
		}
		r, ok := result.(map[string]any)
		if !ok {
			t.Errorf("tool %q: result type = %T, want map[string]any", tool.Name(), result)
			continue
		}
		if r["status"] != "invoked" {
			t.Errorf("tool %q: status = %v, want %q", tool.Name(), r["status"], "invoked")
		}
		if r["project_dir"] != "/tmp/test-project" {
			t.Errorf("tool %q: project_dir = %v, want %q", tool.Name(), r["project_dir"], "/tmp/test-project")
		}
	}
}
