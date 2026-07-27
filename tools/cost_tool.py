# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Cost & Session Summary Tool

Tracks per-turn token usage, accumulates cost, and provides a session summary
mirroring Claude Code's BriefTool + CostTracker.  The CostTracker lives on
AIAgent (one per session) and is updated via a hook after each API call.

Usage:
    tracker = CostTracker()
    tracker.record_turn(input_tokens=500, output_tokens=200, model="claude-sonnet-4")
    tracker.record_tool_call("read")
    tracker.record_file_change("src/main.py", "modified")
    summary = tracker.summarize()
"""

import json
import os
import threading
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Dict, List, Optional


# Approximate USD cost per 1K tokens (defaults — override via env or config)
COST_PER_1K_INPUT = float(os.environ.get("DXRK_COST_PER_1K_INPUT", "0.003"))
COST_PER_1K_OUTPUT = float(os.environ.get("DXRK_COST_PER_1K_OUTPUT", "0.015"))


@dataclass
class ModelUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    cache_read_input_tokens: int = 0
    cache_creation_input_tokens: int = 0
    cost_usd: float = 0.0


@dataclass
class CostTracker:
    _lock: threading.RLock = field(default_factory=threading.RLock)
    _total_input_tokens: int = 0
    _total_output_tokens: int = 0
    _total_cache_read_tokens: int = 0
    _total_cache_creation_tokens: int = 0
    _total_cost_usd: float = 0.0
    _tool_calls: int = 0
    _web_search_requests: int = 0
    _files_changed: List[Dict[str, str]] = field(default_factory=list)
    _session_start: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    _model_usage: Dict[str, ModelUsage] = field(default_factory=dict)

    def record_turn(
        self,
        input_tokens: int,
        output_tokens: int,
        model: str = "unknown",
        cache_read: int = 0,
        cache_creation: int = 0,
        cost_override: Optional[float] = None,
    ) -> None:
        with self._lock:
            self._total_input_tokens += input_tokens
            self._total_output_tokens += output_tokens
            self._total_cache_read_tokens += cache_read
            self._total_cache_creation_tokens += cache_creation

            turn_cost = (
                cost_override
                if cost_override is not None
                else (input_tokens / 1000) * COST_PER_1K_INPUT
                + (output_tokens / 1000) * COST_PER_1K_OUTPUT
            )
            self._total_cost_usd += turn_cost

            if model not in self._model_usage:
                self._model_usage[model] = ModelUsage()
            mu = self._model_usage[model]
            mu.input_tokens += input_tokens
            mu.output_tokens += output_tokens
            mu.cache_read_input_tokens += cache_read
            mu.cache_creation_input_tokens += cache_creation
            mu.cost_usd += turn_cost

    def record_tool_call(self, tool_name: str = "") -> None:
        with self._lock:
            self._tool_calls += 1

    def record_web_search(self) -> None:
        with self._lock:
            self._web_search_requests += 1

    def record_file_change(self, path: str, change_type: str = "modified") -> None:
        with self._lock:
            self._files_changed.append({"path": path, "type": change_type})

    def summarize(self) -> Dict:
        with self._lock:
            return {
                "session_start": self._session_start,
                "total_input_tokens": self._total_input_tokens,
                "total_output_tokens": self._total_output_tokens,
                "total_cache_read_tokens": self._total_cache_read_tokens,
                "total_cache_creation_tokens": self._total_cache_creation_tokens,
                "total_tokens": (
                    self._total_input_tokens
                    + self._total_output_tokens
                    + self._total_cache_read_tokens
                    + self._total_cache_creation_tokens
                ),
                "estimated_cost_usd": round(self._total_cost_usd, 6),
                "tool_calls": self._tool_calls,
                "web_search_requests": self._web_search_requests,
                "files_changed": list(self._files_changed),
                "model_breakdown": {
                    model: {
                        "input_tokens": mu.input_tokens,
                        "output_tokens": mu.output_tokens,
                        "cache_read_tokens": mu.cache_read_input_tokens,
                        "cache_creation_tokens": mu.cache_creation_input_tokens,
                        "cost_usd": round(mu.cost_usd, 6),
                    }
                    for model, mu in sorted(self._model_usage.items())
                },
            }


# Module-level cache so the registry handler can find the tracker instance.
_TRACKER: Optional[CostTracker] = None


def set_tracker(tracker: CostTracker) -> None:
    global _TRACKER
    _TRACKER = tracker


def get_tracker() -> CostTracker:
    global _TRACKER
    if _TRACKER is None:
        _TRACKER = CostTracker()
    return _TRACKER


# ---------------------------------------------------------------------------
# Tool handler
# ---------------------------------------------------------------------------

def session_summary_handler(args: dict, **kw) -> str:
    tracker = kw.get("tracker") or get_tracker()
    return json.dumps({"session_summary": tracker.summarize()}, ensure_ascii=False)


def check_cost_requirements() -> bool:
    return True


SCHEMA = {
    "name": "session_summary",
    "description": (
        "Get a summary of the current session: total tokens used, "
        "estimated API cost, files changed, tool calls, and per-model "
        "breakdown of usage. No arguments needed."
    ),
    "parameters": {
        "type": "object",
        "properties": {},
        "required": [],
    },
}

from tools.registry import registry

registry.register(
    name="session_summary",
    toolset="dev",
    schema=SCHEMA,
    handler=session_summary_handler,
    check_fn=check_cost_requirements,
    emoji="💰",
)
