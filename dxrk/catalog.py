# SPDX-License-Identifier: MIT
from __future__ import annotations

from dxrk.models import AgentID, ComponentID, SkillID, SupportTier

__all__ = [
    "Agent",
    "Component",
    "Skill",
    "all_agents",
    "is_mvp_agent",
    "is_supported_agent",
    "mvp_agents",
    "mvp_components",
    "mvp_skills",
]


class Agent:
    def __init__(
        self,
        id: AgentID,
        name: str = "",
        tier: SupportTier = SupportTier.FULL,
        config_path: str = "",
    ) -> None:
        self.id = id
        self.name = name
        self.tier = tier
        self.config_path = config_path

    def __repr__(self) -> str:
        return f"Agent(id={self.id!r}, name={self.name!r})"


class Component:
    def __init__(self, id: ComponentID, name: str = "", description: str = "") -> None:
        self.id = id
        self.name = name
        self.description = description

    def __repr__(self) -> str:
        return f"Component(id={self.id!r}, name={self.name!r})"


class Skill:
    def __init__(
        self, id: SkillID, name: str = "", category: str = "", priority: str = ""
    ) -> None:
        self.id = id
        self.name = name
        self.category = category
        self.priority = priority

    def __repr__(self) -> str:
        return f"Skill(id={self.id!r}, name={self.name!r})"


_ALL_AGENTS = [
    Agent(
        id=AgentID.CLAUDE_CODE,
        name="Claude Code",
        tier=SupportTier.FULL,
        config_path="~/.claude",
    ),
    Agent(
        id=AgentID.OPENCODE,
        name="OpenCode",
        tier=SupportTier.FULL,
        config_path="~/.config/opencode",
    ),
    Agent(
        id=AgentID.KILOCODE,
        name="Kilo Code",
        tier=SupportTier.FULL,
        config_path="~/.config/kilo",
    ),
    Agent(
        id=AgentID.GEMINI_CLI,
        name="Gemini CLI",
        tier=SupportTier.FULL,
        config_path="~/.gemini",
    ),
    Agent(
        id=AgentID.CODEX, name="Codex", tier=SupportTier.FULL, config_path="~/.codex"
    ),
    Agent(
        id=AgentID.CURSOR, name="Cursor", tier=SupportTier.FULL, config_path="~/.cursor"
    ),
    Agent(
        id=AgentID.VSCODE_COPILOT,
        name="VS Code Copilot",
        tier=SupportTier.FULL,
        config_path="~/.copilot",
    ),
    Agent(
        id=AgentID.ANTIGRAVITY,
        name="Antigravity",
        tier=SupportTier.FULL,
        config_path="~/.gemini/antigravity",
    ),
    Agent(
        id=AgentID.WINDSURF,
        name="Windsurf",
        tier=SupportTier.FULL,
        config_path="~/.codeium/windsurf",
    ),
    Agent(
        id=AgentID.KIMI, name="Kimi Code", tier=SupportTier.FULL, config_path="~/.kimi"
    ),
    Agent(
        id=AgentID.QWEN_CODE,
        name="Qwen Code",
        tier=SupportTier.FULL,
        config_path="~/.qwen",
    ),
    Agent(
        id=AgentID.KIRO_IDE,
        name="Kiro IDE",
        tier=SupportTier.FULL,
        config_path="~/.kiro",
    ),
    Agent(
        id=AgentID.OPENCLAW,
        name="OpenClaw",
        tier=SupportTier.FULL,
        config_path="~/.openclaw",
    ),
    Agent(id=AgentID.PI, name="Pi", tier=SupportTier.FULL, config_path="~/.pi"),
]

_MVP_AGENTS = [
    Agent(
        id=AgentID.CLAUDE_CODE,
        name="Claude Code",
        tier=SupportTier.FULL,
        config_path="~/.claude",
    ),
    Agent(
        id=AgentID.OPENCODE,
        name="OpenCode",
        tier=SupportTier.FULL,
        config_path="~/.config/opencode",
    ),
]


def all_agents() -> list[Agent]:
    return list(_ALL_AGENTS)


def mvp_agents() -> list[Agent]:
    return list(_MVP_AGENTS)


def is_mvp_agent(agent: AgentID) -> bool:
    return any(a.id == agent for a in _MVP_AGENTS)


def is_supported_agent(agent: AgentID) -> bool:
    return any(a.id == agent for a in _ALL_AGENTS)


_MVP_COMPONENTS = [
    Component(
        id=ComponentID.DXRK_MEMORY,
        name="Engram",
        description="Persistent cross-session memory",
    ),
    Component(
        id=ComponentID.SDD, name="SDD", description="Spec-driven development workflow"
    ),
    Component(
        id=ComponentID.SKILLS, name="Skills", description="Curated coding skill library"
    ),
    Component(
        id=ComponentID.CONTEXT7,
        name="Context7",
        description="Latest framework and library docs",
    ),
    Component(
        id=ComponentID.PERSONA,
        name="Persona",
        description="Gentleman, neutral or custom behavior",
    ),
    Component(
        id=ComponentID.PERMISSIONS,
        name="Permissions",
        description="Security-first defaults and guardrails",
    ),
    Component(
        id=ComponentID.DXRK_GUARDIAN,
        name="GGA",
        description="Gentleman Guardian Angel AI provider switcher",
    ),
    Component(
        id=ComponentID.THEME,
        name="Theme",
        description="Gentleman Kanagawa theme overlay",
    ),
    Component(
        id=ComponentID.CLAUDE_THEME,
        name="Claude Theme",
        description="Claude Code-specific theme",
    ),
    Component(
        id=ComponentID.OPENCODE_DXRK_LOGO,
        name="OpenCode Gentle Logo",
        description="Braille rose home logo plugin",
    ),
]


def mvp_components() -> list[Component]:
    return list(_MVP_COMPONENTS)


_PRIORITY_P0 = "p0"

_MVP_SKILLS = [
    Skill(id=SkillID.SDD_INIT, name="sdd-init", category="sdd", priority=_PRIORITY_P0),
    Skill(
        id=SkillID.SDD_APPLY, name="sdd-apply", category="sdd", priority=_PRIORITY_P0
    ),
    Skill(
        id=SkillID.SDD_VERIFY, name="sdd-verify", category="sdd", priority=_PRIORITY_P0
    ),
    Skill(
        id=SkillID.SDD_EXPLORE,
        name="sdd-explore",
        category="sdd",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.SDD_PROPOSE,
        name="sdd-propose",
        category="sdd",
        priority=_PRIORITY_P0,
    ),
    Skill(id=SkillID.SDD_SPEC, name="sdd-spec", category="sdd", priority=_PRIORITY_P0),
    Skill(
        id=SkillID.SDD_DESIGN, name="sdd-design", category="sdd", priority=_PRIORITY_P0
    ),
    Skill(
        id=SkillID.SDD_TASKS, name="sdd-tasks", category="sdd", priority=_PRIORITY_P0
    ),
    Skill(
        id=SkillID.SDD_ARCHIVE,
        name="sdd-archive",
        category="sdd",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.SDD_ONBOARD,
        name="sdd-onboard",
        category="sdd",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.GO_TESTING,
        name="go-testing",
        category="testing",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.SKILL_CREATOR,
        name="skill-creator",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.JUDGMENT_DAY,
        name="judgment-day",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.BRANCH_PR,
        name="branch-pr",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.ISSUE_CREATION,
        name="issue-creation",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.SKILL_REGISTRY,
        name="skill-registry",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.CHAINED_PR,
        name="chained-pr",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.COGNITIVE_DOC,
        name="cognitive-doc-design",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.COMMENT_WRITER,
        name="comment-writer",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
    Skill(
        id=SkillID.WORK_UNIT_COMMITS,
        name="work-unit-commits",
        category="workflow",
        priority=_PRIORITY_P0,
    ),
]


def mvp_skills() -> list[Skill]:
    return list(_MVP_SKILLS)
