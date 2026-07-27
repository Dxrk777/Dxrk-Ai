# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
MCP Hub — Dxrk-native MCP server infrastructure.

Ports the architecture patterns from Claude Code's MCP config + client:
  - Multi-scope config loading (user ~/.dxrk/mcp.json, project .mcp.json)
  - Merge strategy: user overrides project
  - Environment variable expansion (${VAR} and ${VAR:-default})
  - Connection management with reconnection + exponential backoff
  - Connection pooling with concurrency limits

Tool: mcp_hub — list, restart, or status of MCP servers.
"""

import asyncio
import json
import logging
import os
import re
import signal
import threading
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Optional

import httpx

from tools.registry import registry, tool_ok, tool_fail

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
USER_MCP_PATH = DXRK_HOME / "mcp.json"
PROJECT_MCP_NAME = ".mcp.json"

CONNECT_TIMEOUT = 30
TOOL_TIMEOUT = 120
MAX_RECONNECT_RETRIES = 5
INITIAL_BACKOFF = 1.0
MAX_BACKOFF = 60.0
LOCAL_CONCURRENCY = 3
REMOTE_CONCURRENCY = 20

ENV_VAR_RE = re.compile(r"\$\{([^}]+)\}")

# ---------------------------------------------------------------------------
# Types
# ---------------------------------------------------------------------------

ServerType = Optional[str]  # None=stdio, 'sse', 'http', 'ws'


@dataclass
class ServerDef:
    name: str
    scope: str          # "user" | "project"
    type: ServerType
    command: Optional[str] = None
    args: List[str] = field(default_factory=list)
    env: Dict[str, str] = field(default_factory=dict)
    url: Optional[str] = None
    headers: Dict[str, str] = field(default_factory=dict)


@dataclass
class MCPServerHandle:
    name: str
    config: ServerDef
    process: Optional[asyncio.subprocess.Process] = None
    client: Optional[httpx.AsyncClient] = None
    task: Optional[asyncio.Task] = None
    status: str = "disconnected"
    error: Optional[str] = None
    connected_at: Optional[float] = None
    reconnect_count: int = 0


# ---------------------------------------------------------------------------
# Env var expansion (mirrors Claude Code's expandEnvVarsInString)
# ---------------------------------------------------------------------------

def _expand_env_vars(value: str) -> str:
    def _replace(m: re.Match) -> str:
        content = m.group(1)
        parts = content.split(":-", 1)
        var_name = parts[0]
        default = parts[1] if len(parts) > 1 else None
        env_val = os.environ.get(var_name)
        if env_val is not None:
            return env_val
        if default is not None:
            return default
        return m.group(0)

    return ENV_VAR_RE.sub(_replace, value)


def _expand_config(config: dict) -> dict:
    expanded = {}
    for key, val in config.items():
        if isinstance(val, str):
            expanded[key] = _expand_env_vars(val)
        elif isinstance(val, dict):
            expanded[key] = {k: _expand_env_vars(v) if isinstance(v, str) else v
                            for k, v in val.items()}
        elif isinstance(val, list):
            expanded[key] = [_expand_env_vars(v) if isinstance(v, str) else v
                            for v in val]
        else:
            expanded[key] = val
    return expanded


# ---------------------------------------------------------------------------
# Config Loader (mirrors Claude Code's parseMcpConfig / parseMcpConfigFromFilePath)
# ---------------------------------------------------------------------------

def _load_mcp_file(path: Path, scope: str) -> Dict[str, ServerDef]:
    if not path.exists():
        return {}

    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        logger.warning("MCP config parse error %s: %s", path, e)
        return {}

    servers_raw = raw.get("mcpServers") or {}
    result: Dict[str, ServerDef] = {}

    for name, cfg in servers_raw.items():
        cfg = _expand_config(cfg)
        stype = cfg.get("type")
        sd = ServerDef(
            name=name,
            scope=scope,
            type=stype,
            command=cfg.get("command"),
            args=cfg.get("args", []),
            env=cfg.get("env", {}),
            url=cfg.get("url"),
            headers=cfg.get("headers", {}),
        )
        result[name] = sd

    return result


def _find_project_mcp() -> Dict[str, ServerDef]:
    servers: Dict[str, ServerDef] = {}
    current = Path.cwd()

    # Walk up from CWD to root, closest file wins (overrides parent)
    dirs: List[Path] = []
    while True:
        dirs.append(current)
        if current.parent == current:
            break
        current = current.parent

    for d in reversed(dirs):
        candidate = d / PROJECT_MCP_NAME
        if not candidate.exists():
            continue
        found = _load_mcp_file(candidate, "project")
        # Closer dirs override — merge into existing
        for name, sd in found.items():
            servers[name] = sd

    return servers


def load_all_servers() -> Dict[str, ServerDef]:
    user_servers = _load_mcp_file(USER_MCP_PATH, "user")
    project_servers = _find_project_mcp()

    # Merge: user overrides project (mirrors Claude Code's precedence:
    # plugin < user < project < local — here user > project)
    merged = {}
    merged.update(project_servers)
    merged.update(user_servers)
    return merged


# ---------------------------------------------------------------------------
# Connection Manager
# ---------------------------------------------------------------------------

class MCPServerManager:
    """Manages MCP server connections with reconnection + backoff."""

    def __init__(self):
        self._handles: Dict[str, MCPServerHandle] = {}
        self._lock = asyncio.Lock()
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._http_limiter: Optional[asyncio.Semaphore] = None

    # -- Public API ---------------------------------------------------------

    async def start(self):
        self._loop = asyncio.get_running_loop()
        self._http_limiter = asyncio.Semaphore(REMOTE_CONCURRENCY)

    async def connect_all(self, servers: Dict[str, ServerDef]) -> List[MCPServerHandle]:
        handles: List[MCPServerHandle] = []
        async with self._lock:
            for name, config in servers.items():
                if name not in self._handles:
                    self._handles[name] = MCPServerHandle(name=name, config=config)
                handles.append(self._handles[name])

        local_tasks = []
        remote_tasks = []
        for h in handles:
            if h.config.type in (None, "stdio"):
                local_tasks.append(self._connect_one(h))
            else:
                remote_tasks.append(self._connect_one(h))

        results = []
        if local_tasks:
            results.extend(await asyncio.gather(*local_tasks, return_exceptions=True))
        if remote_tasks:
            results.extend(await asyncio.gather(*remote_tasks, return_exceptions=True))
        return handles

    async def connect_one(self, name: str) -> Optional[MCPServerHandle]:
        async with self._lock:
            handle = self._handles.get(name)
        if not handle:
            return None
        await self._connect_one(handle)
        return handle

    async def disconnect_all(self):
        async with self._lock:
            names = list(self._handles.keys())
        for name in names:
            await self.disconnect(name)

    async def disconnect(self, name: str):
        async with self._lock:
            handle = self._handles.pop(name, None)
        if handle is None:
            return
        await self._stop_handle(handle)

    def get_handle(self, name: str) -> Optional[MCPServerHandle]:
        return self._handles.get(name)

    def get_all_handles(self) -> List[MCPServerHandle]:
        return list(self._handles.values())

    def get_status_summary(self) -> dict:
        handles = self.get_all_handles()
        summary = {
            "total": len(handles),
            "connected": 0,
            "connecting": 0,
            "disconnected": 0,
            "failed": 0,
            "servers": [],
        }
        for h in handles:
            status_key = h.status
            if status_key in summary:
                summary[status_key] += 1
            summary["servers"].append({
                "name": h.name,
                "scope": h.config.scope,
                "type": h.config.type or "stdio",
                "status": h.status,
                "error": h.error,
                "connected_at": h.connected_at,
                "reconnect_count": h.reconnect_count,
            })
        return summary

    # -- Internal -----------------------------------------------------------

    async def _connect_one(self, handle: MCPServerHandle):
        if handle.status == "connected":
            return

        handle.status = "connecting"
        handle.error = None

        try:
            if handle.config.type in (None, "stdio"):
                await self._connect_stdio(handle)
            elif handle.config.type == "sse":
                await self._connect_sse(handle)
            elif handle.config.type == "http":
                await self._connect_http(handle)
            elif handle.config.type == "ws":
                await self._connect_ws(handle)
            else:
                raise ValueError(f"Unsupported server type: {handle.config.type}")

            handle.status = "connected"
            handle.connected_at = time.time()
            handle.reconnect_count = 0
            logger.info("MCP server '%s' connected (%s)", handle.name, handle.config.type or "stdio")

        except Exception as e:
            handle.status = "failed"
            handle.error = str(e)
            logger.warning("MCP server '%s' connection failed: %s", handle.name, e)
            self._schedule_reconnect(handle)

    async def _connect_stdio(self, handle: MCPServerHandle):
        cmd = handle.config.command
        if not cmd:
            raise ValueError(f"Server '{handle.name}' has no command")

        env = os.environ.copy()
        env.update(handle.config.env)

        handle.process = await asyncio.create_subprocess_exec(
            cmd,
            *handle.config.args,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    async def _connect_sse(self, handle: MCPServerHandle):
        url = handle.config.url
        if not url:
            raise ValueError(f"SSE server '{handle.name}' has no url")

        async with self._http_limiter:
            client = httpx.AsyncClient(
                headers=handle.config.headers,
                timeout=httpx.Timeout(CONNECT_TIMEOUT, read=None),
            )
            resp = await client.get(url)
            resp.raise_for_status()
            handle.client = client

    async def _connect_http(self, handle: MCPServerHandle):
        url = handle.config.url
        if not url:
            raise ValueError(f"HTTP server '{handle.name}' has no url")

        async with self._http_limiter:
            client = httpx.AsyncClient(
                headers=handle.config.headers,
                timeout=httpx.Timeout(CONNECT_TIMEOUT, read=TOOL_TIMEOUT),
            )
            resp = await client.post(url, json={"jsonrpc": "2.0", "method": "initialize", "id": 1})
            resp.raise_for_status()
            handle.client = client

    async def _connect_ws(self, handle: MCPServerHandle):
        url = handle.config.url
        if not url:
            raise ValueError(f"WebSocket server '{handle.name}' has no url")
        try:
            import websockets
        except ImportError:
            raise ImportError("websockets package required for WS transport: pip install websockets")

        async with self._http_limiter:
            ws = await websockets.connect(url, extra_headers=handle.config.headers, open_timeout=CONNECT_TIMEOUT)
        handle._ws = ws
        handle.client = httpx.AsyncClient()

    async def _stop_handle(self, handle: MCPServerHandle):
        proc = handle.process
        if proc and proc.returncode is None:
            pid = proc.pid
            try:
                os.kill(pid, signal.SIGINT)
                try:
                    await asyncio.wait_for(proc.wait(), timeout=0.5)
                except asyncio.TimeoutError:
                    os.kill(pid, signal.SIGTERM)
                    try:
                        await asyncio.wait_for(proc.wait(), timeout=0.5)
                    except asyncio.TimeoutError:
                        os.kill(pid, signal.SIGKILL)
                        await proc.wait()
            except ProcessLookupError:
                pass

        client = handle.client
        if client and not client.is_closed:
            await client.aclose()

        ws = getattr(handle, "_ws", None)
        if ws is not None:
            await ws.close()

        if handle.task and not handle.task.done():
            handle.task.cancel()
            try:
                await handle.task
            except (asyncio.CancelledError, Exception):
                pass

    def _schedule_reconnect(self, handle: MCPServerHandle):
        if handle.reconnect_count >= MAX_RECONNECT_RETRIES:
            logger.warning("MCP server '%s': max reconnect retries reached", handle.name)
            return

        backoff = min(INITIAL_BACKOFF * (2 ** handle.reconnect_count), MAX_BACKOFF)
        handle.reconnect_count += 1

        loop = self._loop or asyncio.get_event_loop()
        handle.task = loop.create_task(self._reconnect_after(handle, backoff))

    async def _reconnect_after(self, handle: MCPServerHandle, delay: float):
        await asyncio.sleep(delay)
        logger.info("MCP server '%s': reconnecting (attempt %d/%d)",
                     handle.name, handle.reconnect_count, MAX_RECONNECT_RETRIES)
        await self._connect_one(handle)


_manager: Optional[MCPServerManager] = None
_manager_lock = threading.Lock()


def _get_manager() -> MCPServerManager:
    global _manager
    with _manager_lock:
        if _manager is None:
            _manager = MCPServerManager()
        return _manager


# ---------------------------------------------------------------------------
# Tool handler
# ---------------------------------------------------------------------------

def _mcp_hub_handler(args: dict, **kw) -> str:
    action = args.get("action", "list")
    server_name = args.get("server_name")

    manager = _get_manager()

    try:
        loop = asyncio.get_running_loop()
    except RuntimeError:
        loop = None

    if action == "list":
        servers = load_all_servers()
        summary = manager.get_status_summary()
        return tool_ok({
            "configured_servers": {n: {"scope": s.scope, "type": s.type or "stdio"}
                                  for n, s in servers.items()},
            "connections": summary,
        }).to_json()

    elif action == "status":
        summary = manager.get_status_summary()
        return tool_ok(summary).to_json()

    elif action == "restart":
        if not server_name:
            return tool_fail("server_name required for restart").to_json()

        handle = manager.get_handle(server_name)
        if not handle:
            return tool_fail(f"Server '{server_name}' not found").to_json()

        if loop and loop.is_running():
            fut = asyncio.run_coroutine_threadsafe(
                _do_restart(manager, server_name), loop
            )
            try:
                result = fut.result(timeout=CONNECT_TIMEOUT + 5)
            except Exception as e:
                return tool_fail(f"Restart failed: {e}").to_json()
        else:
            result = asyncio.run(_do_restart(manager, server_name))

        return tool_ok(result).to_json()

    else:
        return tool_fail(f"Unknown action: {action}").to_json()


async def _do_restart(manager: MCPServerManager, name: str) -> dict:
    servers = load_all_servers()
    config = servers.get(name)
    if not config:
        return {"error": f"Server '{name}' not found in config"}

    await manager.disconnect(name)

    async with manager._lock:
        manager._handles[name] = MCPServerHandle(name=name, config=config)

    await manager.connect_one(name)
    handle = manager.get_handle(name)
    return {
        "name": name,
        "status": handle.status if handle else "unknown",
        "error": handle.error if handle else None,
    }


SCHEMA = {
    "name": "mcp_hub",
    "description": (
        "Manage MCP (Model Context Protocol) server connections. "
        "List configured servers, check connection status, or restart a server."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["list", "status", "restart"],
                "description": "'list' shows all configured servers + connection state, "
                               "'status' shows current connections, "
                               "'restart' disconnects and reconnects a specific server",
            },
            "server_name": {
                "type": "string",
                "description": "Server name to restart (required for action=restart)",
            },
        },
        "required": ["action"],
    },
}

registry.register(
    name="mcp_hub",
    toolset="mcp",
    schema=SCHEMA,
    handler=_mcp_hub_handler,
    emoji="🔌",
    is_destructive=False,
    is_concurrency_safe=True,
    is_readonly=True,
)
