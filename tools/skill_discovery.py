# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Skill Discovery - Dynamic skill auto-discovery and conditional activation for Dxrk.

Port of Claude Code's `loadSkillsDir.ts` dynamic discovery + conditional skills.

Two mechanisms:
1. Dynamic Discovery: When a file is read/written/edited, walk UP from the file's
   directory looking for `.claude/skills/` directories. Newly found skills get
   loaded and become available.

2. Conditional Activation: Skills with ``paths`` frontmatter (gitignore-style
   patterns) are stored at startup and activated on-demand when matching file
   paths are operated on. Once activated, they stay active for the session.

Usage:
    from tools.skill_discovery import (
        discover_skills_for_paths,
        activate_conditional_skills_for_paths,
        get_discovered_skills,
    )

    # On file read/write/patch:
    new_dirs = discover_skills_for_paths(["/path/to/file"], "/workspace/root")
    activated = activate_conditional_skills_for_paths(["/path/to/file"], "/workspace/root")
"""

import logging
import os
import re
from pathlib import Path
from typing import Dict, List, Set, Optional, Tuple

from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

# ── In-memory state ─────────────────────────────────────────────────────────

# Directories we've already checked (hit or miss), so we don't stat the same
# path on every file operation.
_checked_dirs: Set[str] = set()

# Dynamically discovered skill directories (path → list of skill metadata)
_discovered_dirs: Set[str] = set()

# Conditional skills (with ``paths`` frontmatter) that haven't been activated yet.
# Populated by skills_tool at startup.
_conditional_skills: Dict[str, dict] = {}

# Names of skills that have been activated this session.
_activated_skill_names: Set[str] = set()

# Discovered skill metadata cache (path → parsed skill)
_discovered_skills_cache: Dict[str, List[dict]] = {}


def _walk_up_skills_dirs(file_path: str, cwd: str) -> List[str]:
    """Walk up from *file_path* to *cwd* looking for ``.claude/skills/`` dirs.

    Args:
        file_path: Absolute path to the file being operated on.
        cwd: Current working directory (upper bound for discovery).

    Returns:
        List of newly discovered ``.claude/skills/`` directory paths,
        sorted deepest first (skills closer to the file take precedence).
    """
    normalized_cwd = cwd.rstrip("/")
    current = Path(file_path).resolve().parent
    new_dirs: List[str] = []

    # Walk up to cwd (do NOT include cwd level — cwd skills are loaded at startup)
    while str(current).startswith(normalized_cwd + "/"):
        skill_dir = current / ".claude" / "skills"
        skill_dir_str = str(skill_dir)

        if skill_dir_str not in _checked_dirs:
            _checked_dirs.add(skill_dir_str)
            if skill_dir.is_dir():
                # Quick gitignore check: skip if parent is inside .git, node_modules, etc.
                if any(part.startswith(".") and part != ".claude" for part in current.parts):
                    logger.debug(
                        "[skill_discovery] Skipped hidden dir: %s", skill_dir_str
                    )
                else:
                    new_dirs.append(skill_dir_str)

        parent = current.parent
        if parent == current:
            break  # Reached filesystem root
        current = parent

    # Sort deepest first — more specific skills take precedence
    new_dirs.sort(key=lambda d: d.count("/"), reverse=True)
    return new_dirs


def _load_skills_from_dir(skill_dir: str) -> List[dict]:
    """Load skill metadata from a ``.claude/skills/`` directory.

    Each skill is a subdirectory containing a SKILL.md file.
    Returns a list of dicts with name, description, path, category.

    This mirrors what skills_tool._find_all_skills does but for a single dir.
    """
    from tools.skills_tool import _parse_frontmatter, MAX_NAME_LENGTH, MAX_DESCRIPTION_LENGTH

    base = Path(skill_dir)
    if not base.is_dir():
        return []

    skills: List[dict] = []
    try:
        for entry in sorted(base.iterdir()):
            if not entry.is_dir():
                continue
            skill_md = entry / "SKILL.md"
            if not skill_md.is_file():
                continue

            try:
                content = skill_md.read_text(encoding="utf-8")[:4000]
                frontmatter, body = _parse_frontmatter(content)

                name = frontmatter.get("name", entry.name)[:MAX_NAME_LENGTH]
                description = frontmatter.get("description", "")
                if not description:
                    for line in body.strip().split("\n"):
                        line = line.strip()
                        if line and not line.startswith("#"):
                            description = line
                            break
                if len(description) > MAX_DESCRIPTION_LENGTH:
                    description = description[: MAX_DESCRIPTION_LENGTH - 3] + "..."

                skills.append({
                    "name": name,
                    "description": description,
                    "path": str(skill_md),
                    "discovered": True,
                })
            except (UnicodeDecodeError, PermissionError) as e:
                logger.debug(
                    "[skill_discovery] Failed to read %s: %s", skill_md, e
                )
                continue
            except Exception as e:
                logger.debug(
                    "[skill_discovery] Parse error %s: %s", skill_md, e
                )
                continue
    except PermissionError:
        logger.debug("[skill_discovery] Permission denied: %s", skill_dir)

    return skills


# ── Public API ──────────────────────────────────────────────────────────────


def discover_skills_for_paths(
    file_paths: List[str], cwd: Optional[str] = None
) -> List[str]:
    """Discover new skill directories by walking up from file paths.

    Call this when a file is read, written, or patched.

    Args:
        file_paths: List of file paths to check for nearby skills.
        cwd: Current working directory. Default: os.getcwd().

    Returns:
        List of newly loaded skill names.
    """
    if cwd is None:
        cwd = os.getcwd()

    discovered: List[str] = []

    for fp in file_paths:
        if not fp:
            continue
        # Only process absolute paths or paths under cwd
        resolved = Path(fp).resolve()
        if not resolved.is_absolute():
            resolved = Path(cwd) / fp

        new_dirs = _walk_up_skills_dirs(str(resolved), cwd)
        for d in new_dirs:
            if d not in _discovered_dirs:
                _discovered_dirs.add(d)
                skills = _load_skills_from_dir(d)
                _discovered_skills_cache[d] = skills
                for s in skills:
                    discovered.append(s["name"])
                    logger.info(
                        "[skill_discovery] Discovered skill '%s' from %s",
                        s["name"],
                        d,
                    )

    return discovered


def activate_conditional_skills_for_paths(
    file_paths: List[str], cwd: Optional[str] = None
) -> List[str]:
    """Activate conditional skills whose ``paths`` patterns match the given files.

    Args:
        file_paths: File paths being operated on.
        cwd: Current working directory (for relative path resolution).

    Returns:
        List of newly activated skill names.
    """
    if not _conditional_skills or not file_paths:
        return []

    if cwd is None:
        cwd = os.getcwd()

    activated: List[str] = []

    for skill_name, skill_meta in list(_conditional_skills.items()):
        if skill_name in _activated_skill_names:
            continue

        patterns = skill_meta.get("paths", [])
        if not patterns:
            continue

        # Use gitignore-style matching via pathspec or simple substring
        from pathspec import PathSpec
        from pathspec.patterns import GitWildMatchPattern

        spec = PathSpec(map(GitWildMatchPattern, patterns))
        for fp in file_paths:
            try:
                rel = Path(fp).resolve().relative_to(Path(cwd).resolve())
            except ValueError:
                rel = Path(fp)

            if spec.match_file(str(rel)):
                _activated_skill_names.add(skill_name)
                _conditional_skills.pop(skill_name, None)
                activated.append(skill_name)
                logger.info(
                    "[skill_discovery] Activated conditional skill '%s' (matched: %s)",
                    skill_name,
                    rel,
                )
                break

    return activated


def get_discovered_skills() -> List[dict]:
    """Return all currently discovered skill metadata."""
    result: List[dict] = []
    for skills in _discovered_skills_cache.values():
        result.extend(skills)
    return result


def get_discovered_skill_names() -> List[str]:
    """Return names of all currently discovered skills."""
    return [s["name"] for s in get_discovered_skills()]


def get_activated_conditional_skills() -> List[str]:
    """Return names of activated conditional skills."""
    return list(_activated_skill_names)


def register_conditional_skills(skills: List[dict]) -> None:
    """Register conditional skills for path-based activation.

    Called by skills_tool at startup with skills that have ``paths`` frontmatter.

    Each skill dict must have:
        - ``name``: str
        - ``paths``: List[str] — gitignore-style path patterns
        - Optional: ``description``, ``path``
    """
    for s in skills:
        paths = s.get("paths", [])
        if paths and s["name"] not in _activated_skill_names:
            _conditional_skills[s["name"]] = s


# ── MCP Skills Integration ─────────────────────────────────────────────

_MCP_SKILLS: List[dict] = []
_MCP_SERVERS: Set[str] = set()


def load_mcp_skills(mcp_skills: List[dict], server_name: str = "") -> List[str]:
    """Load skills discovered from an MCP server into the available skills pool.

    Args:
        mcp_skills: List of skill metadata dicts from an MCP server.
            Each must have ``name`` (str) and may have ``description``, ``path``.
        server_name: Name of the MCP server that provided these skills (for logging).

    Returns:
        List of loaded skill names.
    """
    loaded: List[str] = []
    for skill in mcp_skills:
        name = skill.get("name", "")
        if not name:
            continue
        skill["_mcp_server"] = server_name
        skill["discovered"] = False
        _MCP_SKILLS.append(skill)
        loaded.append(name)
        logger.info(
            "[skill_discovery] Loaded MCP skill '%s' from server '%s'",
            name,
            server_name or "unknown",
        )
    if server_name:
        _MCP_SERVERS.add(server_name)
    return loaded


def get_mcp_skills() -> List[dict]:
    """Return all skills loaded from MCP servers."""
    return list(_MCP_SKILLS)


def get_mcp_skill_names() -> List[str]:
    """Return names of all skills loaded from MCP servers."""
    return [s["name"] for s in _MCP_SKILLS]


def get_mcp_server_names() -> List[str]:
    """Return names of MCP servers with loaded skills."""
    return sorted(_MCP_SERVERS)


# ── Skill Search ──────────────────────────────────────────────────────

_SKILL_TOKEN_PATTERN = re.compile(r"[a-z0-9]+")


def _tokenize(text: str) -> List[str]:
    return _SKILL_TOKEN_PATTERN.findall(text.lower())


def search_skills(query: str, max_results: int = 10) -> List[dict]:
    """Search available skills by keyword/description.

    Searches across all skill sources: local skills, discovered skills,
    conditional skills, and MCP skills.

    Args:
        query: Search query string.
        max_results: Maximum results to return (default: 10, max: 50).

    Returns:
        List of matching skill dicts ranked by relevance, each with
        ``name``, ``description``, ``source``, and ``relevance`` fields.
    """
    from tools.skills_tool import _find_all_skills

    max_results = max(1, min(max_results, 50))
    query_tokens = _tokenize(query)

    if not query_tokens:
        return []

    # Collect candidates from all sources
    candidates: List[dict] = []
    seen_names: Set[str] = set()

    # 1. Local skills from skills_tool
    try:
        for skill in _find_all_skills():
            name = skill.get("name", "")
            if name not in seen_names:
                seen_names.add(name)
                candidates.append({
                    **skill,
                    "source": skill.get("category", "local"),
                })
    except Exception as e:
        logger.debug("[skill_discovery] Error loading local skills: %s", e)

    # 2. Discovered skills (from file tree walks)
    for skill in get_discovered_skills():
        name = skill.get("name", "")
        if name not in seen_names:
            seen_names.add(name)
            candidates.append({**skill, "source": "discovered"})

    # 3. MCP skills
    for skill in _MCP_SKILLS:
        name = skill.get("name", "")
        if name not in seen_names:
            seen_names.add(name)
            candidates.append({
                **skill,
                "source": f"mcp:{skill.get('_mcp_server', 'unknown')}",
            })

    # 4. Conditional skills (registered but not yet activated)
    for name, meta in _conditional_skills.items():
        if name not in seen_names:
            seen_names.add(name)
            candidates.append({
                "name": name,
                "description": meta.get("description", ""),
                "source": "conditional",
                "path": meta.get("path", ""),
            })

    # Score candidates
    scored: List[Tuple[int, int, dict]] = []  # (neg_score, name_alpha, candidate)
    for candidate in candidates:
        score = 0
        name_tokens = _tokenize(candidate.get("name", ""))
        desc_text = candidate.get("description", "")
        desc_tokens = _tokenize(desc_text)

        for qt in query_tokens:
            # Exact name match
            if qt in name_tokens:
                score += 10
            elif any(qt in nt for nt in name_tokens):
                score += 5
            # Description match
            if qt in desc_tokens:
                score += 2
            elif qt in desc_text.lower():
                score += 1

        if score > 0:
            scored.append((-score, candidate.get("name", ""), candidate))

    # Sort by relevance desc, then name asc
    scored.sort(key=lambda x: (x[0], x[1]))
    results = scored[:max_results]

    return [
        {
            "name": r[2].get("name", ""),
            "description": r[2].get("description", "")[:200],
            "source": r[2].get("source", "unknown"),
            "path": r[2].get("path", ""),
            "relevance": -r[0],
        }
        for r in results
    ]


# ── Skill Verification ────────────────────────────────────────────────

REQUIRED_FRONTMATTER_FIELDS = {"name", "description"}
RECOMMENDED_FRONTMATTER_FIELDS = {"when_to_use", "version"}


def verify_skill_frontmatter(path: str) -> dict:
    """Validate a SKILL.md file has required frontmatter fields.

    Args:
        path: Absolute or relative path to a SKILL.md file.

    Returns:
        Dict with ``valid`` (bool), ``errors`` (list), ``warnings`` (list),
        and ``fields`` (dict of parsed frontmatter).
    """
    from tools.skills_tool import _parse_frontmatter, MAX_NAME_LENGTH, MAX_DESCRIPTION_LENGTH

    result: dict = {
        "valid": False,
        "errors": [],
        "warnings": [],
        "fields": {},
    }

    skill_path = Path(path)
    if not skill_path.is_file():
        result["errors"].append(f"File not found: {path}")
        return result

    try:
        content = skill_path.read_text(encoding="utf-8")
    except (UnicodeDecodeError, PermissionError, OSError) as e:
        result["errors"].append(f"Cannot read file: {e}")
        return result

    frontmatter, body = _parse_frontmatter(content)

    errors: List[str] = []
    warnings: List[str] = []

    # Check required fields
    for field in REQUIRED_FRONTMATTER_FIELDS:
        value = frontmatter.get(field)
        if not value or (isinstance(value, str) and not value.strip()):
            errors.append(f"Missing required frontmatter field: '{field}'")
        elif field == "name" and len(str(value)) > MAX_NAME_LENGTH:
            errors.append(
                f"Field 'name' exceeds {MAX_NAME_LENGTH} chars "
                f"(got {len(str(value))})"
            )
        elif field == "description" and len(str(value)) > MAX_DESCRIPTION_LENGTH:
            warnings.append(
                f"Field 'description' exceeds {MAX_DESCRIPTION_LENGTH} chars "
                f"(got {len(str(value))})"
            )

    # Check recommended fields
    for field in RECOMMENDED_FRONTMATTER_FIELDS:
        if field not in frontmatter or not frontmatter[field]:
            warnings.append(f"Missing recommended frontmatter field: '{field}'")

    # Check body content exists
    body_stripped = body.strip() if body else ""
    if len(body_stripped) < 10:
        warnings.append("Skill body is empty or very short")

    # Check file naming convention (should be dir/SKILL.md not just SKILL.md)
    if skill_path.name.lower() == "skill.md" and skill_path.parent == skill_path.parent.parent:
        warnings.append(
            "SKILL.md should be inside a named directory "
            "(e.g. my-skill/SKILL.md, not standalone SKILL.md)"
        )

    # Check for linked files reference
    references_dir = skill_path.parent / "references"
    templates_dir = skill_path.parent / "templates"
    assets_dir = skill_path.parent / "assets"

    linked = {}
    if references_dir.is_dir():
        linked["references"] = [
            str(f.name) for f in sorted(references_dir.iterdir()) if f.is_file()
        ]
    if templates_dir.is_dir():
        linked["templates"] = [
            str(f.name) for f in sorted(templates_dir.iterdir()) if f.is_file()
        ]
    if assets_dir.is_dir():
        linked["assets"] = [
            str(f.name) for f in sorted(assets_dir.iterdir()) if f.is_file()
        ]
    if linked:
        result["linked_files"] = linked

    result["valid"] = len(errors) == 0
    result["errors"] = errors
    result["warnings"] = warnings
    result["fields"] = {
        k: str(v)[:500] for k, v in frontmatter.items() if isinstance(v, (str, int, float, bool))
    }

    return result


def clear_discovery_state() -> None:
    """Reset all discovery state (for testing)."""
    _checked_dirs.clear()
    _discovered_dirs.clear()
    _discovered_skills_cache.clear()
    _conditional_skills.clear()
    _activated_skill_names.clear()


# ── MCP Skills Integration ──────────────────────────────────────────────────


def load_mcp_skills() -> List[dict]:
    """Discover skills from connected MCP servers.

    Each MCP server indicates skill availability via its server metadata
    or a dedicated ``skills/list`` request capability.  This function
    queries all connected servers and returns skills as standard dicts
    compatible with ``register_conditional_skills()``.

    Returns:
        List of skill dicts with keys: name, description, path (mcp://server/skill),
        discovered (True), source ("mcp").
    """
    skills: List[dict] = []
    try:
        from tools.mcp_hub import mcp_manager
        handles = mcp_manager.get_all_handles()
    except (ImportError, AttributeError):
        logger.debug("[skill_discovery] MCP hub not available")
        return skills
    except Exception as e:
        logger.debug("[skill_discovery] MCP hub error: %s", e)
        return skills

    for h in handles:
        if h.status != "connected":
            continue
        try:
            server_meta = getattr(h, "server_metadata", None) or {}
            server_caps = server_meta.get("capabilities", {})
            skill_cap = server_caps.get("skills")

            if not skill_cap:
                continue

            skill_list = skill_cap if isinstance(skill_cap, list) else [skill_cap]
            for entry in skill_list:
                name = entry.get("name", "")
                if not name:
                    continue
                skills.append({
                    "name": name,
                    "description": entry.get("description", f"MCP skill from {h.name}"),
                    "path": f"mcp://{h.name}/{name}",
                    "discovered": True,
                    "source": "mcp",
                    "mcp_server": h.name,
                })
        except Exception as e:
            logger.debug("[skill_discovery] MCP skill load error for %s: %s", h.name, e)

    return skills


# ── Skill Search ─────────────────────────────────────────────────────────────


def search_skills(query: str) -> List[dict]:
    """Search available skills by keyword across name and description.

    Uses case-insensitive substring matching across all discovered,
    conditional, and previously loaded skills.

    Args:
        query: Keyword or phrase to search for.

    Returns:
        List of matching skill dicts with an added ``relevance`` field
        (higher = better match).
    """
    if not query or not query.strip():
        return get_discovered_skills()

    q = query.strip().lower()
    results: List[dict] = []

    seen_names: Set[str] = set()

    # Search discovered skills
    for s in get_discovered_skills():
        score = _score_skill_match(s, q)
        if score > 0:
            s = dict(s)
            s["relevance"] = score
            results.append(s)
            seen_names.add(s["name"])

    # Search conditional skills
    for name, meta in _conditional_skills.items():
        if name in seen_names:
            continue
        s = dict(meta)
        s["name"] = name
        score = _score_skill_match(s, q)
        if score > 0:
            s["relevance"] = score
            results.append(s)
            seen_names.add(name)

    # Sort by relevance descending, then name ascending
    results.sort(key=lambda x: (-x.get("relevance", 0), x.get("name", "")))
    return results


def _score_skill_match(skill: dict, query: str) -> int:
    """Score a skill's match against a normalized lowercase query.

    Returns an integer score (higher = better match):
        100  — name exact match
        80   — name starts with query
        60   — name contains query as substring
        50   — description starts with query
        40   — description contains query as substring
        30   — query words appear in name or description
        20   — name subsequence match (fuzzy)
        0    — no match
    """
    name = skill.get("name", "").lower()
    desc = skill.get("description", "").lower()

    if name == query:
        return 100

    if name.startswith(query):
        return 80 + (1 if len(name) == len(query) else 0) * 10

    if query in name:
        return 60

    if desc.startswith(query):
        return 50

    if query in desc:
        return 40

    query_words = query.split()
    matched_words = sum(1 for w in query_words if w in name or w in desc)
    if matched_words == len(query_words) and matched_words > 0:
        return 30

    # Fuzzy subsequence match on name
    if _is_subsequence(query, name):
        return 20

    return 0


def _is_subsequence(needle: str, haystack: str) -> bool:
    """Return True if *needle* characters appear in order in *haystack*."""
    it = iter(haystack)
    return all(ch in it for ch in needle)


# ── Skill Frontmatter Verification ──────────────────────────────────────────


def verify_skill_frontmatter(path: str) -> dict:
    """Validate that a SKILL.md file has the required frontmatter fields.

    Checks:
        - File exists and is readable
        - Has YAML frontmatter (``---`` delimiters)
        - ``name`` field is present and non-empty (max 64 chars)
        - ``description`` field is present and non-empty (max 1024 chars)
        - ``version`` field is valid semver (if present)
        - YAML is well-formed (no parse errors)

    Args:
        path: Absolute or relative path to a SKILL.md file.

    Returns:
        Dict with:
            - valid: bool
            - errors: List[str] of validation failures
            - warnings: List[str] of non-blocking issues
            - frontmatter: Dict of parsed data (if parseable)
    """
    result: dict = {
        "valid": True,
        "errors": [],
        "warnings": [],
        "frontmatter": {},
    }

    skill_path = Path(path)

    # File exists
    if not skill_path.exists():
        result["valid"] = False
        result["errors"].append(f"File not found: {path}")
        return result

    if not skill_path.is_file():
        result["valid"] = False
        result["errors"].append(f"Not a file: {path}")
        return result

    # File is readable
    try:
        content = skill_path.read_text(encoding="utf-8")
    except (UnicodeDecodeError, PermissionError, OSError) as e:
        result["valid"] = False
        result["errors"].append(f"Cannot read file: {e}")
        return result

    # Has frontmatter delimiters
    stripped = content.lstrip()
    if not stripped.startswith("---"):
        result["valid"] = False
        result["errors"].append("Missing YAML frontmatter (file must start with '---')")
        result["warnings"].append("Content will be treated as plain markdown without frontmatter")
        return result

    # Parse frontmatter
    try:
        from tools.skills_tool import _parse_frontmatter
        frontmatter, body = _parse_frontmatter(content)
    except Exception as e:
        result["valid"] = False
        result["errors"].append(f"Frontmatter parse error: {e}")
        return result

    result["frontmatter"] = dict(frontmatter)

    # Validate required: name
    name = frontmatter.get("name", "")
    if not name or not str(name).strip():
        result["valid"] = False
        result["errors"].append("Required field 'name' is missing or empty")
    elif len(str(name)) > 64:
        result["valid"] = False
        result["errors"].append(
            f"Field 'name' exceeds 64 characters (got {len(str(name))})"
        )

    # Validate required: description
    desc = frontmatter.get("description", "")
    if not desc or not str(desc).strip():
        result["valid"] = False
        result["errors"].append("Required field 'description' is missing or empty")
    elif len(str(desc)) > 1024:
        result["warnings"].append(
            f"Field 'description' exceeds 1024 characters (got {len(str(desc))})"
        )

    # Validate optional: version (semver-like)
    version = frontmatter.get("version")
    if version is not None:
        v_str = str(version).strip()
        if v_str:
            import re as _re
            if not _re.match(r"^\d+\.\d+\.\d+", v_str):
                result["warnings"].append(
                    f"Field 'version' does not follow semver format (got '{v_str}')"
                )

    # Check body is non-empty
    if not body or not body.strip():
        result["warnings"].append("Skill has frontmatter but no body content")

    return result
    _MCP_SKILLS.clear()
    _MCP_SERVERS.clear()


# ── Tool Handlers ──────────────────────────────────────────────────────


def skill_verify_handler(args: dict, **kw) -> str:
    path = args.get("path", "").strip()
    if not path:
        return tool_error("A 'path' argument is required.", success=False)
    try:
        result = verify_skill_frontmatter(path)
        return tool_result(success=True, **result)
    except Exception as e:
        logger.exception("skill_verify error: %s", e)
        return tool_error(str(e), success=False)


def skill_search_handler(args: dict, **kw) -> str:
    query = args.get("query", "").strip()
    max_results = args.get("max_results", 10)
    if not isinstance(max_results, int):
        try:
            max_results = int(max_results)
        except (TypeError, ValueError):
            max_results = 10
    if not query:
        return tool_error("A non-empty 'query' is required.", success=False)
    try:
        results = search_skills(query, max_results=max_results)
        return tool_result(success=True, query=query, count=len(results), results=results)
    except Exception as e:
        logger.exception("skill_search error: %s", e)
        return tool_error(str(e), success=False)


def check_skill_discovery_requirements() -> bool:
    return True


# ── Schemas ────────────────────────────────────────────────────────────

SKILL_VERIFY_SCHEMA = {
    "name": "skill_verify",
    "description": (
        "Validate a SKILL.md file has correct frontmatter. Checks for required "
        "fields (name, description), recommended fields (when_to_use, version), "
        "character limits, body content, and linked reference/template directories. "
        "Returns validation errors, warnings, and parsed frontmatter fields."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "Absolute or relative path to a SKILL.md file to validate.",
            },
        },
        "required": ["path"],
    },
}

SKILL_SEARCH_SCHEMA = {
    "name": "skill_search",
    "description": (
        "Search available skills by keyword. Searches across all skill sources: "
        "local installed skills, dynamically discovered skills, MCP-provided skills, "
        "and conditional path-based skills. Results ranked by relevance with name "
        "matches scoring higher than description matches."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "Keyword search query for matching skill names and descriptions.",
            },
            "max_results": {
                "type": "integer",
                "description": "Maximum results to return (default: 10, max: 50).",
                "default": 10,
            },
        },
        "required": ["query"],
    },
}


# ── Registry ───────────────────────────────────────────────────────────

registry.register(
    name="skill_verify",
    toolset="skills",
    schema=SKILL_VERIFY_SCHEMA,
    handler=skill_verify_handler,
    check_fn=check_skill_discovery_requirements,
    emoji="✅",
)

registry.register(
    name="skill_search",
    toolset="skills",
    schema=SKILL_SEARCH_SCHEMA,
    handler=skill_search_handler,
    check_fn=check_skill_discovery_requirements,
    emoji="🔍",
)
