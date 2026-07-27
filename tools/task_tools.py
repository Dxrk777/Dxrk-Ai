# SPDX-License-Identifier: MIT
"""Task Tools Module — V2 Task System (Create/Get/List/Update/Stop).

Five structured tools for task management with blocking dependencies,
status transitions, and auto-verification nudges.

Design:
- In-memory TaskStore (one per session, shared state)
- Auto-increment IDs (no file I/O needed)
- Blocking dependencies (blocks / blockedBy with bidirectional sync)
- TaskCompleted hooks for verification nudge
- Soft delete preserves ID uniqueness
"""

import json
import threading
from typing import Dict, List, Any, Optional


TASK_STATUSES = ("pending", "in_progress", "completed")
TASK_VALID_STATUSES = (*TASK_STATUSES, "deleted")


class TaskStore:
    """In-memory task store. One instance per session.

    Thread-safe: public methods acquire _lock.
    """

    def __init__(self):
        self._lock = threading.Lock()
        self._tasks: Dict[str, Dict[str, Any]] = {}
        self._high_water = 0
        self._completion_count = 0

    def _next_id(self) -> str:
        self._high_water += 1
        return str(self._high_water)

    def create(self, subject: str, description: str = "",
               active_form: str = "", metadata: Dict = None) -> Dict[str, Any]:
        with self._lock:
            task_id = self._next_id()
            task = {
                "id": task_id,
                "subject": subject,
                "description": description,
                "active_form": active_form,
                "status": "pending",
                "blocks": [],
                "blocked_by": [],
                "owner": "",
                "metadata": metadata or {},
            }
            self._tasks[task_id] = task
            return dict(task)

    def get(self, task_id: str) -> Optional[Dict[str, Any]]:
        with self._lock:
            t = self._tasks.get(task_id)
            return dict(t) if t else None

    def list_tasks(self, include_internal: bool = False) -> List[Dict[str, Any]]:
        with self._lock:
            tasks = []
            for t in self._tasks.values():
                if not include_internal and t.get("metadata", {}).get("_internal"):
                    continue
                result = {
                    "id": t["id"],
                    "subject": t["subject"],
                    "status": t["status"],
                    "owner": t.get("owner", ""),
                    "blocked_by": [b for b in t.get("blocked_by", [])
                                   if self._tasks.get(b, {}).get("status") != "completed"],
                }
                tasks.append(result)
            tasks.sort(key=lambda x: int(x["id"]))
            return tasks

    def update(self, task_id: str, **fields) -> Dict[str, Any]:
        with self._lock:
            t = self._tasks.get(task_id)
            if not t:
                return {"success": False, "error": "Task not found"}

            updated = []
            status_change = None

            # Status transition
            if "status" in fields:
                new_status = fields["status"]
                if new_status == "deleted":
                    self._delete_task_internal(task_id)
                    return {"success": True, "task_id": task_id,
                            "updated_fields": ["status"], "deleted": True}
                if new_status in TASK_STATUSES:
                    old = t["status"]
                    if old != new_status:
                        t["status"] = new_status
                        updated.append("status")
                        status_change = {"from": old, "to": new_status}
                        if new_status == "completed":
                            self._completion_count += 1

            # Simple fields
            for field in ("subject", "description", "active_form", "owner"):
                if field in fields and fields[field] != t.get(field):
                    t[field] = fields[field]
                    updated.append(field)

            # Blocking deps (add-only)
            for bid in fields.get("add_blocks", []):
                bid = str(bid)
                if bid not in t["blocks"] and bid in self._tasks:
                    t["blocks"].append(bid)
                    if task_id not in self._tasks[bid].setdefault("blocked_by", []):
                        self._tasks[bid]["blocked_by"].append(task_id)
                    if "blocks" not in updated:
                        updated.append("blocks")

            for bid in fields.get("add_blocked_by", []):
                bid = str(bid)
                if bid not in t["blocked_by"] and bid in self._tasks:
                    t["blocked_by"].append(bid)
                    if task_id not in self._tasks[bid].setdefault("blocks", []):
                        self._tasks[bid]["blocks"].append(task_id)
                    if "blocked_by" not in updated:
                        updated.append("blocked_by")

            # Metadata merge
            if "metadata" in fields and isinstance(fields["metadata"], dict):
                for k, v in fields["metadata"].items():
                    if v is None:
                        t.setdefault("metadata", {}).pop(k, None)
                    else:
                        t.setdefault("metadata", {})[k] = v
                if "metadata" not in updated:
                    updated.append("metadata")

            result = {
                "success": True,
                "task_id": task_id,
                "updated_fields": updated,
            }
            if status_change:
                result["status_change"] = status_change
            if self._completion_count >= 3 and not self._verification_subject:
                result["verification_nudge_needed"] = True

            return result

    def _delete_task_internal(self, task_id: str):
        t = self._tasks.pop(task_id, None)
        if not t:
            return
        # Clean blocks/blockedBy references in other tasks
        for other in self._tasks.values():
            if task_id in other.get("blocks", []):
                other["blocks"] = [b for b in other["blocks"] if b != task_id]
            if task_id in other.get("blocked_by", []):
                other["blocked_by"] = [b for b in other["blocked_by"] if b != task_id]

    @property
    def _verification_subject(self) -> bool:
        return any("verif" in t.get("subject", "").lower()
                   for t in self._tasks.values()
                   if t.get("status") == "completed")

    def stop(self, task_id: str) -> Optional[Dict[str, Any]]:
        with self._lock:
            t = self._tasks.get(task_id)
            if not t:
                return None
            t["status"] = "completed"
            return {"task_id": task_id, "subject": t["subject"]}

    def get_compression_context(self) -> Optional[str]:
        with self._lock:
            active = [t for t in self._tasks.values()
                      if t["status"] in ("pending", "in_progress")]
            if not active:
                return None
            lines = ["[Active tasks preserved across context compression]"]
            for t in active:
                marker = {"completed": "[x]", "in_progress": "[>]",
                          "pending": "[ ]"}.get(t["status"], "[?]")
                lines.append(f"- {marker} #{t['id']} {t['subject']} ({t['status']})")
            return "\n".join(lines)


# --- Global store instance ---
_store: Optional[TaskStore] = None
_store_lock = threading.Lock()


def _get_store() -> TaskStore:
    global _store
    if _store is None:
        with _store_lock:
            if _store is None:
                _store = TaskStore()
    return _store


# =========================================================================
# Tool Handlers
# =========================================================================


def task_create_tool(subject: str, description: str = "",
                     active_form: str = "", metadata: Dict = None,
                     store: TaskStore = None) -> str:
    s = store or _get_store()
    task = s.create(subject, description, active_form, metadata)
    return json.dumps({
        "success": True,
        "task": {"id": task["id"], "subject": task["subject"]},
    }, ensure_ascii=False)


def task_get_tool(task_id: str, store: TaskStore = None) -> str:
    s = store or _get_store()
    task = s.get(task_id)
    if not task:
        return json.dumps({"success": True, "task": None}, ensure_ascii=False)
    return json.dumps({
        "success": True,
        "task": {
            "id": task["id"],
            "subject": task["subject"],
            "description": task["description"],
            "status": task["status"],
            "blocks": task.get("blocks", []),
            "blocked_by": task.get("blocked_by", []),
            "owner": task.get("owner", ""),
        },
    }, ensure_ascii=False)


def task_list_tool(store: TaskStore = None) -> str:
    s = store or _get_store()
    tasks = s.list_tasks()
    if not tasks:
        return json.dumps({"success": True, "tasks": [], "message": "No tasks found"},
                          ensure_ascii=False)
    return json.dumps({"success": True, "tasks": tasks}, ensure_ascii=False)


def task_update_tool(task_id: str, subject: str = None,
                     description: str = None, active_form: str = None,
                     status: str = None, add_blocks: List[str] = None,
                     add_blocked_by: List[str] = None,
                     owner: str = None, metadata: Dict = None,
                     store: TaskStore = None) -> str:
    s = store or _get_store()
    fields = {}
    if subject is not None:
        fields["subject"] = subject
    if description is not None:
        fields["description"] = description
    if active_form is not None:
        fields["active_form"] = active_form
    if status is not None:
        fields["status"] = status
    if add_blocks:
        fields["add_blocks"] = add_blocks
    if add_blocked_by:
        fields["add_blocked_by"] = add_blocked_by
    if owner is not None:
        fields["owner"] = owner
    if metadata is not None:
        fields["metadata"] = metadata

    result = s.update(task_id, **fields)
    return json.dumps(result, ensure_ascii=False)


def task_stop_tool(task_id: str, store: TaskStore = None) -> str:
    s = store or _get_store()
    result = s.stop(task_id)
    if not result:
        return json.dumps({"success": False, "error": "Task not found"},
                          ensure_ascii=False)
    return json.dumps({"success": True, "message": f"Task #{task_id} stopped",
                        "task_id": task_id, "subject": result["subject"]},
                      ensure_ascii=False)


def check_task_requirements() -> bool:
    return True


# =========================================================================
# Schemas + Registry
# =========================================================================

from tools.registry import registry, tool_error

TASK_CREATE_SCHEMA = {
    "name": "task_create",
    "description": "Create a new task with subject and description. Returns the auto-assigned task ID.",
    "parameters": {
        "type": "object",
        "properties": {
            "subject": {"type": "string", "description": "Brief task title"},
            "description": {"type": "string", "description": "What needs to be done"},
            "active_form": {"type": "string", "description": "Present-tense description for spinner (e.g. 'Running tests')"},
            "metadata": {"type": "object", "description": "Optional metadata key-value pairs"},
        },
        "required": ["subject"],
    },
}

TASK_GET_SCHEMA = {
    "name": "task_get",
    "description": "Get task details by ID. Returns status, description, and blocking dependencies.",
    "parameters": {
        "type": "object",
        "properties": {
            "task_id": {"type": "string", "description": "Task ID to retrieve"},
        },
        "required": ["task_id"],
    },
}

TASK_LIST_SCHEMA = {
    "name": "task_list",
    "description": "List all non-internal tasks with their status, owner, and blocking info. Call after completing a task to find the next one to work on.",
    "parameters": {
        "type": "object",
        "properties": {},
    },
}

TASK_UPDATE_SCHEMA = {
    "name": "task_update",
    "description": "Update a task's fields, status, or blocking dependencies. Supports soft delete via status='deleted'. Use add_blocks/add_blocked_by to declare dependencies.",
    "parameters": {
        "type": "object",
        "properties": {
            "task_id": {"type": "string", "description": "Task ID to update"},
            "subject": {"type": "string", "description": "New subject"},
            "description": {"type": "string", "description": "New description"},
            "active_form": {"type": "string", "description": "Spinner text"},
            "status": {"type": "string", "enum": ["pending", "in_progress", "completed", "deleted"],
                       "description": "New status"},
            "add_blocks": {"type": "array", "items": {"type": "string"},
                           "description": "Task IDs that THIS task blocks"},
            "add_blocked_by": {"type": "array", "items": {"type": "string"},
                               "description": "Task IDs that BLOCK this task"},
            "owner": {"type": "string", "description": "Who is working on this"},
            "metadata": {"type": "object", "description": "Metadata to merge. null removes a key."},
        },
        "required": ["task_id"],
    },
}

TASK_STOP_SCHEMA = {
    "name": "task_stop",
    "description": "Stop a running task. Marks it as completed.",
    "parameters": {
        "type": "object",
        "properties": {
            "task_id": {"type": "string", "description": "Task ID to stop"},
        },
        "required": ["task_id"],
    },
}


registry.register(
    name="task_create", toolset="task",
    schema=TASK_CREATE_SCHEMA,
    handler=lambda args, **kw: task_create_tool(
        subject=args["subject"],
        description=args.get("description", ""),
        active_form=args.get("active_form", ""),
        metadata=args.get("metadata"),
        store=kw.get("store")),
    check_fn=check_task_requirements,
    emoji="📌",
)

registry.register(
    name="task_get", toolset="task",
    schema=TASK_GET_SCHEMA,
    handler=lambda args, **kw: task_get_tool(task_id=args["task_id"], store=kw.get("store")),
    check_fn=check_task_requirements,
    emoji="🔍",
)

registry.register(
    name="task_list", toolset="task",
    schema=TASK_LIST_SCHEMA,
    handler=lambda args, **kw: task_list_tool(store=kw.get("store")),
    check_fn=check_task_requirements,
    emoji="📋",
)

registry.register(
    name="task_update", toolset="task",
    schema=TASK_UPDATE_SCHEMA,
    handler=lambda args, **kw: task_update_tool(
        task_id=args["task_id"],
        subject=args.get("subject"),
        description=args.get("description"),
        active_form=args.get("active_form"),
        status=args.get("status"),
        add_blocks=args.get("add_blocks"),
        add_blocked_by=args.get("add_blocked_by"),
        owner=args.get("owner"),
        metadata=args.get("metadata"),
        store=kw.get("store")),
    check_fn=check_task_requirements,
    emoji="✏️",
)

registry.register(
    name="task_stop", toolset="task",
    schema=TASK_STOP_SCHEMA,
    handler=lambda args, **kw: task_stop_tool(
        task_id=args["task_id"],
        store=kw.get("store")),
    check_fn=check_task_requirements,
    emoji="⏹️",
)
