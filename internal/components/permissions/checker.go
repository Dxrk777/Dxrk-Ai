// SPDX-License-Identifier: MIT
// Package permissions provides runtime permission checking with audit logging.
package permissions

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Action represents the type of operation being checked.
type Action string

const (
	ActionRead  Action = "read"
	ActionWrite Action = "write"
	ActionExec  Action = "exec"
	ActionEdit  Action = "edit"
)

// Result is the outcome of a permission check.
type Result struct {
	Allowed bool
	Reason  string
}

// Rule defines a single permission rule.
type Rule struct {
	Action Action
	Target string // Glob pattern (e.g. "*.env", "/tmp/*")
	Allow  bool
}

// Checker evaluates permission requests against a set of rules.
type Checker struct {
	rules         []Rule
	denyByDefault bool
}

// CheckerOption configures a Checker.
type CheckerOption func(*Checker)

// WithDenyByDefault sets the default behavior. Default is true (fail-closed).
func WithDenyByDefault(deny bool) CheckerOption {
	return func(c *Checker) {
		c.denyByDefault = deny
	}
}

// WithRules appends rules to the checker.
func WithRules(rules ...Rule) CheckerOption {
	return func(c *Checker) {
		c.rules = append(c.rules, rules...)
	}
}

// NewChecker creates a Checker with the given options.
func NewChecker(opts ...CheckerOption) *Checker {
	c := &Checker{denyByDefault: true}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Check evaluates whether a given action on a target is allowed.
// Rules are evaluated in order; the first matching rule wins.
// If no rule matches, the denyByDefault policy applies.
func (c *Checker) Check(action Action, target string) Result {
	for _, rule := range c.rules {
		if rule.Action != action {
			continue
		}
		if !matchTarget(rule.Target, target) {
			continue
		}
		return Result{
			Allowed: rule.Allow,
			Reason:  fmt.Sprintf("rule %s %s %s", rule.Action, ifAllow(rule.Allow), rule.Target),
		}
	}

	if c.denyByDefault {
		return Result{
			Allowed: false,
			Reason:  "denied by default (no matching rule)",
		}
	}
	return Result{
		Allowed: true,
		Reason:  "allowed by default (no matching rule)",
	}
}

func ifAllow(allow bool) string {
	if allow {
		return "allow"
	}
	return "deny"
}

// matchTarget checks if a target matches a glob pattern.
// Supports glob patterns via filepath.Match plus ** for recursive matching.
func matchTarget(pattern, target string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "**") {
		return matchRecursive(pattern, target)
	}
	matched, err := filepath.Match(pattern, target)
	if err != nil {
		return false
	}
	return matched
}

// matchRecursive handles ** glob patterns (zero or more directory levels).
// Converts the glob to a regex:
//   - ** matches any path segments
//   - **/ before a segment is optional when ** matches zero levels
func matchRecursive(pattern, target string) bool {
	var buf strings.Builder
	buf.WriteByte('^')

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch {
		case ch == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			if i+2 < len(pattern) && pattern[i+2] == '/' {
				buf.WriteString("(.*/)?")
				i += 2
			} else {
				buf.WriteString(".*")
				i++
			}
		case ch == '*':
			buf.WriteString("[^/]*")
		default:
			switch ch {
			case '.', '?', '+', '^', '$', '{', '}', '|', '(', ')', '[', ']', '\\':
				buf.WriteByte('\\')
			}
			buf.WriteByte(ch)
		}
	}
	buf.WriteByte('$')

	matched, err := regexp.MatchString(buf.String(), target)
	return err == nil && matched
}

// MakeRule is a convenience constructor for Rule.
func MakeRule(action Action, target string, allow bool) Rule {
	return Rule{Action: action, Target: target, Allow: allow}
}

// AllowRule creates an allow rule.
func AllowRule(action Action, target string) Rule {
	return MakeRule(action, target, true)
}

// DenyRule creates a deny rule.
func DenyRule(action Action, target string) Rule {
	return MakeRule(action, target, false)
}

// Common permission presets
var (
	// DenyEnvFiles denies read access to .env files.
	DenyEnvFiles = DenyRule(ActionRead, "*.env")
	// DenySecretsDir denies read access to secrets directories.
	DenySecretsDir = DenyRule(ActionRead, "**/secrets/**")
	// DenyGitForcePush denies destructive git push operations.
	DenyGitForcePush = DenyRule(ActionExec, "git push --force *")
)
