# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Tool Search Tool — find the right tool for the task in natural language.

Models often struggle to recall which tool name matches a given need.
This tool accepts a plain-English description of what the model wants to
do and returns matching registered tools ranked by relevance.

Port of Claude Code's ToolSearchTool (TypeScript) into Dxrk's registry pattern.
"""

import json
import logging
import re

from tools.registry import registry, tool_error

logger = logging.getLogger(__name__)

# ── Schema ────────────────────────────────────────────────────────────────────

TOOL_SEARCH_SCHEMA = {
    "name": "tool_search",
    "description": (
        "Search for the right tool to accomplish a task. Use this when you are "
        "unsure which tool to use. Describe what you want to do in natural "
        "language and this tool returns matching tools ranked by relevance. "
        "Supports required-term prefix: prefix a search term with + to require "
        "it (e.g. 'read +file' requires 'file' in every result). Filter by "
        "toolset with toolset parameter (file, web, browser, dev, system, etc.).\n\n"
        "When you know the exact tool name, use the 'select:' prefix for direct "
        "lookup (e.g. 'select:write_file' returns the write_file tool directly)."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": (
                    "Natural language description of what you want to do, e.g. "
                    "'read a file', 'search the web', 'create a task'. Prefix "
                    "with 'select:' to directly pick a tool by name. Prefix "
                    "terms with + to require them in results (e.g. '+browser click')."
                ),
            },
            "toolset": {
                "type": "string",
                "description": "Optional filter: only return tools from this toolset (file, web, browser, dev, system, task, agent, team, etc.).",
            },
            "max_results": {
                "type": "integer",
                "description": "Maximum number of matching tools to return (default: 8, max: 20).",
                "default": 8,
            },
        },
        "required": ["query"],
    },
}


# ── Helpers ───────────────────────────────────────────────────────────────────

_TOKEN_PATTERN = re.compile(r"[a-z0-9]+")
MAX_RESULTS_DEFAULT = 8
MAX_RESULTS_HARD = 20
SCORE_NAME_EXACT = 10
SCORE_NAME_PART = 5
SCORE_DESC_WORD = 2
SCORE_DESC_CHAR = 1


def _tokenize(text: str) -> list[str]:
    """Split text into lowercase alphanumeric tokens."""
    return _TOKEN_PATTERN.findall(text.lower())


def _parse_query(query: str) -> tuple[bool, str | None, list[str], list[str]]:
    """
    Parse a search query into its components.

    Returns: (is_select, exact_name, required_terms, optional_terms)
    """
    q = query.strip()

    # select: prefix for exact tool name lookup
    select_match = re.match(r"^select:\s*(.+)$", q, re.IGNORECASE)
    if select_match:
        return True, select_match.group(1).strip(), [], []

    # Split on whitespace first to preserve + prefix, then tokenize each piece
    raw_terms = q.split()
    required = []
    optional = []
    for raw in raw_terms:
        if raw.startswith("+") and len(raw) > 1:
            required.extend(_tokenize(raw[1:]))
        else:
            optional.extend(_tokenize(raw))
    return False, None, required, optional


def _build_term_patterns(terms: list[str]) -> dict[str, re.Pattern]:
    """Pre-compile word-boundary regexes for all search terms."""
    patterns = {}
    for t in terms:
        if t not in patterns:
            patterns[t] = re.compile(r"\b" + re.escape(t) + r"\b")
    return patterns


def _score_tool(
    name: str,
    description: str,
    all_terms: list[str],
    term_patterns: dict[str, re.Pattern],
    name_tokens: list[str],
) -> int:
    """Compute relevance score for one tool against all search terms."""
    desc_lower = description.lower()
    score = 0

    for term in all_terms:
        pattern = term_patterns.get(term)

        # Exact name token match
        if term in name_tokens:
            score += SCORE_NAME_EXACT
        elif any(term in t for t in name_tokens):
            score += SCORE_NAME_PART

        # Word-boundary match in description
        if pattern and pattern.search(desc_lower):
            score += SCORE_DESC_WORD
        elif term in desc_lower:
            score += SCORE_DESC_CHAR

    return score


# ── Handler ───────────────────────────────────────────────────────────────────


def _handler(args: dict, **kw) -> str:
    query = args.get("query", "").strip()
    toolset_filter = args.get("toolset", "").strip()
    max_results = args.get("max_results", MAX_RESULTS_DEFAULT)

    if not isinstance(max_results, int):
        try:
            max_results = int(max_results)
        except (TypeError, ValueError):
            max_results = MAX_RESULTS_DEFAULT
    max_results = max(1, min(max_results, MAX_RESULTS_HARD))

    if not query:
        return tool_error("A non-empty query is required.", success=False)

    # ── Parse ──
    is_select, exact_name, required_terms, optional_terms = _parse_query(query)

    # ── Direct select mode ──
    if is_select and exact_name:
        # Support comma-separated multi-select: select:read_file,write_file
        names = [n.strip() for n in exact_name.split(",") if n.strip()]
        found = []
        missing = []
        for n in names:
            entry = registry.get_entry(n)
            if entry:
                found.append(n)
            else:
                missing.append(n)

        result = {
            "query": query,
            "matches": [
                {"name": n, "description": registry.get_entry(n).description, "toolset": registry.get_entry(n).toolset, "relevance": 100}
                for n in found
            ],
            "total": len(found),
            "mode": "select",
        }
        if missing:
            result["missing"] = missing

        return json.dumps(result, ensure_ascii=False)

    # ── Keyword search mode ──
    all_scoring_terms = required_terms + optional_terms if required_terms else optional_terms
    term_patterns = _build_term_patterns(all_scoring_terms)

    try:
        tool_names = registry.get_all_tool_names()
    except Exception as e:
        logger.exception("Failed to get tool names from registry: %s", e)
        return tool_error(f"Registry unavailable: {e}", success=False)

    candidates = []
    for name in tool_names:
        entry = registry.get_entry(name)
        if not entry or not entry.schema:
            continue

        ts = entry.toolset or ""
        if toolset_filter and ts != toolset_filter:
            continue

        description = entry.description or entry.schema.get("description", "")
        name_tokens = _tokenize(name)

        # Required-term filter
        if required_terms:
            desc_lower = description.lower()
            name_tokens_set = set(name_tokens)
            all_required_found = all(
                term in name_tokens_set
                or any(term in t for t in name_tokens)
                or (term_patterns.get(term) and term_patterns[term].search(desc_lower))
                for term in required_terms
            )
            if not all_required_found:
                continue

        score = _score_tool(name, description, all_scoring_terms, term_patterns, name_tokens)
        if score > 0:
            candidates.append({
                "name": name,
                "description": description[:200],
                "toolset": ts,
                "relevance": score,
                "emoji": entry.emoji or "",
            })

    # Sort: higher score first, then alphabetical by name
    candidates.sort(key=lambda x: (-x["relevance"], x["name"]))
    matches = candidates[:max_results]

    return json.dumps({
        "query": query,
        "matches": matches,
        "total": len(candidates),
        "mode": "keyword",
    }, ensure_ascii=False)


# ── Check fn ──────────────────────────────────────────────────────────────────


def check_requirements() -> bool:
    """Always available — the registry is loaded with tool definitions."""
    return True


# ── Registry ──────────────────────────────────────────────────────────────────

registry.register(
    name="tool_search",
    toolset="system",
    schema=TOOL_SEARCH_SCHEMA,
    handler=_handler,
    check_fn=check_requirements,
    emoji="🔍",
)
