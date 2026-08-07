# ADR-0001: Port the Claude Code CLI reference architecture to Go

- **Status**: Accepted
- **Date**: 2026-08-04

## Context

Dxrk originally shipped as a Python-based ecosystem (see `/dxrk`, `pyproject.toml`).
The Claude Code CLI reference implementation (`/home/dxrk/Documentos/anthropic-ai-claude-code-cli-map-main`)
documents a feature set (hooks, HTTP client with retries/proxy/TLS, image format
detection, swarm/event-bus coordination, TUI screens, CLI commands) that the product
tracks. The decision was made to re-implement that surface in Go as a single
self-contained binary (`cmd/dxrk`) with a strict package layout under `internal/`.

## Decision Drivers

- Single static binary, no interpreter or virtualenv required at runtime.
- Strong static typing and a first-class concurrency story (goroutines/atomic) for
  the swarm/event-bus and hook-executor subsystems.
- Reproducible builds and trivial cross-compilation (macOS/Linux/Windows).
- `go vet` + `golangci-lint` as low-friction quality gates.

## Considered Options

- **Option A: Full Go port (chosen)**. One binary, compiler-checked APIs, easy CI.
  Downside: re-authoring effort and drift between Go and Python trees during the
  transition.
- **Option B: Keep Python, wrap in installer** (e.g. pip/scoop shims). Faster to
  ship but keeps runtime-fragility and interpreter dependency.
- **Option C: Hybrid (Go CLI in front of Python core)**. Retained for agent
  adapter bridging (gateway/`tui_gateway`) but rejected as the primary path because
  it splits deployment into two artifacts.

## Decision

Port the tracked Claude Code surface to Go 1.25 under module
`github.com/Dxrk777/Dxrk`, with packages organized as `internal/agents`,
`internal/cli`, `internal/commands`, `internal/components`, `internal/tui`, and
`internal/utils/{hooks,http,image,swarm,session,...}`. CLI entrypoint lives at
`cmd/dxrk`. Python trees remain for adapter/ecosystem features that are not part
of the core CLI surface.

## Consequences

- Build/vet/test can be enforced repo-wide with `go build ./...`, `go vet ./...`,
  and targeted `go test` suites; verification of Fases 1–8e is green on all four
  util packages plus `internal/{tui,cli}`.
- Type checking catches API drift at compile time (see ADR-0002 and ADR-0003,
  which record fixes surfaced by this approach).
- Two language ecosystems must be kept coherent; `docs/architecture.md` documents
  the Go layout so the mapping stays navigable.

## Links

- Updated `docs/architecture.md` (real layout, 42 agent adapters, 63 packages under `internal/`).