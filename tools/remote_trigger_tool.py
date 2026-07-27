# SPDX-License-Identifier: MIT
import json
import os
import uuid
from datetime import datetime, timezone
from pathlib import Path

from tools.registry import registry, tool_error

REMOTE_DIR = Path.home() / ".dxrk" / "remote"
REMOTE_ENV_VAR = "DXRK_REMOTE_ENABLED"


def check_remote_enabled():
    return os.environ.get(REMOTE_ENV_VAR) == "1"


SCHEMA = {
    "name": "remote_trigger",
    "description": "Trigger a task to run in a remote agent/sandbox environment. Stores the task locally for pickup by a remote runner. Requires DXRK_REMOTE_ENABLED=1 environment variable.",
    "parameters": {
        "type": "object",
        "properties": {
            "task_description": {
                "type": "string",
                "description": "Description of the task for the remote agent to execute",
            },
            "config": {
                "type": "object",
                "description": "Optional configuration dict (e.g. runner, timeout, tags)",
                "properties": {
                    "runner": {
                        "type": "string",
                        "description": "Remote runner identifier (e.g. 'sandbox', 'ec2', 'parallels')",
                    },
                    "timeout_minutes": {
                        "type": "number",
                        "description": "Max execution time in minutes",
                    },
                    "tags": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Tags for filtering and organization",
                    },
                },
            },
        },
        "required": ["task_description"],
    },
}


def _handler(args, **kw):
    if not check_remote_enabled():
        return tool_error(
            "Remote trigger is not configured. Set DXRK_REMOTE_ENABLED=1 to enable.",
            code="not_configured",
        )

    task_description = args.get("task_description", "").strip()
    if not task_description:
        return tool_error("task_description is required")

    config = args.get("config") or {}
    task_id = str(uuid.uuid4())
    REMOTE_DIR.mkdir(parents=True, exist_ok=True)

    task_data = {
        "task_id": task_id,
        "task_description": task_description,
        "config": config,
        "status": "pending",
        "created_at": datetime.now(timezone.utc).isoformat(),
    }

    task_file = REMOTE_DIR / f"{task_id}.json"
    task_file.write_text(json.dumps(task_data, indent=2, ensure_ascii=False))

    return json.dumps({
        "result": "Remote task created",
        "task_id": task_id,
        "status": "pending",
        "task_file": str(task_file),
    })


registry.register(
    name="remote_trigger",
    toolset="dev",
    schema=SCHEMA,
    handler=_handler,
    check_fn=check_remote_enabled,
    requires_env=[REMOTE_ENV_VAR],
    emoji="🌐",
)
