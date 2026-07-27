# SPDX-License-Identifier: MIT
import json
from pathlib import Path

from tools.registry import registry, tool_error, tool_result

TASKS_DIR = Path.home() / ".dxrk" / "tasks"

SCHEMA = {
    "name": "task_output",
    "description": (
        "Retrieve the output and status of a background task. "
        "Background tasks store their output and status on disk. "
        "Use this tool to check on a task that was started asynchronously."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "task_id": {
                "type": "string",
                "description": "The task ID to retrieve output for.",
            },
        },
        "required": ["task_id"],
    },
}


def _handler(args, **kw):
    task_id = args.get("task_id", "").strip()
    if not task_id:
        return tool_error("'task_id' is required")

    task_dir = TASKS_DIR / task_id
    if not task_dir.is_dir():
        return tool_error(f"Task '{task_id}' not found at {task_dir}")

    # Read status
    status = "unknown"
    status_file = task_dir / "status.json"
    if status_file.is_file():
        try:
            data = json.loads(status_file.read_text(encoding="utf-8"))
            status = data.get("status", "unknown")
        except (json.JSONDecodeError, OSError):
            status = "unknown"

    # Read output log
    output = ""
    output_log = task_dir / "output.log"
    if output_log.is_file():
        try:
            output = output_log.read_text(encoding="utf-8")
        except OSError:
            output = "(error reading output)"

    # Read exit code if available
    exit_code = None
    exit_file = task_dir / "exit_code"
    if exit_file.is_file():
        try:
            exit_code = int(exit_file.read_text(encoding="utf-8").strip())
        except (ValueError, OSError):
            pass

    return tool_result({
        "task_id": task_id,
        "status": status,
        "output": output,
        "exit_code": exit_code,
    })


registry.register(
    name="task_output",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    emoji="📄",
)
