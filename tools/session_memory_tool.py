# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Session Memory Tool — Auto-extracting Session Notes

Port of Claude Code's SessionMemory system. Maintains structured Markdown notes
per session in ~/.dxrk/session-notes/{session_id}.md. An LLM sub-agent extracts
decisions, code patterns, bugs, config changes, and other key information from
the conversation, then writes structured notes with timestamps.

Design:
- Single `session_note` tool with three actions: extract, list, read
- SessionNoteKeeper: thread-safe note storage with threading.Lock
- Inline LLM extraction (no forked agent — Dxrk uses async_call_llm directly)
"""

import asyncio
import json
import logging
import os
import re
import threading
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, Any, List

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
SESSION_NOTES_DIR = DXRK_HOME / "session-notes"

MAX_SECTION_LENGTH = 2000
MAX_TOTAL_NOTE_TOKENS = 12000

EXTRACTION_SYSTEM_PROMPT = """You are an expert session note-taker. Your task is to review the conversation above and extract key information into a structured markdown note.

DO NOT include any meta-commentary about "note-taking", "extraction", or these instructions. Write directly in the voice of the session.

Focus on extracting:

## Decisions
Architecture, design, or approach decisions. What was decided AND why.

## Code & Patterns
Important code snippets, design patterns, file structures, or naming conventions established.

## Bugs & Fixes
Bugs encountered, root causes, and how they were fixed. Include exact error messages.

## Config Changes
Environment setup, dependency changes, tool configuration, or build system changes.

## Commands
Important commands run — installs, builds, tests, deploys. Include flags and context.

## Findings
Technical discoveries, non-obvious behavior, unexpected interactions.

## Next Steps
What remains to be done. Pending tasks, known issues, follow-up work.

## Summary
A 2-3 sentence TL;DR of what this session accomplished.

Guidelines:
- Be specific: include paths, function names, error messages, exact commands
- Prioritize actionable, non-obvious information
- Skip trivial details (ls, cd, basic git operations)
- If a section has nothing to add, write "—" (em-dash) as a placeholder
- Use markdown formatting: code blocks for code/commands, bold for emphasis
"""


# ---------------------------------------------------------------------------
# SessionNoteKeeper
# ---------------------------------------------------------------------------

class SessionNoteKeeper:
    """
    Thread-safe session note storage.

    Each session gets one Markdown file at:
        ~/.dxrk/session-notes/{session_id}.md

    Notes accumulate across the session — extraction APPENDS to the file
    rather than replacing it, with timestamps for each extraction event.
    """

    def __init__(self):
        self._lock = threading.Lock()
        SESSION_NOTES_DIR.mkdir(parents=True, exist_ok=True)

    # ------------------------------------------------------------------
    # Path resolution
    # ------------------------------------------------------------------

    def _note_path(self, session_id: str) -> Path:
        """Return the filesystem path for a given session's note file."""
        safe = re.sub(r'[^a-zA-Z0-9_.-]', '_', session_id) if session_id else "unknown"
        return SESSION_NOTES_DIR / f"{safe}.md"

    # ------------------------------------------------------------------
    # Read / Write
    # ------------------------------------------------------------------

    def read(self, session_id: str) -> str:
        """Return the full content of a session's note file, or empty string."""
        path = self._note_path(session_id)
        with self._lock:
            if not path.exists():
                return ""
            try:
                return path.read_text(encoding="utf-8")
            except (OSError, IOError) as e:
                logger.warning("Failed to read session note %s: %s", session_id, e)
                return ""

    def write(self, session_id: str, content: str):
        """Write content to a session's note file (atomic replace)."""
        path = self._note_path(session_id)
        with self._lock:
            path.parent.mkdir(parents=True, exist_ok=True)
            try:
                import tempfile
                fd, tmp = tempfile.mkstemp(
                    dir=str(path.parent), suffix=".tmp", prefix=".note_"
                )
                try:
                    with os.fdopen(fd, "w", encoding="utf-8") as f:
                        f.write(content)
                        f.flush()
                        os.fsync(f.fileno())
                    os.replace(tmp, path)
                except BaseException:
                    try:
                        os.unlink(tmp)
                    except OSError:
                        pass
                    raise
            except (OSError, IOError) as e:
                raise RuntimeError(f"Failed to write session note {path}: {e}")

    def append_extraction(self, session_id: str, extracted: str):
        """
        Append a new extraction block to the note file.

        If the file doesn't exist yet, write the extracted content directly.
        If it does, append with a timestamped separator.
        """
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
        block = (
            f"\n\n---\n"
            f"## Extraction — {now}\n\n"
            f"{extracted.strip()}\n"
        )

        with self._lock:
            existing = self.read(session_id)
            if not existing:
                header = (
                    f"# Session Notes\n\n"
                    f"_Auto-generated session memory. Each extraction appends "
                    f"a timestamped block._\n\n"
                    f"---\n"
                    f"## Extraction — {now}\n\n"
                    f"{extracted.strip()}\n"
                )
                self.write(session_id, header)
            else:
                self.write(session_id, existing.rstrip() + "\n" + block)

    # ------------------------------------------------------------------
    # List
    # ------------------------------------------------------------------

    def list_notes(self) -> List[Dict[str, Any]]:
        """Return metadata for all existing note files, newest first."""
        results = []
        with self._lock:
            if not SESSION_NOTES_DIR.exists():
                return results
            for path in sorted(SESSION_NOTES_DIR.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True):
                if path.suffix != ".md":
                    continue
                session_id = path.stem
                size = path.stat().st_size
                mtime = datetime.fromtimestamp(
                    path.stat().st_mtime, tz=timezone.utc
                ).strftime("%Y-%m-%d %H:%M:%S UTC")
                preview = ""
                if size > 0:
                    try:
                        first_line = path.read_text(encoding="utf-8", errors="replace").split("\n")[0]
                        preview = first_line.strip()
                    except OSError:
                        pass
                results.append({
                    "session_id": session_id,
                    "size_chars": size,
                    "last_modified": mtime,
                    "preview": preview,
                })
        return results


# ---------------------------------------------------------------------------
# LLM Extraction
# ---------------------------------------------------------------------------

async def extract_session_notes(
    messages: List[Dict[str, Any]],
    session_id: str,
    existing_notes: str = "",
) -> str:
    """
    Extract key information from conversation messages using the auxiliary LLM.

    Args:
        messages: List of conversation message dicts with 'role' and 'content'.
        session_id: The current session identifier.
        existing_notes: Any existing notes for this session (for context).

    Returns:
        Structured Markdown with extracted information.
    """
    conversation_text = _format_conversation_for_extraction(messages)

    if existing_notes:
        existing_block = (
            f"\n\nEXISTING SESSION NOTES (for context — update or extend):\n"
            f"{existing_notes}\n"
        )
    else:
        existing_block = "\n\n(No existing notes for this session yet.)\n"

    user_prompt = (
        f"Session ID: {session_id}\n"
        f"{existing_block}\n"
        f"CONVERSATION TO EXTRACT FROM:\n"
        f"{conversation_text}\n\n"
        f"Extract structured notes from the above conversation. "
        f"Focus on decisions, code patterns, bugs, config changes, "
        f"commands, findings, and next steps."
    )

    try:
        from agent.auxiliary_client import async_call_llm, extract_content_or_reasoning
    except ImportError:
        logger.warning("auxiliary_client not available — cannot extract session notes")
        return _fallback_extraction(messages, session_id)

    max_retries = 2
    for attempt in range(max_retries):
        try:
            response = await async_call_llm(
                task="session_memory",
                messages=[
                    {"role": "system", "content": EXTRACTION_SYSTEM_PROMPT},
                    {"role": "user", "content": user_prompt},
                ],
                temperature=0.1,
                max_tokens=MAX_TOTAL_NOTE_TOKENS,
            )
            content = extract_content_or_reasoning(response)
            if content:
                return content
            logger.warning("Session extraction LLM returned empty (attempt %d/%d)", attempt + 1, max_retries)
            if attempt < max_retries - 1:
                await asyncio.sleep(1 * (attempt + 1))
        except RuntimeError:
            logger.warning("No auxiliary model available for session extraction")
            return _fallback_extraction(messages, session_id)
        except Exception as e:
            if attempt < max_retries - 1:
                logger.warning("Session extraction attempt %d failed: %s", attempt + 1, e)
                await asyncio.sleep(1 * (attempt + 1))
            else:
                logger.error("Session extraction failed after %d attempts: %s", max_retries, e)
                return _fallback_extraction(messages, session_id)

    return _fallback_extraction(messages, session_id)


def _format_conversation_for_extraction(messages: List[Dict[str, Any]]) -> str:
    """Format messages into a condensed transcript for extraction."""
    parts = []
    for msg in messages:
        role = msg.get("role", "unknown").upper()
        content = msg.get("content", "") or ""
        tool_name = msg.get("tool_name")

        if role == "TOOL" and tool_name:
            if len(content) > 800:
                content = content[:400] + "\n...[truncated]...\n" + content[-400:]
            parts.append(f"[TOOL:{tool_name}]: {content}")
        elif role == "ASSISTANT":
            tool_calls = msg.get("tool_calls")
            if tool_calls and isinstance(tool_calls, list):
                tc_names = [tc.get("name") or tc.get("function", {}).get("name", "?") for tc in tool_calls if isinstance(tc, dict)]
                if tc_names:
                    parts.append(f"[ASSISTANT]: [Called: {', '.join(tc_names)}]")
            if content:
                parts.append(f"[ASSISTANT]: {content}")
        else:
            if content:
                parts.append(f"[{role}]: {content}")

    return "\n\n".join(parts)


def _fallback_extraction(messages: List[Dict[str, Any]], session_id: str) -> str:
    """
    Simple rule-based fallback when the LLM is unavailable.
    Extracts tool names, file paths, and command patterns.
    """
    findings = []
    tool_pattern = re.compile(r'\[TOOL:(\w+)\]')
    file_pattern = re.compile(r'[\w./-]+\.\w{1,6}')
    seen_tools = set()
    seen_files = set()

    for msg in messages:
        role = msg.get("role", "").upper()
        content = msg.get("content", "") or ""

        if role == "TOOL":
            tool_name = msg.get("tool_name", "")
            if tool_name and tool_name not in seen_tools:
                seen_tools.add(tool_name)
                findings.append(f"- Tool used: {tool_name}")

        for m in tool_pattern.finditer(content):
            name = m.group(1)
            if name not in seen_tools:
                seen_tools.add(name)
                findings.append(f"- Tool called: {name}")

        for m in file_pattern.finditer(content):
            path = m.group(0)
            if len(path) > 5 and path not in seen_files:
                seen_files.add(path)
                findings.append(f"- File referenced: {path}")

    findings_block = "\n".join(findings[:30]) if findings else "- (none detected)"

    parts = [
        "## Summary",
        f"_Fallback extraction — LLM was unavailable_",
        "",
        "### Tools Used",
        findings_block,
        "",
        "### Raw Messages",
        f"{len(messages)} messages in this extraction window.",
    ]

    return "\n".join(parts)


# ---------------------------------------------------------------------------
# Tool Handler
# ---------------------------------------------------------------------------

_session_note_store = None
_session_note_lock = threading.Lock()


def _get_store() -> SessionNoteKeeper:
    """Lazy-init singleton SessionNoteKeeper."""
    global _session_note_store
    if _session_note_store is None:
        with _session_note_lock:
            if _session_note_store is None:
                _session_note_store = SessionNoteKeeper()
    return _session_note_store


def session_note_handler(args: Dict[str, Any], **kw) -> str:
    """
    Handle session_note tool calls.

    Actions:
      - extract: Run LLM extraction on the conversation and append notes.
      - list:    List all note files with metadata.
      - read:    Read a specific session's note file.

    The "extract" action bridges sync → async via model_tools._run_async
    because it calls the auxiliary LLM.
    """
    action = args.get("action", "").strip()
    if not action:
        return tool_error("'action' is required (extract, list, or read)")

    store = _get_store()

    if action == "list":
        notes = store.list_notes()
        return json.dumps({
            "success": True,
            "notes": notes,
            "count": len(notes),
        }, ensure_ascii=False)

    if action == "read":
        session_id = args.get("session_id", "").strip()
        if not session_id:
            return tool_error("'session_id' is required for 'read' action")
        content = store.read(session_id)
        if not content:
            return json.dumps({
                "success": True,
                "session_id": session_id,
                "content": "",
                "message": "No notes found for this session.",
            }, ensure_ascii=False)
        return json.dumps({
            "success": True,
            "session_id": session_id,
            "content": content,
            "size_chars": len(content),
            "modified": datetime.fromtimestamp(
                store._note_path(session_id).stat().st_mtime, tz=timezone.utc
            ).strftime("%Y-%m-%d %H:%M:%S UTC") if store._note_path(session_id).exists() else "unknown",
        }, ensure_ascii=False)

    if action == "extract":
        session_id = args.get("session_id", "").strip()
        if not session_id:
            return tool_error("'session_id' is required for 'extract' action")

        messages = kw.get("messages") or args.get("messages", [])
        if not messages:
            return tool_error("No messages provided for extraction")

        existing = store.read(session_id)

        async def _do_extract() -> str:
            return await extract_session_notes(messages, session_id, existing)

        try:
            from model_tools import _run_async
            extracted = _run_async(_do_extract())
        except Exception as e:
            logger.error("Session note extraction failed: %s", e, exc_info=True)
            return tool_error(f"Extraction failed: {e}")

        store.append_extraction(session_id, extracted)
        return json.dumps({
            "success": True,
            "session_id": session_id,
            "message": "Session notes extracted and saved.",
            "content_length": len(extracted),
        }, ensure_ascii=False)

    return tool_error(f"Unknown action '{action}'. Use: extract, list, read")


def check_session_memory_requirements() -> bool:
    """Session memory has no external requirements — always available."""
    return True


# =============================================================================
# OpenAI Function-Calling Schema
# =============================================================================

SESSION_NOTE_SCHEMA = {
    "name": "session_note",
    "description": (
        "Maintain structured Markdown notes for the current session. "
        "Notes persist across the session and capture key information:\n\n"
        "ACTIONS:\n"
        "- extract: Run LLM extraction on recent conversation context. "
        "Identifies decisions, code patterns, bugs/fixes, config changes, commands, "
        "findings, and next steps. Appends a timestamped block to the note file.\n"
        "- list: List all session note files with metadata (size, last modified, preview).\n"
        "- read: Read a specific session's full note file.\n\n"
        "Use 'extract' after significant progress, solving a hard bug, making a "
        "key decision, or when context is large. Notes help recover state after "
        "compression or across sessions."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["extract", "list", "read"],
                "description": "The action to perform: extract (LLM extraction), list (all notes), read (one session).",
            },
            "session_id": {
                "type": "string",
                "description": "Session identifier. Required for 'extract' and 'read'.",
            },
        },
        "required": ["action"],
    },
}


# --- Registry ---
from tools.registry import registry, tool_error

registry.register(
    name="session_note",
    toolset="system",
    schema=SESSION_NOTE_SCHEMA,
    handler=session_note_handler,
    check_fn=check_session_memory_requirements,
    is_async=False,
    emoji="📝",
)
