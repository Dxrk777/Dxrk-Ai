# SPDX-License-Identifier: MIT
from __future__ import annotations

from dxrk.catalog import (
    Agent,
    Component,
    Skill,
    all_agents,
    is_mvp_agent,
    is_supported_agent,
    mvp_agents,
    mvp_components,
    mvp_skills,
)
from dxrk.models import AgentID, ComponentID, SkillID, SupportTier


def test_agent_repr():
    a = Agent(id=AgentID.CLAUDE_CODE, name="Claude Code")
    r = repr(a)
    assert "Agent" in r
    assert "Claude Code" in r


def test_agent_config_path():
    a = Agent(id=AgentID.OPENCODE, name="OpenCode", config_path="~/.config/opencode")
    assert a.config_path == "~/.config/opencode"


def test_all_agents_returns_all():
    agents = all_agents()
    assert len(agents) == 14
    ids = [a.id for a in agents]
    assert AgentID.CLAUDE_CODE in ids
    assert AgentID.OPENCODE in ids
    assert AgentID.PI in ids
    assert AgentID.OPENCLAW in ids


def test_mvp_agents_returns_two():
    agents = mvp_agents()
    assert len(agents) == 2
    ids = [a.id for a in agents]
    assert AgentID.CLAUDE_CODE in ids
    assert AgentID.OPENCODE in ids


def test_is_mvp_agent():
    assert is_mvp_agent(AgentID.CLAUDE_CODE)
    assert is_mvp_agent(AgentID.OPENCODE)
    assert not is_mvp_agent(AgentID.PI)
    assert not is_mvp_agent(AgentID.CURSOR)


def test_is_supported_agent():
    assert is_supported_agent(AgentID.CLAUDE_CODE)
    assert is_supported_agent(AgentID.PI)
    assert is_supported_agent(AgentID.OPENCLAW)


def test_component_repr():
    c = Component(id=ComponentID.DXRK_MEMORY, name="Engram")
    r = repr(c)
    assert "Component" in r
    assert "Engram" in r


def test_mvp_components_count():
    components = mvp_components()
    assert len(components) == 10


def test_mvp_components_all_present():
    ids = [c.id for c in mvp_components()]
    assert ComponentID.DXRK_MEMORY in ids
    assert ComponentID.SDD in ids
    assert ComponentID.PERSONA in ids
    assert ComponentID.PERMISSIONS in ids
    assert ComponentID.DXRK_GUARDIAN in ids
    assert ComponentID.THEME in ids
    assert ComponentID.SKILLS in ids
    assert ComponentID.CONTEXT7 in ids


def test_skill_repr():
    s = Skill(id=SkillID.SDD_INIT, name="sdd-init")
    r = repr(s)
    assert "Skill" in r
    assert "sdd-init" in r


def test_mvp_skills_count():
    skills = mvp_skills()
    assert len(skills) == 20


def test_mvp_skills_all_sdd_present():
    ids = [s.id for s in mvp_skills()]
    sdd_skills = [
        SkillID.SDD_INIT, SkillID.SDD_EXPLORE, SkillID.SDD_PROPOSE,
        SkillID.SDD_SPEC, SkillID.SDD_DESIGN, SkillID.SDD_TASKS,
        SkillID.SDD_APPLY, SkillID.SDD_VERIFY, SkillID.SDD_ARCHIVE,
        SkillID.SDD_ONBOARD,
    ]
    for s in sdd_skills:
        assert s in ids


def test_agent_default_tier():
    a = Agent(id=AgentID.CLAUDE_CODE)
    assert a.tier == SupportTier.FULL


def test_agent_default_config_path():
    a = Agent(id=AgentID.CLAUDE_CODE)
    assert a.config_path == ""
