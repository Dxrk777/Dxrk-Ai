// SPDX-License-Identifier: MIT
package model

import "testing"

func TestClaudeModelAliasString(t *testing.T) {
	tests := []struct {
		alias ClaudeModelAlias
		want  string
	}{
		{ClaudeModelOpus, "opus"},
		{ClaudeModelSonnet, "sonnet"},
		{ClaudeModelHaiku, "haiku"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.alias.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeModelAliasValid(t *testing.T) {
	tests := []struct {
		alias ClaudeModelAlias
		want  bool
	}{
		{ClaudeModelOpus, true},
		{ClaudeModelSonnet, true},
		{ClaudeModelHaiku, true},
		{"", false},
		{"invalid", false},
		{"claude-opus-4", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.alias), func(t *testing.T) {
			if got := tt.alias.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeModelPresetBalanced(t *testing.T) {
	m := ClaudeModelPresetBalanced()
	expectedKeys := []string{
		string(SkillSDDExplore),
		string(SkillSDDPropose),
		string(SkillSDDSpec),
		string(SkillSDDDesign),
		string(SkillSDDTasks),
		string(SkillSDDApply),
		string(SkillSDDVerify),
		string(SkillSDDArchive),
		"default",
	}
	for _, k := range expectedKeys {
		v, ok := m[k]
		if !ok {
			t.Errorf("Balanced preset missing key %q", k)
			continue
		}
		if string(v) == "" {
			t.Errorf("Balanced preset[%q] has empty model alias", k)
		}
	}
	if len(m) != len(expectedKeys) {
		t.Errorf("Balanced preset has %d entries, want %d", len(m), len(expectedKeys))
	}
}

func TestClaudeModelPresetPerformance(t *testing.T) {
	m := ClaudeModelPresetPerformance()
	expectedKeys := []string{
		string(SkillSDDExplore),
		string(SkillSDDPropose),
		string(SkillSDDSpec),
		string(SkillSDDDesign),
		string(SkillSDDTasks),
		string(SkillSDDApply),
		string(SkillSDDVerify),
		string(SkillSDDArchive),
		"default",
	}
	for _, k := range expectedKeys {
		v, ok := m[k]
		if !ok {
			t.Errorf("Performance preset missing key %q", k)
			continue
		}
		if string(v) == "" {
			t.Errorf("Performance preset[%q] has empty model alias", k)
		}
	}
	if len(m) != len(expectedKeys) {
		t.Errorf("Performance preset has %d entries, want %d", len(m), len(expectedKeys))
	}
}

func TestClaudeModelPresetEconomy(t *testing.T) {
	m := ClaudeModelPresetEconomy()
	expectedKeys := []string{
		string(SkillSDDExplore),
		string(SkillSDDPropose),
		string(SkillSDDSpec),
		string(SkillSDDDesign),
		string(SkillSDDTasks),
		string(SkillSDDApply),
		string(SkillSDDVerify),
		string(SkillSDDArchive),
		"default",
	}
	for _, k := range expectedKeys {
		v, ok := m[k]
		if !ok {
			t.Errorf("Economy preset missing key %q", k)
			continue
		}
		if string(v) == "" {
			t.Errorf("Economy preset[%q] has empty model alias", k)
		}
	}
	if len(m) != len(expectedKeys) {
		t.Errorf("Economy preset has %d entries, want %d", len(m), len(expectedKeys))
	}
}
