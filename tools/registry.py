# SPDX-License-Identifier: MIT
"""Central registry for all Dxrk tools.

Each tool file calls ``registry.register()`` at module level to declare its
schema, handler, toolset membership, and availability check.  ``model_tools.py``
queries the registry instead of maintaining its own parallel data structures.

Import chain (circular-import safe):
    tools/registry.py  (no imports from model_tools or tool files)
           ^
    tools/*.py  (import from tools.registry at module level)
           ^
    model_tools.py  (imports tools.registry + all tool modules)
           ^
    run_agent.py, cli.py, batch_runner.py, etc.

Architecture:
    Every tool handler MUST return a JSON string (legacy) OR a ToolResult
    (new pattern).  The ``dispatch()`` method serializes ToolResult at the
    single API boundary — tool implementations never call json.dumps
    directly.

    Tool lifecycle (new pattern):
    1. validate_input(input_schema)  → ValidationResult
    2. check_permissions(context)    → PermissionResult
    3. execute(input, context)       → ToolResult
    4. dispatch() serializes          → JSON string

    Fail-closed defaults via ToolDef:
    - is_destructive=False
    - is_readonly=False
    - is_concurrency_safe=False
    - is_hidden=False
"""

import ast
import dataclasses
import importlib
import json
import logging
import threading
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Set

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# ToolResult — structured result type (new pattern)
# ---------------------------------------------------------------------------
# Every tool handler returns a ToolResult.  The registry serializes it at
# the dispatch boundary so individual tools never call json.dumps.
# This is the single serialization boundary.

@dataclass
class ToolResult:
    """Structured result from a tool execution.

    Tools return this instead of raw JSON strings.  ``dispatch()``
    serializes it once at the API boundary.

    Fields:
        success:  Whether the tool succeeded.
        data:     Arbitrary result payload (dict, list, str, etc.).
        error:    Error message when success=False.  Mutually exclusive
                  with ``data`` in serialized output.
        warning:  Non-fatal advisory attached to the result.
        hint:     Guidance for the model on what to do next.
    """
    success: bool = True
    data: Any = None
    error: Optional[str] = None
    warning: Optional[str] = None
    hint: Optional[str] = None

    def to_dict(self) -> dict:
        d: dict = {}
        if self.error:
            d["error"] = self.error
            d["success"] = False
        else:
            d["success"] = self.success
            if self.data is not None:
                d["data"] = self.data
        if self.warning:
            d["_warning"] = self.warning
        if self.hint:
            d["_hint"] = self.hint
        return d

    def to_json(self) -> str:
        return json.dumps(self.to_dict(), ensure_ascii=False)


def tool_ok(data=None, **kw) -> ToolResult:
    """Shortcut for a successful ToolResult.

    >>> tool_ok({"files": ["a.py", "b.py"]})
    ToolResult(success=True, data={'files': ['a.py', 'b.py']})
    """
    if data is not None:
        return ToolResult(success=True, data=data)
    return ToolResult(success=True, data=kw)


def tool_fail(error: str, hint: str = None, warning: str = None) -> ToolResult:
    """Shortcut for a failed ToolResult.

    >>> tool_fail("file not found")
    ToolResult(success=False, error='file not found')
    """
    return ToolResult(success=False, error=error, hint=hint, warning=warning)


def tool_result_from(data: Any) -> ToolResult:
    """Convert a legacy dict or ToolResult to a ToolResult.

    Handles:
    - ToolResult → pass through
    - dict → ToolResult (uses 'error' key to determine success)
    - str → parses JSON first, then wraps

    This is the glue layer for the migration period.
    """
    if isinstance(data, ToolResult):
        return data
    if isinstance(data, str):
        try:
            data = json.loads(data)
        except (json.JSONDecodeError, TypeError):
            return ToolResult(success=True, data=data)
    if isinstance(data, dict):
        if "error" in data:
            return ToolResult(
                success=False,
                error=data.pop("error", None),
                data=data if data else None,
                warning=data.pop("_warning", None),
                hint=data.pop("_hint", None),
            )
        return ToolResult(
            success=True,
            data=data,
            warning=data.pop("_warning", None) if isinstance(data, dict) else None,
            hint=data.pop("_hint", None) if isinstance(data, dict) else None,
        )
    return ToolResult(success=True, data=data)


# ---------------------------------------------------------------------------
# ValidationResult — input validation outcome
# ---------------------------------------------------------------------------

@dataclass
class ValidationResult:
    """Result of input validation.

    When ``valid`` is True, execution proceeds.  When False, ``message``
    and optionally ``error_code`` are returned to the caller without
    executing the tool.
    """
    valid: bool
    message: str = ""
    error_code: int = 0


# ---------------------------------------------------------------------------
# ToolDef — fail-closed tool definition (new pattern)
# ---------------------------------------------------------------------------
# Tools define their metadata and lifecycle hooks via a ToolDef dataclass.
# The registry applies fail-closed defaults so every tool is:
# - NOT destructive (unless explicitly marked)
# - NOT concurrency-safe (unless explicitly marked)
# - NOT hidden from the model (unless explicitly marked)
# - NOT read-only (unless explicitly marked)

@dataclass
class ToolDef:
    """Fail-closed tool definition.

    All safety flags default to the most restrictive option::
        is_destructive=False
        is_concurrency_safe=False
        is_hidden=False
        is_readonly=False

    Tools MUST opt in to destructive or concurrent behavior explicitly.
    """
    # Identity
    name: str
    toolset: str
    schema: dict

    # Execution
    handler: Callable
    check_fn: Optional[Callable] = None
    validate_fn: Optional[Callable] = None   # (input) -> ValidationResult
    permission_fn: Optional[Callable] = None # (input, ctx) -> PermissionResult

    # Fail-closed safety flags
    is_destructive: bool = False
    is_concurrency_safe: bool = False
    is_hidden: bool = False
    is_readonly: bool = False

    # Metadata
    requires_env: List[str] = field(default_factory=list)
    is_async: bool = False
    description: str = ""
    emoji: str = ""
    max_result_size_chars: Optional[float] = None

    def to_entry(self) -> 'ToolEntry':
        return ToolEntry(
            name=self.name,
            toolset=self.toolset,
            schema=self.schema,
            handler=self.handler,
            check_fn=self.check_fn,
            validate_fn=self.validate_fn,
            permission_fn=self.permission_fn,
            requires_env=self.requires_env,
            is_async=self.is_async,
            description=self.description or self.schema.get("description", ""),
            emoji=self.emoji,
            max_result_size_chars=self.max_result_size_chars,
            is_destructive=self.is_destructive,
            is_concurrency_safe=self.is_concurrency_safe,
            is_hidden=self.is_hidden,
            is_readonly=self.is_readonly,
        )


def _is_registry_register_call(node: ast.AST) -> bool:
    """Return True when *node* is a ``registry.register(...)`` call expression."""
    if not isinstance(node, ast.Expr) or not isinstance(node.value, ast.Call):
        return False
    func = node.value.func
    return (
        isinstance(func, ast.Attribute)
        and func.attr == "register"
        and isinstance(func.value, ast.Name)
        and func.value.id == "registry"
    )


def _module_registers_tools(module_path: Path) -> bool:
    """Return True when the module contains a top-level ``registry.register(...)`` call.

    Only inspects module-body statements so that helper modules which happen
    to call ``registry.register()`` inside a function are not picked up.
    """
    try:
        source = module_path.read_text(encoding="utf-8")
        tree = ast.parse(source, filename=str(module_path))
    except (OSError, SyntaxError):
        return False

    return any(_is_registry_register_call(stmt) for stmt in tree.body)


def discover_builtin_tools(tools_dir: Optional[Path] = None) -> List[str]:
    """Import built-in self-registering tool modules and return their module names."""
    tools_path = Path(tools_dir) if tools_dir is not None else Path(__file__).resolve().parent
    module_names = [
        f"tools.{path.stem}"
        for path in sorted(tools_path.glob("*.py"))
        if path.name not in {"__init__.py", "registry.py", "mcp_tool.py"}
        and _module_registers_tools(path)
    ]

    imported: List[str] = []
    for mod_name in module_names:
        try:
            importlib.import_module(mod_name)
            imported.append(mod_name)
        except Exception as e:
            logger.warning("Could not import tool module %s: %s", mod_name, e)
    return imported


class ToolEntry:
    """Metadata for a single registered tool.

    Supports both legacy (dict handler) and new (ToolDef) patterns.
    The new pattern adds:
    - ``validate_fn`` — input validation before execution
    - ``permission_fn`` — permission check before execution
    - ``is_destructive`` / ``is_concurrency_safe`` / ``is_hidden`` / ``is_readonly``
      — fail-closed safety flags (all default to False / restrictive)
    """

    __slots__ = (
        "name", "toolset", "schema", "handler", "check_fn",
        "validate_fn", "permission_fn",
        "requires_env", "is_async", "description", "emoji",
        "max_result_size_chars",
        "is_destructive", "is_concurrency_safe", "is_hidden", "is_readonly",
    )

    def __init__(self, name, toolset, schema, handler, check_fn,
                 requires_env=None, is_async=False, description="", emoji="",
                 max_result_size_chars=None,
                 validate_fn=None, permission_fn=None,
                 is_destructive=False, is_concurrency_safe=False,
                 is_hidden=False, is_readonly=False):
        self.name = name
        self.toolset = toolset
        self.schema = schema
        self.handler = handler
        self.check_fn = check_fn
        self.validate_fn = validate_fn
        self.permission_fn = permission_fn
        self.requires_env = requires_env or []
        self.is_async = is_async
        self.description = description
        self.emoji = emoji
        self.max_result_size_chars = max_result_size_chars
        self.is_destructive = is_destructive
        self.is_concurrency_safe = is_concurrency_safe
        self.is_hidden = is_hidden
        self.is_readonly = is_readonly


class ToolRegistry:
    """Singleton registry that collects tool schemas + handlers from tool files."""

    def __init__(self):
        self._tools: Dict[str, ToolEntry] = {}
        self._toolset_checks: Dict[str, Callable] = {}
        self._toolset_aliases: Dict[str, str] = {}
        # MCP dynamic refresh can mutate the registry while other threads are
        # reading tool metadata, so keep mutations serialized and readers on
        # stable snapshots.
        self._lock = threading.RLock()

    def _snapshot_state(self) -> tuple[List[ToolEntry], Dict[str, Callable]]:
        """Return a coherent snapshot of registry entries and toolset checks."""
        with self._lock:
            return list(self._tools.values()), dict(self._toolset_checks)

    def _snapshot_entries(self) -> List[ToolEntry]:
        """Return a stable snapshot of registered tool entries."""
        return self._snapshot_state()[0]

    def _snapshot_toolset_checks(self) -> Dict[str, Callable]:
        """Return a stable snapshot of toolset availability checks."""
        return self._snapshot_state()[1]

    def _evaluate_toolset_check(self, toolset: str, check: Callable | None) -> bool:
        """Run a toolset check, treating missing or failing checks as unavailable/available."""
        if not check:
            return True
        try:
            return bool(check())
        except Exception:
            logger.debug("Toolset %s check raised; marking unavailable", toolset)
            return False

    def get_entry(self, name: str) -> Optional[ToolEntry]:
        """Return a registered tool entry by name, or None."""
        with self._lock:
            return self._tools.get(name)

    def get_registered_toolset_names(self) -> List[str]:
        """Return sorted unique toolset names present in the registry."""
        return sorted({entry.toolset for entry in self._snapshot_entries()})

    def get_tool_names_for_toolset(self, toolset: str) -> List[str]:
        """Return sorted tool names registered under a given toolset."""
        return sorted(
            entry.name for entry in self._snapshot_entries()
            if entry.toolset == toolset
        )

    def register_toolset_alias(self, alias: str, toolset: str) -> None:
        """Register an explicit alias for a canonical toolset name."""
        with self._lock:
            existing = self._toolset_aliases.get(alias)
            if existing and existing != toolset:
                logger.warning(
                    "Toolset alias collision: '%s' (%s) overwritten by %s",
                    alias, existing, toolset,
                )
            self._toolset_aliases[alias] = toolset

    def get_registered_toolset_aliases(self) -> Dict[str, str]:
        """Return a snapshot of ``{alias: canonical_toolset}`` mappings."""
        with self._lock:
            return dict(self._toolset_aliases)

    def get_toolset_alias_target(self, alias: str) -> Optional[str]:
        """Return the canonical toolset name for an alias, or None."""
        with self._lock:
            return self._toolset_aliases.get(alias)

    # ------------------------------------------------------------------
    # Registration
    # ------------------------------------------------------------------

    def register(
        self,
        name: str,
        toolset: str,
        schema: dict,
        handler: Callable,
        check_fn: Callable = None,
        requires_env: list = None,
        is_async: bool = False,
        description: str = "",
        emoji: str = "",
        max_result_size_chars: int | float | None = None,
        # New pipeline fields
        validate_fn: Callable = None,
        permission_fn: Callable = None,
        is_destructive: bool = False,
        is_concurrency_safe: bool = False,
        is_hidden: bool = False,
        is_readonly: bool = False,
    ):
        """Register a tool.

        Called at module-import time by each tool file.

        New pattern supports the full lifecycle:
        1. ``validate_fn(input)`` → ValidationResult  (pre-execution)
        2. ``permission_fn(input, ctx)`` → PermissionResult  (pre-execution)
        3. ``handler(input, **ctx)`` → ToolResult | str  (execution)
        4. dispatch() serializes ToolResult at the boundary

        Safety flags default to False (fail-closed):
        - is_destructive: can modify/delete data
        - is_concurrency_safe: safe to run in parallel
        - is_hidden: excluded from model-visible tool lists
        - is_readonly: only reads, never writes
        """
        with self._lock:
            existing = self._tools.get(name)
            if existing and existing.toolset != toolset:
                both_mcp = (
                    existing.toolset.startswith("mcp-")
                    and toolset.startswith("mcp-")
                )
                if both_mcp:
                    logger.debug(
                        "Tool '%s': MCP toolset '%s' overwriting MCP toolset '%s'",
                        name, toolset, existing.toolset,
                    )
                else:
                    logger.error(
                        "Tool registration REJECTED: '%s' (toolset '%s') would "
                        "shadow existing tool from toolset '%s'. Deregister the "
                        "existing tool first if this is intentional.",
                        name, toolset, existing.toolset,
                    )
                    return
            self._tools[name] = ToolEntry(
                name=name,
                toolset=toolset,
                schema=schema,
                handler=handler,
                check_fn=check_fn,
                validate_fn=validate_fn,
                permission_fn=permission_fn,
                requires_env=requires_env or [],
                is_async=is_async,
                description=description or schema.get("description", ""),
                emoji=emoji,
                max_result_size_chars=max_result_size_chars,
                is_destructive=is_destructive,
                is_concurrency_safe=is_concurrency_safe,
                is_hidden=is_hidden,
                is_readonly=is_readonly,
            )
            if check_fn and toolset not in self._toolset_checks:
                self._toolset_checks[toolset] = check_fn

    def register_def(self, tool_def: ToolDef) -> None:
        """Register a tool from a ToolDef dataclass.

        Convenience wrapper around ``register()`` that unpacks a ToolDef.
        """
        self.register(
            name=tool_def.name,
            toolset=tool_def.toolset,
            schema=tool_def.schema,
            handler=tool_def.handler,
            check_fn=tool_def.check_fn,
            validate_fn=tool_def.validate_fn,
            permission_fn=tool_def.permission_fn,
            is_destructive=tool_def.is_destructive,
            is_concurrency_safe=tool_def.is_concurrency_safe,
            is_hidden=tool_def.is_hidden,
            is_readonly=tool_def.is_readonly,
            requires_env=tool_def.requires_env,
            is_async=tool_def.is_async,
            description=tool_def.description,
            emoji=tool_def.emoji,
            max_result_size_chars=tool_def.max_result_size_chars,
        )

    def deregister(self, name: str) -> None:
        """Remove a tool from the registry.

        Also cleans up the toolset check if no other tools remain in the
        same toolset.  Used by MCP dynamic tool discovery to nuke-and-repave
        when a server sends ``notifications/tools/list_changed``.
        """
        with self._lock:
            entry = self._tools.pop(name, None)
            if entry is None:
                return
            # Drop the toolset check and aliases if this was the last tool in
            # that toolset.
            toolset_still_exists = any(
                e.toolset == entry.toolset for e in self._tools.values()
            )
            if not toolset_still_exists:
                self._toolset_checks.pop(entry.toolset, None)
                self._toolset_aliases = {
                    alias: target
                    for alias, target in self._toolset_aliases.items()
                    if target != entry.toolset
                }
        logger.debug("Deregistered tool: %s", name)

    # ------------------------------------------------------------------
    # Schema retrieval
    # ------------------------------------------------------------------

    def get_definitions(self, tool_names: Set[str], quiet: bool = False) -> List[dict]:
        """Return OpenAI-format tool schemas for the requested tool names.

        Only tools whose ``check_fn()`` returns True (or have no check_fn)
        are included.
        """
        result = []
        check_results: Dict[Callable, bool] = {}
        entries_by_name = {entry.name: entry for entry in self._snapshot_entries()}
        for name in sorted(tool_names):
            entry = entries_by_name.get(name)
            if not entry:
                continue
            if entry.check_fn:
                if entry.check_fn not in check_results:
                    try:
                        check_results[entry.check_fn] = bool(entry.check_fn())
                    except Exception:
                        check_results[entry.check_fn] = False
                        if not quiet:
                            logger.debug("Tool %s check raised; skipping", name)
                if not check_results[entry.check_fn]:
                    if not quiet:
                        logger.debug("Tool %s unavailable (check failed)", name)
                    continue
            # Ensure schema always has a "name" field — use entry.name as fallback
            schema_with_name = {**entry.schema, "name": entry.name}
            result.append({"type": "function", "function": schema_with_name})
        return result

    # ------------------------------------------------------------------
    # Dispatch
    # ------------------------------------------------------------------

    def _check_permissions(self, entry: ToolEntry, args: dict, kwargs: dict) -> Optional[str]:
        """Run permission check.  Returns a JSON error string if denied, None if allowed."""
        if not entry.permission_fn:
            return None
        try:
            from tools.permission_system import PermissionBehavior
            presult = entry.permission_fn(args, kwargs)
            if presult.behavior != PermissionBehavior.ALLOW:
                pmsg = presult.message or "Permission denied"
                return tool_fail(pmsg, hint=presult.decision_reason).to_json()
            return None
        except ImportError:
            logger.warning("permission_system not available — skipping permission check for %s", entry.name)
            return None

    def dispatch(self, name: str, args: dict, **kwargs) -> str:
        """Execute a tool handler by name.

        Pipeline (new pattern):
        1. validate_fn(input)   → stops execution if invalid
        2. permission_fn(input) → stops execution if denied
        3. handler(input)       → returns ToolResult or str
        4. normalize + serialize at boundary

        Legacy handlers (returning str / dict) still work — they pass
        through ``tool_result_from()`` to the serialization boundary.

        Async handlers are bridged automatically via ``_run_async()``.
        All unhandled exceptions are caught and formatted consistently.
        """
        entry = self.get_entry(name)
        if not entry:
            return tool_fail(f"Unknown tool: {name}").to_json()
        try:
            # --- pipeline stage 1: validate input ---
            if entry.validate_fn:
                vresult = entry.validate_fn(args)
                if not vresult.valid:
                    return tool_fail(
                        vresult.message,
                        hint=f"error_code={vresult.error_code}",
                    ).to_json()

            # --- pipeline stage 2: check permissions ---
            denied = self._check_permissions(entry, args, kwargs)
            if denied:
                return denied

            # --- pipeline stage 3: execute ---
            if entry.is_async:
                from model_tools import _run_async
                raw = _run_async(entry.handler(args, **kwargs))
            else:
                raw = entry.handler(args, **kwargs)

            # --- pipeline stage 4: normalize + serialize ---
            result = tool_result_from(raw)
            if result.error and not result.success:
                logger.debug("Tool %s returned error: %s", name, result.error)
            return result.to_json()

        except Exception as e:
            logger.exception("Tool %s dispatch error: %s", name, e)
            return tool_fail(
                f"Tool execution failed: {type(e).__name__}: {e}",
            ).to_json()

    # ------------------------------------------------------------------
    # Query helpers  (replace redundant dicts in model_tools.py)
    # ------------------------------------------------------------------

    def get_max_result_size(self, name: str, default: int | float | None = None) -> int | float:
        """Return per-tool max result size, or *default* (or global default)."""
        entry = self.get_entry(name)
        if entry and entry.max_result_size_chars is not None:
            return entry.max_result_size_chars
        if default is not None:
            return default
        from tools.budget_config import DEFAULT_RESULT_SIZE_CHARS
        return DEFAULT_RESULT_SIZE_CHARS

    def get_all_tool_names(self) -> List[str]:
        """Return sorted list of all registered tool names."""
        return sorted(entry.name for entry in self._snapshot_entries())

    def get_schema(self, name: str) -> Optional[dict]:
        """Return a tool's raw schema dict, bypassing check_fn filtering.

        Useful for token estimation and introspection where availability
        doesn't matter — only the schema content does.
        """
        entry = self.get_entry(name)
        return entry.schema if entry else None

    def get_toolset_for_tool(self, name: str) -> Optional[str]:
        """Return the toolset a tool belongs to, or None."""
        entry = self.get_entry(name)
        return entry.toolset if entry else None

    def get_safety_flags(self, name: str) -> Dict[str, bool]:
        """Return safety flags for a tool.

        Returns fail-closed defaults if the tool is not registered:
        ``{"is_destructive": False, "is_readonly": False,
          "is_concurrency_safe": False, "is_hidden": False}``
        """
        entry = self.get_entry(name)
        return {
            "is_destructive": entry.is_destructive if entry else False,
            "is_readonly": entry.is_readonly if entry else False,
            "is_concurrency_safe": entry.is_concurrency_safe if entry else False,
            "is_hidden": entry.is_hidden if entry else False,
        }

    def get_emoji(self, name: str, default: str = "⚡") -> str:
        """Return the emoji for a tool, or *default* if unset."""
        entry = self.get_entry(name)
        return (entry.emoji if entry and entry.emoji else default)

    def get_tool_to_toolset_map(self) -> Dict[str, str]:
        """Return ``{tool_name: toolset_name}`` for every registered tool."""
        return {entry.name: entry.toolset for entry in self._snapshot_entries()}

    def is_toolset_available(self, toolset: str) -> bool:
        """Check if a toolset's requirements are met.

        Returns False (rather than crashing) when the check function raises
        an unexpected exception (e.g. network error, missing import, bad config).
        """
        with self._lock:
            check = self._toolset_checks.get(toolset)
        return self._evaluate_toolset_check(toolset, check)

    def check_toolset_requirements(self) -> Dict[str, bool]:
        """Return ``{toolset: available_bool}`` for every toolset."""
        entries, toolset_checks = self._snapshot_state()
        toolsets = sorted({entry.toolset for entry in entries})
        return {
            toolset: self._evaluate_toolset_check(toolset, toolset_checks.get(toolset))
            for toolset in toolsets
        }

    def get_available_toolsets(self) -> Dict[str, dict]:
        """Return toolset metadata for UI display."""
        toolsets: Dict[str, dict] = {}
        entries, toolset_checks = self._snapshot_state()
        for entry in entries:
            ts = entry.toolset
            if ts not in toolsets:
                toolsets[ts] = {
                    "available": self._evaluate_toolset_check(
                        ts, toolset_checks.get(ts)
                    ),
                    "tools": [],
                    "description": "",
                    "requirements": [],
                }
            toolsets[ts]["tools"].append(entry.name)
            if entry.requires_env:
                for env in entry.requires_env:
                    if env not in toolsets[ts]["requirements"]:
                        toolsets[ts]["requirements"].append(env)
        return toolsets

    def get_toolset_requirements(self) -> Dict[str, dict]:
        """Build a TOOLSET_REQUIREMENTS-compatible dict for backward compat."""
        result: Dict[str, dict] = {}
        entries, toolset_checks = self._snapshot_state()
        for entry in entries:
            ts = entry.toolset
            if ts not in result:
                result[ts] = {
                    "name": ts,
                    "env_vars": [],
                    "check_fn": toolset_checks.get(ts),
                    "setup_url": None,
                    "tools": [],
                }
            if entry.name not in result[ts]["tools"]:
                result[ts]["tools"].append(entry.name)
            for env in entry.requires_env:
                if env not in result[ts]["env_vars"]:
                    result[ts]["env_vars"].append(env)
        return result

    def check_tool_availability(self, quiet: bool = False):
        """Return (available_toolsets, unavailable_info) like the old function."""
        available = []
        unavailable = []
        seen = set()
        entries, toolset_checks = self._snapshot_state()
        for entry in entries:
            ts = entry.toolset
            if ts in seen:
                continue
            seen.add(ts)
            if self._evaluate_toolset_check(ts, toolset_checks.get(ts)):
                available.append(ts)
            else:
                unavailable.append({
                    "name": ts,
                    "env_vars": entry.requires_env,
                    "tools": [e.name for e in entries if e.toolset == ts],
                })
        return available, unavailable


# Module-level singleton
registry = ToolRegistry()


# ---------------------------------------------------------------------------
# Helpers for tool response serialization
# ---------------------------------------------------------------------------
# Every tool handler must return a JSON string.  These helpers eliminate the
# boilerplate ``json.dumps({"error": msg}, ensure_ascii=False)`` that appears
# hundreds of times across tool files.
#
# Usage:
#   from tools.registry import registry, tool_error, tool_result
#
#   return tool_error("something went wrong")
#   return tool_error("not found", code=404)
#   return tool_result(success=True, data=payload)
#   return tool_result(items)            # pass a dict directly


def tool_error(message, **extra) -> str:
    """Return a JSON error string for tool handlers.

    >>> tool_error("file not found")
    '{"error": "file not found"}'
    >>> tool_error("bad input", success=False)
    '{"error": "bad input", "success": false}'
    """
    result = {"error": str(message)}
    if extra:
        result.update(extra)
    return json.dumps(result, ensure_ascii=False)


def tool_result(data=None, **kwargs) -> str:
    """Return a JSON result string for tool handlers.

    Accepts a dict positional arg *or* keyword arguments (not both):

    >>> tool_result(success=True, count=42)
    '{"success": true, "count": 42}'
    >>> tool_result({"key": "value"})
    '{"key": "value"}'
    """
    if data is not None:
        return json.dumps(data, ensure_ascii=False)
    return json.dumps(kwargs, ensure_ascii=False)
