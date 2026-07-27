# SPDX-License-Identifier: MIT
import asyncio
import json

from tools.registry import registry, tool_error

SCHEMA = {
    "name": "sleep",
    "description": "Wait for a specified duration. Use when the user tells you to sleep or rest, when you have nothing to do, or when you're waiting for something.",
    "parameters": {
        "type": "object",
        "properties": {
            "duration_seconds": {
                "type": "number",
                "description": "Number of seconds to sleep for. Can be fractional (e.g. 2.5).",
            }
        },
        "required": ["duration_seconds"],
    },
}


async def _handler(args, **kw):
    duration = args.get("duration_seconds", 0)
    if duration <= 0:
        return tool_error("duration_seconds must be positive")
    await asyncio.sleep(duration)
    return json.dumps({"result": f"Slept for {duration} seconds"})


registry.register(
    name="sleep",
    toolset="dev",
    schema=SCHEMA,
    handler=_handler,
    is_async=True,
    emoji="💤",
)
