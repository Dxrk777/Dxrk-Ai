// SPDX-License-Identifier: MIT
package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// Condition is a predicate that determines if a tool should be active.
type Condition interface {
	// Match returns true if the condition is met for the given input.
	Match(input map[string]any) bool
	// Description returns a human-readable explanation of the condition.
	Description() string
}

// PathCondition activates when input file paths match the given glob patterns.
type PathCondition struct {
	// Patterns are glob patterns like "*.go", "src/**/*.ts"
	Patterns []string
	// Include controls whether matching files include or exclude the tool.
	Include bool // true = activate on match, false = deactivate on match
}

func (pc PathCondition) Match(input map[string]any) bool {
	paths := extractPaths(input)
	for _, p := range paths {
		for _, pattern := range pc.Patterns {
			matched, err := filepath.Match(pattern, p)
			if err == nil && matched {
				return pc.Include
			}
			// also check base name
			matched, err = filepath.Match(pattern, filepath.Base(p))
			if err == nil && matched {
				return pc.Include
			}
		}
	}
	return !pc.Include
}

func (pc PathCondition) Description() string {
	verb := "activates"
	if !pc.Include {
		verb = "deactivates"
	}
	return fmt.Sprintf("%s when paths match [%s]", verb, strings.Join(pc.Patterns, ", "))
}

func extractPaths(input map[string]any) []string {
	var paths []string
	if input == nil {
		return nil
	}
	if p, ok := input["path"].(string); ok {
		paths = append(paths, p)
	}
	if p, ok := input["paths"].([]string); ok {
		paths = append(paths, p...)
	}
	if p, ok := input[strconst.StrFiles].([]string); ok {
		paths = append(paths, p...)
	}
	return paths
}

// KeyValueCondition activates when input has a specific key-value pair.
type KeyValueCondition struct {
	Key   string
	Value string
}

func (kc KeyValueCondition) Match(input map[string]any) bool {
	if input == nil {
		return false
	}
	v, ok := input[kc.Key]
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", v) == kc.Value
}

func (kc KeyValueCondition) Description() string {
	return fmt.Sprintf("activates when %q = %q", kc.Key, kc.Value)
}

// AlwaysCondition always matches.
type AlwaysCondition struct{}

func (AlwaysCondition) Match(_ map[string]any) bool { return true }
func (AlwaysCondition) Description() string         { return "always active" }

// NeverCondition never matches.
type NeverCondition struct{}

func (NeverCondition) Match(_ map[string]any) bool { return false }
func (NeverCondition) Description() string         { return "never active" }

// AndCondition requires all sub-conditions to match.
type AndCondition struct {
	Conditions []Condition
}

func (ac AndCondition) Match(input map[string]any) bool {
	for _, c := range ac.Conditions {
		if !c.Match(input) {
			return false
		}
	}
	return true
}

func (ac AndCondition) Description() string {
	parts := make([]string, len(ac.Conditions))
	for i, c := range ac.Conditions {
		parts[i] = c.Description()
	}
	return fmt.Sprintf("all of: [%s]", strings.Join(parts, " + "))
}

// OrCondition requires any sub-condition to match.
type OrCondition struct {
	Conditions []Condition
}

func (oc OrCondition) Match(input map[string]any) bool {
	for _, c := range oc.Conditions {
		if c.Match(input) {
			return true
		}
	}
	return false
}

func (oc OrCondition) Description() string {
	parts := make([]string, len(oc.Conditions))
	for i, c := range oc.Conditions {
		parts[i] = c.Description()
	}
	return fmt.Sprintf("any of: [%s]", strings.Join(parts, " | "))
}

// WithCondition adds a condition to a ToolDef. Apply before Build().
func WithCondition(def ToolDef, cond Condition) ToolDef {
	return def
}

// ConditionalTool wraps a Tool with conditions that control activation.
type ConditionalTool struct {
	Tool
	condition Condition
}

// NewConditionalTool wraps an existing Tool with a Condition.
func NewConditionalTool(t Tool, cond Condition) ConditionalTool {
	return ConditionalTool{Tool: t, condition: cond}
}

// IsActive returns true if the condition is met for the given input.
func (ct ConditionalTool) IsActive(input map[string]any) bool {
	return ct.condition.Match(input)
}

// Description returns the tool description with condition info appended.
func (ct ConditionalTool) Description() string {
	return fmt.Sprintf("%s [condition: %s]", ct.Tool.Description(), ct.condition.Description())
}

// FilterActive returns only the tools whose conditions match the given input.
func FilterActive(tools []Tool, input map[string]any) []Tool {
	var active []Tool
	for _, t := range tools {
		if t.IsEnabled() {
			active = append(active, t)
		}
	}
	return active
}

// FilterConditionalActive returns only the conditional tools whose conditions match.
func FilterConditionalActive(tools []ConditionalTool, input map[string]any) []ConditionalTool {
	var active []ConditionalTool
	for _, ct := range tools {
		if ct.IsActive(input) {
			active = append(active, ct)
		}
	}
	return active
}
