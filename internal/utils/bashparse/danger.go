package bashparse

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// DangerLevel classifies the severity of a detected danger.
type DangerLevel int

const (
	Safe      DangerLevel = iota // No issues detected
	Warning                      // Potentially unsafe, review recommended
	Dangerous                    // Likely harmful, block or confirm
	Critical                     // Destructive, must block
)

// String returns the label for a DangerLevel.
func (l DangerLevel) String() string {
	switch l {
	case Safe:
		return "safe"
	case Warning:
		return "warning"
	case Dangerous:
		return "dangerous"
	case Critical:
		return strconst.StrCritical
	default:
		return strconst.StrUnknown
	}
}

// DangerCheck describes a single detected danger pattern.
type DangerCheck struct {
	Level      DangerLevel
	Reason     string
	Pattern    string // The matched pattern or regex
	Suggestion string // Recommended safer alternative
}

// AnalyzeDanger performs recursive danger analysis on an AST.
//
// It walks every node, inspecting command names, arguments, and structure
// for destructive or risky patterns. Each finding is returned as a
// DangerCheck with severity, reason, and suggestion.
func AnalyzeDanger(node *ASTNode) []DangerCheck {
	if node == nil {
		return nil
	}
	var checks []DangerCheck
	Walk(*node, func(n ASTNode) bool {
		switch v := n.(type) {
		case *CommandNode:
			checks = append(checks, analyzeCommand(v)...)
		case *PipeNode:
			checks = append(checks, analyzePipeline(v)...)
		case *SubshellNode:
			checks = append(checks, analyzeSubshell(v)...)
		}
		return true
	})
	return checks
}

func analyzeCommand(cmd *CommandNode) []DangerCheck {
	var checks []DangerCheck
	name := strings.ToLower(cmd.Name)
	args := joinArgs(cmd.Args)
	full := strings.ToLower(name + " " + args)

	// Recursive deletion
	if name == "rm" && hasAnyFlag(cmd.Args, "r", "R", "recursive") && hasAnyFlag(cmd.Args, "f", "force") {
		if hasRootTarget(cmd.Args) || hasGlobTarget(cmd.Args) {
			checks = append(checks, DangerCheck{
				Level:      Critical,
				Reason:     "recursive force deletion of root or wildcard",
				Pattern:    "rm -rf",
				Suggestion: "Use rm with specific paths and without -f unless absolutely necessary",
			})
		}
	}

	// Find with -delete
	if name == "find" && strings.Contains(args, "-delete") {
		checks = append(checks, DangerCheck{
			Level:      Dangerous,
			Reason:     "find with -delete removes files recursively",
			Pattern:    "find ... -delete",
			Suggestion: "Use find with -exec rm {} \\; and verify paths first",
		})
	}

	// Fork bomb
	if strings.Contains(full, ":(){ :|:& };:") || strings.Contains(full, "():{") {
		checks = append(checks, DangerCheck{
			Level:      Critical,
			Reason:     "fork bomb detected",
			Pattern:    ":(){ :|:& };:",
			Suggestion: "Never execute fork bombs; this will crash the system",
		})
	}

	// Disk wiping
	if name == "dd" && (strings.Contains(args, "if=/dev/zero") || strings.Contains(args, "if=/dev/urandom") || strings.Contains(args, "of=/dev/")) {
		checks = append(checks, DangerCheck{
			Level:      Critical,
			Reason:     "raw disk write via dd",
			Pattern:    "dd if=/dev/zero",
			Suggestion: "Disk wipes are irreversible; use specific partition targets",
		})
	}
	if name == "mkfs" || strings.HasPrefix(name, "mkfs.") {
		checks = append(checks, DangerCheck{
			Level:      Critical,
			Reason:     "filesystem format detected",
			Pattern:    name,
			Suggestion: "Formatting destroys all data on the target device",
		})
	}
	if name == "fdisk" || name == "parted" {
		if strings.Contains(args, "--delete") || strings.Contains(args, "rm") {
			checks = append(checks, DangerCheck{
				Level:      Dangerous,
				Reason:     "partition deletion",
				Pattern:    name,
				Suggestion: "Partition changes are risky; verify device paths carefully",
			})
		}
	}

	// Network exfiltration
	if (name == "curl" || name == "wget") && pipeToShell(args) {
		checks = append(checks, DangerCheck{
			Level:      Critical,
			Reason:     "remote code execution via pipe to shell",
			Pattern:    name + " ... | sh",
			Suggestion: "Download scripts first, inspect them, then execute",
		})
	}
	if name == "nc" || name == "ncat" || name == "socat" {
		if strings.Contains(args, "-e") || strings.Contains(args, "exec") {
			checks = append(checks, DangerCheck{
				Level:      Dangerous,
				Reason:     "reverse shell or remote execution via netcat",
				Pattern:    "nc -e",
				Suggestion: "Avoid reverse shells; use SSH for remote access",
			})
		}
	}

	// Permission escalation
	if name == "chmod" {
		if strings.Contains(args, "777") || strings.Contains(args, "a+rwx") {
			target := extractChmodTarget(cmd.Args)
			if target == "/" || target == "/*" || target == "." {
				checks = append(checks, DangerCheck{
					Level:      Dangerous,
					Reason:     "world-writable permissions on sensitive path",
					Pattern:    "chmod 777",
					Suggestion: "Grant minimal required permissions instead of 777",
				})
			}
		}
	}
	if name == "chown" && strings.Contains(args, "root") {
		checks = append(checks, DangerCheck{
			Level:      Warning,
			Reason:     "changing ownership to root",
			Pattern:    "chown root",
			Suggestion: "Only change ownership when necessary; document the change",
		})
	}
	if name == "sudo" && strings.Contains(args, "su") {
		checks = append(checks, DangerCheck{
			Level:      Warning,
			Reason:     "escalating to root shell",
			Pattern:    "sudo su",
			Suggestion: "Use sudo with a specific command instead of spawning a root shell",
		})
	}

	// Data destruction
	if name == "shred" {
		checks = append(checks, DangerCheck{
			Level:      Dangerous,
			Reason:     "secure file deletion (data destruction)",
			Pattern:    "shred",
			Suggestion: "Shred is irreversible; ensure the target is correct",
		})
	}
	if name == "wipe" {
		checks = append(checks, DangerCheck{
			Level:      Dangerous,
			Reason:     "data wipe detected",
			Pattern:    "wipe",
			Suggestion: "Wiping is irreversible; confirm the target path",
		})
	}

	// eval
	if name == "eval" {
		checks = append(checks, DangerCheck{
			Level:      Warning,
			Reason:     "dynamic code evaluation via eval",
			Pattern:    "eval",
			Suggestion: "Avoid eval; use direct command execution when possible",
		})
	}

	// Unsafe variable expansion
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "$(") || strings.Contains(arg, "`") {
			if name == "eval" || name == "sh" || name == "bash" || name == "exec" {
				checks = append(checks, DangerCheck{
					Level:      Warning,
					Reason:     "command substitution in execution context",
					Pattern:    arg,
					Suggestion: "Validate and sanitize input before command substitution",
				})
			}
		}
	}

	// Symlink overwrites
	if name == "ln" && strings.Contains(args, "-sf") {
		checks = append(checks, DangerCheck{
			Level:      Warning,
			Reason:     "forced symlink overwrite",
			Pattern:    "ln -sf",
			Suggestion: "Verify target before overwriting symlinks",
		})
	}

	return checks
}

func analyzePipeline(pipe *PipeNode) []DangerCheck {
	var checks []DangerCheck
	// Check for curl/wget piped to shell
	for _, cmd := range pipe.Commands {
		if c, ok := cmd.(*CommandNode); ok {
			name := strings.ToLower(c.Name)
			if name == "curl" || name == "wget" {
				// Check if last command in pipe is sh/bash
				last := pipe.Commands[len(pipe.Commands)-1]
				if lc, ok := last.(*CommandNode); ok {
					lname := strings.ToLower(lc.Name)
					if lname == "sh" || lname == "bash" || lname == "zsh" {
						checks = append(checks, DangerCheck{
							Level:      Critical,
							Reason:     "remote content piped directly to shell",
							Pattern:    name + " ... | " + lname,
							Suggestion: "Download, inspect, then execute; never pipe untrusted content to shell",
						})
					}
				}
			}
		}
	}
	return checks
}

func analyzeSubshell(sub *SubshellNode) []DangerCheck {
	var checks []DangerCheck
	// Subshells themselves are not dangerous, but check content
	return checks
}

// QuickDangerCheck parses a command string and analyzes it in one call.
func QuickDangerCheck(cmd string) (DangerLevel, []DangerCheck) {
	node, err := Parse(cmd)
	if err != nil {
		return Warning, []DangerCheck{{
			Level:   Warning,
			Reason:  "failed to parse command: " + err.Error(),
			Pattern: cmd,
		}}
	}

	checks := AnalyzeDanger(&node)
	level := Safe
	for _, c := range checks {
		if c.Level > level {
			level = c.Level
		}
	}
	return level, checks
}

// IsCommandSafe returns true if the command has no Dangerous or Critical findings.
func IsCommandSafe(cmd string) bool {
	level, _ := QuickDangerCheck(cmd)
	return level < Dangerous
}

// SuggestSafer returns a safer alternative for a dangerous command.
func SuggestSafer(cmd string) string {
	level, checks := QuickDangerCheck(cmd)
	if level < Warning {
		return cmd
	}
	if len(checks) > 0 && checks[0].Suggestion != "" {
		return fmt.Sprintf("%s\nSuggestion: %s", cmd, checks[0].Suggestion)
	}
	return cmd
}

// --- helpers ---

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func hasAnyFlag(args []string, flags ...string) bool {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		for _, f := range flags {
			if strings.Contains(a, f) {
				return true
			}
		}
	}
	return false
}

func hasRootTarget(args []string) bool {
	for _, a := range args {
		if a == "/" || a == "/*" || a == "/root" || a == "/home" {
			return true
		}
	}
	return false
}

func hasGlobTarget(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "*") && !strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}

func pipeToShell(args string) bool {
	return strings.Contains(args, "| sh") || strings.Contains(args, "| bash") || strings.Contains(args, "| zsh")
}

func extractChmodTarget(args []string) string {
	// Skip flags, return last non-flag arg
	var target string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			target = a
		}
	}
	return target
}
