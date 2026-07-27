# SPDX-License-Identifier: MIT
import json
import subprocess
import sys

from tools.registry import registry, tool_error, tool_result

SCHEMA = {
    "name": "run_powershell",
    "description": "Execute a PowerShell command on Windows. Runs powershell.exe with -Command. On non-Windows systems, returns an error.",
    "parameters": {
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "description": "The PowerShell command or script to execute.",
            },
            "description": {
                "type": "string",
                "description": "Clear description of what the command does.",
            },
            "timeout": {
                "type": "integer",
                "description": "Timeout in seconds for the command execution.",
                "default": 30,
            },
        },
        "required": ["command"],
    },
}


def _handler(args, **kw):
    if sys.platform != "win32":
        return tool_error("PowerShell is only available on Windows")

    command = args.get("command", "").strip()
    if not command:
        return tool_error("'command' is required")

    timeout = args.get("timeout", 30) or 30

    try:
        result = subprocess.run(
            ["powershell.exe", "-Command", command],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return tool_result(
            stdout=result.stdout,
            stderr=result.stderr,
            returncode=result.returncode,
        )
    except subprocess.TimeoutExpired:
        return tool_error(f"PowerShell command timed out after {timeout}s")
    except FileNotFoundError:
        return tool_error("powershell.exe not found. Is PowerShell installed?")
    except Exception as e:
        return tool_error(str(e))


registry.register(
    name="run_powershell",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    emoji="\U0001f4bb",
)
