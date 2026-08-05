// Package lsptool provides Language Server Protocol integration for code
// intelligence operations. It implements a JSON-RPC 2.0 client that communicates
// with external language servers (gopls, pylsp, typescript-language-server, etc.)
// to deliver hover information, go-to-definition, find references, completions,
// diagnostics, document symbols, formatting, renaming, and code actions.
//
// The package is organized into four layers:
//
//   - protocol.go — LSP protocol type definitions (positions, ranges, symbols, etc.)
//   - lsp.go      — Low-level JSON-RPC client wrapping a language server process
//   - manager.go  — Multi-language manager that auto-detects file types and
//     maintains one client per language
//   - register.go — Tool definitions that expose LSP operations through the
//     Dxrk-Ai tool registry
//
// All communication uses stdlib only; no external LSP libraries are imported.
package lsptool

import "github.com/Dxrk777/Dxrk-Ai/internal/tools"

// RegisterAll registers every LSP tool in the given registry.
func RegisterAll(reg *tools.Registry) error {
	for _, fn := range []func(*tools.Registry) error{
		registerLSPHover,
		registerLSPGoto,
		registerLSPReferences,
		registerLSPSymbols,
		registerLSPDiagnostics,
		registerLSPRename,
		registerLSPFormat,
	} {
		if err := fn(reg); err != nil {
			return err
		}
	}
	return nil
}
