# SPDX-License-Identifier: MIT
"""Context compaction system — port of Claude Code's compact module.

Provides LLM-powered conversation summarization, auto-trigger, micro-compaction
(clearing old tool results), and session-memory compaction.  Uses Dxrk's
existing LLM client (async_call_llm) for cheap/fast summarization.

Messages format: OpenAI-compatible list of dicts with ``role`` / ``content`` keys.
"""

import asyncio
import json
import logging
import os
import re
import threading
import time
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

from agent.auxiliary_client import async_call_llm, extract_content_or_reasoning
from agent.model_metadata import estimate_messages_tokens_rough, estimate_tokens_rough
from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

COMPACT_OUTPUT_MAX_TOKENS = 4096
COMPACT_DEFAULT_THRESHOLD = 0.80  # fraction of context window
AUTOCOMPACT_BUFFER_TOKENS = 13_000
WARNING_THRESHOLD_BUFFER_TOKENS = 20_000
ERROR_THRESHOLD_BUFFER_TOKENS = 20_000
MANUAL_COMPACT_BUFFER_TOKENS = 3_000
MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES = 3
MICROCOMPACT_MAX_AGE_ROUNDS = 5
SESSION_MEMORY_MIN_TOKENS = 10_000
SESSION_MEMORY_MIN_TEXT_BLOCK_MESSAGES = 5
SESSION_MEMORY_MAX_TOKENS = 40_000
POST_COMPACT_MAX_FILES_TO_RESTORE = 5

# Tokens freed per micro-compacted tool result (placeholder replacement)
MICROCOMPACT_TOKENS_SAVED_PER_RESULT = 250

# Tools whose results are safe to clear during micro-compact
_COMPACTABLE_TOOLS = frozenset({
    "read", "bash", "grep", "glob", "web_search", "web_fetch",
    "edit", "write", "FileReadTool", "BashTool", "GrepTool",
    "GlobTool", "WebSearchTool", "WebFetchTool", "FileEditTool",
    "FileWriteTool",
})

TIME_BASED_MC_CLEARED_MESSAGE = "[Old tool result content cleared]"
PTL_RETRY_MARKER = "[earlier conversation truncated for compaction retry]"

# ---------------------------------------------------------------------------
# Prompts — ported from prompt.ts
# ---------------------------------------------------------------------------

DETAILED_ANALYSIS_INSTRUCTION = """Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Analyze the conversation. For each section identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like file names, code snippets, function signatures, file edits
   - Errors you ran into and how you fixed them
   - Pay special attention to specific user feedback, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly."""

BASE_COMPACT_PROMPT = f"""CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.

{DETAILED_ANALYSIS_INSTRUCTION}

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Include full code snippets where applicable.
4. Errors and fixes: List all errors you ran into, and how you fixed them.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe precisely what was being worked on immediately before this summary.

Please provide your summary based on the conversation so far, following this structure and ensuring precision and thoroughness in your response.

REMINDER: Do NOT call any tools. Respond with plain text only — an <analysis> block followed by a <summary> block."""

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _is_compactable_tool(tool_name: str) -> bool:
    return tool_name in _COMPACTABLE_TOOLS


def _format_tool_result_placeholder() -> str:
    return "[tool result content cleared during compaction - see transcript for details]"


def _now_ms() -> int:
    return int(time.time() * 1000)


def _extract_tool_names(messages: List[Dict]) -> List[str]:
    """Extract tool_use names from assistant messages."""
    names: List[str] = []
    for msg in messages:
        if msg.get("role") != "assistant":
            continue
        content = msg.get("content", "")
        if isinstance(content, str):
            continue
        for block in content:
            if isinstance(block, dict) and block.get("type") == "tool_use":
                names.append(block.get("name", ""))
    return names


def _format_messages_for_summary(messages: List[Dict]) -> str:
    """Format message list into a text block for LLM summarization."""
    parts: List[str] = []
    for msg in messages:
        role = msg.get("role", "unknown")
        content = msg.get("content", "")
        if isinstance(content, list):
            text_parts = []
            for block in content:
                if isinstance(block, dict):
                    btype = block.get("type", "")
                    if btype == "text":
                        text_parts.append(block.get("text", ""))
                    elif btype == "tool_result":
                        result_content = block.get("content", "")
                        if isinstance(result_content, list):
                            for rc in result_content:
                                if isinstance(rc, dict) and rc.get("type") == "text":
                                    text_parts.append(f"[tool_result: {rc.get('text', '')[:200]}]")
                        elif isinstance(result_content, str):
                            text_parts.append(f"[tool_result: {result_content[:200]}]")
                    elif btype == "tool_use":
                        text_parts.append(
                            f"[tool_use: {block.get('name', '?')}({json.dumps(block.get('input', {}))})]"
                        )
                    elif btype == "image":
                        text_parts.append("[image]")
                    elif btype == "document":
                        text_parts.append("[document]")
                else:
                    text_parts.append(str(block)[:200])
            content = "\n".join(text_parts)
        else:
            content = str(content)
        parts.append(f"=== {role.upper()} ===\n{content}")
    return "\n\n".join(parts)


# ---------------------------------------------------------------------------
# Grouping — ported from grouping.ts
# ---------------------------------------------------------------------------

def group_messages_by_api_round(messages: List[Dict]) -> List[List[Dict]]:
    """Group messages at API-round boundaries.

    A boundary fires when a new assistant response begins (different message ID
    from the prior assistant). For well-formed conversations this is an API-safe
    split point — every tool_use is resolved before the next assistant turn.
    """
    groups: List[List[Dict]] = []
    current: List[Dict] = []
    last_assistant_id: Optional[str] = None

    for msg in messages:
        msg_id = msg.get("id") or id(msg)
        is_assistant = msg.get("role") == "assistant"

        if is_assistant and msg_id != last_assistant_id and current:
            groups.append(current)
            current = [msg]
        else:
            current.append(msg)

        if is_assistant:
            last_assistant_id = msg_id

    if current:
        groups.append(current)

    return groups


def find_last_user_messages(messages: List[Dict], count: int = 3) -> List[Dict]:
    """Find the last N user messages (excluding tool_result blocks)."""
    user_msgs: List[Dict] = []
    for msg in reversed(messages):
        if msg.get("role") != "user":
            continue
        content = msg.get("content", "")
        if isinstance(content, list):
            has_non_tool = any(
                isinstance(b, dict) and b.get("type") != "tool_result"
                for b in content
            )
            if not has_non_tool:
                continue
        user_msgs.append(msg)
        if len(user_msgs) >= count:
            break
    return list(reversed(user_msgs))


# ---------------------------------------------------------------------------
# ContextCompactor
# ---------------------------------------------------------------------------

class ContextCompactor:
    """Port of Claude Code's context compaction system.

    Four compaction strategies:
    - ``compact()`` — LLM summarization of old messages
    - ``auto_compact()`` — threshold-based trigger + compact
    - ``micro_compact()`` — clear old tool results without cache invalidation
    - ``session_memory_compact()`` — use stored session memory as cheap summary
    """

    def __init__(
        self,
        model: Optional[str] = None,
        context_window: int = 128_000,
        memory_dir: Optional[Path] = None,
    ):
        self._model = model
        self._context_window = context_window
        self._memory_dir = memory_dir or Path.home() / ".dxrk" / "memory"
        self._last_summarized_message_id: Optional[str] = None
        self._consecutive_failures = 0
        self._lock = threading.Lock()

    # -- Public API -------------------------------------------------------

    async def compact(
        self,
        messages: List[Dict],
        custom_instructions: Optional[str] = None,
        suppress_follow_up: bool = False,
    ) -> List[Dict]:
        """Summarize old messages via LLM, return compressed message list.

        Keeps the most recent API-round worth of messages and replaces the
        rest with a single summary message.
        """
        if not messages:
            return messages

        pre_compact_tokens = estimate_messages_tokens_rough(messages)

        groups = group_messages_by_api_round(messages)

        # Keep the last group (most recent round), summarize everything before
        if len(groups) <= 1:
            return messages

        messages_to_summarize = groups[:-1]
        messages_to_keep = groups[-1]

        summary = await self._call_summarizer(messages_to_summarize, custom_instructions)

        if not summary:
            logger.warning("Compact: summarizer returned empty result")
            return messages

        boundary = {
            "role": "system",
            "content": f"[Compaction boundary — previous context ({pre_compact_tokens} tok) summarized below]",
        }
        summary_msg = {
            "role": "user",
            "content": self._build_summary_message(summary, suppress_follow_up),
            "is_compact_summary": True,
        }

        return [boundary, summary_msg] + messages_to_keep

    async def auto_compact(
        self,
        messages: List[Dict],
        threshold: Optional[float] = None,
    ) -> Tuple[List[Dict], bool]:
        """Auto-trigger when context exceeds threshold.

        Returns (compacted_messages, was_compacted).
        """
        if not messages:
            return messages, False

        effective_threshold = threshold if threshold is not None else COMPACT_DEFAULT_THRESHOLD

        should, token_count, threshold_tokens = self._should_compact(
            messages, effective_threshold,
        )
        if not should:
            return messages, False

        # Session memory first (cheaper)
        sm_result = await self.session_memory_compact(messages)
        if sm_result is not None:
            logger.info(
                "Auto-compact: session memory compaction succeeded "
                "(from %d tokens)", estimate_messages_tokens_rough(messages),
            )
            return sm_result, True

        # Fall back to LLM compaction
        try:
            result = await self.compact(messages, suppress_follow_up=True)
            self._consecutive_failures = 0
            logger.info(
                "Auto-compact: LLM compaction succeeded "
                "(from %d to ~%d tokens)",
                token_count, estimate_messages_tokens_rough(result),
            )
            return result, True
        except Exception as exc:
            self._consecutive_failures += 1
            logger.warning("Auto-compact failed (%d/%d): %s",
                           self._consecutive_failures,
                           MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES, exc)
            if self._consecutive_failures >= MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES:
                logger.warning("Auto-compact circuit breaker tripped")
            return messages, False

    async def micro_compact(
        self,
        messages: List[Dict],
        max_age_rounds: int = MICROCOMPACT_MAX_AGE_ROUNDS,
    ) -> List[Dict]:
        """Remove old tool results without invalidating cache.

        Replaces tool_result content from rounds older than ``max_age_rounds``
        with a placeholder marker.  The cache prefix before the boundary is
        preserved — only the content inside existing blocks changes.
        """
        result: List[Dict] = []
        groups = group_messages_by_api_round(messages)

        for idx, group in enumerate(groups):
            is_recent = (len(groups) - idx) <= max_age_rounds
            if is_recent:
                result.extend(group)
                continue

            cleaned = self._clear_tool_results(group)
            result.extend(cleaned)

        return result

    async def session_memory_compact(
        self,
        messages: List[Dict],
        memory_file: Optional[str] = None,
    ) -> Optional[List[Dict]]:
        """Use stored session memory for cheaper compaction.

        Reads a previously-extracted session memory file and uses it as the
        summary instead of calling the LLM.  Returns ``None`` when no session
        memory is available or it's insufficient to cover the conversation.
        """
        memory_text = self._read_session_memory(memory_file)
        if not memory_text:
            return None

        # Check if session memory has actual content (not just template)
        if self._is_memory_empty_template(memory_text):
            return None

        # Determine which messages to keep
        start_index = self._calculate_keep_index(messages)
        messages_to_keep = messages[start_index:]

        post_compact = estimate_messages_tokens_rough([{"role": "user", "content": memory_text}] + messages_to_keep)
        if post_compact >= estimate_messages_tokens_rough(messages):
            # Session memory is too large, wouldn't save anything
            return None

        boundary = {
            "role": "system",
            "content": "[Session memory compaction — extracted summary restored below]",
        }
        summary_msg = {
            "role": "user",
            "content": f"This session is being continued from a previous conversation. "
                       f"The session memory below covers the earlier portion.\n\n{memory_text}\n\n"
                       f"Recent messages are preserved verbatim.",
            "is_compact_summary": True,
        }

        return [boundary, summary_msg] + messages_to_keep

    # -- Internal ---------------------------------------------------------

    def _should_compact(
        self,
        messages: List[Dict],
        threshold: float,
    ) -> Tuple[bool, int, int]:
        """Check if messages exceed (threshold * context_window)."""
        token_count = estimate_messages_tokens_rough(messages)
        threshold_tokens = int(self._context_window * threshold)
        return token_count >= threshold_tokens, token_count, threshold_tokens

    async def _call_summarizer(
        self,
        messages: List[Dict],
        custom_instructions: Optional[str] = None,
    ) -> Optional[str]:
        """Call LLM with the compact prompt to summarize a block of messages."""
        prompt = BASE_COMPACT_PROMPT
        if custom_instructions:
            prompt += f"\n\nAdditional Instructions:\n{custom_instructions}"

        formatted = _format_messages_for_summary(messages)

        try:
            response = await async_call_llm(
                task="compression",
                provider=None,
                messages=[
                    {"role": "user", "content": f"{prompt}\n\nConversation to summarize:\n\n{formatted}"},
                ],
                max_tokens=COMPACT_OUTPUT_MAX_TOKENS,
                temperature=0.3,
            )
            text = extract_content_or_reasoning(response)
            if not text:
                return None
            return self._format_summary(text)
        except Exception as exc:
            logger.error("Compact summarizer call failed: %s", exc)
            return None

    def _format_summary(self, raw: str) -> str:
        """Strip <analysis> blocks, format <summary> tags — ported from formatCompactSummary."""
        result = raw
        result = re.sub(r"<analysis>[\s\S]*?</analysis>", "", result)
        match = re.search(r"<summary>([\s\S]*?)</summary>", result)
        if match:
            content = match.group(1) or ""
            result = re.sub(r"<summary>[\s\S]*?</summary>", f"Summary:\n{content.strip()}", result)
        result = re.sub(r"\n\n+", "\n\n", result)
        return result.strip()

    def _build_summary_message(self, summary: str, suppress_follow_up: bool) -> str:
        text = (
            "This session is being continued from a previous conversation that "
            "ran out of context. The summary below covers the earlier portion.\n\n"
            f"{summary}"
        )
        if suppress_follow_up:
            text += (
                "\n\nContinue the conversation from where it left off without "
                "asking the user any further questions. Resume directly — do not "
                "acknowledge the summary, do not recap what was happening, do not "
                "preface with \"I'll continue\" or similar. Pick up the last task "
                "as if the break never happened."
            )
        return text

    def _clear_tool_results(self, messages: List[Dict]) -> List[Dict]:
        """Replace tool_result content with placeholder in a message group."""
        result: List[Dict] = []
        for msg in messages:
            if msg.get("role") != "user":
                result.append(msg)
                continue

            content = msg.get("content", "")
            if not isinstance(content, list):
                result.append(msg)
                continue

            new_content: List[Dict] = []
            touched = False
            for block in content:
                if isinstance(block, dict) and block.get("type") == "tool_result":
                    tool_use_id = block.get("tool_use_id", "")
                    if tool_use_id:
                        new_content.append({
                            **block,
                            "content": TIME_BASED_MC_CLEARED_MESSAGE,
                        })
                        touched = True
                        continue
                new_content.append(block)

            if touched:
                result.append({**msg, "content": new_content})
            else:
                result.append(msg)

        return result

    def _calculate_keep_index(self, messages: List[Dict]) -> int:
        """Calculate starting index for messages to keep after compaction.

        Starts from last_summarized_message_id, expands backwards to meet
        minimum token and text-block message counts.
        """
        if not messages:
            return 0

        if self._last_summarized_message_id:
            base_idx = next(
                (i for i, m in enumerate(messages)
                 if m.get("id") == self._last_summarized_message_id),
                -1,
            )
            start = base_idx + 1 if base_idx >= 0 else len(messages)
        else:
            start = len(messages)

        # Expand backwards to meet minimums
        total_tokens = sum(
            estimate_messages_tokens_rough([m])
            for m in messages[start:]
        )
        text_block_count = sum(
            1 for m in messages[start:]
            if self._has_text_blocks(m)
        )

        if total_tokens >= SESSION_MEMORY_MIN_TOKENS and text_block_count >= SESSION_MEMORY_MIN_TEXT_BLOCK_MESSAGES:
            return start

        for i in range(start - 1, -1, -1):
            msg_tokens = estimate_messages_tokens_rough([messages[i]])
            total_tokens += msg_tokens
            if self._has_text_blocks(messages[i]):
                text_block_count += 1
            start = i

            if total_tokens >= SESSION_MEMORY_MAX_TOKENS:
                break
            if total_tokens >= SESSION_MEMORY_MIN_TOKENS and text_block_count >= SESSION_MEMORY_MIN_TEXT_BLOCK_MESSAGES:
                break

        return start

    @staticmethod
    def _has_text_blocks(msg: Dict) -> bool:
        content = msg.get("content", "")
        if isinstance(content, str):
            return bool(content.strip())
        if isinstance(content, list):
            return any(
                isinstance(b, dict) and b.get("type") == "text"
                for b in content
            )
        return False

    @staticmethod
    def _is_memory_empty_template(memory_text: str) -> bool:
        """Check if session memory is just a template with no real content."""
        stripped = memory_text.strip()
        return not stripped or len(stripped) < 50

    def _read_session_memory(self, memory_file: Optional[str] = None) -> Optional[str]:
        if memory_file:
            path = Path(memory_file)
        else:
            path = self._memory_dir / "session_memory.txt"

        if not path.is_file():
            return None

        try:
            return path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as exc:
            logger.warning("Failed to read session memory %s: %s", path, exc)
            return None

    @staticmethod
    def calculate_token_warning_state(
        token_usage: int,
        context_window: int,
    ) -> Dict[str, Any]:
        """Calculate warning levels — ported from calculateTokenWarningState."""
        effective = context_window - COMPACT_OUTPUT_MAX_TOKENS
        threshold = effective - AUTOCOMPACT_BUFFER_TOKENS
        percent_left = max(0, round(((threshold - token_usage) / threshold) * 100))

        return {
            "percent_left": percent_left,
            "is_above_warning": token_usage >= (threshold - WARNING_THRESHOLD_BUFFER_TOKENS),
            "is_above_error": token_usage >= (threshold - ERROR_THRESHOLD_BUFFER_TOKENS),
            "is_above_auto_compact": token_usage >= threshold,
            "is_at_blocking_limit": token_usage >= (effective - MANUAL_COMPACT_BUFFER_TOKENS),
        }


# ---------------------------------------------------------------------------
# Module-level singleton
# ---------------------------------------------------------------------------

_compactor: Optional[ContextCompactor] = None
_compactor_lock = threading.Lock()


def get_compactor() -> ContextCompactor:
    global _compactor
    if _compactor is None:
        with _compactor_lock:
            if _compactor is None:
                _compactor = ContextCompactor()
    return _compactor


# ---------------------------------------------------------------------------
# Tool handler
# ---------------------------------------------------------------------------

SCHEMA = {
    "name": "compact_context",
    "description": (
        "Compress the current conversation context to free up space. "
        "Actions: compact (LLM summarization of old messages), "
        "auto (threshold-based trigger), "
        "micro (clear old tool results)."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["compact", "auto", "micro"],
                "description": "Compaction strategy to apply",
            },
            "messages": {
                "type": "array",
                "items": {"type": "object"},
                "description": "List of conversation messages (OpenAI format)",
            },
            "threshold": {
                "type": "number",
                "description": "Auto-compact threshold as fraction (default 0.80)",
            },
            "max_age_rounds": {
                "type": "integer",
                "description": "Micro-compact: max API rounds to keep (default 5)",
            },
        },
        "required": ["action", "messages"],
    },
}


async def _handler(args: Dict[str, Any], **kw) -> str:
    action = args.get("action", "")
    messages = args.get("messages", [])

    if not isinstance(messages, list):
        return tool_error("messages must be a list")

    compactor = get_compactor()

    try:
        if action == "compact":
            result = await compactor.compact(
                messages,
                custom_instructions=args.get("custom_instructions"),
                suppress_follow_up=args.get("suppress_follow_up", False),
            )
            pre = estimate_messages_tokens_rough(messages)
            post = estimate_messages_tokens_rough(result)
            return tool_result({
                "messages": result,
                "stats": {
                    "pre_compact_tokens": pre,
                    "post_compact_tokens": post,
                    "savings": pre - post,
                    "compression_ratio": round(post / pre, 4) if pre else 1.0,
                },
            })

        elif action == "auto":
            result, was_compacted = await compactor.auto_compact(
                messages,
                threshold=args.get("threshold"),
            )
            pre = estimate_messages_tokens_rough(messages)
            post = estimate_messages_tokens_rough(result)
            return tool_result({
                "messages": result,
                "was_compacted": was_compacted,
                "stats": {
                    "pre_compact_tokens": pre,
                    "post_compact_tokens": post,
                    "savings": pre - post,
                },
            })

        elif action == "micro":
            result = await compactor.micro_compact(
                messages,
                max_age_rounds=args.get("max_age_rounds", MICROCOMPACT_MAX_AGE_ROUNDS),
            )
            pre = estimate_messages_tokens_rough(messages)
            post = estimate_messages_tokens_rough(result)
            return tool_result({
                "messages": result,
                "stats": {
                    "pre_compact_tokens": pre,
                    "post_compact_tokens": post,
                    "savings": pre - post,
                },
            })

        else:
            return tool_error(f"Unknown action: {action}. Use compact, auto, or micro.")

    except Exception as exc:
        logger.exception("compact_context error: %s", exc)
        return tool_error(f"Compaction failed: {type(exc).__name__}: {exc}")


registry.register(
    name="compact_context",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    is_async=True,
    emoji="🧹",
    description="Compress conversation context via LLM summarization, threshold-based auto-trigger, or tool-result cleanup.",
    is_readonly=True,
    is_concurrency_safe=False,
)


# ---------------------------------------------------------------------------
# Standalone CLI
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    import sys
    import json

    async def _main():
        args = json.loads(sys.argv[1]) if len(sys.argv) > 1 else {}
        action = args.get("action", "auto")
        messages = args.get("messages", [])

        if not messages:
            print(json.dumps({"error": "No messages provided"}))
            return

        compactor = ContextCompactor()
        result = await getattr(compactor, f"{action}_compact")(messages)
        print(json.dumps({"messages": result}, ensure_ascii=False, indent=2))

    asyncio.run(_main())
