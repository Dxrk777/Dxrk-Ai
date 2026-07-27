# SPDX-License-Identifier: MIT
#!/usr/bin/env python3

import json
import os
from pathlib import Path
from datetime import datetime, timezone

from tools.registry import registry, tool_error, tool_result

DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
PLAN_DIR = DXRK_HOME / "plan"
CURRENT_PLAN_PATH = PLAN_DIR / "current.json"


def _ensure_plan_dir():
    PLAN_DIR.mkdir(parents=True, exist_ok=True)


def _read_plan() -> dict | None:
    if not CURRENT_PLAN_PATH.exists():
        return None
    return json.loads(CURRENT_PLAN_PATH.read_text())


def _write_plan(data: dict) -> None:
    _ensure_plan_dir()
    CURRENT_PLAN_PATH.write_text(
        json.dumps(data, indent=2, ensure_ascii=False, default=str)
    )


def enter_plan_mode_handler(args: dict, **kw) -> str:
    try:
        scope = args.get("scope", "")
        instructions = args.get("instructions", "")

        _ensure_plan_dir()

        plan = {
            "scope": scope,
            "instructions": instructions,
            "created_at": datetime.now(timezone.utc).isoformat(),
            "updated_at": datetime.now(timezone.utc).isoformat(),
            "status": "planning",
            "plan": "",
        }

        _write_plan(plan)

        return tool_result(
            success=True,
            message="Entered plan mode. Plan file created at ~/.dxrk/plan/current.json. "
                    "Use ExitPlanMode when ready to present the plan for approval.",
            plan=plan,
        )

    except Exception as e:
        return tool_error(str(e))


def exit_plan_mode_handler(args: dict, **kw) -> str:
    try:
        plan_id = args.get("plan_id")
        plan = _read_plan()

        if not plan:
            return tool_error(
                "No active plan found. Call enter_plan_mode first."
            )

        if plan_id and plan.get("id") and plan["id"] != plan_id:
            return tool_error(f"Plan ID mismatch: {plan_id}")

        plan["status"] = "executing"
        plan["updated_at"] = datetime.now(timezone.utc).isoformat()
        _write_plan(plan)

        return tool_result(
            success=True,
            message="Plan approved. Exiting plan mode. You can now start implementation.",
            plan=plan,
            file_path=str(CURRENT_PLAN_PATH),
        )

    except Exception as e:
        return tool_error(str(e))


ENTER_PLAN_SCHEMA = {
    "name": "enter_plan_mode",
    "description": (
        "Enter plan mode to design an approach before writing code. "
        "Creates a plan file at ~/.dxrk/plan/current.json and sets status to 'planning'. "
        "Use this before non-trivial implementation to get user approval on the approach."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "scope": {
                "type": "string",
                "description": "Description of what the plan covers",
            },
            "instructions": {
                "type": "string",
                "description": "Additional instructions or constraints for the plan",
            },
        },
        "required": [],
    },
}

EXIT_PLAN_SCHEMA = {
    "name": "exit_plan_mode",
    "description": (
        "Exit plan mode and present the plan for user approval. "
        "Marks the current plan as 'executing' and returns the plan content. "
        "Call this after writing the plan to the plan file."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "plan_id": {
                "type": "string",
                "description": "Optional plan ID to confirm which plan is being approved",
            },
        },
        "required": [],
    },
}

registry.register(
    name="enter_plan_mode",
    toolset="dev",
    schema=ENTER_PLAN_SCHEMA,
    handler=enter_plan_mode_handler,
    emoji="📋",
)

registry.register(
    name="exit_plan_mode",
    toolset="dev",
    schema=EXIT_PLAN_SCHEMA,
    handler=exit_plan_mode_handler,
    emoji="✅",
)
