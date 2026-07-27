# Sync and Cloud

[Back to Codebase Guide](../CODEBASE-GUIDE.md)

Dxrk sync refreshes managed agent configuration. Dxrk-Memory sync exports/imports memory. Cloud sync is not implemented in this source tree.

## Sync boundaries

| Flow | Command surface | Owner | What changes |
|---|---|---|---|
| Dxrk config sync | `dxrk sync` | `internal/cli/sync.go`, components, adapters | Agent prompts, skills, MCP configs, SDD profiles, Dxrk-Guardian assets. |
| Dxrk-Memory git-friendly sync | `dxrk-memory sync`, `dxrk-memory sync --import` | External Dxrk-Memory runtime | `.dxrk-memory/` memory export/import for team sharing. |
| Cloud sync | Not present in Dxrk source | External or future Dxrk-Memory capability | Do not document implementation here without source. |
| Autosync | Not present in Dxrk source | External or future Dxrk-Memory capability | Do not imply background sync exists in this repo. |

## Dxrk sync path

```text
dxrk sync
  -> parse sync flags
  -> discover installed agents from ~/.dxrk/state.json or explicit flags
  -> build managed selection
  -> run component injectors
  -> verify readiness
  -> report files changed or no-op
```

Important behavior from `internal/cli/sync.go`:

- Default sync scope includes SDD, Dxrk-Memory, Context7, Dxrk-Guardian, and skills.
- Persona, permissions, and theme are user-adjacent and not included by default.
- OpenCode SDD profile flags preserve and update profile model assignments.
- Idempotency matters: `FilesChanged == 0` means managed assets were already current.

## Git-friendly memory sync

Dxrk-Memory team sharing is documented in [Dxrk-Memory Commands](../dxrk-memory.md). The important maintainer distinction: `dxrk-memory sync` exports memory to `.dxrk-memory/`; `dxrk sync` refreshes agent configuration.

## Remote transport boundary

No remote transport implementation is present in this repository beyond update/download logic for external binaries and releases. Do not describe an Dxrk-Memory cloud transport, cloud server, or cloud store split as Dxrk code unless that code is added here.

## Cloud server/cloud store split

This repository does not contain cloud server or cloud store packages. If future Dxrk-Memory cloud docs are added, document them as an external Dxrk-Memory responsibility and keep this page focused on how Dxrk discovers, installs, or configures that capability.

## Contributor checklist

- [ ] Use `dxrk sync` for managed config, not memory export/import.
- [ ] Use `dxrk-memory sync` docs for memory sharing behavior.
- [ ] Keep sync changes idempotent and test `FilesChanged` expectations.
- [ ] Do not touch untracked local `.dxrk-memory/cloud.json` or `.dxrk-memory/dxrk-memory.db` during docs or sync work.

## Navigation

Previous: [Interfaces](interfaces.md) | Next: [Dashboard](dashboard.md)
