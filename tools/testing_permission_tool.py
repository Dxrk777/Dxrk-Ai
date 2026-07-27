# SPDX-License-Identifier: MIT
"""Testing Permission tool -- simulate a permission check for any tool+input without executing it."""

import json
import logging
from tools.registry import registry, tool_error, tool_result
from tools.permission_system import PermissionBehavior

logger = logging.getLogger(__name__)

TEST_PERMISSION_SCHEMA = {
    "name": "test_permission",
    "description": (
        "Test the permission system against a specific tool+input combination "
        "without actually executing the tool. Returns the permission decision "
        "(allow/deny/ask) and which rule matched."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "tool_name": {
                "type": "string",
                "description": "Name of the tool to test permission for.",
            },
            "input": {
                "type": "object",
                "description": "The input arguments to test the permission decision against.",
                "properties": {},
                "additionalProperties": True,
            },
        },
        "required": ["tool_name", "input"],
    },
}


def test_permission_handler(args, **kw):
    tool_name = args.get("tool_name")
    inp = args.get("input", {})

    if not tool_name:
        return tool_error("Missing required parameter 'tool_name'")
    if not isinstance(inp, dict):
        return tool_error("'input' must be an object (dict)")

    try:
        from tools.permission_system import PermissionManager
        manager = PermissionManager()
        result = manager.check_tool(tool_name, inp)
    except Exception as exc:
        return tool_error(f"Permission system error: {exc}")

    behavior_str = result.behavior.value if hasattr(result.behavior, "value") else str(result.behavior)

    response = {
        "tool_name": tool_name,
        "decision": behavior_str,
        "matched_rule": result.decision_reason or "no matching rule",
    }
    if result.message:
        response["message"] = result.message
    if result.suggestions:
        response["suggestions"] = result.suggestions

    return tool_result(response)


registry.register(
    name="test_permission",
    toolset="system",
    schema=TEST_PERMISSION_SCHEMA,
    handler=test_permission_handler,
    emoji="🔒",
)
