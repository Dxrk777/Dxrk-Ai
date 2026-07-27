# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Analytics Sink — Event logging infrastructure for Dxrk.

Port of Claude Code's analytics service with event queue, multi-sink dispatch,
file-based persistence, and PII sanitization.

Architecture:
    AnalyticsHub (singleton) receives events → queues → dispatches to all sinks
        ├─ FileSink:    writes ~/.dxrk/analytics/{date}.jsonl
        ├─ ConsoleSink: prints to stderr when DXRK_DEBUG=1
        └─ [extensible via register_sink()]

Usage:
    from tools.analytics_tool import analytics_hub, AnalyticsEvent

    analytics_hub.emit(AnalyticsEvent("tool_used", {"tool": "fuzzy_find"}))
    analytics_hub.flush()

Tool: analytics_event → {name, properties?, flush?} — fire-and-forget event logging.
"""

import json
import logging
import os
import re
import threading
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Set
from uuid import uuid4

from tools.registry import registry, tool_error, tool_ok

logger = logging.getLogger(__name__)

# ── Data model ───────────────────────────────────────────────────────────────

DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
ANALYTICS_DIR = DXRK_HOME / "analytics"


@dataclass
class AnalyticsEvent:
    name: str
    properties: Dict[str, Any] = field(default_factory=dict)
    timestamp: Optional[str] = None
    session_id: Optional[str] = None

    def __post_init__(self):
        if self.timestamp is None:
            self.timestamp = datetime.now(timezone.utc).isoformat()
        if self.session_id is None:
            self.session_id = _get_session_id()

    def to_dict(self) -> dict:
        return {
            "name": self.name,
            "properties": dict(self.properties),
            "timestamp": self.timestamp,
            "session_id": self.session_id,
        }


# ── Session ID ───────────────────────────────────────────────────────────────

_session_id: Optional[str] = None


def _get_session_id() -> str:
    global _session_id
    if _session_id is None:
        _session_id = os.environ.get("DXRK_SESSION_ID", str(uuid4()))
    return _session_id


# ── Sanitizer — strips sensitive patterns from event properties ─────────────

_SENSITIVE_PATTERNS = [
    re.compile(r"(?i)(api[_-]?key|apikey|secret|token|password|passwd|credential)\s*[:=]\s*\S+"),
    re.compile(r"(?i)(sk-[a-zA-Z0-9]{20,}|pk-[a-zA-Z0-9]{20,})"),
    re.compile(r"(?i)(ghp_|gho_|ghu_|ghs_|ghr_)[a-zA-Z0-9]{36,}"),
    re.compile(r"/home/[^/]+/"),
    re.compile(r"/Users/[^/]+/"),
]

_PATH_LIKE = re.compile(r"(?:^|(?<=\s))(/[\w.\-/]+)")


class Sanitizer:
    def __init__(self, patterns: Optional[List[re.Pattern]] = None):
        self._patterns = patterns or _SENSITIVE_PATTERNS
        self._path_pattern = _PATH_LIKE

    def sanitize_properties(self, properties: Dict[str, Any]) -> Dict[str, Any]:
        cleaned: Dict[str, Any] = {}
        for key, value in properties.items():
            if isinstance(value, str):
                value = self._sanitize_string(value)
            elif isinstance(value, dict):
                value = self.sanitize_properties(value)
            elif isinstance(value, list):
                value = [self._sanitize_string(v) if isinstance(v, str) else v for v in value]
            cleaned[key] = value
        return cleaned

    def _sanitize_string(self, text: str) -> str:
        for pattern in self._patterns:
            text = pattern.sub("<REDACTED>", text)
        text = self._path_pattern.sub("<PATH>", text)
        return text


_default_sanitizer = Sanitizer()

# ── Sink base class ──────────────────────────────────────────────────────────


class AnalyticsSink(ABC):
    @abstractmethod
    def write(self, event: AnalyticsEvent) -> None:
        ...

    @abstractmethod
    def flush(self) -> None:
        ...


# ── FileSink ─────────────────────────────────────────────────────────────────


class FileSink(AnalyticsSink):
    def __init__(self, base_dir: Optional[Path] = None):
        self._base_dir = base_dir or ANALYTICS_DIR
        self._base_dir.mkdir(parents=True, exist_ok=True)
        self._file: Optional[object] = None
        self._current_date: Optional[str] = None

    def _get_file(self):
        today = datetime.now(timezone.utc).strftime("%Y-%m-%d")
        if self._current_date != today:
            if self._file:
                try:
                    self._file.close()
                except Exception:
                    pass
            self._current_date = today
            log_path = self._base_dir / f"{today}.jsonl"
            try:
                log_path.parent.mkdir(parents=True, exist_ok=True)
                self._file = open(log_path, "a", encoding="utf-8")
            except (OSError, PermissionError) as e:
                logger.debug("[analytics] FileSink open error: %s", e)
                self._file = None
        return self._file

    def write(self, event: AnalyticsEvent) -> None:
        f = self._get_file()
        if f is None:
            return
        try:
            f.write(json.dumps(event.to_dict(), ensure_ascii=False) + "\n")
            f.flush()
        except (OSError, AttributeError) as e:
            logger.debug("[analytics] FileSink write error: %s", e)

    def flush(self) -> None:
        if self._file:
            try:
                self._file.flush()
            except Exception:
                pass


# ── ConsoleSink ──────────────────────────────────────────────────────────────


class ConsoleSink(AnalyticsSink):
    def __init__(self):
        self._enabled = os.environ.get("DXRK_DEBUG") == "1"

    def write(self, event: AnalyticsEvent) -> None:
        if not self._enabled:
            return
        import sys
        line = json.dumps(event.to_dict(), ensure_ascii=False)
        print(f"[analytics] {line}", file=sys.stderr, flush=True)

    def flush(self) -> None:
        if self._enabled:
            import sys
            sys.stderr.flush()


# ── AnalyticsHub — queue + dispatch ─────────────────────────────────────────


class AnalyticsHub:
    def __init__(self, sanitizer: Optional[Sanitizer] = None):
        self._sinks: List[AnalyticsSink] = []
        self._queue: List[AnalyticsEvent] = []
        self._flush_immediately = True
        self._sanitizer = sanitizer or _default_sanitizer
        self._lock = threading.Lock()

    def register_sink(self, sink: AnalyticsSink) -> None:
        with self._lock:
            self._sinks.append(sink)

    def emit(self, event: AnalyticsEvent) -> None:
        sanitized_props = self._sanitizer.sanitize_properties(event.properties)
        event.properties = sanitized_props

        if self._flush_immediately:
            with self._lock:
                for sink in self._sinks:
                    try:
                        sink.write(event)
                    except Exception as e:
                        logger.debug("[analytics] Sink error: %s", e)
        else:
            with self._lock:
                self._queue.append(event)

    def flush(self) -> None:
        with self._lock:
            events = self._queue
            self._queue = []
            for sink in self._sinks:
                try:
                    for event in events:
                        sink.write(event)
                    sink.flush()
                except Exception as e:
                    logger.debug("[analytics] Sink flush error: %s", e)


# ── Singleton ────────────────────────────────────────────────────────────────

analytics_hub = AnalyticsHub()
analytics_hub.register_sink(FileSink())
analytics_hub.register_sink(ConsoleSink())

# ── Tool registration ────────────────────────────────────────────────────────

SCHEMA = {
    "name": "analytics_event",
    "description": "Log an analytics event. Fire-and-forget: events are queued and dispatched to configured sinks (file, console). Properties are sanitized to remove sensitive patterns (API keys, tokens, file paths).",
    "parameters": {
        "type": "object",
        "properties": {
            "name": {
                "type": "string",
                "description": "Event name (e.g. 'tool_used', 'session_start', 'error_occurred').",
            },
            "properties": {
                "type": "object",
                "description": "Optional event properties (flat key-value pairs, strings are sanitized).",
                "default": {},
            },
            "flush": {
                "type": "boolean",
                "description": "If true, flush all pending events immediately.",
                "default": False,
            },
        },
        "required": ["name"],
    },
}


def _handler(args, **kw):
    name = args.get("name", "").strip()
    if not name:
        return tool_error("Event name is required")

    properties = args.get("properties", {}) or {}
    if not isinstance(properties, dict):
        return tool_error("properties must be a dict")

    event = AnalyticsEvent(name=name, properties=properties)
    analytics_hub.emit(event)

    if args.get("flush"):
        analytics_hub.flush()

    return tool_ok({"logged": name})


registry.register(
    name="analytics_event",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    is_concurrency_safe=True,
    is_readonly=True,
    is_hidden=True,
    emoji="📊",
)
