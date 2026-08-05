package bashparse

import (
	"fmt"
	"strings"
)

// NodeType identifies the kind of AST node.
type NodeType int

const (
	NodeCommand    NodeType = iota // Simple command with name and arguments
	NodePipe                       // Pipeline of commands connected by |
	NodeSequence                   // Commands connected by ;
	NodeAnd                        // Commands connected by &&
	NodeOr                         // Commands connected by ||
	NodeSubshell                   // Command group inside ()
	NodeRedirect                   // I/O redirection
	NodeBackground                 // Command followed by &
	NodeCompound                   // Brace group { ... }
)

// String returns the human-readable name of a NodeType.
func (t NodeType) String() string {
	switch t {
	case NodeCommand:
		return "Command"
	case NodePipe:
		return "Pipe"
	case NodeSequence:
		return "Sequence"
	case NodeAnd:
		return "And"
	case NodeOr:
		return "Or"
	case NodeSubshell:
		return "Subshell"
	case NodeRedirect:
		return "Redirect"
	case NodeBackground:
		return "Background"
	case NodeCompound:
		return "Compound"
	default:
		return "Unknown"
	}
}

// ASTNode is the interface implemented by all AST node types.
type ASTNode interface {
	Type() NodeType
	String() string
	Children() []ASTNode
}

// Location tracks the source position of a node for error reporting.
type Location struct {
	Line   int // 1-based line number
	Column int // 1-based column number
	Offset int // 0-based byte offset from input start
}

// CommandNode represents a simple command with a name, arguments, and
// optional environment variable assignments.
type CommandNode struct {
	Name string
	Args []string
	Env  map[string]string
	Loc  Location
}

func (n *CommandNode) Type() NodeType      { return NodeCommand }
func (n *CommandNode) Children() []ASTNode { return nil }

func (n *CommandNode) String() string {
	var b strings.Builder
	for k, v := range n.Env {
		fmt.Fprintf(&b, "%s=%s ", k, v)
	}
	b.WriteString(n.Name)
	for _, a := range n.Args {
		b.WriteByte(' ')
		if strings.ContainsAny(a, " \t\"'\\$`|;&<>()!") {
			fmt.Fprintf(&b, "%q", a)
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// PipeNode represents a pipeline of commands connected by | operators.
type PipeNode struct {
	Commands []ASTNode
	Loc      Location
}

func (n *PipeNode) Type() NodeType      { return NodePipe }
func (n *PipeNode) Children() []ASTNode { return n.Commands }

func (n *PipeNode) String() string {
	parts := make([]string, len(n.Commands))
	for i, c := range n.Commands {
		parts[i] = c.String()
	}
	return strings.Join(parts, " | ")
}

// SequenceNode represents commands connected by semicolons.
type SequenceNode struct {
	Commands []ASTNode
	Loc      Location
}

func (n *SequenceNode) Type() NodeType      { return NodeSequence }
func (n *SequenceNode) Children() []ASTNode { return n.Commands }

func (n *SequenceNode) String() string {
	parts := make([]string, len(n.Commands))
	for i, c := range n.Commands {
		parts[i] = c.String()
	}
	return strings.Join(parts, "; ")
}

// AndNode represents commands connected by the && operator.
type AndNode struct {
	Left  ASTNode
	Right ASTNode
	Loc   Location
}

func (n *AndNode) Type() NodeType      { return NodeAnd }
func (n *AndNode) Children() []ASTNode { return []ASTNode{n.Left, n.Right} }
func (n *AndNode) String() string      { return fmt.Sprintf("%s && %s", n.Left, n.Right) }

// OrNode represents commands connected by the || operator.
type OrNode struct {
	Left  ASTNode
	Right ASTNode
	Loc   Location
}

func (n *OrNode) Type() NodeType      { return NodeOr }
func (n *OrNode) Children() []ASTNode { return []ASTNode{n.Left, n.Right} }
func (n *OrNode) String() string      { return fmt.Sprintf("%s || %s", n.Left, n.Right) }

// SubshellNode represents a command group executed in a subshell.
type SubshellNode struct {
	Body ASTNode
	Loc  Location
}

func (n *SubshellNode) Type() NodeType      { return NodeSubshell }
func (n *SubshellNode) Children() []ASTNode { return []ASTNode{n.Body} }
func (n *SubshellNode) String() string      { return fmt.Sprintf("(%s)", n.Body) }

// RedirectNode represents an I/O redirection.
type RedirectNode struct {
	Fd     int        // File descriptor (default 1 for >, 0 for <)
	Op     RedirectOp // The redirection operator
	Target string     // Target file or fd number
	Body   ASTNode    // The command being redirected (may be nil)
	Loc    Location
}

func (n *RedirectNode) Type() NodeType { return NodeRedirect }
func (n *RedirectNode) Children() []ASTNode {
	if n.Body != nil {
		return []ASTNode{n.Body}
	}
	return nil
}

func (n *RedirectNode) String() string {
	fd := ""
	if n.Fd != 1 && n.Fd != 0 {
		fd = fmt.Sprintf("%d", n.Fd)
	}
	body := ""
	if n.Body != nil {
		body = n.Body.String() + " "
	}
	return fmt.Sprintf("%s%s%s %s", body, fd, n.Op, n.Target)
}

// RedirectOp enumerates the supported redirection operators.
type RedirectOp int

const (
	RedirectRead    RedirectOp = iota // <
	RedirectWrite                     // >
	RedirectAppend                    // >>
	RedirectDupeIn                    // 2>&1 (dup stderr to stdout)
	RedirectDupeOut                   // <&1 (dup stdout to stderr)
	RedirectPipe                      // | (pipeline)
)

// String returns the shell representation of a RedirectOp.
func (op RedirectOp) String() string {
	switch op {
	case RedirectRead:
		return "<"
	case RedirectWrite:
		return ">"
	case RedirectAppend:
		return ">>"
	case RedirectDupeIn:
		return ">&"
	case RedirectDupeOut:
		return "<&"
	case RedirectPipe:
		return "|"
	default:
		return "?"
	}
}

// BackgroundNode represents a command executed in the background with &.
type BackgroundNode struct {
	Command ASTNode
	Loc     Location
}

func (n *BackgroundNode) Type() NodeType      { return NodeBackground }
func (n *BackgroundNode) Children() []ASTNode { return []ASTNode{n.Command} }
func (n *BackgroundNode) String() string      { return n.Command.String() + " &" }

// CompoundNode represents a brace group { ... } or if/while/for construct.
type CompoundNode struct {
	Body []ASTNode
	Loc  Location
}

func (n *CompoundNode) Type() NodeType      { return NodeCompound }
func (n *CompoundNode) Children() []ASTNode { return n.Body }

func (n *CompoundNode) String() string {
	parts := make([]string, len(n.Body))
	for i, c := range n.Body {
		parts[i] = c.String()
	}
	return "{ " + strings.Join(parts, "; ") + " }"
}

// Walk calls fn for every node in the tree rooted at n, depth-first.
func Walk(n ASTNode, fn func(ASTNode) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for _, child := range n.Children() {
		Walk(child, fn)
	}
}

// CollectCommands extracts all CommandNode instances from the tree.
func CollectCommands(n ASTNode) []*CommandNode {
	var cmds []*CommandNode
	Walk(n, func(node ASTNode) bool {
		if c, ok := node.(*CommandNode); ok {
			cmds = append(cmds, c)
		}
		return true
	})
	return cmds
}
