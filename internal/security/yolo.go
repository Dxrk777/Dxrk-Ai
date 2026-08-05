// SPDX-License-Identifier: MIT
package security

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ---- Auto-Mode Tool Classification ----

// RiskLevel classifies the risk of executing a tool with given input.
type RiskLevel int

const (
	RiskNone RiskLevel = iota
	RiskLow
	RiskMedium
	RiskHigh
	RiskCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskNone:
		return "none"
	case RiskLow:
		return "low"
	case RiskMedium:
		return strconst.StrMedium2
	case RiskHigh:
		return "high"
	case RiskCritical:
		return strconst.StrCritical
	default:
		return strconst.StrUnknown
	}
}

// ClassificationDecision is the outcome of auto-mode classification.
type ClassificationDecision struct {
	Action   string    `json:"action"` // "allow", "ask", "deny"
	Risk     RiskLevel `json:"risk"`
	Reason   string    `json:"reason"`
	ToolName string    `json:"tool_name"`
	Command  string    `json:"command,omitempty"`
}

// ---- Tool Risk Profiles ----

// SafeForAutoMode tools that can run without user confirmation in auto mode.
var SafeForAutoMode = map[string]bool{
	"Read":                true,
	"Glob":                true,
	"Grep":                true,
	"LS":                  true,
	strconst.StrListfiles: true,
	strconst.StrTodoread:  true,
	strconst.StrWebsearch: true,
	strconst.StrWebfetch:  true,
}

// NeedsConfirmation tools that always need user confirmation.
var NeedsConfirmation = map[string]bool{
	"Bash":              true,
	strconst.StrExecute: true,
	strconst.StrWrite:   true,
	"Edit":              true,
	"MultiEdit":         true,
}

// ReadTools tools that are read-only.
var ReadTools = map[string]bool{
	"Read":                true,
	"Glob":                true,
	"Grep":                true,
	"LS":                  true,
	strconst.StrListfiles: true,
	strconst.StrTodoread:  true,
	strconst.StrWebsearch: true,
	strconst.StrWebfetch:  true,
}

// ClassifyForAutoMode determines the action for a tool in auto mode.
func ClassifyForAutoMode(toolName, command string) ClassificationDecision {
	// Always ask for dangerous tools
	if NeedsConfirmation[toolName] {
		return ClassificationDecision{
			Action:   "ask",
			Risk:     RiskHigh,
			Reason:   fmt.Sprintf("tool %q requires confirmation", toolName),
			ToolName: toolName,
			Command:  command,
		}
	}

	// Safe for auto mode
	if SafeForAutoMode[toolName] {
		return ClassificationDecision{
			Action:   "allow",
			Risk:     RiskNone,
			Reason:   fmt.Sprintf("tool %q is safe for auto mode", toolName),
			ToolName: toolName,
			Command:  command,
		}
	}

	// Unknown tool — ask
	return ClassificationDecision{
		Action:   "ask",
		Risk:     RiskMedium,
		Reason:   fmt.Sprintf("unknown tool %q — requires confirmation", toolName),
		ToolName: toolName,
		Command:  command,
	}
}

// ---- Bash Command Risk Assessment ----

// AssessBashRisk evaluates the risk level of a bash command.
func AssessBashRisk(command string) RiskLevel {
	if command == "" {
		return RiskNone
	}

	result := ParseForSecurity(command)

	if !result.IsSafe {
		return RiskHigh
	}

	name := ExtractCommandName(command)
	if name == "" {
		return RiskMedium
	}

	// Check for read-only commands
	if IsReadOnlyCommand(command) {
		return RiskLow
	}

	// Check for file modification commands
	modifyCmds := map[string]bool{
		"rm": true, "mv": true, "cp": true, "chmod": true, "chown": true,
		"mkdir": true, "rmdir": true, "touch": true, "ln": true,
		"dd": true, "mkfs": true,
	}
	if modifyCmds[name] {
		return RiskMedium
	}

	// Check for network commands
	networkCmds := map[string]bool{
		"curl": true, "wget": true, "nc": true, "ncat": true, "socat": true,
		"ssh": true, "scp": true, "rsync": true, "telnet": true,
	}
	if networkCmds[name] {
		return RiskMedium
	}

	// Check for system commands
	systemCmds := map[string]bool{
		"kill": true, "pkill": true, "killall": true, "systemctl": true,
		"service": true, "mount": true, "umount": true, "fdisk": true,
		"crontab": true, "at": true,
	}
	if systemCmds[name] {
		return RiskHigh
	}

	return RiskLow
}

// ---- Dangerous Command Patterns ----

// DangerousPattern describes a known dangerous command pattern.
type DangerousPattern struct {
	Pattern   string
	Reason    string
	Risk      RiskLevel
	ToolNames []string // which tools this applies to
}

// KnownDangerousPatterns lists patterns that should always be flagged.
var KnownDangerousPatterns = []DangerousPattern{
	{
		Pattern:   "rm -rf /",
		Reason:    "recursive deletion of root filesystem",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "rm -rf ~",
		Reason:    "recursive deletion of home directory",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "mkfs",
		Reason:    "filesystem formatting",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   ":(){ :|:& };:",
		Reason:    "fork bomb",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "dd if=/dev/zero",
		Reason:    "disk zeroing",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "chmod -R 777",
		Reason:    "world-writable recursive permissions",
		Risk:      RiskHigh,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "curl | sh",
		Reason:    "pipe remote code to shell",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "wget | sh",
		Reason:    "pipe remote code to shell",
		Risk:      RiskCritical,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "eval $",
		Reason:    "eval of variable expansion",
		Risk:      RiskHigh,
		ToolNames: []string{"Bash"},
	},
	{
		Pattern:   "exec ",
		Reason:    "process replacement",
		Risk:      RiskHigh,
		ToolNames: []string{"Bash"},
	},
}

// CheckDangerousPatterns scans a command against known dangerous patterns.
func CheckDangerousPatterns(command string, toolName string) []DangerousPattern {
	var matches []DangerousPattern
	cmdLower := strings.ToLower(command)

	for _, dp := range KnownDangerousPatterns {
		if len(dp.ToolNames) > 0 {
			found := false
			for _, t := range dp.ToolNames {
				if t == toolName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if strings.Contains(cmdLower, dp.Pattern) {
			matches = append(matches, dp)
		}
	}

	return matches
}

// ---- Circuit Breaker ----

// CircuitBreakerState tracks auto-mode circuit breaker status.
type CircuitBreakerState struct {
	Tripped       bool
	TripCount     int
	TripThreshold int
	LastTrip      int64 // unix timestamp
	ResetAfter    int64 // seconds
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, resetAfterSec int64) *CircuitBreakerState {
	return &CircuitBreakerState{
		TripThreshold: threshold,
		ResetAfter:    resetAfterSec,
	}
}

// RecordFailure records a failure and trips the breaker if threshold reached.
func (cb *CircuitBreakerState) RecordFailure() bool {
	cb.TripCount++
	if cb.TripCount >= cb.TripThreshold {
		cb.Tripped = true
		return true // breaker tripped
	}
	return false
}

// Reset clears the circuit breaker state.
func (cb *CircuitBreakerState) Reset() {
	cb.Tripped = false
	cb.TripCount = 0
	cb.LastTrip = 0
}

// ShouldBlock returns true if the circuit breaker is tripped.
func (cb *CircuitBreakerState) ShouldBlock() bool {
	return cb.Tripped
}
