# Mental Model

[Back to Codebase Guide](../CODEBASE-GUIDE.md)

Dxrk is an ecosystem configurator for AI coding agents. It owns the installer, sync flows, adapters, and managed asset injection; it does not own the runtime behavior of external agents or Dxrk-Memory internals.

## What this project is

| It is | Evidence in this repo |
|---|---|
| Go CLI and Bubbletea TUI | `cmd/dxrk/main.go`, `internal/app/`, `internal/tui/` |
| Agent configuration orchestrator | `internal/agents/`, `internal/components/`, `internal/planner/`, `internal/pipeline/` |
| Asset distributor | `internal/assets/`, `internal/components/sdd/`, `internal/components/skills/` |
| External tool integrator | `internal/components/dxrk-memory/`, `internal/components/dxrk-guardian/`, `internal/components/mcp/` |

## What this project is not

| It is not | Use this boundary |
|---|---|
| Dxrk-Memory's memory database implementation | This repo installs and configures Dxrk-Memory, then documents commands in `docs/dxrk-memory.md`. |
| A local dashboard server | No dashboard or HTMX server files are present in the repository. |
| A generic package manager | Dependency hints and installs support Dxrk components only. |
| The AI agent runtime | Claude Code, OpenCode, Cursor, and other agents consume generated config outside this repo. |

## 90-second architecture model

```text
cmd/dxrk/main.go
  -> internal/app          command dispatch, version wiring, help
  -> internal/cli          non-interactive install/sync/uninstall/restore flows
  -> internal/tui          interactive Bubbletea screens and async messages

install/sync path
  -> internal/system       platform and dependency detection
  -> internal/model        IDs and shared domain types
  -> internal/catalog      supported agent/component definitions
  -> internal/planner      dependency expansion and ordering
  -> internal/pipeline     staged execution and rollback
  -> internal/components   reusable component injection
  -> internal/agents       per-agent file paths and strategies
  -> internal/verify       post-apply readiness report
```

## Core invariants

- **Backups before mutation**: managed installs, syncs, and uninstalls must preserve a rollback path.
- **Adapters own paths**: per-agent config roots and strategy decisions stay in `internal/agents/<agent>/`.
- **Components own behavior**: Dxrk-Memory, SDD, skills, MCP, persona, permissions, Dxrk-Guardian, and plugins stay in `internal/components/<component>/`.
- **Planner owns ordering**: do not hand-sort component dependencies in CLI or TUI flows.
- **Sync stays idempotent**: running sync twice should not rewrite already-current managed assets.
- **External internals stay external**: do not document or change Dxrk-Memory cloud/dashboard/store internals as if they live here.

## Contributor checklist

- [ ] Name the user-facing behavior first.
- [ ] Identify whether the change belongs to an adapter, component, planner, pipeline, or UI screen.
- [ ] Check existing tests near that package before adding new patterns.
- [ ] Link detailed user behavior to existing docs instead of duplicating full references.

## Navigation

Previous: [Codebase Guide](../CODEBASE-GUIDE.md) | Next: [Repository map](repository-map.md)
