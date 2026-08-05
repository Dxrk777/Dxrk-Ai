# ADR-0003: Remove unused mutex from EnterprisePolicy value type

- **Status**: Accepted
- **Date**: 2026-08-04

## Context

`internal/plugin/policy.go` defined:

```go
type EnterprisePolicy struct {
    mu sync.RWMutex // unused lock inside a value type
    // ...
}
```

`PolicyManager` is the real concurrency boundary (`pm.mu sync.RWMutex`, used in
~10 sites). The embedded `EnterprisePolicy.mu` was never taken
(`grep ".policy.mu"` finds no uses), but its presence made the value type unsafe to
copy, tripping `go vet` copylocks at four sites: assignment in `LoadPolicy`
(policy.go:115), read in `SavePolicy` (:123), `json.MarshalIndent(policy, ...)`
(:129), and `return pm.policy` in `GetPolicy` (:140).

## Decision Drivers

- `go vet ./...` must be clean as a project quality gate.
- Remove dead state rather than paper over it with pointer indirection.

## Considered Options

- **Option A: Delete `EnterprisePolicy.mu` (chosen)**. The lock is dead; the
  manager owns synchronization. Value copies become safe; zero behavior change.
- **Option B: Convert to `*EnterprisePolicy` throughout**. Adds pointer plumbing
  and allocation churn for a lock that is never used; rejected.

## Decision

Delete the `mu sync.RWMutex` field from `EnterprisePolicy`. Keep all value
semantics and the `PolicyManager.mu` lock unchanged.

## Consequences

- `go vet ./...` clean for `internal/plugin`; no behavioral change.
- `go test ./internal/plugin/...` green (0.002s).
- Value-type copyability of `EnterprisePolicy` is now well-defined.

## Links

- `internal/plugin/policy.go` (`LoadPolicy`, `SavePolicy`, `GetPolicy`, `CheckInstall`).