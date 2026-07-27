# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Team Management Tools — Create and delete agent teams via file-based storage.

Mirrors Claude Code's TeamCreateTool + TeamDeleteTool using a simple
directory ``~/.dxrk/teams/{name}/`` with a ``team.json`` manifest.

Tools:
  - team_create:  create a new team with name, members, and description
  - team_delete:  delete a team by name (refuses if members still active)
"""

import json
import os
import shutil
import threading
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional

TEAMS_DIR = Path.home() / ".dxrk" / "teams"


class TeamManager:
    """File-based team storage.

    Each team lives at ``{TEAMS_DIR}/{sanitized_name}/`` with a ``team.json``
    manifest containing name, description, created_at, and member list.
    """

    _lock = threading.Lock()

    # ------------------------------------------------------------------
    # Paths
    # ------------------------------------------------------------------

    @staticmethod
    def _team_dir(name: str) -> Path:
        safe = name.strip().replace(" ", "_").replace("/", "_")
        return TEAMS_DIR / safe

    @staticmethod
    def _manifest_path(name: str) -> Path:
        return TeamManager._team_dir(name) / "team.json"

    # ------------------------------------------------------------------
    # CRUD
    # ------------------------------------------------------------------

    @classmethod
    def create(
        cls,
        name: str,
        members: List[Dict],
        description: str = "",
    ) -> Dict:
        """Create a new team.

        *name* must be non-empty and must not collide with an existing team.

        *members* is a list of dicts, each with at least ``{"name": str}``.
        The calling agent is automatically set as the team lead.

        Returns a dict with team metadata.
        """
        if not name or not name.strip():
            return {"success": False, "error": "team name is required"}

        safe_name = name.strip()

        with cls._lock:
            manifest_path = cls._manifest_path(safe_name)
            if manifest_path.exists():
                return {
                    "success": False,
                    "error": f'Team "{safe_name}" already exists',
                }

            team = {
                "name": safe_name,
                "description": description or "",
                "created_at": datetime.now(timezone.utc).isoformat(),
                "members": [],
            }

            lead_added = False
            seen_names = set()
            for m in members:
                member_name = m.get("name", "").strip()
                if not member_name:
                    continue
                if member_name.lower() in seen_names:
                    continue
                seen_names.add(member_name.lower())

                entry = {
                    "name": member_name,
                    "agent_type": m.get("agent_type", member_name),
                    "joined_at": datetime.now(timezone.utc).isoformat(),
                    "is_active": m.get("is_active", True),
                }
                if not lead_added:
                    entry["role"] = "lead"
                    lead_added = True
                else:
                    entry["role"] = "member"

                team["members"].append(entry)

            if not team["members"]:
                return {"success": False, "error": "at least one member is required"}

            team_dir = cls._team_dir(safe_name)
            team_dir.mkdir(parents=True, exist_ok=True)
            manifest_path.write_text(
                json.dumps(team, ensure_ascii=False, indent=2),
                encoding="utf-8",
            )

        return {
            "success": True,
            "team_name": safe_name,
            "team_path": str(team_dir),
            "member_count": len(team["members"]),
        }

    @classmethod
    def delete(cls, name: str, force: bool = False) -> Dict:
        """Delete a team by name.

        Refuses if the team has active non-lead members, unless *force* is
        True (mirroring TeamDeleteTool's active-member guard).
        """
        if not name or not name.strip():
            return {"success": False, "error": "team name is required"}

        safe_name = name.strip()

        with cls._lock:
            manifest_path = cls._manifest_path(safe_name)
            if not manifest_path.exists():
                return {"success": False, "error": f'Team "{safe_name}" not found'}

            if not force:
                team = json.loads(manifest_path.read_text(encoding="utf-8"))
                active = [
                    m["name"]
                    for m in team.get("members", [])
                    if m.get("is_active") is True and m.get("role") != "lead"
                ]
                if active:
                    return {
                        "success": False,
                        "error": (
                            f"Cannot delete team with {len(active)} active "
                            f"member(s): {', '.join(active)}. "
                            "Set member inactive first or use force=true."
                        ),
                    }

            team_dir = cls._team_dir(safe_name)
            if team_dir.exists():
                shutil.rmtree(str(team_dir))

        return {
            "success": True,
            "message": f'Team "{safe_name}" deleted',
        }

    @classmethod
    def list_teams(cls) -> List[Dict]:
        """List all teams on disk."""
        if not TEAMS_DIR.exists():
            return []
        result: List[Dict] = []
        for d in sorted(TEAMS_DIR.iterdir()):
            manifest = d / "team.json"
            if manifest.exists():
                try:
                    data = json.loads(manifest.read_text(encoding="utf-8"))
                    result.append({
                        "name": data.get("name", d.name),
                        "description": data.get("description", ""),
                        "member_count": len(data.get("members", [])),
                        "created_at": data.get("created_at", ""),
                    })
                except (json.JSONDecodeError, OSError):
                    pass
        return result


# ---------------------------------------------------------------------------
# Tool handlers
# ---------------------------------------------------------------------------

def team_create_handler(args: dict, **kw) -> str:
    name = args.get("name", "").strip()
    members = args.get("members", [])
    description = args.get("description", "")

    result = TeamManager.create(name, members, description)
    return json.dumps(result, ensure_ascii=False)


def team_delete_handler(args: dict, **kw) -> str:
    name = args.get("name", "").strip()
    force = args.get("force", False)

    result = TeamManager.delete(name, force)
    return json.dumps(result, ensure_ascii=False)


def check_team_requirements() -> bool:
    return True


TEAM_CREATE_SCHEMA = {
    "name": "team_create",
    "description": (
        "Create a team of agents for multi-agent coordination. "
        "The first member in the list automatically becomes the team lead."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "name": {
                "type": "string",
                "description": "Unique name for the team",
            },
            "members": {
                "type": "array",
                "description": "List of team members. First entry becomes lead.",
                "items": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string", "description": "Agent name"},
                        "agent_type": {
                            "type": "string",
                            "description": "Agent role (e.g. researcher, coder)",
                        },
                    },
                    "required": ["name"],
                },
            },
            "description": {
                "type": "string",
                "description": "Purpose or description for the team",
            },
        },
        "required": ["name", "members"],
    },
}

TEAM_DELETE_SCHEMA = {
    "name": "team_delete",
    "description": (
        "Delete a team by name. Refuses if non-lead members are still active "
        "unless force=true."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "name": {
                "type": "string",
                "description": "Name of the team to delete",
            },
            "force": {
                "type": "boolean",
                "description": "Delete even if members are active (default: false)",
            },
        },
        "required": ["name"],
    },
}

from tools.registry import registry

registry.register(
    name="team_create",
    toolset="agent",
    schema=TEAM_CREATE_SCHEMA,
    handler=team_create_handler,
    check_fn=check_team_requirements,
    emoji="👥",
)

registry.register(
    name="team_delete",
    toolset="agent",
    schema=TEAM_DELETE_SCHEMA,
    handler=team_delete_handler,
    check_fn=check_team_requirements,
    emoji="🗑️",
)
