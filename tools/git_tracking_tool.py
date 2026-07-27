# SPDX-License-Identifier: MIT
import json
import re

from tools.registry import registry, tool_error, tool_result


def _git_cmd_re(subcmd, suffix=""):
    return re.compile(
        rf'\bgit(?:\s+-[cC]\s+\S+|\s+--\S+=\S+)*\s+{subcmd}\b{suffix}'
    )


GIT_COMMIT_RE = _git_cmd_re("commit")
GIT_PUSH_RE = _git_cmd_re("push")
GIT_CHERRY_PICK_RE = _git_cmd_re("cherry-pick")
GIT_MERGE_RE = _git_cmd_re("merge", "(?!-)")
GIT_REBASE_RE = _git_cmd_re("rebase")

GH_PR_ACTIONS = [
    (re.compile(r"\bgh\s+pr\s+create\b"), "created"),
    (re.compile(r"\bgh\s+pr\s+edit\b"), "edited"),
    (re.compile(r"\bgh\s+pr\s+merge\b"), "merged"),
    (re.compile(r"\bgh\s+pr\s+comment\b"), "commented"),
    (re.compile(r"\bgh\s+pr\s+close\b"), "closed"),
    (re.compile(r"\bgh\s+pr\s+ready\b"), "ready"),
]

GLAB_MR_CREATE_RE = re.compile(r"\bglab\s+mr\s+create\b")
COMMIT_MSG_RE = re.compile(r"-m\s+([\"'])(.*?)\1", re.DOTALL)


def _extract_commit_message(command):
    match = COMMIT_MSG_RE.search(command)
    return match.group(2) if match else None


def _extract_push_target(command):
    after = re.split(r"\bgit\s+push\b", command, maxsplit=1)
    if len(after) < 2:
        return None
    tokens = after[1].strip().split()
    args = [t for t in tokens if not t.startswith("-")]
    if len(args) >= 2:
        return args[-1]
    return None


def _extract_ref(command, verb):
    after = re.split(rf"\bgit\s+{re.escape(verb)}\b", command, maxsplit=1)
    if len(after) < 2:
        return None
    for t in after[1].strip().split():
        if re.match(r"^[&|;><]", t):
            break
        if t.startswith("-"):
            continue
        return t
    return None


SCHEMA = {
    "name": "track_git_operation",
    "description": "Analyze a bash command string for git operations (commit, push, merge, rebase, gh pr, glab mr). Returns structured info about detected operations.",
    "parameters": {
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "description": "The bash command to analyze for git operations.",
            },
            "auto_detect": {
                "type": "boolean",
                "description": "Auto-detect git operations from command.",
                "default": True,
            },
        },
        "required": ["command"],
    },
}


def _handler(args, **kw):
    command = args.get("command", "").strip()
    if not command:
        return tool_error("'command' is required")

    auto_detect = args.get("auto_detect", True)
    if not auto_detect:
        return tool_result(command=command, operations=[])

    operations = []

    commit_match = GIT_COMMIT_RE.search(command)
    cherry_pick_match = GIT_CHERRY_PICK_RE.search(command)
    if commit_match or cherry_pick_match:
        message = _extract_commit_message(command)
        is_cherry_pick = bool(cherry_pick_match)
        is_amended = bool(re.search(r"--amend\b", command))
        if is_cherry_pick:
            kind = "cherry-picked"
        elif is_amended:
            kind = "amended"
        else:
            kind = "committed"
        op = {"type": "commit", "kind": kind}
        if message:
            op["message"] = message
        operations.append(op)

    if GIT_PUSH_RE.search(command):
        branch = _extract_push_target(command)
        op = {"type": "push"}
        if branch:
            op["branch"] = branch
        operations.append(op)

    if GIT_MERGE_RE.search(command):
        ref = _extract_ref(command, "merge")
        op = {"type": "merge"}
        if ref:
            op["ref"] = ref
        operations.append(op)

    if GIT_REBASE_RE.search(command):
        ref = _extract_ref(command, "rebase")
        op = {"type": "rebase"}
        if ref:
            op["ref"] = ref
        operations.append(op)

    for pattern, action in GH_PR_ACTIONS:
        if pattern.search(command):
            operations.append({"type": "pr", "action": action})

    if GLAB_MR_CREATE_RE.search(command):
        operations.append({"type": "mr", "action": "created"})

    return tool_result(
        command=command,
        has_operations=len(operations) > 0,
        operations=operations,
    )


registry.register(
    name="track_git_operation",
    toolset="dev",
    schema=SCHEMA,
    handler=_handler,
    emoji="\U0001f500",
)
