# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Agent Message Tool — File-based agent-to-agent messaging.

Mirrors Claude Code's SendMessageTool using a simple directory-based mailbox
at ~/.dxrk/mailbox/{agent_id}/.

Tools:
  - send_message:  write a message to a target agent's inbox
  - read_messages: drain (read + delete) messages from own inbox
"""

import json
import os
import threading
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional

MAILBOX_DIR = Path.home() / ".dxrk" / "mailbox"


class AgentMailbox:
    """File-based per-agent mailbox.

    Each agent gets a directory ``{MAILBOX_DIR}/{agent_id}/`` containing
    numbered JSON files as messages.  This is thread-safe via a module-level
    lock and atomic write-then-rename.
    """

    _lock = threading.Lock()

    @staticmethod
    def _inbox_path(agent_id: str) -> Path:
        return MAILBOX_DIR / agent_id

    @staticmethod
    def _ensure_dir(agent_id: str) -> Path:
        path = AgentMailbox._inbox_path(agent_id)
        path.mkdir(parents=True, exist_ok=True)
        return path

    @classmethod
    def send(cls, target_agent: str, message: str, sender: str = "unknown") -> Dict:
        """Write a message to *target_agent*'s inbox.

        Returns a dict with status metadata.
        """
        from tools.registry import tool_error

        if not target_agent or not target_agent.strip():
            return {"success": False, "error": "target_agent is required"}
        if not message or not message.strip():
            return {"success": False, "error": "message is required"}

        entry = {
            "from": sender,
            "text": message,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

        with cls._lock:
            inbox = cls._ensure_dir(target_agent)
            # Find next available sequence number
            existing = sorted(inbox.glob("*.json"))
            seq = (len(existing) + 1) if existing else 1
            tmp = inbox / f".{seq}.tmp"
            final = inbox / f"{seq}.json"
            tmp.write_text(json.dumps(entry, ensure_ascii=False), encoding="utf-8")
            tmp.rename(final)

        return {
            "success": True,
            "message": f"Message sent to {target_agent}'s inbox",
            "target": target_agent,
        }

    @classmethod
    def read_inbox(cls, agent_id: str) -> List[Dict]:
        """Read and remove all messages from *agent_id*'s inbox.

        Returns a list of message dicts (oldest first).
        """
        with cls._lock:
            inbox = cls._inbox_path(agent_id)
            if not inbox.exists():
                return []
            messages: List[Dict] = []
            for path in sorted(inbox.glob("*.json")):
                try:
                    data = json.loads(path.read_text(encoding="utf-8"))
                    data["seq"] = int(path.stem)
                    messages.append(data)
                except (json.JSONDecodeError, OSError):
                    pass
                path.unlink(missing_ok=True)
            return messages

    @classmethod
    def count_messages(cls, agent_id: str) -> int:
        """Return the number of unread messages for *agent_id*."""
        with cls._lock:
            inbox = cls._inbox_path(agent_id)
            if not inbox.exists():
                return 0
            return len(list(inbox.glob("*.json")))


# ---------------------------------------------------------------------------
# Tool handlers
# ---------------------------------------------------------------------------

def send_message_handler(args: dict, **kw) -> str:
    target = args.get("target_agent", "").strip()
    message = args.get("message", "").strip()
    sender = args.get("sender", "agent") or "agent"

    result = AgentMailbox.send(target, message, sender)
    return json.dumps(result, ensure_ascii=False)


def read_messages_handler(args: dict, **kw) -> str:
    agent_id = args.get("agent_id", "").strip() or kw.get("agent_id", "")
    if not agent_id:
        return json.dumps(
            {"success": False, "error": "agent_id is required"},
            ensure_ascii=False,
        )

    messages = AgentMailbox.read_inbox(agent_id)
    return json.dumps(
        {"success": True, "messages": messages, "count": len(messages)},
        ensure_ascii=False,
    )


def check_agent_message_requirements() -> bool:
    return True


SEND_MESSAGE_SCHEMA = {
    "name": "send_message",
    "description": (
        "Send a message to another agent's mailbox. "
        "The target agent will find the message when it calls read_messages."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "target_agent": {
                "type": "string",
                "description": "Agent ID or name to send the message to",
            },
            "message": {
                "type": "string",
                "description": "The message content",
            },
            "sender": {
                "type": "string",
                "description": "Who this message is from (default: agent)",
            },
        },
        "required": ["target_agent", "message"],
    },
}

READ_MESSAGES_SCHEMA = {
    "name": "read_messages",
    "description": "Read and clear all pending messages from your agent's inbox.",
    "parameters": {
        "type": "object",
        "properties": {
            "agent_id": {
                "type": "string",
                "description": "Your agent ID to check messages for",
            },
        },
        "required": ["agent_id"],
    },
}

from tools.registry import registry

registry.register(
    name="send_message",
    toolset="agent",
    schema=SEND_MESSAGE_SCHEMA,
    handler=send_message_handler,
    check_fn=check_agent_message_requirements,
    emoji="💬",
)

registry.register(
    name="read_messages",
    toolset="agent",
    schema=READ_MESSAGES_SCHEMA,
    handler=read_messages_handler,
    check_fn=check_agent_message_requirements,
    emoji="📨",
)
