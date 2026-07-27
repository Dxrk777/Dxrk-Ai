// SPDX-License-Identifier: MIT
package tools

import (
	"fmt"
	"slices"
)

// ErrDuplicateTool is returned when a tool is registered more than once.
var ErrDuplicateTool = fmt.Errorf("tool already registered")

// ErrToolNotFound is returned when a tool is not found in the registry.
var ErrToolNotFound = fmt.Errorf("tool not found")

// Registry manages a set of tools.
type Registry struct {
	tools map[string]Tool
}

// New creates an empty Registry.
func New() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) error {
	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateTool, t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools sorted by name.
func (r *Registry) List() []Tool {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	slices.Sort(names)

	result := make([]Tool, len(names))
	for i, name := range names {
		result[i] = r.tools[name]
	}
	return result
}

// ListEnabled returns all enabled tools sorted by name.
func (r *Registry) ListEnabled() []Tool {
	var enabled []Tool
	for _, t := range r.tools {
		if t.IsEnabled() {
			enabled = append(enabled, t)
		}
	}
	slices.SortFunc(enabled, func(a, b Tool) int {
		switch {
		case a.Name() < b.Name():
			return -1
		case a.Name() > b.Name():
			return 1
		default:
			return 0
		}
	})
	return enabled
}

// Remove deletes a tool from the registry.
func (r *Registry) Remove(name string) error {
	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	delete(r.tools, name)
	return nil
}

// Len returns the number of registered tools.
func (r *Registry) Len() int {
	return len(r.tools)
}
