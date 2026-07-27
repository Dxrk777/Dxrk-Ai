# SPDX-License-Identifier: MIT
#!/usr/bin/env python3

import json
import os
from pathlib import Path
from datetime import datetime, timezone

try:
    import yaml
except ImportError:
    yaml = None

from tools.registry import registry, tool_error, tool_result

DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
CONFIG_PATH = DXRK_HOME / "config.yaml"


def _load_config() -> dict:
    if not CONFIG_PATH.exists():
        return {}
    if yaml:
        with open(CONFIG_PATH) as f:
            return yaml.safe_load(f) or {}
    text = CONFIG_PATH.read_text()
    return json.loads(text) if text.strip().startswith("{") else {}


def _save_config(cfg: dict) -> None:
    CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    if yaml:
        with open(CONFIG_PATH, "w") as f:
            yaml.safe_dump(cfg, f, default_flow_style=False)
    else:
        CONFIG_PATH.write_text(json.dumps(cfg, indent=2, ensure_ascii=False))


def _get_value(cfg: dict, key: str):
    parts = key.split(".")
    cur = cfg
    for p in parts:
        if isinstance(cur, dict) and p in cur:
            cur = cur[p]
        else:
            return None
    return cur


def _set_value(cfg: dict, key: str, value) -> tuple[dict, object]:
    parts = key.split(".")
    cur = cfg
    for p in parts[:-1]:
        if p not in cur or not isinstance(cur[p], dict):
            cur[p] = {}
        cur = cur[p]
    prev = cur.get(parts[-1])
    cur[parts[-1]] = value
    return cfg, prev


def config_handler(args: dict, **kw) -> str:
    try:
        action = args.get("action", "list")
        key = args.get("key")
        value = args.get("value")

        cfg = _load_config()

        if action == "get":
            if not key:
                return tool_error("`key` is required for action=get")
            val = _get_value(cfg, key)
            return tool_result(action="get", key=key, value=val)

        elif action == "set":
            if not key:
                return tool_error("`key` and `value` are required for action=set")
            cfg, prev = _set_value(cfg, key, value)
            _save_config(cfg)
            return tool_result(
                action="set", key=key, value=value, previous=prev
            )

        elif action == "list":
            return tool_result(action="list", config=cfg)

        else:
            return tool_error(f"Unknown action: {action}")

    except Exception as e:
        return tool_error(str(e))


SCHEMA = {
    "name": "config",
    "description": (
        "Get, set, or list Dxrk configuration values "
        "from ~/.dxrk/config.yaml. Use `get` to read a value by dot-separated "
        "key, `set` to write a value, and `list` to show all config."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["get", "set", "list"],
                "description": "Operation: 'get' reads, 'set' writes, 'list' shows all",
            },
            "key": {
                "type": "string",
                "description": "Dot-separated config key (e.g. 'model' or 'permissions.defaultMode')",
            },
            "value": {
                "description": "Value to set (for action=set); supports string, number, boolean",
            },
        },
        "required": ["action"],
    },
}

registry.register(
    name="config",
    toolset="dev",
    schema=SCHEMA,
    handler=config_handler,
    emoji="⚙️",
)
