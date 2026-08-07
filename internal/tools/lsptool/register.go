package lsptool

import (
	"fmt"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

// globalManager is lazily initialized on first use by any tool.
var globalManager *LSPManager

func getManager() *LSPManager {
	if globalManager == nil {
		globalManager = NewLSPManager()
	}
	return globalManager
}

// ── Tool definitions ─────────────────────────────────────────────────────────

func registerLSPHover(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_hover", Description: "Get hover info (type, docs, signature) for the symbol at a position.",
		InputSchema: positionSchema(strconst.StrFilePath, "line", "character"), Validate: validatePositionInput,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return executeLSPFeature(input, func(c *LSPClient, p TextDocumentPositionParams) (any, error) { return c.Hover(p) })
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPGoto(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_goto", Description: "Go to definition of the symbol at a position.",
		InputSchema: positionSchema(strconst.StrFilePath, "line", "character"), Validate: validatePositionInput,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return executeLSPFeature(input, func(c *LSPClient, p TextDocumentPositionParams) (any, error) { return c.GotoDefinition(p) })
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPReferences(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_references", Description: "Find all references to the symbol at a position.",
		InputSchema: positionSchema(strconst.StrFilePath, "line", "character"), Validate: validatePositionInput,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return executeLSPFeature(input, func(c *LSPClient, p TextDocumentPositionParams) (any, error) { return c.FindReferences(p) })
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPSymbols(reg *tools.Registry) error {
	fileOnly := func(input map[string]any) error {
		if input == nil || input[strconst.StrFilePath] == nil {
			return fmt.Errorf("file_path is required")
		}
		if _, ok := input[strconst.StrFilePath].(string); !ok {
			return fmt.Errorf("file_path must be a string")
		}
		return nil
	}
	fileSchema := map[string]any{
		"type": strconst.StrObject,
		strconst.StrProperties: map[string]any{
			strconst.StrFilePath: map[string]any{"type": strconst.StrString, strconst.StrDescription: strconst.StrAbsolutePathToTheSourceFile},
		},
		strconst.StrRequired: []string{strconst.StrFilePath},
	}
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_symbols", Description: "Get the document symbol outline (functions, classes, variables) for a file.",
		InputSchema: fileSchema, Validate: fileOnly,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			filePath := input[strconst.StrFilePath].(string)
			client, err := getManager().GetClientForFile(filePath)
			if err != nil {
				return nil, err
			}
			symbols, err := client.DocumentSymbols(filePathToURI(filePath))
			if err != nil {
				return nil, err
			}
			return formatSymbols(symbols), nil
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPDiagnostics(reg *tools.Registry) error {
	fileOnly := func(input map[string]any) error {
		if input == nil || input[strconst.StrFilePath] == nil {
			return fmt.Errorf("file_path is required")
		}
		if _, ok := input[strconst.StrFilePath].(string); !ok {
			return fmt.Errorf("file_path must be a string")
		}
		return nil
	}
	fileSchema := map[string]any{
		"type": strconst.StrObject,
		strconst.StrProperties: map[string]any{
			strconst.StrFilePath: map[string]any{"type": strconst.StrString, strconst.StrDescription: strconst.StrAbsolutePathToTheSourceFile},
		},
		strconst.StrRequired: []string{strconst.StrFilePath},
	}
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_diagnostics", Description: "Get diagnostics (errors, warnings, hints) for a file.",
		InputSchema: fileSchema, Validate: fileOnly,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			filePath := input[strconst.StrFilePath].(string)
			client, err := getManager().GetClientForFile(filePath)
			if err != nil {
				return nil, err
			}
			return formatDiagnostics(client.Diagnostics(filePathToURI(filePath))), nil
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPRename(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_rename", Description: "Rename a symbol at a position. Returns workspace edits mapping file paths to changes.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFilePath: map[string]any{"type": strconst.StrString, strconst.StrDescription: strconst.StrAbsolutePathToTheSourceFile},
				"line":               map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Zero-based line number", strconst.StrMinimum: 0},
				"character":          map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Zero-based column number", strconst.StrMinimum: 0},
				"new_name":           map[string]any{"type": strconst.StrString, strconst.StrDescription: "The new name for the symbol"},
			},
			strconst.StrRequired: []string{strconst.StrFilePath, "line", "character", "new_name"},
		},
		Validate: func(input map[string]any) error {
			if err := validatePositionInput(input); err != nil {
				return err
			}
			if input["new_name"] == nil {
				return fmt.Errorf("new_name is required")
			}
			if _, ok := input["new_name"].(string); !ok {
				return fmt.Errorf("new_name must be a string")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			filePath := input[strconst.StrFilePath].(string)
			client, err := getManager().GetClientForFile(filePath)
			if err != nil {
				return nil, err
			}
			edit, err := client.Rename(RenameParams{
				TextDocument: TextDocumentIdentifier{URI: filePathToURI(filePath)},
				Position:     parsePosition(input),
				NewName:      input["new_name"].(string),
			})
			if err != nil {
				return nil, err
			}
			return formatWorkspaceEdit(edit), nil
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerLSPFormat(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name: "lsp_format", Description: "Format a document using the language server's formatter.",
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFilePath: map[string]any{"type": strconst.StrString, strconst.StrDescription: strconst.StrAbsolutePathToTheSourceFile},
				"tab_size":           map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Spaces per indent level (default 4)", "default": 4, strconst.StrMinimum: 1},
				"insert_spaces":      map[string]any{"type": "boolean", strconst.StrDescription: "Use spaces instead of tabs (default true)", "default": true},
			},
			strconst.StrRequired: []string{strconst.StrFilePath},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrFilePath] == nil {
				return fmt.Errorf("file_path is required")
			}
			if _, ok := input[strconst.StrFilePath].(string); !ok {
				return fmt.Errorf("file_path must be a string")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			filePath := input[strconst.StrFilePath].(string)
			client, err := getManager().GetClientForFile(filePath)
			if err != nil {
				return nil, err
			}
			opts := FormattingOptions{TabSize: 4, InsertSpaces: true}
			if v, ok := input["tab_size"].(float64); ok {
				opts.TabSize = int(v)
			}
			if v, ok := input["insert_spaces"].(bool); ok {
				opts.InsertSpaces = v
			}
			edits, err := client.FormatDocument(filePathToURI(filePath), opts)
			if err != nil {
				return nil, err
			}
			return formatTextEdits(edits), nil
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

// ── Shared helpers ───────────────────────────────────────────────────────────

func positionSchema(fileKey, lineKey, charKey string) map[string]any {
	return map[string]any{
		"type": strconst.StrObject,
		strconst.StrProperties: map[string]any{
			fileKey: map[string]any{"type": strconst.StrString, strconst.StrDescription: strconst.StrAbsolutePathToTheSourceFile},
			lineKey: map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Zero-based line number", strconst.StrMinimum: 0},
			charKey: map[string]any{"type": strconst.StrInteger, strconst.StrDescription: "Zero-based column number", strconst.StrMinimum: 0},
		},
		strconst.StrRequired: []string{fileKey, lineKey, charKey},
	}
}

type featureFunc func(*LSPClient, TextDocumentPositionParams) (any, error)

func executeLSPFeature(input map[string]any, fn featureFunc) (any, error) {
	filePath := input[strconst.StrFilePath].(string)
	client, err := getManager().GetClientForFile(filePath)
	if err != nil {
		return nil, err
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: filePathToURI(filePath)},
		Position:     parsePosition(input),
	}
	return fn(client, params)
}

func parsePosition(input map[string]any) Position {
	p := Position{}
	if v, ok := input["line"].(float64); ok {
		p.Line = int(v)
	}
	if v, ok := input["character"].(float64); ok {
		p.Character = int(v)
	}
	return p
}

func validatePositionInput(input map[string]any) error {
	if input == nil || input[strconst.StrFilePath] == nil {
		return fmt.Errorf("file_path is required")
	}
	if _, ok := input[strconst.StrFilePath].(string); !ok {
		return fmt.Errorf("file_path must be a string")
	}
	if input["line"] == nil {
		return fmt.Errorf("line is required")
	}
	if input["character"] == nil {
		return fmt.Errorf("character is required")
	}
	return nil
}

// ── Output formatters ────────────────────────────────────────────────────────

func formatSymbols(symbols []DocumentSymbol) map[string]any {
	out := make([]map[string]any, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, formatDocSymbol(s))
	}
	return map[string]any{"symbols": out, strconst.StrCount: len(out)}
}

func formatDocSymbol(s DocumentSymbol) map[string]any {
	m := map[string]any{
		"name": s.Name,
		"kind": SymbolKindName(s.Kind),
		"range": map[string]any{
			strconst.StrStartLine: s.Range.Start.Line, strconst.StrStartChar: s.Range.Start.Character,
			strconst.StrEndLine: s.Range.End.Line, strconst.StrEndChar: s.Range.End.Character,
		},
	}
	if s.Detail != "" {
		m["detail"] = s.Detail
	}
	if len(s.Children) > 0 {
		children := make([]map[string]any, 0, len(s.Children))
		for _, c := range s.Children {
			children = append(children, formatDocSymbol(c))
		}
		m["children"] = children
	}
	return m
}

func formatDiagnostics(diags []Diagnostic) map[string]any {
	out := make([]map[string]any, 0, len(diags))
	for _, d := range diags {
		out = append(out, map[string]any{
			"severity": DiagnosticSeverityName(d.Severity), "message": d.Message,
			"source": d.Source, "range": map[string]any{
				strconst.StrStartLine: d.Range.Start.Line, strconst.StrStartChar: d.Range.Start.Character,
				strconst.StrEndLine: d.Range.End.Line, strconst.StrEndChar: d.Range.End.Character,
			},
		})
	}
	return map[string]any{"diagnostics": out, strconst.StrCount: len(out)}
}

func formatWorkspaceEdit(edit *WorkspaceEdit) map[string]any {
	if edit == nil {
		return map[string]any{"changes": map[string]any{}, "file_count": 0}
	}
	changes := make(map[string]any, len(edit.Changes))
	for uri, edits := range edit.Changes {
		editsOut := make([]map[string]any, 0, len(edits))
		for _, e := range edits {
			editsOut = append(editsOut, map[string]any{
				strconst.StrStartLine: e.Range.Start.Line, strconst.StrStartChar: e.Range.Start.Character,
				strconst.StrEndLine: e.Range.End.Line, strconst.StrEndChar: e.Range.End.Character,
				"new_text": e.NewText,
			})
		}
		changes[uriToFilePath(uri)] = editsOut
	}
	return map[string]any{"changes": changes, "file_count": len(edit.Changes)}
}

func formatTextEdits(edits []TextEdit) map[string]any {
	out := make([]map[string]any, 0, len(edits))
	for _, e := range edits {
		out = append(out, map[string]any{
			strconst.StrStartLine: e.Range.Start.Line, strconst.StrStartChar: e.Range.Start.Character,
			strconst.StrEndLine: e.Range.End.Line, strconst.StrEndChar: e.Range.End.Character,
			"new_text": e.NewText,
		})
	}
	return map[string]any{"edits": out, strconst.StrCount: len(out)}
}
