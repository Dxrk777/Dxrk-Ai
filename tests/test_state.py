# SPDX-License-Identifier: MIT
from __future__ import annotations

import json
from pathlib import Path

import pytest

from dxrk.state import (
    InstallState,
    ModelAssignmentState,
    read,
    state_path,
    write,
)


def test_state_path_returns_correct_path():
    result = state_path("/home/user")
    assert result == Path("/home/user/.dxrk/state.json")


def test_state_path_with_path_object():
    p = Path("/tmp/home")
    result = state_path(p)
    assert result == Path("/tmp/home/.dxrk/state.json")


def test_write_and_read_roundtrip(temp_dir: Path):
    s = InstallState(
        installed_agents=["claude-code", "opencode"],
        claude_model_assignments={"orchestrator": "opus"},
        kiro_model_assignments={"default": "sonnet"},
        model_assignments={"orchestrator": ModelAssignmentState("opencode", "sonnet")},
    )
    write(temp_dir, s)

    loaded = read(temp_dir)
    assert loaded.installed_agents == ["claude-code", "opencode"]
    assert loaded.claude_model_assignments == {"orchestrator": "opus"}
    assert loaded.kiro_model_assignments == {"default": "sonnet"}
    assert loaded.model_assignments is not None
    assert loaded.model_assignments["orchestrator"].provider_id == "opencode"


def test_write_creates_directory(temp_dir: Path):
    nested = temp_dir / "sub" / "dir"
    s = InstallState(installed_agents=["claude-code"])
    write(nested, s)
    assert (nested / ".dxrk" / "state.json").exists()


def test_read_empty_state_raises(temp_dir: Path):
    with pytest.raises(FileNotFoundError):
        read(temp_dir)


def test_read_malformed_json(temp_dir: Path):
    p = state_path(temp_dir)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text("not json")
    with pytest.raises(json.JSONDecodeError):
        read(temp_dir)


def test_model_assignment_state():
    m = ModelAssignmentState(provider_id="opencode", model_id="sonnet")
    assert m.provider_id == "opencode"
    assert m.model_id == "sonnet"


def test_model_assignment_state_defaults():
    m = ModelAssignmentState()
    assert m.provider_id == ""
    assert m.model_id == ""


def test_install_state_defaults():
    s = InstallState()
    assert s.installed_agents == []
    assert s.claude_model_assignments is None
    assert s.kiro_model_assignments is None
    assert s.model_assignments is None
