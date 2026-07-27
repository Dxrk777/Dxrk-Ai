# SPDX-License-Identifier: MIT
import json
from pathlib import Path

from tools.registry import registry, tool_error, tool_result

TASKS_DIR = Path.home() / ".dxrk" / "tasks"

SCHEMA = {
    "name": "session_brief",
    "description": (
        "Return a brief summary of the current session context: "
        "active tasks, recently modified files, and pending todos. "
        "Use at the start of a session or after context compression "
        "to re-establish awareness of what is in progress."
    ),
    "parameters": {
        "type": "object",
        "properties": {},
        "required": [],
    },
}


def _read_task_status() -> list:
    """Read task status files from ~/.dxrk/tasks/."""
    if not TASKS_DIR.is_dir():
        return []
    tasks = []
    for entry in sorted(TASKS_DIR.iterdir()):
        if not entry.is_dir():
            continue
        status_file = entry / "status.json"
        if status_file.is_file():
            try:
                data = json.loads(status_file.read_text(encoding="utf-8"))
                data.setdefault("task_id", entry.name)
                tasks.append(data)
            except (json.JSONDecodeError, OSError):
                tasks.append({"task_id": entry.name, "status": "unknown"})
        else:
            output_log = entry / "output.log"
            if output_log.is_file():
                tasks.append({"task_id": entry.name, "status": "running"})
    return tasks


def _read_active_todos(todo_store) -> list:
    """Read pending/in-progress todos from the shared TodoStore if available."""
    try:
        items = todo_store.read()
        return [
            {"id": t["id"], "content": t["content"], "status": t["status"]}
            for t in items
            if t["status"] in ("pending", "in_progress")
        ]
    except Exception:
        return []


def _handler(args, **kw):
    sections = []

    # --- Tasks ---
    tasks = _read_task_status()
    active_tasks = [t for t in tasks if t.get("status") in ("running", "pending")]
    if active_tasks:
        lines = [f"- {t['task_id']}: {t.get('description', t.get('status', 'unknown'))}" for t in active_tasks]
        sections.append("## Active Tasks\n" + "\n".join(lines))
    else:
        sections.append("## Active Tasks\n(none)")

    # --- Todos ---
    todo_store = kw.get("store")
    if todo_store:
        todos = _read_active_todos(todo_store)
        if todos:
            lines = [f"- {t['id']}. {t['content']} ({t['status']})" for t in todos]
            sections.append("## Active Todos\n" + "\n".join(lines))

    # --- Recent files (from file_state if available) ---
    try:
        from tools.file_state import file_state_store
        recent = list(file_state_store.list_recent(limit=10))
        if recent:
            lines = [f"- {entry['path']} (modified: {entry.get('mtime', '?')})" for entry in recent]
            sections.append("## Recently Modified Files\n" + "\n".join(lines))
    except Exception:
        pass

    summary = "\n\n".join(sections)
    return tool_result({"summary": summary, "task_count": len(active_tasks)})


registry.register(
    name="session_brief",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    emoji="📋",
)
