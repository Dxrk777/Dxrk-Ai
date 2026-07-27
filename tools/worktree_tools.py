# SPDX-License-Identifier: MIT
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path

from tools.registry import registry, tool_error

_VALID_BRANCH_RE = re.compile(r"^[a-zA-Z0-9._/-]+$")
_MAX_BRANCH_LENGTH = 64


def _git(*args: str, cwd: str | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git"] + list(args),
        capture_output=True,
        text=True,
        cwd=cwd,
    )


def _find_git_root(path: str | None = None) -> str | None:
    start = path or os.getcwd()
    r = _git("rev-parse", "--show-toplevel", cwd=start)
    if r.returncode != 0:
        return None
    return r.stdout.strip()


def _get_current_branch(path: str) -> str | None:
    r = _git("rev-parse", "--abbrev-ref", "HEAD", cwd=path)
    if r.returncode != 0:
        return None
    return r.stdout.strip()


def _validate_branch_name(branch: str) -> str | None:
    if len(branch) > _MAX_BRANCH_LENGTH:
        return f"Branch name exceeds {_MAX_BRANCH_LENGTH} characters"
    if not _VALID_BRANCH_RE.match(branch):
        return "Branch name may only contain letters, digits, dots, underscores, dashes, and forward slashes"
    for segment in branch.split("/"):
        if segment in (".", ".."):
            return "Branch name must not contain '.' or '..' path segments"
    return None


def _check_dirty(repo_path: str) -> bool:
    r = _git("status", "--porcelain", cwd=repo_path)
    return bool(r.stdout.strip())


SCHEMA_ENTER = {
    "name": "enter_worktree",
    "description": "Create a git worktree for isolated parallel work and switch into it. Creates a new branch in a fresh worktree based on the current HEAD (or base_branch if specified). Cannot run inside an existing worktree.",
    "parameters": {
        "type": "object",
        "properties": {
            "branch": {
                "type": "string",
                "description": "Name for the new worktree branch. Letters, digits, dots, underscores, dashes, and forward slashes only.",
            },
            "base_branch": {
                "type": "string",
                "description": "Optional branch/tag/commit to base the worktree on. Defaults to current HEAD.",
            },
            "path": {
                "type": "string",
                "description": "Optional path for the worktree. Defaults to ../<branch> relative to the repo root.",
            },
        },
        "required": ["branch"],
    },
}


def _enter_handler(args, **kw):
    branch = (args.get("branch") or "").strip()
    if not branch:
        return tool_error("branch is required")

    err = _validate_branch_name(branch)
    if err:
        return tool_error(f"Invalid branch name: {err}")

    current_dir = os.getcwd()
    repo_root = _find_git_root(current_dir)
    if not repo_root:
        return tool_error("Not in a git repository")

    if _check_dirty(repo_root):
        return tool_error(
            "Repository has uncommitted changes. Commit, stash, or discard before creating a worktree."
        )

    base_branch = (args.get("base_branch") or "").strip() or "HEAD"
    worktree_path = (args.get("path") or "").strip() or str(Path(repo_root).parent / branch)

    existing = _git("worktree", "list", cwd=repo_root)
    if branch in existing.stdout:
        return tool_error(f"Branch '{branch}' already exists. Use a different name.")

    r = _git("worktree", "add", "-b", branch, worktree_path, base_branch, cwd=repo_root)
    if r.returncode != 0:
        return tool_error(f"Failed to create worktree: {r.stderr.strip()}")

    return json.dumps({
        "result": "Worktree created",
        "worktree_path": worktree_path,
        "worktree_branch": branch,
        "base": base_branch,
        "message": f"Created worktree at {worktree_path} on branch {branch}. "
        f"The session is now working in the worktree.",
    })


SCHEMA_EXIT = {
    "name": "exit_worktree",
    "description": "Exit and optionally remove a git worktree. Use 'keep' to leave it on disk, 'remove' to delete it. When removing, provide discard_changes=true if the worktree has uncommitted work.",
    "parameters": {
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "Path to the worktree to exit/remove.",
            },
            "action": {
                "type": "string",
                "enum": ["keep", "remove"],
                "description": "'keep' leaves the worktree on disk; 'remove' deletes it.",
            },
            "discard_changes": {
                "type": "boolean",
                "description": "Required true when action is 'remove' and the worktree has uncommitted changes or unpushed commits.",
            },
        },
        "required": ["path", "action"],
    },
}


def _exit_handler(args, **kw):
    path = (args.get("path") or "").strip()
    if not path:
        return tool_error("path is required")

    if not os.path.isdir(path):
        return tool_error(f"Worktree path does not exist: {path}")

    action = (args.get("action") or "").strip().lower()
    if action not in ("keep", "remove"):
        return tool_error("action must be 'keep' or 'remove'")

    repo_root = _find_git_root(path)
    if not repo_root:
        return tool_error(f"Not a git repository: {path}")

    branch = _get_current_branch(path)

    if action == "remove":
        dirty = _check_dirty(path)

        r = _git("rev-list", "--count", "HEAD", "--not", "--remotes", cwd=path)
        unpushed = 0
        if r.returncode == 0:
            unpushed = int(r.stdout.strip() or "0")

        has_changes = dirty or unpushed > 0
        discard = args.get("discard_changes", False)

        if has_changes and not discard:
            parts = []
            if dirty:
                parts.append("uncommitted changes")
            if unpushed > 0:
                parts.append(f"{unpushed} unpushed commit(s)")
            return tool_error(
                f"Worktree has {' and '.join(parts)}. "
                "Set discard_changes=true to discard, or use action='keep' to preserve."
            )

        r = _git("worktree", "remove", path, cwd=repo_root)
        if r.returncode != 0:
            r = _git("worktree", "remove", "--force", path, cwd=repo_root)
            if r.returncode != 0:
                return tool_error(f"Failed to remove worktree: {r.stderr.strip()}")

        r = _git("branch", "-D", branch, cwd=repo_root)
        _  # best-effort

        return json.dumps({
            "result": "Worktree removed",
            "action": "remove",
            "worktree_path": path,
            "worktree_branch": branch,
            "discarded_changes": dirty,
            "discarded_commits": unpushed,
            "message": f"Removed worktree at {path}.",
        })

    return json.dumps({
        "result": "Worktree kept",
        "action": "keep",
        "worktree_path": path,
        "worktree_branch": branch,
        "message": f"Exited worktree. Work preserved at {path} on branch {branch}.",
    })


registry.register(
    name="enter_worktree",
    toolset="dev",
    schema=SCHEMA_ENTER,
    handler=_enter_handler,
    emoji="🌳",
)
registry.register(
    name="exit_worktree",
    toolset="dev",
    schema=SCHEMA_EXIT,
    handler=_exit_handler,
    emoji="🏁",
)
