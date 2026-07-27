# SPDX-License-Identifier: MIT
"""Memdir — file-based persistent memory system.

Port of Claude Code's memdir (file-based memory) system.

Stores memories as Markdown files with YAML frontmatter in
``~/.dxrk/memory/{category}/{topic}.md``.

Categories (closed taxonomy):
  - user:      preferences, role, goals, communication style
  - feedback:  corrections, confirmations, guidance on approach
  - project:   ongoing work, goals, initiatives, deadlines
  - reference: pointers to external systems, docs, dashboards

MEMORY.md in the root is a navigation index (no frontmatter). Each
entry is one line: ``- [Title](category/topic.md) — one-line hook``.
"""

from __future__ import annotations

import logging
import os
import re
import threading
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

import yaml

from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

MEMORY_HOME = Path.home() / ".dxrk" / "memory"
ENTRYPOINT_NAME = "MEMORY.md"
MAX_ENTRYPOINT_LINES = 200
MAX_ENTRYPOINT_BYTES = 25_000

CATEGORIES = ("user", "feedback", "project", "reference")

DEFAULT_FRONTMATTER: Dict[str, Any] = {
    "type": "",
    "date": "",
    "tags": [],
    "related": [],
    "importance": 3,
}

# ---------------------------------------------------------------------------
# Frontmatter helpers
# ---------------------------------------------------------------------------


def _parse_frontmatter(text: str) -> tuple[Dict[str, Any], str]:
    """Parse YAML frontmatter (``---`` delimited) from markdown text.

    Returns ``(frontmatter_dict, body)``.  Invalid or missing frontmatter
    returns empty dict and the original text as body.
    """
    text = text.strip()
    if not text.startswith("---"):
        return {}, text

    end = text.find("---", 3)
    if end == -1:
        return {}, text

    raw = text[3:end].strip()
    body = text[end + 3 :].strip()

    try:
        fm = yaml.safe_load(raw)
        if not isinstance(fm, dict):
            return {}, body
        return fm, body
    except yaml.YAMLError:
        logger.debug("Failed to parse frontmatter YAML", exc_info=True)
        return {}, body


def _build_frontmatter(fm: Dict[str, Any]) -> str:
    """Render a dict as YAML frontmatter with ``---`` delimiters."""
    raw = yaml.dump(
        {k: v for k, v in fm.items() if v is not None and v != "" and v != []},
        default_flow_style=False,
        allow_unicode=True,
        sort_keys=False,
    ).strip()
    return f"---\n{raw}\n---"


def _make_frontmatter(
    category: str,
    tags: Optional[List[str]] = None,
    related: Optional[List[str]] = None,
    importance: int = 3,
) -> Dict[str, Any]:
    """Build default frontmatter for a new memory."""
    return {
        "type": category,
        "date": date.today().isoformat(),
        "tags": tags or [],
        "related": related or [],
        "importance": max(1, min(5, importance)),
    }


# ---------------------------------------------------------------------------
# Freshness
# ---------------------------------------------------------------------------


def memory_age_days(mtime_ms: float) -> int:
    """Floor-rounded days since modification."""
    return max(0, int((datetime.now(timezone.utc).timestamp() * 1000 - mtime_ms) / 86_400_000))


def memory_age(mtime_ms: float) -> str:
    """Human-readable age string."""
    d = memory_age_days(mtime_ms)
    if d == 0:
        return "today"
    if d == 1:
        return "yesterday"
    return f"{d} days ago"


def memory_freshness_text(mtime_ms: float) -> str:
    """Staleness caveat for memories >1 day old.  Returns '' for fresh."""
    d = memory_age_days(mtime_ms)
    if d <= 1:
        return ""
    return (
        f"This memory is {d} days old. "
        f"Memories are point-in-time observations, not live state — "
        f"claims about code behavior or file:line citations may be outdated. "
        f"Verify against current code before asserting as fact."
    )


def freshness(date_str: str) -> str:
    """Return 'N days ago' (or 'today'/'yesterday') for an ISO date string."""
    if not date_str:
        return "unknown"
    try:
        d = date.fromisoformat(date_str)
        delta = (date.today() - d).days
    except (ValueError, TypeError):
        return "unknown"
    if delta < 0:
        return "unknown"
    if delta == 0:
        return "today"
    if delta == 1:
        return "yesterday"
    return f"{delta} days ago"


# ---------------------------------------------------------------------------
# MemoryStore
# ---------------------------------------------------------------------------


class MemoryScanResult:
    """Header metadata from a scanned memory file."""

    __slots__ = ("filename", "filepath", "mtime_ms", "description", "category", "tags", "importance")

    def __init__(
        self,
        filename: str,
        filepath: str,
        mtime_ms: float,
        description: str | None = None,
        category: str | None = None,
        tags: list[str] | None = None,
        importance: int = 3,
    ):
        self.filename = filename
        self.filepath = filepath
        self.mtime_ms = mtime_ms
        self.description = description
        self.category = category
        self.tags = tags or []
        self.importance = importance


class MemoryStore:
    """File-based persistent memory with YAML frontmatter.

    Thread-safe: all public write methods acquire ``self._lock``.
    Reads are protected by the same lock to ensure consistency.
    """

    def __init__(self, memory_dir: str | Path | None = None):
        self._memory_dir = Path(memory_dir or MEMORY_HOME)
        self._lock = threading.Lock()
        self._ensure_dirs()

    # -- directory layout --------------------------------------------------

    def _ensure_dirs(self) -> None:
        """Create the memory hierarchy if it doesn't exist."""
        self._memory_dir.mkdir(parents=True, exist_ok=True)
        for cat in CATEGORIES:
            (self._memory_dir / cat).mkdir(parents=True, exist_ok=True)

    def _category_path(self, category: str) -> Path:
        p = self._memory_dir / category
        p.mkdir(parents=True, exist_ok=True)
        return p

    def _topic_path(self, category: str, topic: str) -> Path:
        name = topic if topic.endswith(".md") else f"{topic}.md"
        return self._category_path(category) / name

    def _entrypoint_path(self) -> Path:
        return self._memory_dir / ENTRYPOINT_NAME

    @property
    def memory_dir(self) -> Path:
        return self._memory_dir

    # -- index (MEMORY.md) -------------------------------------------------

    def _read_index(self) -> list[dict[str, str]]:
        """Parse MEMORY.md entries.  Returns list of ``{title, path, hook}``."""
        path = self._entrypoint_path()
        if not path.exists():
            return []

        entries: list[dict[str, str]] = []
        # Match: - [Title](path/to/file.md) — hook text
        pattern = re.compile(r"^\s*-\s+\[([^\]]+)\]\(([^)]+)\)\s*(?:[—–-]+)?\s*(.*)")
        try:
            for line in path.read_text(encoding="utf-8").splitlines():
                m = pattern.match(line)
                if m:
                    entries.append({"title": m.group(1), "path": m.group(2), "hook": m.group(3).strip()})
        except OSError:
            pass
        return entries

    def _write_index(self, entries: list[dict[str, str]]) -> None:
        """Write MEMORY.md from entry dicts."""
        lines: list[str] = []
        for e in entries:
            hook = f" — {e['hook']}" if e.get("hook") else ""
            lines.append(f"- [{e['title']}]({e['path']}){hook}")
        content = "\n".join(lines) + "\n" if lines else ""
        try:
            self._entrypoint_path().write_text(content, encoding="utf-8")
        except OSError as e:
            logger.warning("Failed to write MEMORY.md: %s", e)

    def _add_to_index(self, title: str, rel_path: str, hook: str = "") -> None:
        """Append an entry to MEMORY.md."""
        entries = self._read_index()
        # Avoid duplicates by path
        entries = [e for e in entries if e["path"] != rel_path]
        entries.append({"title": title, "path": rel_path, "hook": hook})
        self._write_index(entries)

    def _remove_from_index(self, rel_path: str) -> None:
        """Remove an entry from MEMORY.md by relative path."""
        entries = self._read_index()
        entries = [e for e in entries if e["path"] != rel_path]
        self._write_index(entries)

    # -- CRUD ---------------------------------------------------------------

    def save(
        self,
        category: str,
        topic: str,
        content: str,
        *,
        title: str | None = None,
        tags: list[str] | None = None,
        related: list[str] | None = None,
        importance: int = 3,
        frontmatter: dict | None = None,
    ) -> dict:
        """Save or update a memory file.

        Args:
            category: One of ``user``, ``feedback``, ``project``, ``reference``.
            topic: Slug-like topic name (e.g. ``testing-preferences``).
            content: Markdown body.
            title: Display title (defaults to topic).
            tags: Optional list of tags.
            related: Optional list of related topic filenames.
            importance: 1-5 importance scale.
            frontmatter: Override full frontmatter dict (takes precedence).

        Returns:
            ``{"path": ..., "entrypoint_path": ...}``
        """
        category = category.lower()
        if category not in CATEGORIES:
            return {"error": f"Invalid category '{category}'. Choose from: {', '.join(CATEGORIES)}"}

        topic = topic.strip().replace(" ", "-").lower()
        if not topic:
            return {"error": "Topic cannot be empty."}

        title = title or topic
        fm = _make_frontmatter(category, tags, related, importance) if frontmatter is None else frontmatter
        fm["type"] = category

        doc = f"{_build_frontmatter(fm)}\n\n{content.strip()}\n"

        with self._lock:
            path = self._topic_path(category, topic)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(doc, encoding="utf-8")

            rel_path = str(path.relative_to(self._memory_dir))
            self._add_to_index(title, rel_path, hook=content.split("\n")[0][:120] if content.strip() else "")

        return {
            "path": str(path),
            "entrypoint_path": str(self._entrypoint_path()),
        }

    def read(self, category: str, topic: str) -> dict | None:
        """Read a memory file.

        Returns ``{"frontmatter": ..., "body": ..., "path": ..., "freshness": ...}``
        or ``None`` if not found.
        """
        path = self._topic_path(category, topic)
        if not path.exists():
            return None

        with self._lock:
            text = path.read_text(encoding="utf-8")
            mtime_ms = path.stat().st_mtime * 1000

        fm, body = _parse_frontmatter(text)

        return {
            "frontmatter": fm,
            "body": body,
            "path": str(path),
            "freshness": memory_age(mtime_ms),
            "freshness_text": memory_freshness_text(mtime_ms),
        }

    def delete(self, category: str, topic: str) -> bool:
        """Delete a memory file and remove it from the index."""
        with self._lock:
            path = self._topic_path(category, topic)
            if not path.exists():
                return False
            path.unlink()
            rel_path = str(path.relative_to(self._memory_dir))
            self._remove_from_index(rel_path)
        return True

    def list_category(self, category: str) -> list[dict]:
        """List all memories in a category.

        Returns list of ``{"topic", "path", "title", "hook", "mtime_ms", "freshness"}``.
        """
        cat_path = self._category_path(category)
        if not cat_path.is_dir():
            return []

        results: list[dict] = []
        with self._lock:
            for f in sorted(cat_path.iterdir()):
                if not f.name.endswith(".md"):
                    continue
                # Read frontmatter only (first ~50 lines)
                text = f.read_text(encoding="utf-8")
                fm, body = _parse_frontmatter(text)
                mtime_ms = f.stat().st_mtime * 1000

                first_line = body.strip().split("\n")[0] if body.strip() else ""
                results.append({
                    "topic": f.stem,
                    "path": str(f),
                    "title": fm.get("name", f.stem),
                    "description": fm.get("description", first_line[:120]),
                    "tags": fm.get("tags", []),
                    "importance": fm.get("importance", 3),
                    "date": fm.get("date", ""),
                    "freshness": freshness(fm.get("date", "")),
                    "mtime_ms": mtime_ms,
                })

        return results

    def list_all(self) -> dict[str, list[dict]]:
        """List all memories grouped by category."""
        return {cat: self.list_category(cat) for cat in CATEGORIES}

    def get_index(self) -> dict:
        """Return summary of all stored memories.

        Returns ``{"categories": ..., "total": N, "entrypoint": ...}``.
        """
        by_cat = self.list_all()
        total = sum(len(v) for v in by_cat.values())

        index_entries = self._read_index()

        return {
            "categories": {cat: len(by_cat[cat]) for cat in CATEGORIES},
            "total": total,
            "index_entries": len(index_entries),
            "entrypoint": str(self._entrypoint_path()),
            "memory_dir": str(self._memory_dir),
        }

    def scan_all(self) -> list[MemoryScanResult]:
        """Scan ALL memory files (all categories), sorted newest-first.

        Returns a flat list of ``MemoryScanResult``.
        """
        results: list[MemoryScanResult] = []

        with self._lock:
            for cat in CATEGORIES:
                cat_path = self._category_path(cat)
                if not cat_path.is_dir():
                    continue
                for f in sorted(cat_path.iterdir()):
                    if not f.name.endswith(".md"):
                        continue
                    try:
                        text = f.read_text(encoding="utf-8")
                        fm, _ = _parse_frontmatter(text)
                        stat = f.stat()
                        results.append(MemoryScanResult(
                            filename=str(f.relative_to(self._memory_dir)),
                            filepath=str(f),
                            mtime_ms=stat.st_mtime * 1000,
                            description=fm.get("description") or fm.get("name"),
                            category=fm.get("type", cat),
                            tags=fm.get("tags", []),
                            importance=fm.get("importance", 3),
                        ))
                    except OSError:
                        continue

        results.sort(key=lambda r: r.mtime_ms, reverse=True)
        return results

    def format_manifest(self, memories: list[MemoryScanResult]) -> str:
        """Format a list of ``MemoryScanResult`` as a text manifest."""
        lines: list[str] = []
        for m in memories:
            tag = f"[{m.category}] " if m.category else ""
            ts = datetime.fromtimestamp(m.mtime_ms / 1000, tz=timezone.utc).isoformat()
            desc = m.description or ""
            lines.append(f"- {tag}{m.filename} ({ts}): {desc}")
        return "\n".join(lines)


# ---------------------------------------------------------------------------
# MemoryIndex
# ---------------------------------------------------------------------------


class MemoryIndex:
    """Read-side index for querying memories by relevance, scanning, and
    freshness tracking.

    ``find_relevant`` uses simple keyword overlap scoring (TF-like) as the
    default strategy.  The architecture supports swapping in an LLM-based
    selector — inject a custom ``relevance_fn`` at construction time.
    """

    def __init__(
        self,
        store: MemoryStore,
        relevance_fn: callable | None = None,
    ):
        self._store = store
        # relevance_fn(query, memories) -> list[MemoryScanResult]
        self._relevance_fn = relevance_fn or self._default_relevance

    def scan(self, path: str | None = None) -> list[MemoryScanResult]:
        """Scan .md files with frontmatter parsing.

        Args:
            path: Optional subdirectory to scan (e.g. ``user/``).
                  Defaults to full memory directory.
        """
        if path is None:
            return self._store.scan_all()

        scan_path = Path(path)
        if not scan_path.is_absolute():
            scan_path = self._store.memory_dir / scan_path

        if not scan_path.exists():
            return []

        results: list[MemoryScanResult] = []

        if scan_path.is_file() and scan_path.suffix == ".md":
            # Single file scan
            try:
                text = scan_path.read_text(encoding="utf-8")
                fm, _ = _parse_frontmatter(text)
                stat = scan_path.stat()
                rel = scan_path.relative_to(self._store.memory_dir)
                results.append(MemoryScanResult(
                    filename=str(rel),
                    filepath=str(scan_path),
                    mtime_ms=stat.st_mtime * 1000,
                    description=fm.get("description") or fm.get("name"),
                    category=fm.get("type"),
                    tags=fm.get("tags", []),
                    importance=fm.get("importance", 3),
                ))
            except OSError:
                pass
        elif scan_path.is_dir():
            with self._store._lock:
                for f in sorted(scan_path.glob("**/*.md")):
                    if f.name == ENTRYPOINT_NAME:
                        continue
                    try:
                        text = f.read_text(encoding="utf-8")
                        fm, _ = _parse_frontmatter(text)
                        stat = f.stat()
                        rel = f.relative_to(self._store.memory_dir)
                        results.append(MemoryScanResult(
                            filename=str(rel),
                            filepath=str(f),
                            mtime_ms=stat.st_mtime * 1000,
                            description=fm.get("description") or fm.get("name"),
                            category=fm.get("type"),
                            tags=fm.get("tags", []),
                            importance=fm.get("importance", 3),
                        ))
                    except OSError:
                        continue
            results.sort(key=lambda r: r.mtime_ms, reverse=True)

        return results

    def find_relevant(self, query: str, top_k: int = 5) -> list[MemoryScanResult]:
        """Find memories relevant to a query.

        Default strategy: keyword overlap scoring against filename,
        description, and tags.  Returns up to ``top_k`` results.

        Pass a custom ``relevance_fn`` to the constructor for LLM-based
        selection (see ``findRelevantMemories`` in the original TS source).
        """
        memories = self._store.scan_all()
        if not memories:
            return []

        return self._relevance_fn(query, memories)[:top_k]

    def freshness(self, date_str: str) -> str:
        """Return a human-readable age string for an ISO date."""
        return freshness(date_str)

    # -- default relevance strategy (keyword overlap) ----------------------

    @staticmethod
    def _default_relevance(query: str, memories: list[MemoryScanResult]) -> list[MemoryScanResult]:
        """Score memories by keyword overlap with query.

        Weighted score:
          - description/name match:  +3 per keyword
          - tags match:              +2 per keyword
          - filename match:          +1 per keyword
        """
        if not query.strip():
            return memories

        tokens = set(
            t.lower().strip(".,!?;:#$%^&*()[]{}<>/\\|-_\"'")
            for t in query.split()
            if len(t) > 1
        )
        if not tokens:
            return memories

        scored: list[tuple[float, MemoryScanResult]] = []
        for m in memories:
            score = 0.0
            text_for_search = (
                (m.description or "").lower()
                + " "
                + m.filename.lower()
                + " "
                + " ".join(m.tags).lower()
            )

            for token in tokens:
                if token in text_for_search:
                    if m.description and token in m.description.lower():
                        score += 3.0
                    if any(token in t.lower() for t in m.tags):
                        score += 2.0
                    if token in m.filename.lower():
                        score += 1.0
                    else:
                        score += 0.5

            # Boost by importance (0.2 per level above 3)
            score += max(0, m.importance - 3) * 0.2

            if score > 0:
                scored.append((score, m))

        scored.sort(key=lambda x: (-x[0], -x[1].mtime_ms))
        return [m for _, m in scored]


# ---------------------------------------------------------------------------
# Tool schemas
# ---------------------------------------------------------------------------

MEMORIZE_SCHEMA = {
    "name": "memorize",
    "description": (
        "Save a memory to persistent file-based storage. "
        "Memories survive across sessions and are organized into four categories:\n\n"
        "- **user**: preferences, role, goals, communication style, personal details\n"
        "- **feedback**: corrections, guidance on approach, things to keep/stop doing\n"
        "- **project**: ongoing work, deadlines, initiatives, non-obvious context\n"
        "- **reference**: pointers to external systems (dashboards, docs, channels)\n\n"
        "Each memory is stored as a Markdown file with YAML frontmatter and "
        "indexed in MEMORY.md. "
        "Use this when the user says 'remember this', corrects you, shares a "
        "preference, or when you learn something that will be useful in future sessions.\n\n"
        "Do NOT save: code structure (derivable from code), git history, "
        "ephemeral task state, or things already in CLAUDE.md."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["save", "delete", "read"],
                "description": "Action: save (create/update), delete (remove), read (retrieve).",
            },
            "category": {
                "type": "string",
                "enum": ["user", "feedback", "project", "reference"],
                "description": "Memory category.",
            },
            "topic": {
                "type": "string",
                "description": "Slug-like topic name (e.g. 'testing-preferences', 'auth-middleware-compliance'). Used as the filename.",
            },
            "content": {
                "type": "string",
                "description": "Markdown body content. Required for 'save' action.",
            },
            "title": {
                "type": "string",
                "description": "Display title (defaults to topic).",
            },
            "tags": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Optional tags for search.",
            },
            "importance": {
                "type": "integer",
                "description": "Importance 1-5 (default 3). Higher = more likely to be recalled.",
            },
        },
        "required": ["action", "category", "topic"],
    },
}

RECALL_SCHEMA = {
    "name": "recall",
    "description": (
        "Search memories for information relevant to a query. "
        "Returns up to 5 matching memories with their frontmatter metadata "
        "and freshness information.\n\n"
        "Use this when you need to remember past context, user preferences, "
        "or decisions made in earlier sessions. "
        "Results include staleness warnings for memories older than 1 day."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "Search query describing what you're looking for.",
            },
            "category": {
                "type": "string",
                "enum": ["user", "feedback", "project", "reference"],
                "description": "Optional: narrow search to a single category.",
            },
        },
        "required": ["query"],
    },
}

MEMORY_INDEX_SCHEMA = {
    "name": "memory_index",
    "description": (
        "Show the memory index: list all memories grouped by category, "
        "with counts, per-file freshness, and tags. "
        "Use this to get an overview of what is stored in memory "
        "before deciding which memories to read in detail or update."
    ),
    "parameters": {
        "type": "object",
        "properties": {},
        "required": [],
    },
}

# ---------------------------------------------------------------------------
# Shared store singleton
# ---------------------------------------------------------------------------

_store: MemoryStore | None = None
_index: MemoryIndex | None = None


def _get_store() -> MemoryStore:
    global _store
    if _store is None:
        _store = MemoryStore()
    return _store


def _get_index() -> MemoryIndex:
    global _index
    if _index is None:
        _index = MemoryIndex(_get_store())
    return _index


# ---------------------------------------------------------------------------
# Tool handlers
# ---------------------------------------------------------------------------


def _memorize_handler(args: dict, **kw) -> str:
    """Handle the ``memorize`` tool."""
    store = _get_store()
    action = args.get("action", "save")
    category = args.get("category", "")
    topic = args.get("topic", "")

    if action == "save":
        content = (args.get("content") or "").strip()
        if not content:
            return tool_error("content is required for 'save' action.")
        result = store.save(
            category=category,
            topic=topic,
            content=content,
            title=args.get("title"),
            tags=args.get("tags"),
            importance=args.get("importance", 3),
        )
        if "error" in result:
            return tool_error(result["error"])
        return tool_result({
            "action": "saved",
            "path": result["path"],
            "category": category,
            "topic": topic,
        })

    elif action == "read":
        result = store.read(category, topic)
        if result is None:
            return tool_error(f"Memory not found: {category}/{topic}")
        return tool_result(result)

    elif action == "delete":
        ok = store.delete(category, topic)
        if not ok:
            return tool_error(f"Memory not found: {category}/{topic}")
        return tool_result({"action": "deleted", "category": category, "topic": topic})

    return tool_error(f"Unknown action '{action}'. Use: save, read, delete.")


def _recall_handler(args: dict, **kw) -> str:
    """Handle the ``recall`` tool."""
    index = _get_index()
    query = (args.get("query") or "").strip()
    if not query:
        return tool_error("query is required.")

    category = args.get("category")

    if category:
        memories = index.scan(path=str(MemoryStore()._category_path(category)))
        relevant = MemoryIndex._default_relevance(query, memories)[:5]
    else:
        relevant = index.find_relevant(query, top_k=5)

    results = []
    for m in relevant:
        # Read full content for matched items
        fm, body = _parse_frontmatter(Path(m.filepath).read_text(encoding="utf-8")) if Path(m.filepath).exists() else ({}, "")
        results.append({
            "filename": m.filename,
            "path": m.filepath,
            "description": m.description or "",
            "category": m.category or "",
            "tags": fm.get("tags", []),
            "importance": fm.get("importance", 3),
            "freshness": memory_age(m.mtime_ms),
            "freshness_text": memory_freshness_text(m.mtime_ms),
            "body": body[:500] + ("..." if len(body) > 500 else ""),
        })

    if not results:
        return tool_result({
            "query": query,
            "count": 0,
            "results": [],
            "message": "No relevant memories found.",
        })

    return tool_result({
        "query": query,
        "count": len(results),
        "results": results,
    })


def _memory_index_handler(args: dict, **kw) -> str:
    """Handle the ``memory_index`` tool."""
    store = _get_store()
    idx = store.get_index()
    by_cat = store.list_all()

    categories = {}
    for cat in CATEGORIES:
        items = by_cat.get(cat, [])
        categories[cat] = {
            "count": len(items),
            "memories": [
                {
                    "topic": i["topic"],
                    "title": i["title"],
                    "freshness": i["freshness"],
                    "tags": i["tags"],
                    "importance": i["importance"],
                }
                for i in items
            ],
        }

    return tool_result({
        "total": idx["total"],
        "memory_dir": idx["memory_dir"],
        "entrypoint": idx["entrypoint"],
        "index_entries": idx["index_entries"],
        "categories": categories,
    })


# ---------------------------------------------------------------------------
# Requirements check
# ---------------------------------------------------------------------------


def _check_memdir_requirements() -> bool:
    """PyYAML is the only dependency — always available in Dxrk env."""
    try:
        import yaml  # noqa: F401
        return True
    except ImportError:
        return False


# ---------------------------------------------------------------------------
# Registry registration
# ---------------------------------------------------------------------------

registry.register(
    name="memorize",
    toolset="memdir",
    schema=MEMORIZE_SCHEMA,
    handler=_memorize_handler,
    check_fn=_check_memdir_requirements,
    emoji="💾",
    is_destructive=False,
    is_concurrency_safe=False,
)

registry.register(
    name="recall",
    toolset="memdir",
    schema=RECALL_SCHEMA,
    handler=_recall_handler,
    check_fn=_check_memdir_requirements,
    emoji="🔍",
    is_destructive=False,
    is_concurrency_safe=False,
)

registry.register(
    name="memory_index",
    toolset="memdir",
    schema=MEMORY_INDEX_SCHEMA,
    handler=_memory_index_handler,
    check_fn=_check_memdir_requirements,
    emoji="📇",
    is_destructive=False,
    is_concurrency_safe=False,
)
