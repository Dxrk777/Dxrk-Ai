# SPDX-License-Identifier: MIT
"""MCP Resource tools -- list and read resources across all connected MCP servers."""

import json
import logging
from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

LIST_MCP_RESOURCES_SCHEMA = {
    "name": "list_mcp_resources",
    "description": "List available resources from all connected MCP servers, optionally filtered by server name.",
    "parameters": {
        "type": "object",
        "properties": {
            "server_name": {
                "type": "string",
                "description": "Optional server name to filter resources by. Omit to list resources from all servers.",
            }
        },
    },
}

READ_MCP_RESOURCE_SCHEMA = {
    "name": "read_mcp_resource",
    "description": "Read a specific MCP resource by URI from a given server.",
    "parameters": {
        "type": "object",
        "properties": {
            "server_name": {
                "type": "string",
                "description": "Name of the MCP server to read the resource from.",
            },
            "uri": {
                "type": "string",
                "description": "The resource URI to read.",
            },
        },
        "required": ["server_name", "uri"],
    },
}


def _get_connected_servers():
    """Return dict of {name: server} from the MCP module."""
    try:
        from tools.mcp_tool import _servers as mcp_servers, _lock
        with _lock:
            return dict(mcp_servers)
    except ImportError:
        return {}
    except Exception as exc:
        logger.debug("Failed to read MCP server state: %s", exc)
        return {}


def _run_on_mcp_loop(coro, timeout=30):
    from tools.mcp_tool import _run_on_mcp_loop as _run
    return _run(coro, timeout=timeout)


def _interrupted_call_result():
    return json.dumps({
        "error": "MCP call interrupted: user sent a new message"
    })


def list_mcp_resources_handler(args, **kw):
    server_name_filter = args.get("server_name")
    servers = _get_connected_servers()

    if not servers:
        return tool_result({"resources": [], "message": "No MCP servers are connected."})

    if server_name_filter:
        if server_name_filter not in servers:
            avail = ", ".join(sorted(servers.keys()))
            return tool_error(
                f"Server '{server_name_filter}' not found. "
                f"Available servers: {avail}"
            )
        candidates = {server_name_filter: servers[server_name_filter]}
    else:
        candidates = servers

    results = []
    for sname, server in candidates.items():
        if not server or not server.session:
            continue

        async def _list(srv=server):
            async with srv._rpc_lock:
                result = await srv.session.list_resources()
            resources = []
            for r in (result.resources if hasattr(result, "resources") else []):
                entry = {"server": sname}
                if hasattr(r, "uri"):
                    entry["uri"] = str(r.uri)
                if hasattr(r, "name"):
                    entry["name"] = r.name
                if hasattr(r, "description") and r.description:
                    entry["description"] = r.description
                if hasattr(r, "mimeType") and r.mimeType:
                    entry["mimeType"] = r.mimeType
                resources.append(entry)
            return resources

        try:
            resources = _run_on_mcp_loop(_list(), timeout=server.tool_timeout)
            results.extend(resources)
        except InterruptedError:
            return _interrupted_call_result()
        except Exception as exc:
            logger.error("MCP %s/list_resources failed: %s", sname, exc)
            results.append({"server": sname, "error": str(exc)})

    if not results:
        msg = (
            f"Server '{server_name_filter}' has no resources."
            if server_name_filter
            else "No resources found across any connected MCP server."
        )
        return tool_result({"resources": [], "message": msg})

    return tool_result({"resources": results})


def read_mcp_resource_handler(args, **kw):
    server_name = args.get("server_name")
    uri = args.get("uri")
    if not server_name:
        return tool_error("Missing required parameter 'server_name'")
    if not uri:
        return tool_error("Missing required parameter 'uri'")

    servers = _get_connected_servers()
    server = servers.get(server_name)
    if not server:
        avail = ", ".join(sorted(servers.keys())) if servers else "none connected"
        return tool_error(
            f"Server '{server_name}' not found or not connected. "
            f"Available servers: {avail}"
        )

    async def _read():
        async with server._rpc_lock:
            result = await server.session.read_resource(uri)
        contents = []
        for block in (result.contents if hasattr(result, "contents") else []):
            entry = {}
            if hasattr(block, "uri"):
                entry["uri"] = str(block.uri)
            if hasattr(block, "mimeType") and block.mimeType:
                entry["mimeType"] = block.mimeType
            if hasattr(block, "text"):
                entry["text"] = block.text
            elif hasattr(block, "blob"):
                entry["blob"] = f"[binary data, {len(block.blob)} bytes]"
            contents.append(entry)
        return {"contents": contents}

    try:
        data = _run_on_mcp_loop(_read(), timeout=server.tool_timeout)
        return tool_result(data)
    except InterruptedError:
        return _interrupted_call_result()
    except Exception as exc:
        return tool_error(f"MCP read_resource call failed: {exc}")


registry.register(
    name="list_mcp_resources",
    toolset="system",
    schema=LIST_MCP_RESOURCES_SCHEMA,
    handler=list_mcp_resources_handler,
    emoji="📋",
)

registry.register(
    name="read_mcp_resource",
    toolset="system",
    schema=READ_MCP_RESOURCE_SCHEMA,
    handler=read_mcp_resource_handler,
    emoji="📖",
)
