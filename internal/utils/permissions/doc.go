// SPDX-License-Identifier: MIT

// Package permissions provides comprehensive permission management, policy
// evaluation, and access control utilities for Dxrk-Ai.
//
// The package implements a multi-layered permission system inspired by the
// Claude Code architecture, supporting enterprise-grade access control with
// the following components:
//
//   - Policy Engine: Rule-based evaluation with conditions, priorities, and
//     conflict detection.
//   - Layered Policy: 5-layer hierarchy (Organization > Project > User >
//     Session > Default) with rule merging across layers.
//   - Permission Cache: Thread-safe LRU cache with TTL and optional disk
//     persistence for session-level decisions.
//   - Tool Classification: Automatic categorization of tools and resources
//     by type and risk level.
//   - Audit Logging: Ring-buffer audit trail with query, export, and
//     streaming capabilities.
//
// # Usage
//
// Typical usage combines layers, classification, and caching:
//
//	lp := permissions.NewLayeredPolicy()
//	lp.LoadProjectPolicy("/path/to/project")
//	lp.LoadUserPolicy(configDir)
//
//	ctx := &permissions.EvalContext{
//	    ToolName:  "Bash",
//	    Resource:  "rm -rf /tmp/cache",
//	    WorkingDir: "/home/user/project",
//	}
//
//	action, rule, err := lp.Evaluate(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	switch action {
//	case permissions.Allow:
//	    execute(ctx.ToolName, ctx.Resource)
//	case permissions.Deny:
//	    log.Printf("denied by rule %s", rule.ID)
//	case permissions.Ask:
//	    if confirm(ctx) {
//	        execute(ctx.ToolName, ctx.Resource)
//	    }
//	}
//
// # Thread Safety
//
// All exported types that mutate state are safe for concurrent use.
// The cache and audit log use sync.RWMutex for reader/writer separation.
package permissions
