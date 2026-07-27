# SPDX-License-Identifier: MIT
from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any, Optional


@dataclass
class ToolCall:
    name: str
    input: dict
    id: str = ""


@dataclass
class ToolResult:
    name: str
    output: str
    error: Optional[str] = None
    duration_ms: float = 0.0


def partition_tool_calls(
    calls: list[ToolCall],
    get_safety_flags: Callable[[str], dict],
) -> list[list[ToolCall]]:
    plan: list[list[ToolCall]] = []
    last_batch_batchable = False

    for call in calls:
        flags = get_safety_flags(call.name)
        is_batchable = flags.get("is_readonly", False) and flags.get(
            "is_concurrency_safe", False
        )

        if last_batch_batchable and is_batchable:
            plan[-1].append(call)
        else:
            plan.append([call])
            last_batch_batchable = is_batchable

    return plan


async def execute_batch_async(
    batch: list[ToolCall],
    executor: Callable[[ToolCall], Awaitable[ToolResult]],
) -> list[ToolResult]:
    return await asyncio.gather(*(executor(call) for call in batch))


async def execute_plan_async(
    plan: list[list[ToolCall]],
    executor: Callable[[ToolCall], Awaitable[ToolResult]],
) -> list[ToolResult]:
    results: list[ToolResult] = []
    for batch in plan:
        batch_results = await execute_batch_async(batch, executor)
        results.extend(batch_results)
    return results
