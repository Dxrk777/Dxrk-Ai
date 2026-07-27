# SPDX-License-Identifier: MIT
import asyncio
import json
import os
import signal

from tools.registry import registry, tool_error

_REPL_SESSIONS: dict[str, "ReplSession"] = {}


class ReplSession:
    def __init__(self, session_id: str, language: str):
        self.session_id = session_id
        self.language = language
        self.process: asyncio.subprocess.Process | None = None
        self._buffer = ""

    async def start(self):
        if self.language == "python":
            cmd = ["python3", "-q", "-u"]
        elif self.language == "node":
            cmd = ["node", "-e", "require('repl').start({prompt: '', useColors: false, ignoreUndefined: true})"]
        elif self.language == "bash":
            cmd = ["bash", "--norc", "--noprofile"]
        else:
            raise ValueError(f"Unsupported language: {self.language}")

        self.process = await asyncio.create_subprocess_exec(
            *cmd,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
            env={**os.environ, "TERM": "dumb", "PAGER": "cat"},
        )

    async def execute(self, code: str) -> str:
        if not self.process:
            return tool_error("REPL session not started")
        if self.process.stdin.is_closing():
            return tool_error("REPL session is closed")
        self.process.stdin.write((code + "\n").encode())
        await self.process.stdin.drain()
        await asyncio.sleep(0.2)
        try:
            data = await asyncio.wait_for(self.process.stdout.read(65536), timeout=5.0)
        except asyncio.TimeoutError:
            data = b"(output truncated: still running after 5s)"
        result = data.decode("utf-8", errors="replace")
        self._buffer += result
        return self._buffer

    async def stop(self):
        if self.process:
            self.process.send_signal(signal.SIGTERM)
            try:
                await asyncio.wait_for(self.process.wait(), timeout=3.0)
            except asyncio.TimeoutError:
                self.process.kill()
                await self.process.wait()
            self.process = None
        self._buffer = ""


async def _repl_start(args, **kw):
    lang = (args.get("language") or "").strip().lower()
    if lang not in ("python", "node", "bash"):
        return tool_error(f"Unsupported language '{lang}'. Use python, node, or bash.")
    sid = args.get("session_id") or f"repl-{id(lang)}"
    if sid in _REPL_SESSIONS:
        return tool_error(f"Session '{sid}' already exists. Stop it first.")
    session = ReplSession(sid, lang)
    try:
        await session.start()
    except Exception as e:
        return tool_error(f"Failed to start REPL: {e}")
    _REPL_SESSIONS[sid] = session
    return json.dumps({"result": f"REPL session '{sid}' started for {lang}", "session_id": sid})


async def _repl_execute(args, **kw):
    sid = args.get("session_id", "")
    session = _REPL_SESSIONS.get(sid)
    if not session:
        return tool_error(f"No REPL session '{sid}'. Start one with repl_start first.")
    code = args.get("code", "")
    if not code:
        return tool_error("code is required")
    try:
        output = await session.execute(code)
    except Exception as e:
        return tool_error(f"REPL execution error: {e}")
    return json.dumps({"result": output, "session_id": sid})


async def _repl_stop(args, **kw):
    sid = args.get("session_id", "")
    session = _REPL_SESSIONS.pop(sid, None)
    if not session:
        return tool_error(f"No active REPL session '{sid}'")
    await session.stop()
    return json.dumps({"result": f"REPL session '{sid}' stopped"})


SCHEMA_START = {
    "name": "repl_start",
    "description": "Start an interactive REPL session (python, node, or bash). Returns a session_id for use with repl_execute and repl_stop.",
    "parameters": {
        "type": "object",
        "properties": {
            "language": {
                "type": "string",
                "enum": ["python", "node", "bash"],
                "description": "REPL language",
            },
            "session_id": {
                "type": "string",
                "description": "Optional session identifier",
            },
        },
        "required": ["language"],
    },
}
SCHEMA_EXEC = {
    "name": "repl_execute",
    "description": "Execute code in an active REPL session. Returns accumulated output.",
    "parameters": {
        "type": "object",
        "properties": {
            "session_id": {"type": "string", "description": "Session ID from repl_start"},
            "code": {"type": "string", "description": "Code to execute"},
        },
        "required": ["session_id", "code"],
    },
}
SCHEMA_STOP = {
    "name": "repl_stop",
    "description": "Stop and clean up an active REPL session.",
    "parameters": {
        "type": "object",
        "properties": {
            "session_id": {"type": "string", "description": "Session ID from repl_start"},
        },
        "required": ["session_id"],
    },
}

registry.register(name="repl_start", toolset="dev", schema=SCHEMA_START, handler=_repl_start, is_async=True, emoji="🔁")
registry.register(name="repl_execute", toolset="dev", schema=SCHEMA_EXEC, handler=_repl_execute, is_async=True, emoji="▶️")
registry.register(name="repl_stop", toolset="dev", schema=SCHEMA_STOP, handler=_repl_stop, is_async=True, emoji="⏹️")
