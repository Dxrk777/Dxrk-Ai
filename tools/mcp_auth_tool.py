# SPDX-License-Identifier: MIT
"""MCP Auth tool -- trigger OAuth authentication for an MCP server."""

import json
import logging
import os
from pathlib import Path
from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

MCP_AUTH_SCHEMA = {
    "name": "mcp_auth",
    "description": (
        "Start OAuth authentication for an MCP server that requires authorization. "
        "Returns an auth URL for the user to open in their browser, or writes an "
        "auth request file for manual processing."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "server_name": {
                "type": "string",
                "description": "Name of the MCP server to authenticate.",
            },
            "auth_type": {
                "type": "string",
                "description": "Authentication type override (oauth, token, etc.). Auto-detected if omitted.",
                "enum": ["oauth", "token", "basic"],
            },
        },
        "required": ["server_name"],
    },
}


def _get_dxrk_home():
    return Path(os.environ.get("DXRK_HOME", str(Path.home() / ".dxrk")))


def _write_auth_request(server_name: str, auth_type: str):
    auth_dir = _get_dxrk_home() / "mcp-auth"
    auth_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "server_name": server_name,
        "auth_type": auth_type,
        "message": (
            f"Authentication requested for MCP server '{server_name}' "
            f"using {auth_type} flow."
        ),
    }
    req_path = auth_dir / f"{server_name}.json"
    req_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    return req_path


def _find_server_config(server_name: str):
    try:
        from tools.mcp_tool import _load_mcp_config
        configs = _load_mcp_config()
        return configs.get(server_name)
    except Exception:
        return None


def mcp_auth_handler(args, **kw):
    server_name = args.get("server_name")
    if not server_name:
        return tool_error("Missing required parameter 'server_name'")

    auth_type = (args.get("auth_type") or "").lower().strip()

    config = _find_server_config(server_name)

    if config and not auth_type:
        auth_type = (config.get("auth") or "oauth").lower().strip()

    if not auth_type:
        auth_type = "oauth"

    # Check if mcp_oauth_manager is available for proper OAuth flow
    try:
        from tools.mcp_oauth_manager import get_manager
        has_oauth_manager = True
    except ImportError:
        has_oauth_manager = False

    # Check if the server is already connected
    try:
        from tools.mcp_tool import _servers as mcp_servers, _lock
        with _lock:
            server = mcp_servers.get(server_name)
    except Exception:
        server = None

    if server and server.session is not None:
        return tool_result({
            "status": "already_authenticated",
            "server": server_name,
            "message": f"MCP server '{server_name}' is already connected and authenticated.",
        })

    if has_oauth_manager and auth_type == "oauth":
        if config and "url" in config:
            try:
                manager = get_manager()
                from tools.mcp_oauth import remove_oauth_tokens
                remove_oauth_tokens(server_name)
                provider = manager.get_or_build_provider(
                    server_name, config["url"], config.get("oauth"),
                )
                # Kick off the flow if the provider has a get_authorization_url
                if hasattr(provider, "get_authorization_url"):
                    auth_url = provider.get_authorization_url()
                    return tool_result({
                        "status": "auth_url",
                        "server": server_name,
                        "auth_url": auth_url,
                        "message": (
                            f"Ask the user to open this URL in their browser "
                            f"to authorize the {server_name} MCP server:\n\n{auth_url}"
                        ),
                    })
            except Exception as exc:
                logger.debug("MCP OAuth manager flow failed for '%s': %s", server_name, exc)

    req_path = _write_auth_request(server_name, auth_type)

    return tool_result({
        "status": "auth_request_written",
        "server": server_name,
        "auth_type": auth_type,
        "file": str(req_path),
        "message": (
            f"Authentication request for '{server_name}' written to {req_path}. "
            f"Ask the user to complete the {auth_type} flow for this server."
        ),
    })


registry.register(
    name="mcp_auth",
    toolset="system",
    schema=MCP_AUTH_SCHEMA,
    handler=mcp_auth_handler,
    emoji="🔑",
)
