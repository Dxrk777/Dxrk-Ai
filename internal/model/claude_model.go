// SPDX-License-Identifier: MIT
package model

// ClaudeModelAlias represents one of the three Claude model tiers used for
// per-phase model assignments in the SDD orchestrator.
//
// Only three values are valid: ClaudeModelOpus, ClaudeModelSonnet, ClaudeModelHaiku.
type ClaudeModelAlias string

const claudeModelDefaultKey = "default"

const (
	// ClaudeModelOpus is the high-capability tier, best for architectural decisions
	// and orchestration. Maps to the current claude-opus-* family.
	ClaudeModelOpus ClaudeModelAlias = "opus"

	// ClaudeModelSonnet is the balanced tier, suitable for most SDD phases.
	// Maps to the current claude-sonnet-* family.
	ClaudeModelSonnet ClaudeModelAlias = "sonnet"

	// ClaudeModelHaiku is the lightweight tier, ideal for mechanical tasks like
	// archiving or simple copy work. Maps to the current claude-haiku-* family.
	ClaudeModelHaiku ClaudeModelAlias = "haiku"
)

// String returns the string representation of the alias.
func (a ClaudeModelAlias) String() string {
	return string(a)
}

// Valid reports whether the alias is one of the three known Claude model tiers.
func (a ClaudeModelAlias) Valid() bool {
	switch a {
	case ClaudeModelOpus, ClaudeModelSonnet, ClaudeModelHaiku:
		return true
	default:
		return false
	}
}

// ClaudeModelPresetBalanced returns the default model assignment table.
// It balances cost and capability for Claude sub-agents: architecture phases use opus;
// implementation and validation use sonnet; archiving uses haiku.
func ClaudeModelPresetBalanced() map[string]ClaudeModelAlias {
	return map[string]ClaudeModelAlias{
		string(SkillSDDExplore): ClaudeModelSonnet,
		string(SkillSDDPropose): ClaudeModelOpus,
		string(SkillSDDSpec):    ClaudeModelSonnet,
		string(SkillSDDDesign):  ClaudeModelOpus,
		string(SkillSDDTasks):   ClaudeModelSonnet,
		string(SkillSDDApply):   ClaudeModelSonnet,
		string(SkillSDDVerify):  ClaudeModelSonnet,
		string(SkillSDDArchive): ClaudeModelHaiku,
		claudeModelDefaultKey:   ClaudeModelSonnet,
	}
}

// ClaudeModelPresetPerformance returns a model assignment table optimised for
// output quality. Architecture, planning, and verification phases all use opus.
func ClaudeModelPresetPerformance() map[string]ClaudeModelAlias {
	return map[string]ClaudeModelAlias{
		string(SkillSDDExplore): ClaudeModelSonnet,
		string(SkillSDDPropose): ClaudeModelOpus,
		string(SkillSDDSpec):    ClaudeModelSonnet,
		string(SkillSDDDesign):  ClaudeModelOpus,
		string(SkillSDDTasks):   ClaudeModelSonnet,
		string(SkillSDDApply):   ClaudeModelSonnet,
		string(SkillSDDVerify):  ClaudeModelOpus,
		string(SkillSDDArchive): ClaudeModelHaiku,
		claudeModelDefaultKey:   ClaudeModelSonnet,
	}
}

// ClaudeModelPresetEconomy returns a model assignment table optimised for cost.
// Every phase uses sonnet except archive, which uses haiku.
func ClaudeModelPresetEconomy() map[string]ClaudeModelAlias {
	return map[string]ClaudeModelAlias{
		string(SkillSDDExplore): ClaudeModelSonnet,
		string(SkillSDDPropose): ClaudeModelSonnet,
		string(SkillSDDSpec):    ClaudeModelSonnet,
		string(SkillSDDDesign):  ClaudeModelSonnet,
		string(SkillSDDTasks):   ClaudeModelSonnet,
		string(SkillSDDApply):   ClaudeModelSonnet,
		string(SkillSDDVerify):  ClaudeModelSonnet,
		string(SkillSDDArchive): ClaudeModelHaiku,
		claudeModelDefaultKey:   ClaudeModelSonnet,
	}
}
