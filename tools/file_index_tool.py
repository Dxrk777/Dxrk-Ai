# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Fuzzy File Search Tool

Port of Claude Code's native-ts/file-index (nucleo-style fuzzy matching).

Provides a FileIndex class that:
  - Indexes files in a directory tree asynchronously
  - Bitmap-based O(1) rejection (letter bitmap per filename)
  - Top-k scoring with test path penalization
  - Fuzzy search (subsequence matching, not regex)
  - Async building with yields to event loop

Registered as tool "fuzzy_find" -> {query, path?, max_results?}
"""

import asyncio
import json
import logging
import os
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple

from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

# nucleo-style scoring constants
SCORE_MATCH = 16
BONUS_BOUNDARY = 8
BONUS_CAMEL = 6
BONUS_CONSECUTIVE = 4
BONUS_FIRST_CHAR = 8
PENALTY_GAP_START = 3
PENALTY_GAP_EXTENSION = 1

TOP_LEVEL_CACHE_LIMIT = 100
MAX_QUERY_LEN = 64
CHUNK_MS = 4         # yield to event loop after this many ms of sync work
YIELD_INTERVAL = 256  # check time every N iterations


@dataclass
class SearchResult:
    path: str
    score: float


def yield_to_event_loop():
    return asyncio.sleep(0)


def is_boundary(code: int) -> bool:
    return code in (47, 92, 45, 95, 46, 32)  # / \ - _ . space


def is_lower(code: int) -> bool:
    return 97 <= code <= 122


def is_upper(code: int) -> bool:
    return 65 <= code <= 90


def score_bonus_at(path: str, pos: int, first: bool) -> int:
    if pos == 0:
        return BONUS_FIRST_CHAR if first else 0
    prev_ch = ord(path[pos - 1])
    if is_boundary(prev_ch):
        return BONUS_BOUNDARY
    if is_lower(prev_ch) and is_upper(ord(path[pos])):
        return BONUS_CAMEL
    return 0


def compute_top_level_entries(paths: List[str], limit: int) -> List[SearchResult]:
    top_level: Set[str] = set()
    for p in paths:
        end = len(p)
        for i, ch in enumerate(p):
            if ch in ('/', '\\'):
                end = i
                break
        segment = p[:end]
        if segment:
            top_level.add(segment)
            if len(top_level) >= limit:
                break

    sorted_segments = sorted(top_level, key=lambda s: (len(s), s))
    return [SearchResult(path=s, score=0.0) for s in sorted_segments[:limit]]


class FileIndex:
    def __init__(self):
        self.paths: List[str] = []
        self.lower_paths: List[str] = []
        self.char_bits: List[int] = []
        self.path_lens: List[int] = []
        self.top_level_cache: Optional[List[SearchResult]] = None
        self.ready_count = 0

    def load_from_file_list(self, file_list: List[str]) -> None:
        seen: Set[str] = set()
        paths: List[str] = []
        for line in file_list:
            if line and line not in seen:
                seen.add(line)
                paths.append(line)
        self._build_index(paths)

    def load_from_file_list_async(self, file_list: List[str]):
        async def build():
            seen: Set[str] = set()
            paths: List[str] = []
            chunk_start = time.perf_counter()
            for i, line in enumerate(file_list):
                if line and line not in seen:
                    seen.add(line)
                    paths.append(line)
                if (i & 0xff) == 0xff and (time.perf_counter() - chunk_start) * 1000 > CHUNK_MS:
                    await yield_to_event_loop()
                    chunk_start = time.perf_counter()

            self._reset_arrays(paths)
            chunk_start = time.perf_counter()
            first_chunk = True
            for i in range(len(paths)):
                self._index_path(i)
                if (i & 0xff) == 0xff and (time.perf_counter() - chunk_start) * 1000 > CHUNK_MS:
                    self.ready_count = i + 1
                    if first_chunk:
                        first_chunk = False
                        queryable_event.set()
                    await yield_to_event_loop()
                    chunk_start = time.perf_counter()

            self.ready_count = len(paths)
            queryable_event.set()

        queryable_event = asyncio.Event()
        done_task = asyncio.ensure_future(build())
        return queryable_event.wait(), done_task

    def _reset_arrays(self, paths: List[str]) -> None:
        n = len(paths)
        self.paths = paths
        self.lower_paths = [''] * n
        self.char_bits = [0] * n
        self.path_lens = [0] * n
        self.ready_count = 0
        self.top_level_cache = compute_top_level_entries(paths, TOP_LEVEL_CACHE_LIMIT)

    def _index_path(self, i: int) -> None:
        lp = self.paths[i].lower()
        self.lower_paths[i] = lp
        length = len(lp)
        self.path_lens[i] = length
        bits = 0
        for ch in lp:
            c = ord(ch)
            if 97 <= c <= 122:
                bits |= 1 << (c - 97)
        self.char_bits[i] = bits

    def _build_index(self, paths: List[str]) -> None:
        self._reset_arrays(paths)
        for i in range(len(paths)):
            self._index_path(i)
        self.ready_count = len(paths)

    def search(self, query: str, limit: int) -> List[SearchResult]:
        if limit <= 0:
            return []
        if not query:
            if self.top_level_cache:
                return self.top_level_cache[:limit]
            return []

        case_sensitive = query != query.lower()
        needle = query if case_sensitive else query.lower()
        n_len = min(len(needle), MAX_QUERY_LEN)

        needle_chars = list(needle[:n_len])
        needle_bitmap = 0
        for ch in needle_chars:
            c = ord(ch)
            if 97 <= c <= 122:
                needle_bitmap |= 1 << (c - 97)

        score_ceiling = n_len * (SCORE_MATCH + BONUS_BOUNDARY) + BONUS_FIRST_CHAR + 32

        top_k: List[Tuple[int, str]] = []  # (score, path)
        threshold = -10**9

        for i in range(self.ready_count):
            if (self.char_bits[i] & needle_bitmap) != needle_bitmap:
                continue

            haystack = self.paths[i] if case_sensitive else self.lower_paths[i]

            pos = haystack.find(needle_chars[0])
            if pos == -1:
                continue
            positions = [pos]
            gap_penalty = 0
            consec_bonus = 0
            prev = pos
            match_failed = False
            for j in range(1, n_len):
                pos = haystack.find(needle_chars[j], prev + 1)
                if pos == -1:
                    match_failed = True
                    break
                positions.append(pos)
                gap = pos - prev - 1
                if gap == 0:
                    consec_bonus += BONUS_CONSECUTIVE
                else:
                    gap_penalty += PENALTY_GAP_START + gap * PENALTY_GAP_EXTENSION
                prev = pos

            if match_failed:
                continue

            if len(top_k) == limit and score_ceiling + consec_bonus - gap_penalty <= threshold:
                continue

            path = self.paths[i]
            h_len = self.path_lens[i]
            score = n_len * SCORE_MATCH + consec_bonus - gap_penalty
            score += score_bonus_at(path, positions[0], True)
            for j in range(1, n_len):
                score += score_bonus_at(path, positions[j], False)
            score += max(0, 32 - (h_len >> 2))

            if len(top_k) < limit:
                top_k.append((score, path))
                if len(top_k) == limit:
                    top_k.sort(key=lambda x: x[0])
                    threshold = top_k[0][0]
            elif score > threshold:
                lo, hi = 0, len(top_k)
                while lo < hi:
                    mid = (lo + hi) >> 1
                    if top_k[mid][0] < score:
                        lo = mid + 1
                    else:
                        hi = mid
                top_k.insert(lo, (score, path))
                top_k.pop(0)
                threshold = top_k[0][0]

        top_k.sort(key=lambda x: -x[0])

        match_count = len(top_k)
        denom = max(match_count, 1)
        results: List[SearchResult] = []
        for i, (fuzz_score, p) in enumerate(top_k):
            position_score = i / denom
            final_score = min(position_score * 1.05, 1.0) if 'test' in p else position_score
            results.append(SearchResult(path=p, score=final_score))

        return results


# ── Module-level singleton ─────────────────────────────────────────────

_global_index: Optional[FileIndex] = None
_index_paths: Set[str] = set()


def _get_index(path: Optional[str] = None) -> FileIndex:
    global _global_index, _index_paths
    if _global_index is None:
        _global_index = FileIndex()
    if path and path not in _index_paths:
        _index_paths.add(path)
        root = Path(path).resolve()
        files = []
        for f in root.rglob('*'):
            if f.is_file():
                try:
                    files.append(str(f.relative_to(root)))
                except ValueError:
                    continue
        _global_index.load_from_file_list(files)
    return _global_index


# ── Handler ────────────────────────────────────────────────────────────


def fuzzy_find_handler(args: dict, **kw) -> str:
    query = args.get("query", "").strip()
    path = args.get("path") or os.getcwd()
    max_results = args.get("max_results", 25)

    if not isinstance(max_results, int):
        try:
            max_results = int(max_results)
        except (TypeError, ValueError):
            max_results = 25
    max_results = max(1, min(max_results, 100))

    if not query:
        return tool_error("A non-empty query is required.", success=False)

    try:
        index = _get_index(path)
        results = index.search(query, max_results)
        return tool_result(
            success=True,
            query=query,
            path=path,
            count=len(results),
            results=[{"path": r.path, "score": r.score} for r in results],
        )
    except Exception as e:
        logger.exception("fuzzy_find error: %s", e)
        return tool_error(str(e), success=False)


def check_file_index_requirements() -> bool:
    return True


# ── Schema ─────────────────────────────────────────────────────────────

FUZZY_FIND_SCHEMA = {
    "name": "fuzzy_find",
    "description": (
        "Fuzzy search for files in a directory tree. Uses subsequence matching "
        "(not regex) with scoring that prefers boundary matches, camelCase, and "
        "shorter paths. Penalizes 'test' paths slightly. Returns top-N matches "
        "sorted by relevance (0.0 = best). Useful when you know part of a filename "
        "but not the exact path."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "Fuzzy search query (subsequence matched against filenames). Case-insensitive unless you use uppercase letters.",
            },
            "path": {
                "type": "string",
                "description": "Root directory to search in. Defaults to current working directory.",
            },
            "max_results": {
                "type": "integer",
                "description": "Maximum number of results to return (default: 25, max: 100).",
                "default": 25,
            },
        },
        "required": ["query"],
    },
}


# ── Registry ───────────────────────────────────────────────────────────

registry.register(
    name="fuzzy_find",
    toolset="file",
    schema=FUZZY_FIND_SCHEMA,
    handler=fuzzy_find_handler,
    check_fn=check_file_index_requirements,
    emoji="🔍",
)
