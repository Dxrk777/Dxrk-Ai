// SPDX-License-Identifier: MIT
package model

type AgentID string

const (
	AgentClaudeCode    AgentID = "claude-code"
	AgentOpenCode      AgentID = "opencode"
	AgentKilocode      AgentID = "kilocode"
	AgentGeminiCLI     AgentID = "gemini-cli"
	AgentCursor        AgentID = "cursor"
	AgentVSCodeCopilot AgentID = "vscode-copilot"
	AgentCodex         AgentID = "codex"
	AgentAntigravity   AgentID = "antigravity"
	AgentWindsurf      AgentID = "windsurf"
	AgentKimi          AgentID = "kimi"
	AgentQwenCode      AgentID = "qwen-code"
	AgentKiroIDE       AgentID = "kiro-ide"
	AgentOpenClaw      AgentID = "openclaw"
	AgentPi            AgentID = "pi"
	AgentAider         AgentID = "aider"
	AgentCline         AgentID = "cline"
	AgentRooCode       AgentID = "roo-code"
	AgentContinue      AgentID = "continue"
	AgentJunie         AgentID = "junie"
	AgentAmazonQ       AgentID = "amazon-q"
	AgentOpenHands     AgentID = "openhands"
	AgentZedAI         AgentID = "zed-ai"
	AgentCopilot       AgentID = "github-copilot"
	AgentDevin         AgentID = "devin"
	AgentCody          AgentID = "cody"
	AgentTabnine       AgentID = "tabnine"
	AgentReplit        AgentID = "replit"
	AgentVoid          AgentID = "void"
	AgentHermes        AgentID = "hermes"
	AgentAmp           AgentID = "amp"
	AgentTrae          AgentID = "trae"
	AgentConductor     AgentID = "conductor"
	AgentRunCell       AgentID = "runcell"
	AgentLoopoperators AgentID = "looperators"
	AgentPearAI        AgentID = "pearai"
	AgentBolt          AgentID = "bolt"
	AgentLovable       AgentID = "lovable"
	AgentV0            AgentID = "v0"
	AgentBlackbox      AgentID = "blackbox"
	AgentQodo          AgentID = "qodo"
	AgentJetBrains     AgentID = "jetbrains"
	AgentZCode         AgentID = "zcode"
)

// SupportTier indicates how fully an agent supports the Dxrk AI ecosystem.
// All current agents receive the full SDD orchestrator, skill files, MCP config,
// and system prompt injection. The tier is kept as metadata for display purposes.
type SupportTier string

const (
	// TierFull — the agent receives all ecosystem features: SDD orchestrator,
	// skill files, MCP servers, system prompt, and sub-agent delegation.
	TierFull SupportTier = "full"
)

type ComponentID string

const (
	ComponentDxrkMemory        ComponentID = "dxrk-memory"
	ComponentSDD               ComponentID = "sdd"
	ComponentSkills            ComponentID = "skills"
	ComponentContext7          ComponentID = "context7"
	ComponentPersona           ComponentID = "persona"
	ComponentPermission        ComponentID = "permissions"
	ComponentDxrkGuardian      ComponentID = "dxrk-guardian"
	ComponentTheme             ComponentID = "theme"
	ComponentClaudeTheme       ComponentID = "claude-theme"
	ComponentOpenCodeDxrkLogo  ComponentID = "opencode-dxrk-logo"
	ComponentChecker           ComponentID = "checker"
	ComponentInternalMCPServer ComponentID = "internal-mcp-server"
)

type UninstallMode string

const (
	UninstallModePartial      UninstallMode = "partial"
	UninstallModeFull         UninstallMode = "full"
	UninstallModeFullRemove   UninstallMode = "full-remove"
	UninstallModeCleanInstall UninstallMode = "clean-install"
)

type DxrkMemoryUninstallScope string

const (
	DxrkMemoryUninstallScopeGlobal  DxrkMemoryUninstallScope = "global"
	DxrkMemoryUninstallScopeProject DxrkMemoryUninstallScope = "project"
)

type SkillID string

const (
	SkillSDDInit         SkillID = "sdd-init"
	SkillSDDApply        SkillID = "sdd-apply"
	SkillSDDVerify       SkillID = "sdd-verify"
	SkillSDDExplore      SkillID = "sdd-explore"
	SkillSDDPropose      SkillID = "sdd-propose"
	SkillSDDSpec         SkillID = "sdd-spec"
	SkillSDDDesign       SkillID = "sdd-design"
	SkillSDDTasks        SkillID = "sdd-tasks"
	SkillSDDArchive      SkillID = "sdd-archive"
	SkillSDDOnboard      SkillID = "sdd-onboard"
	SkillGoTesting       SkillID = "go-testing"
	SkillCreator         SkillID = "skill-creator"
	SkillJudgmentDay     SkillID = "judgment-day"
	SkillBranchPR        SkillID = "branch-pr"
	SkillIssueCreation   SkillID = "issue-creation"
	SkillSkillRegistry   SkillID = "skill-registry"
	SkillChainedPR       SkillID = "chained-pr"
	SkillCognitiveDoc    SkillID = "cognitive-doc-design"
	SkillCommentWriter   SkillID = "comment-writer"
	SkillWorkUnitCommits SkillID = "work-unit-commits"
)

type PersonaID string

const (
	PersonaDxrk    PersonaID = "dxrk"
	PersonaNeutral PersonaID = "neutral"
	PersonaCustom  PersonaID = "custom"
)

// SystemPromptStrategy defines how an agent's system prompt file is managed.
type SystemPromptStrategy int

const (
	// StrategyMarkdownSections uses <!-- dxrk:ID --> markers to inject sections
	// into an existing file without clobbering user content (Claude Code CLAUDE.md).
	StrategyMarkdownSections SystemPromptStrategy = iota
	// StrategyFileReplace replaces the entire system prompt file (OpenCode AGENTS.md).
	StrategyFileReplace
	// StrategyAppendToFile appends content to an existing system prompt file.
	StrategyAppendToFile
	// StrategyInstructionsFile writes a dedicated instructions file (e.g. .instructions.md).
	StrategyInstructionsFile
	// StrategyJinjaModules writes separate module files that are included into a
	// thin Jinja2 template (e.g. Kimi's KIMI.md).
	StrategyJinjaModules
	// StrategySteeringFile writes a Kiro steering file with inclusion: always frontmatter.
	StrategySteeringFile
)

// MCPStrategy defines how MCP server configs are written for an agent.
type MCPStrategy int

const (
	// StrategySeparateMCPFiles writes one JSON file per server in a dedicated directory
	// (e.g., ~/.claude/mcp/context7.json).
	StrategySeparateMCPFiles MCPStrategy = iota
	// StrategyMergeIntoSettings merges mcpServers into a settings.json file
	// (e.g., OpenCode, Gemini CLI).
	StrategyMergeIntoSettings
	// StrategyMCPConfigFile writes to a dedicated mcp.json config file (e.g., Cursor ~/.cursor/mcp.json).
	StrategyMCPConfigFile
	// StrategyTOMLFile writes MCP config to a TOML file (e.g., Codex ~/.codex/config.toml).
	StrategyTOMLFile
)

type PresetID string

const (
	PresetFullDxrk      PresetID = "full-dxrk"
	PresetEcosystemOnly PresetID = "ecosystem-only"
	PresetMinimal       PresetID = "minimal"
	PresetCustom        PresetID = "custom"
)

type SDDModeID string

const (
	SDDModeSingle SDDModeID = "single"
	SDDModeMulti  SDDModeID = "multi"
)

// SDDProfileStrategyID defines how sync handles OpenCode SDD profiles.
type SDDProfileStrategyID string

const (
	// SDDProfileStrategyGeneratedMulti is the default/backward-compatible mode:
	// named profiles coexist in opencode.json as suffixed agents and are detected
	// from sdd-orchestrator-{name} keys during regular sync.
	SDDProfileStrategyGeneratedMulti SDDProfileStrategyID = "generated-multi"
	// SDDProfileStrategyExternalSingleActive supports external profile managers
	// that keep profile state outside opencode.json and activate one runtime
	// profile without requiring a restart.
	SDDProfileStrategyExternalSingleActive SDDProfileStrategyID = "external-single-active"
)

type OpenCodeCommunityPluginID string

const (
	OpenCodePluginSubAgentStatusline  OpenCodeCommunityPluginID = "sub-agent-statusline"
	OpenCodePluginSDDDxrkMemoryManage OpenCodeCommunityPluginID = "sdd-dxrk-memory-plugin"
	OpenCodePluginDxrkLogo            OpenCodeCommunityPluginID = "dxrk-logo"
)

// Profile represents a named SDD orchestrator configuration with model assignments.
// The default profile (Name="" or Name="default") maps to the base sdd-orchestrator.
// Named profiles generate sdd-orchestrator-{Name} + suffixed sub-agents.
type Profile struct {
	Name              string                     // e.g. "cheap", "premium"; empty = default
	OrchestratorModel ModelAssignment            // orchestrator model
	PhaseAssignments  map[string]ModelAssignment // key = phase name (e.g. "sdd-apply")
}
