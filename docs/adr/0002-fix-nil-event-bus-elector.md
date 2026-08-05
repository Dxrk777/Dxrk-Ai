# ADR-0002: Fix nil event bus in SwarmCoordinator leader election

- **Status**: Accepted
- **Date**: 2026-08-04

## Context

`internal/utils/swarm/coordinator.go` built its `LeaderElection` before creating
the `EventBus`:

```go
election := NewLeaderElection(swarmConfig, registry, nil) // nil *EventBus
eventBus := NewEventBus(ctx)
```

The election loop (`tryElect`, `election.go:115`) publishes `EventLeaderElected`
/ `EventLeaderLost` events on the stored bus. With a nil receiver this panics with
`sync/atomic.(*Bool).Load -> (*EventBus).Publish` (events.go:65). The panic only
surfaced when the full `internal/utils/swarm` test suite exercised the coordinator
path, not in isolated `TestLeaderElection`/`TestEventBus` runs.

## Decision Drivers

- The coordinator unconditionally starts the election on `Start()`, so the bus must
  always exist before any publish can happen.
- A nil-safe guard would hide real wiring mistakes; ordering is the correct fix.

## Considered Options

- **Option A: Reorder construction (chosen)**. Create `eventBus := NewEventBus(ctx)`
  first, then `NewLeaderElection(swarmConfig, registry, eventBus)`.
- **Option B: Nil-guard in `Publish`/`tryElect`**. Masks the wiring bug instead of
  fixing it; rejected.

## Decision

Reorder constructor statements in `coordinator.go` so `EventBus` is created before
`LeaderElection` and passed through. No nil-guard added to `Publish`.

## Consequences

- Production fix: coordinator-driven elections no longer SIGSEGV.
- `go test ./internal/utils/swarm/...` green (0.555s).
- Do not revert; this is a real wiring bug, not a test artifact.

## Links

- `internal/utils/swarm/coordinator.go`
- `internal/utils/swarm/election.go` (`tryElect`, event publishes), `events.go` (`Publish`).