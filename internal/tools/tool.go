// SPDX-License-Identifier: MIT
// Package tools provides a registry and builder for LLM-invokable tools.
//
// Ported from Claude Code's buildTool() pattern (see references/claude-code-source/Tool-ARCHITECTURE.md):
//   - ToolDef with fail-closed defaults
//   - Immutable Tool runtime from Build()
//   - Lifecycle: validate → execute → result
package tools

import (
	"context"
	"fmt"

	"github.com/Dxrk777/Dxrk/internal/log"
)

// Tool is the immutable runtime representation of a tool.
// Created via Build() from a ToolDef.
type Tool struct {
	name             string
	description      string
	inputSchema      map[string]any
	execute          func(Context, map[string]any) (any, error)
	validate         func(map[string]any) error
	isEnabled        bool
	isReadOnly       bool
	isConcurrentSafe bool
}

func (t Tool) Name() string                { return t.name }
func (t Tool) Description() string         { return t.description }
func (t Tool) InputSchema() map[string]any { return t.inputSchema }
func (t Tool) IsEnabled() bool             { return t.isEnabled }
func (t Tool) IsReadOnly() bool            { return t.isReadOnly }
func (t Tool) IsConcurrentSafe() bool      { return t.isConcurrentSafe }

// Validate checks input against the tool's validation function.
// Returns nil if input is valid, or a descriptive error.
func (t Tool) Validate(input map[string]any) error {
	if t.validate == nil {
		return nil
	}
	return t.validate(input)
}

// Execute runs the tool with the given context and input.
func (t Tool) Execute(ctx Context, input map[string]any) (any, error) {
	if !t.isEnabled {
		return nil, fmt.Errorf("tool %q is not enabled", t.name)
	}
	if err := t.Validate(input); err != nil {
		return nil, fmt.Errorf("validate %q: %w", t.name, err)
	}
	return t.execute(ctx, input)
}

// PermissionChecker is the interface for checking file-level permissions.
type PermissionChecker interface {
	Check(action string, target string) (allowed bool, reason string)
}

// PermissionAudit is the interface for logging permission decisions.
type PermissionAudit interface {
	Record(action, target string, allowed bool, reason string)
}

// Context carries execution-scoped state.
type Context struct {
	context.Context
	Logger            log.Logger
	PermissionChecker PermissionChecker
	PermissionAudit   PermissionAudit
}

// ToolDef defines a tool with sensible defaults applied by Build().
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
	Execute     func(Context, map[string]any) (any, error)
	Validate    func(map[string]any) error

	// IsEnabled controls whether the tool is available. Defaults to true.
	IsEnabled *bool
	// IsReadOnly marks the tool as side-effect-free. Defaults to false.
	IsReadOnly *bool
	// IsConcurrentSafe marks the tool as safe for parallel execution. Defaults to false.
	IsConcurrentSafe *bool
}

// Build creates an immutable Tool from a ToolDef, applying fail-closed defaults.
func Build(def ToolDef) (Tool, error) {
	if def.Name == "" {
		return Tool{}, fmt.Errorf("tool name is required")
	}
	if def.Execute == nil {
		return Tool{}, fmt.Errorf("tool %q: execute function is required", def.Name)
	}

	t := Tool{
		name:        def.Name,
		description: def.Description,
		inputSchema: def.InputSchema,
		execute:     def.Execute,
		validate:    def.Validate,
		isEnabled:   true,
	}

	if def.IsEnabled != nil {
		t.isEnabled = *def.IsEnabled
	}
	if def.IsReadOnly != nil {
		t.isReadOnly = *def.IsReadOnly
	}
	if def.IsConcurrentSafe != nil {
		t.isConcurrentSafe = *def.IsConcurrentSafe
	}

	return t, nil
}

// DefaultEnabled returns a pointer to true for use in ToolDef.IsEnabled.
func DefaultEnabled() *bool { b := true; return &b }

// DefaultDisabled returns a pointer to false for use in ToolDef fields.
func DefaultDisabled() *bool { b := false; return &b }
