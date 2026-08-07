package bashparse

import (
	"fmt"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// builtins is the set of bash built-in commands.
var builtins = map[string]bool{
	"alias": true, "bg": true, "break": true, "builtin": true,
	"cd": true, "command": true, "compgen": true, "complete": true,
	"continue": true, "declare": true, "dirs": true, "disown": true,
	"echo": true, "enable": true, "eval": true, "exec": true,
	"exit": true, "export": true, "false": true, "fc": true,
	"fg": true, "getopts": true, "hash": true, "help": true,
	"history": true, "jobs": true, "kill": true, "let": true,
	strconst.StrLocal: true, "logout": true, "mapfile": true, "popd": true,
	"printf": true, "pushd": true, "pwd": true, "read": true,
	"readonly": true, "return": true, "set": true, "shift": true,
	"shopt": true, "source": true, "suspend": true, "test": true,
	"time": true, "times": true, "trap": true, "true": true,
	"type": true, "ulimit": true, "umask": true, "unalias": true,
	"unset": true, "wait": true,
}

// NormalizeCommand strips extra whitespace and normalizes quotes in a command string.
func NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	// Collapse multiple spaces
	var b strings.Builder
	prevSpace := false
	inSingle := false
	inDouble := false
	escaped := false

	for _, r := range cmd {
		if escaped {
			b.WriteRune(r)
			escaped = false
			prevSpace = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			b.WriteRune(r)
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if (r == ' ' || r == '\t') && !inSingle && !inDouble {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

// ExtractBaseCommand returns the primary command name from a command string.
// It strips leading sudo, env, nohup, and command prefixes.
func ExtractBaseCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	// Skip known prefixes
	prefixes := []string{"sudo ", "env ", "nohup ", "command ", "builtin "}
	for {
		lower := strings.ToLower(cmd)
		found := false
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				cmd = strings.TrimSpace(cmd[len(p):])
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	// Extract first word
	if idx := strings.IndexAny(cmd, " \t"); idx >= 0 {
		return cmd[:idx]
	}
	return cmd
}

// CommandSignature returns a normalized signature string for deduplication.
// Two commands with the same signature are functionally equivalent.
func CommandSignature(node ASTNode) string {
	if node == nil {
		return ""
	}

	switch n := node.(type) {
	case *CommandNode:
		var b strings.Builder
		b.WriteString(ExtractBaseCommand(n.Name))
		for _, a := range n.Args {
			b.WriteByte(' ')
			b.WriteString(a)
		}
		return NormalizeCommand(b.String())

	case *PipeNode:
		parts := make([]string, len(n.Commands))
		for i, c := range n.Commands {
			parts[i] = CommandSignature(c)
		}
		return strings.Join(parts, " | ")

	case *SequenceNode:
		parts := make([]string, len(n.Commands))
		for i, c := range n.Commands {
			parts[i] = CommandSignature(c)
		}
		return strings.Join(parts, "; ")

	case *AndNode:
		return CommandSignature(n.Left) + " && " + CommandSignature(n.Right)

	case *OrNode:
		return CommandSignature(n.Left) + " || " + CommandSignature(n.Right)

	case *SubshellNode:
		return "(" + CommandSignature(n.Body) + ")"

	case *BackgroundNode:
		return CommandSignature(n.Command) + " &"

	case *RedirectNode:
		body := ""
		if n.Body != nil {
			body = CommandSignature(n.Body) + " "
		}
		return fmt.Sprintf("%s%d%s %s", body, n.Fd, n.Op, n.Target)

	case *CompoundNode:
		parts := make([]string, len(n.Body))
		for i, c := range n.Body {
			parts[i] = CommandSignature(c)
		}
		return "{ " + strings.Join(parts, "; ") + " }"

	default:
		return node.String()
	}
}

// IsBuiltIn returns true if cmd is a bash built-in command.
func IsBuiltIn(cmd string) bool {
	cmd = ExtractBaseCommand(cmd)
	return builtins[strings.ToLower(cmd)]
}

// IsExternal returns true if cmd is an external (non-builtin) command.
func IsExternal(cmd string) bool {
	return !IsBuiltIn(cmd) && ExtractBaseCommand(cmd) != ""
}

// ExpandVariables performs basic environment variable expansion on a command string.
// It handles $VAR, ${VAR}, and $((expr)) patterns. Unresolved variables are
// left as empty strings.
func ExpandVariables(cmd string, env map[string]string) string {
	if env == nil {
		env = make(map[string]string)
	}

	var b strings.Builder
	i := 0
	runes := []rune(cmd)

	for i < len(runes) {
		// $((expr)) — strip arithmetic expansion
		if i+1 < len(runes) && runes[i] == '$' && runes[i+1] == '(' &&
			i+2 < len(runes) && runes[i+2] == '(' {
			end := findClosing(runes, i+2, '(', ')')
			if end > 0 {
				expr := string(runes[i+3 : end])
				if val, ok := evalSimpleArithmetic(expr, env); ok {
					b.WriteString(val)
				}
				i = end + 1
				continue
			}
		}

		// $(cmd) — command substitution (leave as-is, not safe to execute)
		if i+1 < len(runes) && runes[i] == '$' && runes[i+1] == '(' {
			end := findClosing(runes, i+1, '(', ')')
			if end > 0 {
				b.WriteRune('$')
				b.WriteString(string(runes[i+1 : end+1]))
				i = end + 1
				continue
			}
		}

		// ${VAR} or $VAR
		if runes[i] == '$' && i+1 < len(runes) && (isIdentChar(runes[i+1]) || runes[i+1] == '{') {
			varName, end := parseVarName(runes, i+1)
			if varName != "" {
				if val, ok := env[varName]; ok {
					b.WriteString(val)
				}
				i = end
				continue
			}
		}

		b.WriteRune(runes[i])
		i++
	}

	return b.String()
}

func isIdentChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func parseVarName(runes []rune, start int) (string, int) {
	if start >= len(runes) {
		return "", start
	}

	if runes[start] == '{' {
		// ${VAR} or ${VAR:-default}
		end := start + 1
		for end < len(runes) && runes[end] != '}' {
			end++
		}
		if end < len(runes) {
			name := string(runes[start+1 : end])
			// Handle ${VAR:-default}
			if idx := strings.Index(name, ":-"); idx >= 0 {
				return name[:idx], end + 1
			}
			if idx := strings.Index(name, ":="); idx >= 0 {
				return name[:idx], end + 1
			}
			return name, end + 1
		}
		return "", start
	}

	// $VAR
	end := start
	for end < len(runes) && isIdentChar(runes[end]) {
		end++
	}
	return string(runes[start:end]), end
}

func findClosing(runes []rune, start int, open, closing rune) int {
	depth := 0
	for i := start; i < len(runes); i++ {
		switch runes[i] {
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func evalSimpleArithmetic(expr string, env map[string]string) (string, bool) {
	expr = strings.TrimSpace(expr)
	// Simple variable lookup: $((VAR))
	if val, ok := env[expr]; ok {
		return val, true
	}
	// Simple integer: $((42))
	var n int
	if _, err := fmt.Sscanf(expr, "%d", &n); err == nil {
		return expr, true
	}
	return "", false
}
