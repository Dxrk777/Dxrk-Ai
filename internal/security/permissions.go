// SPDX-License-Identifier: MIT
package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ---- Permission Rule Sources (5-layer hierarchy) ----

// SettingSource identifies where a permission rule originates.
type SettingSource int

const (
	SourceUser    SettingSource = iota // ~/.claude/settings.json
	SourceProject                      // .claude/settings.json (committed)
	SourceLocal                        // .claude/settings.local.json (gitignored)
	SourceFlag                         // --allowedTools / --disallowedTools
	SourcePolicy                       // Enterprise policy (highest priority)
)

func (s SettingSource) String() string {
	switch s {
	case SourceUser:
		return "user"
	case SourceProject:
		return strconst.StrProject
	case SourceLocal:
		return strconst.StrLocal
	case SourceFlag:
		return "flag"
	case SourcePolicy:
		return "policy"
	default:
		return strconst.StrUnknown
	}
}

// Priority returns the evaluation priority (higher = evaluated first).
func (s SettingSource) Priority() int {
	switch s {
	case SourcePolicy:
		return 50
	case SourceFlag:
		return 40
	case SourceLocal:
		return 30
	case SourceProject:
		return 20
	case SourceUser:
		return 10
	default:
		return 0
	}
}

// ---- Permission Rules ----

// PermissionBehavior determines the outcome when a rule matches.
type PermissionBehavior int

const (
	Allow PermissionBehavior = iota
	Deny
)

func (b PermissionBehavior) String() string {
	if b == Allow {
		return "allow"
	}
	return "deny"
}

// PermissionRule defines a single allow/deny rule for tool execution.
type PermissionRule struct {
	Tool     string             `json:"tool"`              // "Bash", "Read", "Write", etc.
	Prefix   string             `json:"prefix,omitempty"`  // "git commit" — exact prefix match
	Pattern  string             `json:"pattern,omitempty"` // "npm test *" — glob pattern
	Behavior PermissionBehavior `json:"behavior"`
	Source   SettingSource      `json:"source"`
}

// ToolPermissionRulesConfig is the JSON schema for settings files.
type ToolPermissionRulesConfig struct {
	Permissions []PermissionRuleJSON `json:"permissions"`
}

// PermissionRuleJSON is the JSON representation of a permission rule.
type PermissionRuleJSON struct {
	Tool     string `json:"tool"`
	Prefix   string `json:"prefix,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Behavior string `json:"behavior"` // "allow" or "deny"
}

// ---- Permission Pipeline ----

// PermissionResult is the outcome of checking a tool permission.
type PermissionResult int

const (
	PermAllowed PermissionResult = iota
	PermDenied
	PermNeedsPrompt
)

func (p PermissionResult) String() string {
	switch p {
	case PermAllowed:
		return "allowed"
	case PermDenied:
		return "denied"
	case PermNeedsPrompt:
		return "needs_prompt"
	default:
		return strconst.StrUnknown
	}
}

// PermissionContext holds the state for permission checking.
type PermissionContext struct {
	mu         sync.RWMutex
	rules      []PermissionRule
	denials    map[string]int // tool name → denial count
	maxDenials int
}

// NewPermissionContext creates a new permission context.
func NewPermissionContext() *PermissionContext {
	return &PermissionContext{
		denials:    make(map[string]int),
		maxDenials: 5, // after 5 denials, auto-deny for session
	}
}

// LoadRulesFromFile loads permission rules from a JSON settings file.
func (pc *PermissionContext) LoadRulesFromFile(path string, source SettingSource) error {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read permissions file %q: %w", path, err)
	}

	var config ToolPermissionRulesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse permissions file %q: %w", path, err)
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	for _, r := range config.Permissions {
		behavior := Allow
		if r.Behavior == "deny" {
			behavior = Deny
		}
		pc.rules = append(pc.rules, PermissionRule{
			Tool:     r.Tool,
			Prefix:   r.Prefix,
			Pattern:  r.Pattern,
			Behavior: behavior,
			Source:   source,
		})
	}
	return nil
}

// LoadAllSources loads rules from all 5 standard locations.
func (pc *PermissionContext) LoadAllSources(homeDir, projectDir string) error {
	sources := []struct {
		path   string
		source SettingSource
	}{
		{filepath.Join(homeDir, ".claude", "settings.json"), SourceUser},
		{filepath.Join(projectDir, ".claude", "settings.json"), SourceProject},
		{filepath.Join(projectDir, ".claude", "settings.local.json"), SourceLocal},
	}

	for _, s := range sources {
		if err := pc.LoadRulesFromFile(s.path, s.source); err != nil {
			return err
		}
	}
	return nil
}

// AddFlagRules adds rules from CLI flags.
func (pc *PermissionContext) AddFlagRules(tools []string, behavior PermissionBehavior) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for _, tool := range tools {
		pc.rules = append(pc.rules, PermissionRule{
			Tool:     tool,
			Behavior: behavior,
			Source:   SourceFlag,
		})
	}
}

// AddPolicyRules adds rules from enterprise policy.
func (pc *PermissionContext) AddPolicyRules(rules []PermissionRule) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for _, r := range rules {
		r.Source = SourcePolicy
		pc.rules = append(pc.rules, r)
	}
}

// Check evaluates whether a tool+command is allowed, denied, or needs prompting.
// Priority: policy > flag > local > project > user. Deny wins over Allow at same priority.
func (pc *PermissionContext) Check(toolName, command string) (PermissionResult, string) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Check auto-deny threshold
	if pc.denials[toolName] >= pc.maxDenials {
		return PermDenied, fmt.Sprintf("auto-denied after %d denials", pc.maxDenials)
	}

	// Sort rules by priority (highest first)
	sorted := make([]PermissionRule, len(pc.rules))
	copy(sorted, pc.rules)
	// Stable sort by priority (descending)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Source.Priority() > sorted[j-1].Source.Priority(); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	// Evaluate rules: first matching rule at highest priority wins
	highestPriority := -1
	var bestResult PermissionResult
	var bestReason string

	for _, rule := range sorted {
		if !matchesRule(rule, toolName, command) {
			continue
		}
		priority := rule.Source.Priority()
		if priority > highestPriority {
			highestPriority = priority
			if rule.Behavior == Deny {
				bestResult = PermDenied
				bestReason = fmt.Sprintf("denied by %s rule", rule.Source)
			} else {
				bestResult = PermAllowed
				bestReason = fmt.Sprintf("allowed by %s rule", rule.Source)
			}
		}
	}

	if highestPriority >= 0 {
		return bestResult, bestReason
	}

	// No rule matched — needs prompting
	return PermNeedsPrompt, "no matching rule"
}

// RecordDenial increments the denial count for a tool.
func (pc *PermissionContext) RecordDenial(toolName string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.denials[toolName]++
}

// ResetDenials clears all denial counts.
func (pc *PermissionContext) ResetDenials() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.denials = make(map[string]int)
}

// Rules returns a copy of all loaded rules.
func (pc *PermissionContext) Rules() []PermissionRule {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	out := make([]PermissionRule, len(pc.rules))
	copy(out, pc.rules)
	return out
}

// ---- Rule Matching ----

func matchesRule(rule PermissionRule, toolName, command string) bool {
	if !strings.EqualFold(rule.Tool, toolName) {
		return false
	}

	// If no prefix/pattern, matches any invocation of the tool
	if rule.Prefix == "" && rule.Pattern == "" {
		return true
	}

	cmd := strings.TrimSpace(command)

	// Exact prefix match (word-aligned)
	if rule.Prefix != "" {
		if strings.HasPrefix(cmd, rule.Prefix) {
			// Word boundary check
			rest := cmd[len(rule.Prefix):]
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' {
				return true
			}
		}
	}

	// Glob pattern match
	if rule.Pattern != "" {
		if matchGlob(rule.Pattern, cmd) {
			return true
		}
	}

	return false
}

// matchGlob performs simple glob matching (* and ?).
func matchGlob(pattern, s string) bool {
	// Convert glob to regex-like matching
	pi, si := 0, 0
	starPi, starSi := -1, -1

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			pi++
			si++
			continue
		}
		if pi < len(pattern) && pattern[pi] == '*' {
			starPi = pi
			starSi = si
			pi++
			continue
		}
		if starPi >= 0 {
			pi = starPi + 1
			starSi++
			si = starSi
			continue
		}
		return false
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// ---- Standard Tool Permissions ----

// SafeTools lists tools that are always allowed without prompting.
var SafeTools = map[string]bool{
	"Read":                true,
	"Glob":                true,
	"Grep":                true,
	"LS":                  true,
	strconst.StrListfiles: true,
	strconst.StrTodoread:  true,
}

// AlwaysAskTools lists tools that always require user approval.
var AlwaysAskTools = map[string]bool{
	"Bash":              true,
	strconst.StrExecute: true,
}

// ReadOnlyTools lists tools that are read-only.
var ReadOnlyTools = map[string]bool{
	"Read":                true,
	"Glob":                true,
	"Grep":                true,
	"LS":                  true,
	strconst.StrListfiles: true,
	strconst.StrTodoread:  true,
	strconst.StrWebfetch:  true,
	strconst.StrWebsearch: true,
}

// ClassifyTool determines if a tool is safe, needs asking, or is denied.
func ClassifyTool(toolName string) PermissionResult {
	if SafeTools[toolName] {
		return PermAllowed
	}
	if AlwaysAskTools[toolName] {
		return PermNeedsPrompt
	}
	// Unknown tools need prompting
	return PermNeedsPrompt
}

// DetectUnreachableRules finds rules shadowed by higher-priority rules.
func DetectUnreachableRules(rules []PermissionRule) []string {
	var unreachable []string

	// Group by tool+prefix/pattern
	type key struct {
		tool, match string
	}
	groups := map[key][]PermissionRule{}
	for _, r := range rules {
		match := r.Prefix
		if match == "" {
			match = r.Pattern
		}
		k := key{r.Tool, match}
		groups[k] = append(groups[k], r)
	}

	for k, group := range groups {
		if len(group) <= 1 {
			continue
		}
		// Find highest priority
		highest := group[0]
		for _, r := range group[1:] {
			if r.Source.Priority() > highest.Source.Priority() {
				highest = r
			}
		}
		// All others are unreachable
		for _, r := range group {
			if r != highest {
				unreachable = append(unreachable, fmt.Sprintf(
					"rule %s/%s (%s) shadowed by %s/%s (%s)",
					r.Tool, k.match, r.Behavior,
					highest.Tool, k.match, highest.Behavior,
				))
			}
		}
	}

	return unreachable
}
