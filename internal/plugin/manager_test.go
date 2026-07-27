// SPDX-License-Identifier: MIT
package plugin

import (
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func TestNewManager(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestRegisterAndLoadAll(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")

	count, err := m.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("LoadAll() = %d, want 0", count)
	}
}

func TestRegisterPlugin(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")

	p := Plugin{
		Name: "test-plugin",
		Register: func(r *tools.Registry) error {
			tool, err := tools.Build(tools.ToolDef{
				Name: "plugin_tool",
				Execute: func(ctx tools.Context, input map[string]any) (any, error) {
					return "plugin result", nil
				},
			})
			if err != nil {
				return err
			}
			return r.Register(tool)
		},
	}

	if err := m.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	count, err := m.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("LoadAll() = %d, want 1", count)
	}

	tool, ok := reg.Get("plugin_tool")
	if !ok {
		t.Fatal("plugin tool not registered")
	}
	if tool.Name() != "plugin_tool" {
		t.Fatalf("Name() = %q, want %q", tool.Name(), "plugin_tool")
	}
}

func TestRegister_EmptyName(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")

	err := m.Register(Plugin{Register: func(r *tools.Registry) error { return nil }})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegister_NilRegister(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")

	err := m.Register(Plugin{Name: "noop"})
	if err == nil {
		t.Fatal("expected error for nil Register")
	}
}

func TestPlugins_List(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/plugins")

	p1 := Plugin{Name: "p1", Register: func(r *tools.Registry) error { return nil }}
	p2 := Plugin{Name: "p2", Register: func(r *tools.Registry) error { return nil }}

	_ = m.Register(p1)
	_ = m.Register(p2)

	list := m.Plugins()
	if len(list) != 2 {
		t.Fatalf("Plugins() = %d, want 2", len(list))
	}
}

func TestDiscover_NonExistentDir(t *testing.T) {
	reg := tools.New()
	m := NewManager(reg, "/tmp/nonexistent-plugins-dir")

	manifests, err := m.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("Discover() = %d, want 0", len(manifests))
	}
}

func TestDiscover_EmptyDir(t *testing.T) {
	reg := tools.New()
	dir := t.TempDir()
	m := NewManager(reg, dir)

	manifests, err := m.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("Discover() = %d, want 0", len(manifests))
	}
}
