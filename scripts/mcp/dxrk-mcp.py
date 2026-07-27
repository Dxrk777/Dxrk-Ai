# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Dxrk MCP Server — expose Dxrk ecosystem capabilities as MCP tools.

Provides tools for agent detection, system info, skills catalog, and diagnostics
so any MCP client can interact with the Dxrk ecosystem.

Usage:
    python3 scripts/mcp/dxrk-mcp.py

MCP client config (.opencode/mcp.json or claude_desktop_config.json):
    {
        "mcpServers": {
            "dxrk-mcp": {
                "command": "python3",
                "args": ["scripts/mcp/dxrk-mcp.py"]
            }
        }
    }
"""

from __future__ import annotations

import logging
import os
import platform
import sys
from pathlib import Path

logger = logging.getLogger("dxrk-mcp")

_MCP_AVAILABLE = False
try:
    from mcp.server.fastmcp import FastMCP

    _MCP_AVAILABLE = True
except ImportError:
    FastMCP = None

DXRK_ROOT = Path(__file__).resolve().parent.parent.parent


def _detect_agents() -> list[dict]:
    results = []
    try:
        from dxrk.agents.factory import create_registry

        reg = create_registry()
        for agent_id in reg.supported_agents:
            adapter = reg.get(agent_id)
            if adapter is None:
                continue
            try:
                result = adapter.detect()
                results.append(
                    {
                        "agent": agent_id.value,
                        "installed": result.installed,
                        "binary_path": result.binary_path,
                        "config_found": result.config_found,
                        "supports_auto_install": adapter.supports_auto_install,
                        "supports_mcp": adapter.supports_mcp,
                        "supports_skills": adapter.supports_skills,
                    }
                )
            except Exception as e:
                results.append(
                    {
                        "agent": agent_id.value,
                        "installed": False,
                        "error": str(e),
                    }
                )
    except ImportError as e:
        logger.warning("Dxrk module not available: %s", e)
        results.append({"error": f"Dxrk module not available: {e}"})
    return results


def _get_system_info() -> dict:
    return {
        "os": platform.system(),
        "os_release": platform.release(),
        "arch": platform.machine(),
        "python_version": platform.python_version(),
        "cwd": os.getcwd(),
        "project_root": str(DXRK_ROOT),
        "project_root_exists": DXRK_ROOT.exists(),
    }


def _get_skills_summary() -> list[dict]:
    skills = []
    skills_dir = DXRK_ROOT / "skills"
    if skills_dir.exists():
        for d in sorted(skills_dir.iterdir()):
            if d.is_dir():
                skill_file = d / "SKILL.md"
                name = d.name
                description = ""
                if skill_file.exists():
                    content = skill_file.read_text()
                    for line in content.splitlines():
                        if line.startswith("description:"):
                            description = line.split(":", 1)[1].strip().strip('"')
                            break
                skills.append(
                    {
                        "name": name,
                        "description": description,
                        "path": str(skill_file) if skill_file.exists() else "",
                    }
                )
    agent_skills_dir = DXRK_ROOT / ".agents" / "skills"
    if agent_skills_dir.exists():
        for d in sorted(agent_skills_dir.iterdir()):
            if d.is_dir() or d.suffix == ".md":
                skill_file = d if d.is_file() else d / "SKILL.md"
                name = d.stem if d.is_file() else d.name
                description = ""
                if skill_file.exists() and skill_file.is_file():
                    content = skill_file.read_text()
                    for line in content.splitlines():
                        if line.startswith("description:"):
                            description = line.split(":", 1)[1].strip().strip('"')
                            break
                skills.append(
                    {
                        "name": f"dxrk-{name}",
                        "description": description,
                        "path": str(skill_file) if skill_file.exists() else "",
                    }
                )
    return skills


def _run_diagnostic() -> dict:
    issues = []
    info = _get_system_info()

    mcp_json = DXRK_ROOT / ".opencode" / "mcp.json"
    if not mcp_json.exists():
        issues.append("Missing .opencode/mcp.json")

    scripts_dir = DXRK_ROOT / "scripts"
    if not scripts_dir.exists():
        issues.append("Missing scripts/ directory")

    try:
        from dxrk.agents.factory import create_registry

        reg = create_registry()
        agent_count = len(reg.supported_agents)
    except ImportError:
        agent_count = 0
        issues.append("Dxrk Python module not importable")

    mcp_configs_valid = 0
    mcp_configs_missing = 0
    try:
        import json
        import shutil

        if mcp_json.exists():
            config = json.loads(mcp_json.read_text())
            for name, server in config.get("mcpServers", {}).items():
                cmd = server.get("command", "")
                args = server.get("args", [])
                if args and args[0].endswith((".py", ".sh", ".js", ".ts")):
                    script_path = Path(args[0])
                    if not script_path.is_absolute():
                        script_path = DXRK_ROOT / script_path
                    if script_path.exists():
                        mcp_configs_valid += 1
                    else:
                        mcp_configs_missing += 1
                        issues.append(
                            f"MCP server '{name}' points to missing script: {script_path}"
                        )
                elif shutil.which(cmd):
                    mcp_configs_valid += 1
                else:
                    mcp_configs_missing += 1
                    issues.append(f"MCP server '{name}' command not found: {cmd}")
    except Exception as e:
        issues.append(f"Error reading MCP config: {e}")

    return {
        "system": info,
        "project_exists": DXRK_ROOT.exists(),
        "agents_registered": agent_count,
        "mcp_servers_configured": mcp_configs_valid,
        "mcp_servers_missing_scripts": mcp_configs_missing,
        "skills_available": len(_get_skills_summary()),
        "issues": issues,
        "healthy": len(issues) == 0,
    }


def create_server():
    if not _MCP_AVAILABLE or FastMCP is None:
        raise RuntimeError("mcp package not installed")
    mcp = FastMCP("dxrk-mcp", log_level="WARNING")

    @mcp.tool()
    def detect_agents() -> list[dict]:
        return _detect_agents()

    @mcp.tool()
    def detect_agent(agent_name: str) -> dict:
        results = _detect_agents()
        for r in results:
            if r.get("agent") == agent_name.lower():
                return r
        return {"agent": agent_name, "error": f"Agent '{agent_name}' not found"}

    @mcp.tool()
    def get_system_info() -> dict:
        return _get_system_info()

    @mcp.tool()
    def list_skills(query: str = "") -> list[dict]:
        skills = _get_skills_summary()
        if query:
            q = query.lower()
            skills = [
                s
                for s in skills
                if q in s["name"].lower() or q in s["description"].lower()
            ]
        return skills

    @mcp.tool()
    def run_diagnostic() -> dict:
        return _run_diagnostic()

    return mcp


def main() -> None:
    logging.basicConfig(
        level=logging.WARNING, format="%(levelname)s:%(name)s:%(message)s"
    )

    if not _MCP_AVAILABLE:
        print("mcp package not installed. Run: pip install mcp", file=sys.stderr)
        sys.exit(1)

    server = create_server()
    server.run()


if __name__ == "__main__":
    main()
