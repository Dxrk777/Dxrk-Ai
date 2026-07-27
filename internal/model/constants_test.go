// SPDX-License-Identifier: MIT
package model

import "testing"

func TestAgentIDConstantsNonEmpty(t *testing.T) {
	all := []AgentID{
		AgentClaudeCode,
		AgentOpenCode,
		AgentKilocode,
		AgentGeminiCLI,
		AgentCursor,
		AgentVSCodeCopilot,
		AgentCodex,
		AgentAntigravity,
		AgentWindsurf,
		AgentKimi,
		AgentQwenCode,
		AgentKiroIDE,
		AgentOpenClaw,
		AgentPi,
	}
	for _, id := range all {
		if string(id) == "" {
			t.Errorf("AgentID constant is empty")
		}
	}
}

func TestAgentIDConstantsUnique(t *testing.T) {
	all := []AgentID{
		AgentClaudeCode,
		AgentOpenCode,
		AgentKilocode,
		AgentGeminiCLI,
		AgentCursor,
		AgentVSCodeCopilot,
		AgentCodex,
		AgentAntigravity,
		AgentWindsurf,
		AgentKimi,
		AgentQwenCode,
		AgentKiroIDE,
		AgentOpenClaw,
		AgentPi,
	}
	seen := make(map[string]bool)
	for _, id := range all {
		s := string(id)
		if seen[s] {
			t.Errorf("duplicate AgentID constant: %q", s)
		}
		seen[s] = true
	}
	if len(seen) != len(all) {
		t.Errorf("got %d unique values, want %d", len(seen), len(all))
	}
}
