# SPDX-License-Identifier: MIT
from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum


class AgentID(str, Enum):
    CLAUDE_CODE = "claude-code"
    OPENCODE = "opencode"
    KILOCODE = "kilocode"
    GEMINI_CLI = "gemini-cli"
    CURSOR = "cursor"
    VSCODE_COPILOT = "vscode-copilot"
    CODEX = "codex"
    ANTIGRAVITY = "antigravity"
    WINDSURF = "windsurf"
    KIMI = "kimi"
    QWEN_CODE = "qwen-code"
    KIRO_IDE = "kiro-ide"
    OPENCLAW = "openclaw"
    PI = "pi"


class ComponentID(str, Enum):
    DXRK_MEMORY = "DXRK_MEMORY"
    SDD = "sdd"
    SKILLS = "skills"
    CONTEXT7 = "context7"
    MEMPALACE = "mempalace"
    PERSONA = "persona"
    PERMISSIONS = "permissions"
    DXRK_GUARDIAN = "DXRK_GUARDIAN"
    THEME = "theme"
    CLAUDE_THEME = "claude-theme"
    OPENCODE_DXRK_LOGO = "opencode-dxrk-logo"


class UninstallMode(str, Enum):
    PARTIAL = "partial"
    FULL = "full"
    FULL_REMOVE = "full-remove"
    CLEAN_INSTALL = "clean-install"


class DxrkMemoryUninstallScope(str, Enum):
    GLOBAL = "global"
    PROJECT = "project"


class SkillID(str, Enum):
    SDD_INIT = "sdd-init"
    SDD_APPLY = "sdd-apply"
    SDD_VERIFY = "sdd-verify"
    SDD_EXPLORE = "sdd-explore"
    SDD_PROPOSE = "sdd-propose"
    SDD_SPEC = "sdd-spec"
    SDD_DESIGN = "sdd-design"
    SDD_TASKS = "sdd-tasks"
    SDD_ARCHIVE = "sdd-archive"
    SDD_ONBOARD = "sdd-onboard"
    GO_TESTING = "go-testing"
    SKILL_CREATOR = "skill-creator"
    JUDGMENT_DAY = "judgment-day"
    BRANCH_PR = "branch-pr"
    ISSUE_CREATION = "issue-creation"
    SKILL_REGISTRY = "skill-registry"
    CHAINED_PR = "chained-pr"
    COGNITIVE_DOC = "cognitive-doc-design"
    COMMENT_WRITER = "comment-writer"
    WORK_UNIT_COMMITS = "work-unit-commits"


class ClaudeModelAlias(str, Enum):
    OPUS = "opus"
    SONNET = "sonnet"
    HAIKU = "haiku"

    def valid(self) -> bool:
        return self in (
            ClaudeModelAlias.OPUS,
            ClaudeModelAlias.SONNET,
            ClaudeModelAlias.HAIKU,
        )


def claude_model_preset_balanced() -> dict[str, ClaudeModelAlias]:
    return {
        "orchestrator": ClaudeModelAlias.OPUS,
        "sdd-explore": ClaudeModelAlias.SONNET,
        "sdd-propose": ClaudeModelAlias.OPUS,
        "sdd-spec": ClaudeModelAlias.SONNET,
        "sdd-design": ClaudeModelAlias.OPUS,
        "sdd-tasks": ClaudeModelAlias.SONNET,
        "sdd-apply": ClaudeModelAlias.SONNET,
        "sdd-verify": ClaudeModelAlias.SONNET,
        "sdd-archive": ClaudeModelAlias.HAIKU,
        "default": ClaudeModelAlias.SONNET,
    }


def claude_model_preset_performance() -> dict[str, ClaudeModelAlias]:
    return {
        "orchestrator": ClaudeModelAlias.OPUS,
        "sdd-explore": ClaudeModelAlias.SONNET,
        "sdd-propose": ClaudeModelAlias.OPUS,
        "sdd-spec": ClaudeModelAlias.SONNET,
        "sdd-design": ClaudeModelAlias.OPUS,
        "sdd-tasks": ClaudeModelAlias.SONNET,
        "sdd-apply": ClaudeModelAlias.SONNET,
        "sdd-verify": ClaudeModelAlias.OPUS,
        "sdd-archive": ClaudeModelAlias.HAIKU,
        "default": ClaudeModelAlias.SONNET,
    }


def claude_model_preset_economy() -> dict[str, ClaudeModelAlias]:
    return {
        "orchestrator": ClaudeModelAlias.SONNET,
        "sdd-explore": ClaudeModelAlias.SONNET,
        "sdd-propose": ClaudeModelAlias.SONNET,
        "sdd-spec": ClaudeModelAlias.SONNET,
        "sdd-design": ClaudeModelAlias.SONNET,
        "sdd-tasks": ClaudeModelAlias.SONNET,
        "sdd-apply": ClaudeModelAlias.SONNET,
        "sdd-verify": ClaudeModelAlias.SONNET,
        "sdd-archive": ClaudeModelAlias.HAIKU,
        "default": ClaudeModelAlias.SONNET,
    }


class PersonaID(str, Enum):
    DXRK = "dxrk"
    NEUTRAL = "neutral"
    CUSTOM = "custom"


class SystemPromptStrategy(int, Enum):
    MARKDOWN_SECTIONS = 0
    FILE_REPLACE = 1
    APPEND_TO_FILE = 2
    INSTRUCTIONS_FILE = 3
    JINJA_MODULES = 4
    STEERING_FILE = 5


class MCPStrategy(int, Enum):
    SEPARATE_MCP_FILES = 0
    MERGE_INTO_SETTINGS = 1
    MCP_CONFIG_FILE = 2
    TOML_FILE = 3


class PresetID(str, Enum):
    FULL_DXRK = "full-dxrk"
    ECOSYSTEM_ONLY = "ecosystem-only"
    MINIMAL = "minimal"
    CUSTOM = "custom"


class SDDModeID(str, Enum):
    SINGLE = "single"
    MULTI = "multi"


class SDDProfileStrategyID(str, Enum):
    GENERATED_MULTI = "generated-multi"
    EXTERNAL_SINGLE_ACTIVE = "external-single-active"


class OpenCodeCommunityPluginID(str, Enum):
    SUB_AGENT_STATUSLINE = "sub-agent-statusline"
    SDD_ENGRAM_PLUGIN = "sdd-DXRK_MEMORY-plugin"


class SupportTier(str, Enum):
    FULL = "full"


class PlanStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    SUCCEEDED = "succeeded"
    FAILED = "failed"


class RunResult(str, Enum):
    SKIPPED = "skipped"
    SUCCESS = "success"
    FAILED = "failed"


@dataclass
class ModelAssignment:
    provider_id: str = ""
    model_id: str = ""

    def full_id(self) -> str:
        return f"{self.provider_id}/{self.model_id}"


@dataclass
class Profile:
    name: str = ""
    orchestrator_model: ModelAssignment = field(default_factory=ModelAssignment)
    phase_assignments: dict[str, ModelAssignment] = field(default_factory=dict)


@dataclass
class Selection:
    agents: list[AgentID] = field(default_factory=list)
    components: list[ComponentID] = field(default_factory=list)
    skills: list[SkillID] = field(default_factory=list)
    persona: PersonaID = PersonaID.DXRK
    preset: PresetID = PresetID.FULL_DXRK
    sdd_mode: SDDModeID = SDDModeID.SINGLE
    sdd_profile_strategy: SDDProfileStrategyID = SDDProfileStrategyID.GENERATED_MULTI
    strict_tdd: bool = False
    model_assignments: dict[str, ModelAssignment] = field(default_factory=dict)
    claude_model_assignments: dict[str, str] = field(default_factory=dict)
    kiro_model_assignments: dict[str, str] = field(default_factory=dict)
    profiles: list[Profile] = field(default_factory=list)
    opencode_plugins: list[OpenCodeCommunityPluginID] = field(default_factory=list)

    def has_agent(self, agent_id: AgentID) -> bool:
        return agent_id in self.agents

    def has_component(self, component_id: ComponentID) -> bool:
        return component_id in self.components


@dataclass
class SyncOverrides:
    model_assignments: dict[str, ModelAssignment] | None = None
    claude_model_assignments: dict[str, str] | None = None
    kiro_model_assignments: dict[str, str] | None = None
    sdd_mode: SDDModeID | None = None
    sdd_profile_strategy: SDDProfileStrategyID | None = None
    strict_tdd: bool | None = None
    profiles: list[Profile] = field(default_factory=list)


@dataclass
class PlanStep:
    id: str = ""
    name: str = ""
    status: PlanStatus = PlanStatus.PENDING
    result: RunResult = RunResult.SKIPPED
    error: str = ""


@dataclass
class Plan:
    id: str = ""
    selection: Selection = field(default_factory=Selection)
    status: PlanStatus = PlanStatus.PENDING
    steps: list[PlanStep] = field(default_factory=list)


# Shorthand aliases matching Go constant style
AgentClaudeCode = AgentID.CLAUDE_CODE
AgentOpenCode = AgentID.OPENCODE
AgentKilocode = AgentID.KILOCODE
AgentGeminiCLI = AgentID.GEMINI_CLI
AgentCursor = AgentID.CURSOR
AgentVSCodeCopilot = AgentID.VSCODE_COPILOT
AgentCodex = AgentID.CODEX
AgentAntigravity = AgentID.ANTIGRAVITY
AgentWindsurf = AgentID.WINDSURF
AgentKimi = AgentID.KIMI
AgentQwenCode = AgentID.QWEN_CODE
AgentKiroIDE = AgentID.KIRO_IDE
AgentOpenClaw = AgentID.OPENCLAW
AgentPi = AgentID.PI

AGENTS = [
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
]

ComponentEngram = ComponentID.DXRK_MEMORY
ComponentSDD = ComponentID.SDD
ComponentSkills = ComponentID.SKILLS
ComponentContext7 = ComponentID.CONTEXT7
ComponentPersona = ComponentID.PERSONA
ComponentPermission = ComponentID.PERMISSIONS
ComponentGGA = ComponentID.DXRK_GUARDIAN
ComponentTheme = ComponentID.THEME
ComponentClaudeTheme = ComponentID.CLAUDE_THEME
ComponentOpenCodeDxrkLogo = ComponentID.OPENCODE_DXRK_LOGO

COMPONENTS = [
    ComponentEngram,
    ComponentSDD,
    ComponentSkills,
    ComponentContext7,
    ComponentPersona,
    ComponentPermission,
    ComponentGGA,
    ComponentTheme,
    ComponentClaudeTheme,
    ComponentOpenCodeDxrkLogo,
]
