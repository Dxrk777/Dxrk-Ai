package lsptool

// ── Core position / range ────────────────────────────────────────────────────

// Position represents a zero-based line/column position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a start/end pair of positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// ── Document identifiers ─────────────────────────────────────────────────────

// TextDocumentIdentifier is a URI reference to a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier adds a version number for change tracking.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentPositionParams bundles a document URI with a cursor position.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ── Hover ────────────────────────────────────────────────────────────────────

// HoverResult is the response from textDocument/hover.
type HoverResult struct {
	Contents MarkupContent `json:"contents"`
	Range    Range         `json:"range,omitempty"`
}

// MarkupContent holds rich documentation (plaintext or markdown).
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// ── Completion ───────────────────────────────────────────────────────────────

// CompletionKind enumerates completion item kinds.
type CompletionKind int

const (
	CompletionKindText          CompletionKind = 1
	CompletionKindMethod        CompletionKind = 2
	CompletionKindFunction      CompletionKind = 3
	CompletionKindConstructor   CompletionKind = 4
	CompletionKindField         CompletionKind = 5
	CompletionKindVariable      CompletionKind = 6
	CompletionKindClass         CompletionKind = 7
	CompletionKindInterface     CompletionKind = 8
	CompletionKindModule        CompletionKind = 9
	CompletionKindProperty      CompletionKind = 10
	CompletionKindUnit          CompletionKind = 11
	CompletionKindValue         CompletionKind = 12
	CompletionKindEnum          CompletionKind = 13
	CompletionKindKeyword       CompletionKind = 14
	CompletionKindSnippet       CompletionKind = 15
	CompletionKindColor         CompletionKind = 16
	CompletionKindFile          CompletionKind = 17
	CompletionKindReference     CompletionKind = 18
	CompletionKindFolder        CompletionKind = 19
	CompletionKindEnumMember    CompletionKind = 20
	CompletionKindConstant      CompletionKind = 21
	CompletionKindStruct        CompletionKind = 22
	CompletionKindEvent         CompletionKind = 23
	CompletionKindOperator      CompletionKind = 24
	CompletionKindTypeParameter CompletionKind = 25
)

// CompletionItem represents a single auto-completion suggestion.
type CompletionItem struct {
	Label               string         `json:"label"`
	Kind                CompletionKind `json:"kind,omitempty"`
	Detail              string         `json:"detail,omitempty"`
	Documentation       string         `json:"documentation,omitempty"`
	InsertText          string         `json:"insertText,omitempty"`
	TextEdit            *TextEdit      `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit     `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string       `json:"commitCharacters,omitempty"`
	SortText            string         `json:"sortText,omitempty"`
	FilterText          string         `json:"filterText,omitempty"`
	Preselect           bool           `json:"preselect,omitempty"`
}

// CompletionList wraps the CompletionList response shape.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete,omitempty"`
	Items        []CompletionItem `json:"items"`
}

// ── Symbols ──────────────────────────────────────────────────────────────────

// SymbolKind enumerates symbol kinds for document and workspace symbols.
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// DocumentSymbol represents a hierarchical symbol inside a document.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents a flat workspace symbol.
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// SymbolKindName returns a human-readable name for a SymbolKind.
func SymbolKindName(k SymbolKind) string {
	switch k {
	case SymbolKindFile:
		return "File"
	case SymbolKindModule:
		return "Module"
	case SymbolKindNamespace:
		return "Namespace"
	case SymbolKindPackage:
		return "Package"
	case SymbolKindClass:
		return "Class"
	case SymbolKindMethod:
		return "Method"
	case SymbolKindProperty:
		return "Property"
	case SymbolKindField:
		return "Field"
	case SymbolKindConstructor:
		return "Constructor"
	case SymbolKindEnum:
		return "Enum"
	case SymbolKindInterface:
		return "Interface"
	case SymbolKindFunction:
		return "Function"
	case SymbolKindVariable:
		return "Variable"
	case SymbolKindConstant:
		return "Constant"
	case SymbolKindString:
		return "String"
	case SymbolKindNumber:
		return "Number"
	case SymbolKindBoolean:
		return "Boolean"
	case SymbolKindArray:
		return "Array"
	case SymbolKindObject:
		return "Object"
	case SymbolKindKey:
		return "Key"
	case SymbolKindNull:
		return "Null"
	case SymbolKindEnumMember:
		return "EnumMember"
	case SymbolKindStruct:
		return "Struct"
	case SymbolKindEvent:
		return "Event"
	case SymbolKindOperator:
		return "Operator"
	case SymbolKindTypeParameter:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}

// ── Location / TextEdit ──────────────────────────────────────────────────────

// Location links a URI to a range within that document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextEdit replaces a range in a document with new text.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// ── Diagnostics ──────────────────────────────────────────────────────────────

// DiagnosticSeverity indicates the severity of a diagnostic message.
type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

// DiagnosticSeverityName returns a human-readable name for a DiagnosticSeverity.
func DiagnosticSeverityName(s DiagnosticSeverity) string {
	switch s {
	case DiagnosticSeverityError:
		return "Error"
	case DiagnosticSeverityWarning:
		return "Warning"
	case DiagnosticSeverityInformation:
		return "Information"
	case DiagnosticSeverityHint:
		return "Hint"
	default:
		return "Unknown"
	}
}

// Diagnostic describes a problem reported by the language server.
type Diagnostic struct {
	Range              Range                `json:"range"`
	Severity           DiagnosticSeverity   `json:"severity,omitempty"`
	Code               any                  `json:"code,omitempty"`
	Source             string               `json:"source,omitempty"`
	Message            string               `json:"message"`
	RelatedInformation []RelatedInformation `json:"relatedInformation,omitempty"`
}

// RelatedInformation links a diagnostic to another location.
type RelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// ── Workspace edits / Rename / Code actions ──────────────────────────────────

// WorkspaceEdit holds a set of text edits keyed by document URI.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// RenameParams bundles the inputs for a rename request.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// CodeActionKind constants.
const (
	CodeActionKindQuickFix              = "quickfix"
	CodeActionKindRefactor              = "refactor"
	CodeActionKindRefactorExtract       = "refactor.extract"
	CodeActionKindRefactorInline        = "refactor.inline"
	CodeActionKindSourceOrganizeImports = "source.organizeImports"
)

// CodeAction represents an action the user can take.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
}

// CodeActionParams bundles the inputs for a codeAction request.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeActionContext carries the diagnostics that triggered the actions.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ── Formatting ───────────────────────────────────────────────────────────────

// FormattingOptions describes how the document should be formatted.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// ── Text document content change events ──────────────────────────────────────

// TextDocumentContentChangeEvent describes a change to a text document.
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}
