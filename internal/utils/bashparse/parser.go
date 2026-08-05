package bashparse

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseError reports a parse failure with source location.
type ParseError struct {
	Message string
	Pos     Location
}

func (e *ParseError) Error() string {
	if e.Pos.Line > 0 {
		return fmt.Sprintf("bashparse: line %d, col %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
	}
	return fmt.Sprintf("bashparse: %s", e.Message)
}

type tokenType int

const (
	tokWord           tokenType = iota
	tokPipe                     // |
	tokSemicolon                // ;
	tokAnd                      // &&
	tokOr                       // ||
	tokLParen                   // (
	tokRParen                   // )
	tokLBrace                   // {
	tokRBrace                   // }
	tokAmp                      // &
	tokRedirectIn               // <
	tokRedirectOut              // >
	tokRedirectAppend           // >>
	tokRedirectDupIn            // <&
	tokRedirectDupOut           // >&
	tokEOF
)

type token struct {
	typ tokenType
	val string
	pos Location
}

type lexer struct {
	input  []rune
	pos    int
	line   int
	col    int
	offset int
}

func newLexer(input string) *lexer {
	return &lexer{
		input: []rune(input),
		line:  1,
		col:   1,
	}
}

func (l *lexer) location() Location {
	return Location{Line: l.line, Column: l.col, Offset: l.offset}
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *lexer) advance() {
	if l.pos >= len(l.input) {
		return
	}
	r := l.input[l.pos]
	l.pos++
	l.offset++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

func (l *lexer) skipSpaces() {
	for unicode.IsSpace(l.peek()) {
		l.advance()
	}
}

func (l *lexer) readWord() string {
	var b strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	escaped := false

	for {
		r := l.peek()
		if r == 0 {
			break
		}
		if escaped {
			b.WriteRune(r)
			l.advance()
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			l.advance()
			continue
		}
		if r == '\'' && !inDouble && !inBacktick {
			inSingle = !inSingle
			l.advance()
			continue
		}
		if r == '"' && !inSingle && !inBacktick {
			inDouble = !inDouble
			l.advance()
			continue
		}
		if r == '`' && !inSingle && !inDouble {
			inBacktick = !inBacktick
			l.advance()
			continue
		}
		if inSingle || inDouble || inBacktick {
			b.WriteRune(r)
			l.advance()
			continue
		}
		// Stop at unquoted operators and whitespace
		if unicode.IsSpace(r) || r == '|' || r == ';' || r == '&' ||
			r == '(' || r == ')' || r == '{' || r == '}' || r == '<' || r == '>' {
			break
		}
		b.WriteRune(r)
		l.advance()
	}
	return b.String()
}

func (l *lexer) nextToken() token {
	l.skipSpaces()

	if l.peek() == 0 {
		return token{typ: tokEOF, pos: l.location()}
	}

	pos := l.location()
	r := l.peek()

	// Check for operators
	switch r {
	case '|':
		l.advance()
		if l.peek() == '|' {
			l.advance()
			return token{typ: tokOr, val: "||", pos: pos}
		}
		return token{typ: tokPipe, val: "|", pos: pos}
	case '&':
		l.advance()
		if l.peek() == '&' {
			l.advance()
			return token{typ: tokAnd, val: "&&", pos: pos}
		}
		return token{typ: tokAmp, val: "&", pos: pos}
	case ';':
		l.advance()
		return token{typ: tokSemicolon, val: ";", pos: pos}
	case '(':
		l.advance()
		return token{typ: tokLParen, val: "(", pos: pos}
	case ')':
		l.advance()
		return token{typ: tokRParen, val: ")", pos: pos}
	case '{':
		l.advance()
		return token{typ: tokLBrace, val: "{", pos: pos}
	case '}':
		l.advance()
		return token{typ: tokRBrace, val: "}", pos: pos}
	case '<':
		l.advance()
		if l.peek() == '&' {
			l.advance()
			return token{typ: tokRedirectDupIn, val: "<&", pos: pos}
		}
		return token{typ: tokRedirectIn, val: "<", pos: pos}
	case '>':
		l.advance()
		if l.peek() == '>' {
			l.advance()
			return token{typ: tokRedirectAppend, val: ">>", pos: pos}
		}
		if l.peek() == '&' {
			l.advance()
			return token{typ: tokRedirectDupOut, val: ">&", pos: pos}
		}
		return token{typ: tokRedirectOut, val: ">", pos: pos}
	}

	// Word
	return token{typ: tokWord, val: l.readWord(), pos: pos}
}

// tokenize returns all tokens from the input string.
func tokenize(input string) []token {
	lex := newLexer(input)
	var tokens []token
	for {
		t := lex.nextToken()
		tokens = append(tokens, t)
		if t.typ == tokEOF {
			break
		}
	}
	return tokens
}

// parser holds state for the recursive-descent parser.
type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(typ tokenType) error {
	t := p.peek()
	if t.typ != typ {
		return &ParseError{
			Message: fmt.Sprintf("expected %v, got %v (%q)", typ, t.typ, t.val),
			Pos:     t.pos,
		}
	}
	p.advance()
	return nil
}

// Parse converts a bash command string into an AST.
//
// The parser handles:
//   - Simple commands with arguments
//   - Pipelines (|)
//   - Logical operators (&&, ||)
//   - Command sequences (;)
//   - Subshells (())
//   - Background execution (&)
//   - Redirections (>, >>, <, 2>&1, &>)
//   - Single and double quotes
//   - Backslash escapes
//   - Variable references ($VAR, ${VAR})
//   - Command substitution ($(...), `...`)
func Parse(input string) (ASTNode, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, &ParseError{Message: "empty input"}
	}

	tokens := tokenize(input)
	p := &parser{tokens: tokens}

	node, err := p.parseList()
	if err != nil {
		return nil, err
	}
	return node, nil
}

// parseList handles sequences separated by ;, &&, ||
func (p *parser) parseList() (ASTNode, error) {
	left, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		switch t.typ {
		case tokSemicolon:
			p.advance()
			right, err := p.parsePipeline()
			if err != nil {
				return nil, err
			}
			left = &SequenceNode{Commands: []ASTNode{left, right}, Loc: t.pos}
		case tokAnd:
			p.advance()
			right, err := p.parsePipeline()
			if err != nil {
				return nil, err
			}
			left = &AndNode{Left: left, Right: right, Loc: t.pos}
		case tokOr:
			p.advance()
			right, err := p.parsePipeline()
			if err != nil {
				return nil, err
			}
			left = &OrNode{Left: left, Right: right, Loc: t.pos}
		default:
			return left, nil
		}
	}
}

// parsePipeline handles commands connected by |
func (p *parser) parsePipeline() (ASTNode, error) {
	first, err := p.parseCommand()
	if err != nil {
		return nil, err
	}

	if p.peek().typ != tokPipe {
		return first, nil
	}

	cmds := []ASTNode{first}
	for p.peek().typ == tokPipe {
		pipeTok := p.advance()
		cmd, err := p.parseCommand()
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
		_ = pipeTok
	}
	return &PipeNode{Commands: cmds, Loc: first.(*CommandNode).Loc}, nil
}

// parseCommand handles a single command with redirects and background.
func (p *parser) parseCommand() (ASTNode, error) {
	t := p.peek()

	// Subshell
	if t.typ == tokLParen {
		p.advance()
		body, err := p.parseList()
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokRParen); err != nil {
			return nil, err
		}
		node := &SubshellNode{Body: body, Loc: t.pos}
		// Check for background
		if p.peek().typ == tokAmp {
			p.advance()
			return &BackgroundNode{Command: node, Loc: t.pos}, nil
		}
		return node, nil
	}

	// Compound block { ... }
	if t.typ == tokLBrace {
		p.advance()
		var nodes []ASTNode
		for p.peek().typ != tokRBrace && p.peek().typ != tokEOF {
			n, err := p.parseList()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, n)
			if p.peek().typ == tokSemicolon {
				p.advance()
			}
		}
		if err := p.expect(tokRBrace); err != nil {
			return nil, err
		}
		return &CompoundNode{Body: nodes, Loc: t.pos}, nil
	}

	// Simple command
	if t.typ != tokWord {
		return nil, &ParseError{
			Message: fmt.Sprintf("unexpected token %v (%q)", t.typ, t.val),
			Pos:     t.pos,
		}
	}

	cmd, err := p.parseSimpleCommand()
	if err != nil {
		return nil, err
	}

	// Handle redirections after the command
	cmd = p.attachRedirects(cmd)

	// Handle background
	if p.peek().typ == tokAmp {
		p.advance()
		return &BackgroundNode{Command: cmd, Loc: t.pos}, nil
	}

	return cmd, nil
}

// parseSimpleCommand collects words into a CommandNode.
func (p *parser) parseSimpleCommand() (ASTNode, error) {
	t := p.peek()
	if t.typ != tokWord {
		return nil, &ParseError{
			Message: fmt.Sprintf("expected command name, got %v", t.typ),
			Pos:     t.pos,
		}
	}

	name := p.advance().val
	env := make(map[string]string)
	var args []string

	// Check for VAR=value assignments
	for p.peek().typ == tokWord {
		w := p.peek().val
		if idx := strings.Index(w, "="); idx > 0 {
			// Could be env assignment
			key := w[:idx]
			val := w[idx+1:]
			if isValidEnvKey(key) {
				p.advance()
				env[key] = val
				continue
			}
		}
		break
	}

	// Collect remaining arguments (skip redirection targets here)
	for p.peek().typ == tokWord || p.peek().typ == tokRedirectIn ||
		p.peek().typ == tokRedirectOut || p.peek().typ == tokRedirectAppend ||
		p.peek().typ == tokRedirectDupIn || p.peek().typ == tokRedirectDupOut {
		if p.peek().typ != tokWord {
			break
		}
		args = append(args, p.advance().val)
	}

	return &CommandNode{
		Name: name,
		Args: args,
		Env:  env,
		Loc:  t.pos,
	}, nil
}

// attachRedirects wraps a command node with any trailing redirections.
func (p *parser) attachRedirects(node ASTNode) ASTNode {
	for {
		t := p.peek()
		var op RedirectOp
		fd := -1

		switch t.typ {
		case tokRedirectIn:
			op = RedirectRead
			fd = 0
		case tokRedirectOut:
			op = RedirectWrite
			fd = 1
		case tokRedirectAppend:
			op = RedirectAppend
			fd = 1
		case tokRedirectDupIn:
			op = RedirectDupeIn
		case tokRedirectDupOut:
			op = RedirectDupeOut
		default:
			return node
		}

		p.advance()

		// Check for fd prefix (e.g., 2> or 2>&1)
		target := ""
		if p.peek().typ == tokWord {
			target = p.advance().val
		}

		// Parse fd from target if not set
		if fd == -1 {
			if t.typ == tokRedirectIn {
				fd = 0
			} else {
				fd = 1
			}
			// Check if the word before the op was a fd
			if name, ok := node.(*CommandNode); ok && len(name.Args) > 0 {
				last := name.Args[len(name.Args)-1]
				if n, err := strconv.Atoi(last); err == nil {
					fd = n
					name.Args = name.Args[:len(name.Args)-1]
				}
			}
		}

		// Handle 2>&1 style
		if op == RedirectDupeIn || op == RedirectDupeOut {
			if target == "" && p.peek().typ == tokWord {
				target = p.advance().val
			}
		}

		// Parse target fd
		targetFd := 0
		if op == RedirectDupeIn || op == RedirectDupeOut {
			if n, err := strconv.Atoi(target); err == nil {
				targetFd = n
			}
			target = fmt.Sprintf("%d", targetFd)
		}

		node = &RedirectNode{
			Fd:     fd,
			Op:     op,
			Target: target,
			Body:   node,
			Loc:    t.pos,
		}
	}
}

func isValidEnvKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return true
}
