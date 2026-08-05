// SPDX-License-Identifier: MIT
package security

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// ---- AST Node Types ----

// NodeType represents the type of a shell AST node.
type NodeType int

const (
	NodeCommand NodeType = iota
	NodePipeline
	NodeList
	NodeSubshell
	NodeRedirect
	NodeVariable
	NodeAssignment
	NodeWord
	NodeOperator
	NodeHeredoc
	NodeComment
	NodeUnknown
)

func (n NodeType) String() string {
	switch n {
	case NodeCommand:
		return "command"
	case NodePipeline:
		return "pipeline"
	case NodeList:
		return "list"
	case NodeSubshell:
		return "subshell"
	case NodeRedirect:
		return "redirect"
	case NodeVariable:
		return "variable"
	case NodeAssignment:
		return "assignment"
	case NodeWord:
		return "word"
	case NodeOperator:
		return "operator"
	case NodeHeredoc:
		return "heredoc"
	case NodeComment:
		return "comment"
	default:
		return strconst.StrUnknown
	}
}

// ASTNode represents a node in the parsed shell AST.
type ASTNode struct {
	Type     NodeType
	Value    string
	Children []*ASTNode
	Pos      int
	Len      int
}

// ParseForSecurityResult is the outcome of security-focused shell parsing.
type ParseForSecurityResult struct {
	Root       *ASTNode
	Command    string
	EnvVars    []string
	Operators  []string
	IsSafe     bool
	Violations []string
}

// ---- Safe builtins (commands that are considered safe) ----

var safeBuiltins = map[string]bool{
	"echo": true, "printf": true, "cat": true, "head": true, "tail": true,
	"wc": true, "ls": true, "pwd": true, "whoami": true, "date": true,
	"env": true, "printenv": true, "which": true, "type": true, "file": true,
	"stat": true, "readlink": true, "dirname": true, "basename": true,
	"sort": true, "uniq": true, "tr": true, "cut": true, "paste": true,
	"column": true, "tee": true, "xargs": true, "find": true, "grep": true,
	"rg": true, "ag": true, "awk": true, "sed": true, "jq": true, "yq": true,
	"diff": true, "comm": true, "mktemp": true, "realpath": true,
	"go": true, "cargo": true, "rustc": true, "node": true, "npm": true,
	"npx": true, "pnpm": true, "yarn": true, "bun": true, "deno": true,
	"python": true, "python3": true, "pip": true, "pip3": true, "uv": true,
	"git": true, "gh": true, "docker": true, "podman": true,
	"make": true, "cmake": true, "meson": true,
	"mkdir": true, "touch": true, "cp": true, "mv": true, "ln": true,
	"chmod": true, "chown": true, "df": true, "du": true, "tree": true,
	"sha256sum": true, "md5sum": true, "base64": true,
	"sleep": true, "true": true, "false": true, "test": true,
}

// ---- Regex patterns for dangerous constructs ----

var (
	cmdSubstPattern  = regexp.MustCompile(`\$\(`)
	backtickPattern  = regexp.MustCompile("`[^`]*`")
	varExpandPattern = regexp.MustCompile(`\$\{[^}]*\}`)
	evalPattern      = regexp.MustCompile(`(?:^|\s)(?:eval|exec|source|\.)\s`)
	sudoPattern      = regexp.MustCompile(`(?:^|\s)(?:sudo|su|doas)\s`)
	hexEscapePattern = regexp.MustCompile(`\\x[0-9a-fA-F]{2}`)
	nullBytePattern  = regexp.MustCompile(`\x00`)
)

// ---- Public API ----

// ParseForSecurity parses a shell command and returns a security analysis.
// Fail-closed: any parse error results in IsSafe=false with violations.
func ParseForSecurity(command string) ParseForSecurityResult {
	if command == "" {
		return ParseForSecurityResult{
			Command: command,
			IsSafe:  true,
		}
	}

	// Input length guard
	if len(command) > 10000 {
		return ParseForSecurityResult{
			Command: command,
			IsSafe:  false,
			Violations: []string{
				"command exceeds maximum length (10000 bytes)",
			},
		}
	}

	// Null byte check
	if nullBytePattern.MatchString(command) {
		return ParseForSecurityResult{
			Command: command,
			IsSafe:  false,
			Violations: []string{
				"null byte detected in command",
			},
		}
	}

	// Phase 1: Regex-based pattern detection (fast path)
	violations := detectDangerousPatterns(command)

	// Phase 2: Lightweight AST parsing
	root, parseErr := parseShell(command)
	if parseErr != nil {
		violations = append(violations, fmt.Sprintf("parse error: %v", parseErr))
		return ParseForSecurityResult{
			Command:    command,
			Root:       nil,
			IsSafe:     false,
			Violations: violations,
		}
	}

	// Phase 3: AST-based analysis
	envVars, operators := analyzeAST(root)
	astViolations := validateAST(root)
	violations = append(violations, astViolations...)

	return ParseForSecurityResult{
		Root:       root,
		Command:    command,
		EnvVars:    envVars,
		Operators:  operators,
		IsSafe:     len(violations) == 0,
		Violations: violations,
	}
}

// ---- Phase 1: Regex pattern detection ----

func detectDangerousPatterns(command string) []string {
	var violations []string

	if cmdSubstPattern.MatchString(command) {
		violations = append(violations, "command substitution detected: $()")
	}
	if backtickPattern.MatchString(command) {
		violations = append(violations, "backtick substitution detected")
	}
	if varExpandPattern.MatchString(command) {
		violations = append(violations, "variable expansion detected: ${}")
	}
	if evalPattern.MatchString(command) {
		violations = append(violations, "eval/exec/source builtin detected")
	}
	if sudoPattern.MatchString(command) {
		violations = append(violations, "privilege escalation detected: sudo/su/doas")
	}
	if hexEscapePattern.MatchString(command) {
		violations = append(violations, "hex escape sequence detected")
	}

	return violations
}

// ---- Phase 2: Lightweight shell parser ----

// parseShell performs a lightweight shell parse (no external deps).
// Returns an AST or error. Fail-closed on any ambiguity.
func parseShell(input string) (*ASTNode, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	return buildAST(tokens)
}

type tokenType int

const (
	tokWord tokenType = iota
	tokPipe
	tokAmpAmp
	tokPipePipe
	tokSemicolon
	tokAnd
	tokRedirectOut
	tokRedirectAppend
	tokRedirectIn
	tokHeredoc
	tokLParen
	tokRParen
	tokLBrace
	tokRBrace
	tokDollar
	tokBacktick
	tokNewline
	tokEOF
)

type token struct {
	typ   tokenType
	value string
	pos   int
}

func (t token) String() string {
	return fmt.Sprintf("tok(%v, %q, %d)", t.typ, t.value, t.pos)
}

// tokenize splits a shell command into tokens.
func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0
	depth := 0

	for i < len(input) {
		ch := input[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}

		// Newline
		if ch == '\n' {
			tokens = append(tokens, token{typ: tokNewline, pos: i})
			i++
			continue
		}

		// Comments
		if ch == '#' {
			end := strings.IndexByte(input[i:], '\n')
			if end == -1 {
				break
			}
			i += end + 1
			continue
		}

		// Quoted strings — skip to closing quote
		if ch == '"' || ch == '\'' {
			quote := ch
			i++ // skip opening quote
			for i < len(input) && input[i] != quote {
				if input[i] == '\\' && i+1 < len(input) {
					i += 2 // skip escaped char
				} else {
					i++
				}
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string starting at position %d", i)
			}
			i++ // skip closing quote
			continue
		}

		// Dollar in various forms
		if ch == '$' {
			if i+1 < len(input) && input[i+1] == '(' {
				// Command substitution: $()
				tokens = append(tokens, token{typ: tokDollar, value: "$(", pos: i})
				i += 2
				depth++
				continue
			}
			// Variable reference
			j := i + 1
			if j < len(input) && input[j] == '{' {
				// ${var}
				j++
				for j < len(input) && input[j] != '}' {
					j++
				}
				if j < len(input) {
					j++ // closing }
				}
			} else {
				// $var
				for j < len(input) && (isAlphaNum(input[j]) || input[j] == '_') {
					j++
				}
			}
			tokens = append(tokens, token{typ: tokWord, value: input[i:j], pos: i})
			i = j
			continue
		}

		// Backtick
		if ch == '`' {
			tokens = append(tokens, token{typ: tokBacktick, value: "`", pos: i})
			i++
			depth++
			continue
		}

		// Closing subshell / group
		if ch == ')' {
			if depth > 0 {
				depth--
			}
			tokens = append(tokens, token{typ: tokRParen, value: ")", pos: i})
			i++
			continue
		}

		if ch == '(' {
			depth++
			tokens = append(tokens, token{typ: tokLParen, value: "(", pos: i})
			i++
			continue
		}

		// Redirections
		if ch == '>' {
			if i+1 < len(input) && input[i+1] == '>' {
				tokens = append(tokens, token{typ: tokRedirectAppend, value: ">>", pos: i})
				i += 2
			} else {
				tokens = append(tokens, token{typ: tokRedirectOut, value: ">", pos: i})
				i++
			}
			continue
		}

		if ch == '<' {
			switch {
			case i+2 < len(input) && input[i+1] == '<' && input[i+2] == '<':
				tokens = append(tokens, token{typ: tokHeredoc, value: "<<<", pos: i})
				i += 3
			case i+1 < len(input) && input[i+1] == '<':
				tokens = append(tokens, token{typ: tokHeredoc, value: "<<", pos: i})
				i += 2
			default:
				tokens = append(tokens, token{typ: tokRedirectIn, value: "<", pos: i})
				i++
			}
			continue
		}

		// Pipes and operators
		if ch == '|' {
			if i+1 < len(input) && input[i+1] == '|' {
				tokens = append(tokens, token{typ: tokPipePipe, value: "||", pos: i})
				i += 2
			} else {
				tokens = append(tokens, token{typ: tokPipe, value: "|", pos: i})
				i++
			}
			continue
		}

		if ch == '&' {
			if i+1 < len(input) && input[i+1] == '&' {
				tokens = append(tokens, token{typ: tokAmpAmp, value: "&&", pos: i})
				i += 2
			} else {
				// background &
				tokens = append(tokens, token{typ: tokWord, value: "&", pos: i})
				i++
			}
			continue
		}

		if ch == ';' {
			tokens = append(tokens, token{typ: tokSemicolon, value: ";", pos: i})
			i++
			continue
		}

		// Word (command name, argument, etc.)
		j := i
		for j < len(input) {
			c := input[j]
			if c == ' ' || c == '\t' || c == '\n' || c == ';' || c == '|' || c == '&' || c == '>' || c == '<' || c == '(' || c == ')' || c == '`' || c == '#' {
				break
			}
			if c == '"' || c == '\'' {
				// embedded quote in word — skip
				j++
				for j < len(input) && input[j] != c {
					if input[j] == '\\' && j+1 < len(input) {
						j += 2
					} else {
						j++
					}
				}
				if j < len(input) {
					j++
				}
				continue
			}
			if c == '$' && j+1 < len(input) && (input[j+1] == '(' || input[j+1] == '{' || isAlphaNum(input[j+1]) || input[j+1] == '_') {
				// stop word at variable/command substitution
				break
			}
			j++
		}
		if j > i {
			tokens = append(tokens, token{typ: tokWord, value: input[i:j], pos: i})
		}
		i = j
	}

	tokens = append(tokens, token{typ: tokEOF, pos: i})
	return tokens, nil
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// buildAST constructs an AST from tokens.
func buildAST(tokens []token) (*ASTNode, error) {
	root := &ASTNode{Type: NodeList, Value: "root"}
	i := 0

	for i < len(tokens) {
		tok := tokens[i]
		switch tok.typ {
		case tokEOF:
			return root, nil
		case tokNewline:
			i++
			continue
		case tokWord:
			// Collect command + arguments until a separator
			cmd := &ASTNode{Type: NodeCommand, Value: tok.value, Pos: tok.pos}
			i++
		argLoop:
			for i < len(tokens) {
				next := tokens[i]
				switch next.typ {
				case tokWord:
					cmd.Children = append(cmd.Children, &ASTNode{Type: NodeWord, Value: next.value, Pos: next.pos})
					i++
				case tokRedirectOut, tokRedirectAppend, tokRedirectIn, tokHeredoc:
					redir := &ASTNode{Type: NodeRedirect, Value: next.value, Pos: next.pos}
					// next token should be the target
					i++
					if i < len(tokens) && tokens[i].typ == tokWord {
						redir.Children = append(redir.Children, &ASTNode{Type: NodeWord, Value: tokens[i].value, Pos: tokens[i].pos})
						i++
					}
					cmd.Children = append(cmd.Children, redir)
				default:
					break argLoop
				}
			}
			root.Children = append(root.Children, cmd)
		case tokPipe:
			// Wrap last two children in a pipeline
			if len(root.Children) >= 2 {
				last := root.Children[len(root.Children)-1]
				prev := root.Children[len(root.Children)-2]
				pipeline := &ASTNode{
					Type:     NodePipeline,
					Value:    "|",
					Children: []*ASTNode{prev, last},
					Pos:      tok.pos,
				}
				root.Children = root.Children[:len(root.Children)-2]
				root.Children = append(root.Children, pipeline)
			}
			i++
		case tokSemicolon, tokAmpAmp, tokPipePipe:
			op := &ASTNode{Type: NodeOperator, Value: tok.value, Pos: tok.pos}
			root.Children = append(root.Children, op)
			i++
		case tokLParen:
			depth := 1
			subTokens := []token{}
			i++ // skip (
			start := tok.pos
			for i < len(tokens) && depth > 0 {
				if tokens[i].typ == tokLParen {
					depth++
				} else if tokens[i].typ == tokRParen {
					depth--
					if depth == 0 {
						i++ // skip )
						break
					}
				}
				subTokens = append(subTokens, tokens[i])
				i++
			}
			sub, err := buildAST(subTokens)
			if err != nil {
				return nil, err
			}
			sub.Type = NodeSubshell
			sub.Pos = start
			root.Children = append(root.Children, sub)
		default:
			i++
		}
	}

	return root, nil
}

// ---- Phase 3: AST analysis ----

func analyzeAST(root *ASTNode) (envVars, operators []string) {
	if root == nil {
		return nil, nil
	}

	for _, child := range root.Children {
		switch child.Type {
		case NodeCommand:
			name := commandName(child)
			if isEnvAssignment(child) {
				envVars = append(envVars, child.Value)
			}
			operators = append(operators, name)
		case NodePipeline, NodeSubshell:
			envVars2, ops2 := analyzeAST(child)
			envVars = append(envVars, envVars2...)
			operators = append(operators, ops2...)
		case NodeOperator:
			operators = append(operators, child.Value)
		case NodeRedirect:
			operators = append(operators, child.Value)
		}
	}
	return envVars, operators
}

func validateAST(root *ASTNode) []string {
	if root == nil {
		return nil
	}

	var violations []string

	for _, child := range root.Children {
		switch child.Type {
		case NodeCommand:
			name := commandName(child)
			if name != "" && !safeBuiltins[name] {
				// Unknown command — not necessarily a violation, but flagged
				// Only flag truly dangerous ones
				if isDangerousCommand(name) {
					violations = append(violations, fmt.Sprintf("dangerous command: %s", name))
				}
			}
		case NodePipeline, NodeSubshell:
			violations = append(violations, validateAST(child)...)
		}
	}

	return violations
}

func commandName(cmd *ASTNode) string {
	if cmd == nil || cmd.Type != NodeCommand {
		return ""
	}
	// Strip any assignment prefix
	val := cmd.Value
	if idx := strings.Index(val, "="); idx > 0 && !strings.Contains(val[:idx], " ") {
		return "" // it's an assignment
	}
	return val
}

func isEnvAssignment(node *ASTNode) bool {
	if node == nil || node.Type != NodeCommand {
		return false
	}
	val := node.Value
	if idx := strings.Index(val, "="); idx > 0 && idx < len(val)-1 {
		prefix := val[:idx]
		for _, r := range prefix {
			if !isAlphaNum(byte(r)) && r != '_' {
				return false
			}
		}
		return true
	}
	return false
}

func isDangerousCommand(name string) bool {
	dangerous := map[string]bool{
		"eval": true, "exec": true, "source": true, "sudo": true, "su": true,
		"doas": true, "rm": true, "dd": true, "mkfs": true, strconst.StrFormat: true,
		":(){": true, // fork bomb pattern
	}
	return dangerous[name]
}

// ---- Utilities ----

// ExtractCommandName extracts the first command name from a shell command string.
// Returns empty string on parse failure (fail-closed).
func ExtractCommandName(command string) string {
	result := ParseForSecurity(command)
	if result.Root == nil {
		return ""
	}
	for _, child := range result.Root.Children {
		if child.Type == NodeCommand {
			return child.Value
		}
	}
	return ""
}

// HasDangerousPatterns checks if a command contains any dangerous shell patterns.
func HasDangerousPatterns(command string) bool {
	return len(detectDangerousPatterns(command)) > 0
}

// SanitizeForLog removes sensitive patterns from a command string for logging.
func SanitizeForLog(command string) string {
	// Remove anything that looks like a token or key
	sensitive := regexp.MustCompile(`(sk-[a-zA-Z0-9_-]{20,}|token[=:]\s*\S+)`)
	result := sensitive.ReplaceAllString(command, "[REDACTED]")

	// Truncate very long commands
	if len(result) > 500 {
		result = result[:500] + "...[truncated]"
	}

	// Remove ANSI escape codes
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	result = ansi.ReplaceAllString(result, "")

	return strings.TrimSpace(result)
}

// IsReadOnlyCommand checks if a command is read-only (no side effects).
func IsReadOnlyCommand(command string) bool {
	ro := map[string]bool{
		"echo": true, "printf": true, "cat": true, "head": true, "tail": true,
		"wc": true, "ls": true, "pwd": true, "whoami": true, "date": true,
		"env": true, "printenv": true, "which": true, "type": true, "file": true,
		"stat": true, "readlink": true, "dirname": true, "basename": true,
		"sort": true, "uniq": true, "tr": true, "cut": true, "diff": true,
		"comm": true, "tree": true, "df": true, "du": true,
	}
	name := ExtractCommandName(command)
	return ro[name]
}

// MaxCommandLength is the maximum allowed command length.
const MaxCommandLength = 10000

// QuoteSafety wraps a command for safe shell execution.
func QuoteSafety(command string) string {
	if !strings.ContainsAny(command, " \t\n\"'\\$`|&;><") {
		return command
	}
	// Use single quotes with internal escaping
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range command {
		if r == '\'' {
			b.WriteString("'\"'\"'")
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// StripAnsi removes ANSI escape codes from a string.
func StripAnsi(s string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansi.ReplaceAllString(s, "")
}

// IsQuiet checks if a string contains only whitespace or is empty.
func IsQuiet(s string) bool {
	return strings.TrimSpace(s) == ""
}

// TruncateWithSuffix truncates a string to maxLen, adding suffix if truncated.
func TruncateWithSuffix(s string, maxLen int, suffix string) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-len(suffix)] + suffix
}

// RedactSensitive removes tokens, keys, and passwords from strings.
func RedactSensitive(s string) string {
	// sk-ant-* tokens
	t1 := regexp.MustCompile(`sk-[a-zA-Z0-9_-]{20,}`)
	s = t1.ReplaceAllString(s, "[REDACTED]")

	// API key patterns
	t2 := regexp.MustCompile(`(?i)(?:api[_-]?key|token|secret|password)[=:]\s*\S+`)
	s = t2.ReplaceAllString(s, "[REDACTED]")

	return s
}

// IsASCII checks if a string contains only ASCII characters.
func IsASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
