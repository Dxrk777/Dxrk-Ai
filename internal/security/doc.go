// SPDX-License-Identifier: MIT
// Package security provides defense-in-depth security for Dxrk-Ai tool execution.
//
// Architecture ported from Claude Code CLI's security subsystem:
//   - Fail-closed AST parsing for shell commands
//   - 5-layer permission rule hierarchy (user/project/local/flag/policy)
//   - JWT refresh scheduler with automatic renewal
//   - Dangerous pattern detection (command injection, metacharacters)
//   - Tool classification for auto-mode risk assessment
//
// Defense layers (ordered by evaluation):
//  1. Input validation (max length, character filtering)
//  2. AST parsing (fail-closed: any parse error = deny)
//  3. Pattern matching (regex for dangerous patterns)
//  4. Permission rules (5-source hierarchy with priority)
//  5. User prompting (interactive approval for unknown commands)
package security
