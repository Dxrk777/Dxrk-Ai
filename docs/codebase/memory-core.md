# Memory Core

[Back to Codebase Guide](../CODEBASE-GUIDE.md)

Dxrk wires Dxrk-Memory into agents; Dxrk-Memory owns the memory store. This page explains the boundary so maintainers do not confuse installer code with memory database code.

## Store responsibilities

| Responsibility | Owner in this repository | Actual memory owner |
|---|---|---|
| Install or download the `dxrk-memory` binary | `internal/components/dxrk-memory/install.go`, `download.go` | External Dxrk-Memory project/runtime |
| Add MCP config for agents | `internal/components/dxrk-memory/inject.go` | Agent consumes the config |
| Run `dxrk-memory setup` where supported | `internal/components/dxrk-memory/setup.go`, CLI runtime | Dxrk-Memory CLI implements setup behavior |
| Document user commands | `docs/dxrk-memory.md` | Dxrk-Memory CLI/MCP implementation |
| Store sessions, observations, prompts, relations, sync mutations | Not implemented here | Dxrk-Memory store |

## Memory entities

Dxrk docs and prompt assets refer to these Dxrk-Memory concepts, but their schema is not defined in this repo.

| Concept | Maintainer meaning |
|---|---|
| Sessions | Work periods that can be summarized and recovered later. |
| Observations | Saved decisions, discoveries, bug fixes, patterns, or artifacts. |
| Prompts | User prompts captured so later saves can attach intent. |
| Relations | Semantic links or conflict judgments between memories. |
| Sync mutations | Export/import changes used by Dxrk-Memory sync workflows. |

For command and MCP tool descriptions, link to [Dxrk-Memory Commands](../dxrk-memory.md) instead of duplicating an API reference.

## Save and retrieve flow

```text
AI agent receives prompt
  |
  v
Dxrk-installed prompt tells agent to use Dxrk-Memory MCP tools
  |
  v
Agent calls `dxrk-memory mcp --tools=agent` via configured MCP entry
  |
  +--> save: mem_save / mem_session_summary / related tools
  +--> retrieve: mem_context / mem_search / mem_get_observation
  |
  v
Dxrk-Memory runtime stores and searches memory outside dxrk source
```

## Memory invariants

- **MCP command must be stable**: `internal/components/dxrk-memory/inject.go` prefers stable command paths and preserves existing absolute paths where needed.
- **Agent setup is capability-based**: `SetupAgentSlug` intentionally returns no setup target for agents that use direct config injection.
- **Prompt assets must stay accurate**: Dxrk-Memory instructions in `internal/assets/` must match the public behavior documented in `docs/dxrk-memory.md`.
- **No schema invention**: do not document tables, HTTP routes, dashboard pages, or cloud internals unless source or external Dxrk-Memory docs confirm them.

## Contributor checklist

- [ ] Decide whether the change belongs to Dxrk wiring or Dxrk-Memory itself.
- [ ] Update MCP injection tests when changing config shape.
- [ ] Keep `docs/dxrk-memory.md` as the user-facing command reference.
- [ ] Do not read or modify local `.dxrk-memory/dxrk-memory.db` as part of codebase docs or tests.

## Navigation

Previous: [Repository map](repository-map.md) | Next: [Interfaces](interfaces.md)
