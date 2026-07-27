// SPDX-License-Identifier: MIT
package model

import "testing"

// TestSelectionHasStrictTDDField verifies that the Selection struct has a
// StrictTDD bool field.
func TestSelectionHasAgent(t *testing.T) {
	s := Selection{
		Agents: []AgentID{AgentClaudeCode, AgentOpenCode, AgentKiroIDE},
	}
	if !s.HasAgent(AgentClaudeCode) {
		t.Error("HasAgent(ClaudeCode) = false, want true")
	}
	if !s.HasAgent(AgentOpenCode) {
		t.Error("HasAgent(OpenCode) = false, want true")
	}
	if s.HasAgent(AgentCursor) {
		t.Error("HasAgent(Cursor) = true, want false")
	}
	if s.HasAgent(AgentGeminiCLI) {
		t.Error("HasAgent(GeminiCLI) = true, want false")
	}
}

func TestSelectionHasComponent(t *testing.T) {
	s := Selection{
		Components: []ComponentID{ComponentDxrkMemory, ComponentSDD, ComponentSkills},
	}
	if !s.HasComponent(ComponentDxrkMemory) {
		t.Error("HasComponent(Engram) = false, want true")
	}
	if !s.HasComponent(ComponentSDD) {
		t.Error("HasComponent(SDD) = false, want true")
	}
	if s.HasComponent(ComponentPersona) {
		t.Error("HasComponent(Persona) = true, want false")
	}
	if s.HasComponent(ComponentTheme) {
		t.Error("HasComponent(Theme) = true, want false")
	}
}

func TestSelectionHasStrictTDDField(t *testing.T) {
	s := Selection{}
	// Field must be accessible and default to false.
	if s.StrictTDD {
		t.Fatal("Selection.StrictTDD default = true, want false")
	}

	s.StrictTDD = true
	if !s.StrictTDD {
		t.Fatal("Selection.StrictTDD set to true but read back as false")
	}
}

// TestSyncOverridesHasStrictTDDPointer verifies that SyncOverrides has a
// *bool StrictTDD field (nil = no override semantics).
func TestSyncOverridesHasStrictTDDPointer(t *testing.T) {
	o := SyncOverrides{}
	// Nil means "no override".
	if o.StrictTDD != nil {
		t.Fatal("SyncOverrides.StrictTDD default = non-nil, want nil")
	}

	enabled := true
	o.StrictTDD = &enabled
	if o.StrictTDD == nil || !*o.StrictTDD {
		t.Fatal("SyncOverrides.StrictTDD pointer set to true but read back incorrectly")
	}

	disabled := false
	o.StrictTDD = &disabled
	if o.StrictTDD == nil || *o.StrictTDD {
		t.Fatal("SyncOverrides.StrictTDD pointer set to false but read back incorrectly")
	}
}
