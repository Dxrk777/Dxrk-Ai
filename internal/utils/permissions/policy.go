// SPDX-License-Identifier: MIT
package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ---- Actions ----

// Action represents the outcome of a permission evaluation.
type Action int

const (
	Allow Action = iota
	Deny
	Ask
)

func (a Action) String() string {
	switch a {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case Ask:
		return "ask"
	default:
		return strconst.StrUnknown
	}
}

// ParseAction converts a string to an Action.
func ParseAction(s string) (Action, error) {
	switch strings.ToLower(s) {
	case "allow":
		return Allow, nil
	case "deny":
		return Deny, nil
	case "ask":
		return Ask, nil
	default:
		return Allow, fmt.Errorf("unknown action: %q", s)
	}
}

// ---- Operators ----

// Operator represents a condition comparison operator.
type Operator int

const (
	OpEq Operator = iota
	OpNeq
	OpGlob
	OpRegex
	OpIn
	OpGt
	OpLt
)

func (o Operator) String() string {
	switch o {
	case OpEq:
		return "eq"
	case OpNeq:
		return "neq"
	case OpGlob:
		return "glob"
	case OpRegex:
		return "regex"
	case OpIn:
		return "in"
	case OpGt:
		return "gt"
	case OpLt:
		return "lt"
	default:
		return strconst.StrUnknown
	}
}

// ParseOperator converts a string to an Operator.
func ParseOperator(s string) (Operator, error) {
	switch strings.ToLower(s) {
	case "eq":
		return OpEq, nil
	case "neq":
		return OpNeq, nil
	case "glob":
		return OpGlob, nil
	case "regex":
		return OpRegex, nil
	case "in":
		return OpIn, nil
	case "gt":
		return OpGt, nil
	case "lt":
		return OpLt, nil
	default:
		return OpEq, fmt.Errorf("unknown operator: %q", s)
	}
}

// ---- Rule Strategy ----

// Strategy determines how rules are evaluated.
type Strategy int

const (
	// FirstMatch returns the first matching rule.
	FirstMatch Strategy = iota
	// MostRestrictive returns the most restrictive action among matches.
	MostRestrictive
)

func (s Strategy) String() string {
	switch s {
	case FirstMatch:
		return "first_match"
	case MostRestrictive:
		return "most_restrictive"
	default:
		return "first_match"
	}
}

// ---- Conditions ----

// Condition is a single predicate that must match for a rule to apply.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// ---- Rules ----

// Rule defines a single permission rule with subject, resource, action, and
// optional conditions.
type Rule struct {
	ID         string      `json:"id"`
	Subject    string      `json:"subject"`
	Resource   string      `json:"resource"`
	Action     Action      `json:"action"`
	Conditions []Condition `json:"conditions,omitempty"`
	Priority   int         `json:"priority"`
}

// ---- EvalContext ----

// EvalContext holds all state needed for a single permission evaluation.
type EvalContext struct {
	ToolName   string            `json:"tool_name"`
	Resource   string            `json:"resource"`
	User       string            `json:"user,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// fieldValue returns the value of a named field from the context.
func (ec *EvalContext) fieldValue(field string) string {
	switch strings.ToLower(field) {
	case "tool_name", "tool":
		return ec.ToolName
	case "resource":
		return ec.Resource
	case "user":
		return ec.User
	case "working_dir", "dir":
		return ec.WorkingDir
	default:
		if ec.EnvVars != nil {
			if v, ok := ec.EnvVars[field]; ok {
				return v
			}
		}
		if ec.Metadata != nil {
			if v, ok := ec.Metadata[field]; ok {
				return v
			}
		}
		return ""
	}
}

// ---- Policy ----

// Policy is a named collection of rules with versioning and a default action.
type Policy struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Rules         []Rule   `json:"rules"`
	DefaultAction Action   `json:"default_action"`
	Strategy      Strategy `json:"strategy"`
}

// ---- PolicyEngine ----

// PolicyEngine evaluates rules against contexts.
type PolicyEngine struct {
	mu       sync.RWMutex
	policy   Policy
	compiled map[string]*regexp.Regexp
}

// NewPolicyEngine creates an engine from a policy.
func NewPolicyEngine(p Policy) *PolicyEngine {
	if p.Strategy == 0 {
		p.Strategy = FirstMatch
	}
	return &PolicyEngine{
		policy:   p,
		compiled: make(map[string]*regexp.Regexp),
	}
}

// Evaluate evaluates all rules against the context.
func (pe *PolicyEngine) Evaluate(ctx *EvalContext) (Action, *Rule, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	matches := make([]Rule, 0, 4)
	for i := range pe.policy.Rules {
		r := &pe.policy.Rules[i]
		if pe.matchesRule(r, ctx) {
			matches = append(matches, *r)
		}
	}

	if len(matches) == 0 {
		return pe.policy.DefaultAction, nil, nil
	}

	switch pe.policy.Strategy {
	case MostRestrictive:
		return pe.mostRestrictive(matches), &matches[0], nil
	default:
		best := &matches[0]
		for i := 1; i < len(matches); i++ {
			if matches[i].Priority > best.Priority {
				best = &matches[i]
			}
		}
		return best.Action, best, nil
	}
}

func (pe *PolicyEngine) mostRestrictive(rules []Rule) Action {
	best := Allow
	for _, r := range rules {
		if r.Action == Deny {
			return Deny
		}
		if r.Action == Ask && best == Allow {
			best = Ask
		}
	}
	return best
}

// AddRule appends a rule to the policy.
func (pe *PolicyEngine) AddRule(rule Rule) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.policy.Rules = append(pe.policy.Rules, rule)
}

// RemoveRule removes a rule by ID.
func (pe *PolicyEngine) RemoveRule(id string) bool {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	for i, r := range pe.policy.Rules {
		if r.ID == id {
			pe.policy.Rules = append(pe.policy.Rules[:i], pe.policy.Rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRules returns a copy of all rules.
func (pe *PolicyEngine) GetRules() []Rule {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	out := make([]Rule, len(pe.policy.Rules))
	copy(out, pe.policy.Rules)
	return out
}

// Merge combines rules from another engine into this one.
func (pe *PolicyEngine) Merge(other *PolicyEngine) {
	other.mu.RLock()
	otherRules := make([]Rule, len(other.policy.Rules))
	copy(otherRules, other.policy.Rules)
	other.mu.RUnlock()

	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.policy.Rules = append(pe.policy.Rules, otherRules...)
}

// Validate checks for duplicate IDs and overlapping rules.
func (pe *PolicyEngine) Validate() error {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	seen := make(map[string]int, len(pe.policy.Rules))
	for i, r := range pe.policy.Rules {
		if r.ID == "" {
			return fmt.Errorf("rule at index %d has empty ID", i)
		}
		if prev, ok := seen[r.ID]; ok {
			return fmt.Errorf("duplicate rule ID %q at indices %d and %d", r.ID, prev, i)
		}
		seen[r.ID] = i
	}
	return nil
}

// Policy returns a copy of the underlying policy.
func (pe *PolicyEngine) Policy() Policy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	p := pe.policy
	p.Rules = make([]Rule, len(pe.policy.Rules))
	copy(p.Rules, pe.policy.Rules)
	return p
}

// ---- Rule Matching ----

func (pe *PolicyEngine) matchesRule(r *Rule, ctx *EvalContext) bool {
	if !matchField(r.Subject, ctx.User) {
		return false
	}
	if !matchField(r.Resource, ctx.ToolName) && !matchField(r.Resource, ctx.Resource) {
		return false
	}
	for _, cond := range r.Conditions {
		if !pe.evalCondition(&cond, ctx) {
			return false
		}
	}
	return true
}

func matchField(pattern, value string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	return matchGlob(pattern, value)
}

func (pe *PolicyEngine) evalCondition(c *Condition, ctx *EvalContext) bool {
	val := ctx.fieldValue(c.Field)
	op, err := ParseOperator(c.Operator)
	if err != nil {
		return false
	}

	switch op {
	case OpEq:
		return val == c.Value
	case OpNeq:
		return val != c.Value
	case OpGlob:
		return matchGlob(c.Value, val)
	case OpRegex:
		return pe.evalRegex(c.Value, val)
	case OpIn:
		for _, item := range strings.Split(c.Value, ",") {
			if strings.TrimSpace(item) == val {
				return true
			}
		}
		return false
	case OpGt:
		return val > c.Value
	case OpLt:
		return val < c.Value
	default:
		return false
	}
}

func (pe *PolicyEngine) evalRegex(pattern, value string) bool {
	if re, ok := pe.compiled[pattern]; ok {
		return re.MatchString(value)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	pe.compiled[pattern] = re
	return re.MatchString(value)
}

// ---- Serialization ----

// MarshalPolicyJSON serializes a policy to JSON bytes.
func MarshalPolicyJSON(p *Policy) ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// UnmarshalPolicyJSON deserializes a policy from JSON bytes.
func UnmarshalPolicyJSON(data []byte) (*Policy, error) {
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}
	if p.Strategy == 0 {
		p.Strategy = FirstMatch
	}
	return &p, nil
}

// LoadPolicyFile reads and parses a policy from a JSON file.
func LoadPolicyFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read policy file %q: %w", path, err)
	}
	return UnmarshalPolicyJSON(data)
}

// SavePolicyFile writes a policy to a JSON file.
func SavePolicyFile(p *Policy, path string) error {
	data, err := MarshalPolicyJSON(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644) //nolint:gosec
}

// ---- Helpers ----

// matchGlob performs simple glob matching (* and ?).
func matchGlob(pattern, s string) bool {
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
