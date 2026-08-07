// Package hooks provides a flexible, extensible hook system for Dxrk.
//
// It implements the Claude Code hooks specification with support for:
//   - Pre/Post tool use hooks
//   - User prompt submission hooks
//   - Notification and stop hooks
//   - Subagent lifecycle hooks
//
// # Features
//
//   - Pattern-based hook matching (glob/regex) against tool names and paths
//   - Async execution via worker pool with configurable concurrency
//   - Per-hook timeout, retry, and circuit breaker policies
//   - Structured logging and metrics collection
//   - Thread-safe registry with hot-reload support
//
// # Hook Types
//
//   - PreToolUse: Runs before a tool executes, can modify input or abort
//   - PostToolUse: Runs after a tool completes, receives output
//   - UserPromptSubmit: Runs when user submits a prompt
//   - Notification: Runs on system notifications
//   - Stop: Runs when agent stops
//   - SubagentStop: Runs when a subagent stops
//
// # Concurrency
//
// The [HookRegistry] is safe for concurrent use. The [HookExecutor] and
// [HookQueue] are designed for concurrent execution from multiple goroutines.
//
// # Error Handling
//
// All exported functions return errors when operations cannot complete.
// Hook execution errors are captured in [HookResult] and do not propagate
// unless configured to do so.
package hooks
