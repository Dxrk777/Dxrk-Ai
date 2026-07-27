# SPDX-License-Identifier: MIT
from __future__ import annotations

import fnmatch
import re
import threading
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional


__all__ = [
    "PermissionBehavior",
    "PermissionResult",
    "PermissionRule",
    "PermissionContext",
    "PermissionManager",
    "default_permission_manager",
]


class PermissionBehavior(Enum):
    ALLOW = "allow"
    DENY = "deny"
    ASK = "ask"
    PASSTHROUGH = "passthrough"


@dataclass
class PermissionResult:
    behavior: PermissionBehavior
    updated_input: Optional[dict] = None
    message: Optional[str] = None
    decision_reason: Optional[str] = None
    suggestions: Optional[list[str]] = None


@dataclass
class PermissionRule:
    source: str
    behavior: PermissionBehavior
    tool_name: str
    command_pattern: Optional[str] = None
    path_pattern: Optional[str] = None
    reason: str = ""


class PermissionContext:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self.deny_rules: list[PermissionRule] = []
        self.allow_rules: list[PermissionRule] = []

    def add_rule(self, rule: PermissionRule) -> None:
        with self._lock:
            if rule.behavior is PermissionBehavior.DENY:
                self.deny_rules.append(rule)
            elif rule.behavior is PermissionBehavior.ALLOW:
                self.allow_rules.append(rule)

    def remove_rules_by_source(self, source: str) -> None:
        with self._lock:
            self.deny_rules = [r for r in self.deny_rules if r.source != source]
            self.allow_rules = [r for r in self.allow_rules if r.source != source]

    def _match_tool_name(self, rule_tool_name: str, actual_tool_name: str) -> bool:
        if "*" in rule_tool_name or "?" in rule_tool_name:
            return fnmatch.fnmatch(actual_tool_name, rule_tool_name)
        return rule_tool_name == actual_tool_name

    def _find_exact_deny(self, tool_name: str) -> Optional[PermissionRule]:
        for rule in self.deny_rules:
            if (
                rule.command_pattern is None
                and rule.path_pattern is None
                and self._match_tool_name(rule.tool_name, tool_name)
            ):
                return rule
        return None

    def _find_exact_allow(self, tool_name: str) -> Optional[PermissionRule]:
        for rule in self.allow_rules:
            if (
                rule.command_pattern is None
                and rule.path_pattern is None
                and self._match_tool_name(rule.tool_name, tool_name)
            ):
                return rule
        return None

    def check_tool(self, tool_name: str, input_data: dict) -> PermissionResult:
        with self._lock:
            exact_deny = self._find_exact_deny(tool_name)
            if exact_deny is not None:
                return PermissionResult(
                    behavior=PermissionBehavior.DENY,
                    message=exact_deny.reason or f"Tool '{tool_name}' is denied",
                    decision_reason=f"explicit deny rule from {exact_deny.source}",
                )

            exact_allow = self._find_exact_allow(tool_name)
            if exact_allow is not None:
                return PermissionResult(
                    behavior=PermissionBehavior.ALLOW,
                    updated_input=input_data,
                    decision_reason=f"explicit allow rule from {exact_allow.source}",
                )

            deny_by_pattern = self._match_by_pattern(
                self.deny_rules, tool_name, input_data
            )
            if deny_by_pattern is not None:
                return PermissionResult(
                    behavior=PermissionBehavior.DENY,
                    message=deny_by_pattern.reason or f"Tool '{tool_name}' input denied by pattern",
                    decision_reason=f"pattern match deny from {deny_by_pattern.source}",
                )

            allow_by_pattern = self._match_by_pattern(
                self.allow_rules, tool_name, input_data
            )
            if allow_by_pattern is not None:
                return PermissionResult(
                    behavior=PermissionBehavior.ALLOW,
                    updated_input=input_data,
                    decision_reason=f"pattern match allow from {allow_by_pattern.source}",
                )

        return PermissionResult(
            behavior=PermissionBehavior.ASK,
            message=f"Tool '{tool_name}' requires confirmation",
            decision_reason="default: no matching rule",
        )

    def _match_by_pattern(
        self,
        rules: list[PermissionRule],
        tool_name: str,
        input_data: dict,
    ) -> Optional[PermissionRule]:
        for rule in rules:
            if not self._match_tool_name(rule.tool_name, tool_name):
                continue
            if rule.command_pattern is not None:
                command = input_data.get("command", "")
                if re.search(rule.command_pattern, str(command)):
                    return rule
            elif rule.path_pattern is not None:
                path = input_data.get("path", input_data.get("file_path", ""))
                if fnmatch.fnmatch(str(path), rule.path_pattern):
                    return rule
            elif rule.command_pattern is None and rule.path_pattern is None:
                continue
        return None

    def filter_denied_tools(self, tool_names: list[str]) -> list[str]:
        with self._lock:
            return [
                name
                for name in tool_names
                if self._find_exact_deny(name) is None
            ]

    def reset(self) -> None:
        with self._lock:
            self.deny_rules.clear()
            self.allow_rules.clear()


class PermissionManager:
    _instance: Optional[PermissionManager] = None
    _instance_lock: threading.Lock = threading.Lock()

    def __new__(cls) -> PermissionManager:
        if cls._instance is None:
            with cls._instance_lock:
                if cls._instance is None:
                    cls._instance = super().__new__(cls)
                    cls._instance._context = PermissionContext()
                    cls._instance._global_lock = threading.Lock()
        return cls._instance

    @property
    def context(self) -> PermissionContext:
        return self._context

    def add_rule(self, rule: PermissionRule) -> None:
        self._context.add_rule(rule)

    def remove_rules_by_source(self, source: str) -> None:
        self._context.remove_rules_by_source(source)

    def check_tool(self, tool_name: str, input_data: dict) -> PermissionResult:
        return self._context.check_tool(tool_name, input_data)

    def filter_denied_tools(self, tool_names: list[str]) -> list[str]:
        return self._context.filter_denied_tools(tool_names)

    def reset(self) -> None:
        self._context.reset()


def default_permission_manager() -> PermissionManager:
    return PermissionManager()
