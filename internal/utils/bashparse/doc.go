// Package bashparse provides bash command parsing, AST representation,
// and danger analysis utilities for Dxrk-Ai.
//
// The package implements a recursive-descent parser that converts bash command
// strings into a structured abstract syntax tree (AST). The AST captures the
// full structure of a command including pipes, sequences, subshells,
// redirections, and background execution.
//
// Usage:
//
//	node, err := bashparse.Parse("ls -la | grep '.go' && echo done")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	checks := bashparse.AnalyzeDanger(node)
//	for _, check := range checks {
//	    fmt.Printf("[%s] %s\n", check.Level, check.Reason)
//	}
//
// The danger analysis module scans AST nodes for destructive patterns such as
// recursive deletion, fork bombs, disk wiping, and privilege escalation.
// Each finding includes a severity level, the matched pattern, and a
// suggestion for a safer alternative.
//
// Command normalization utilities help deduplicate and classify commands by
// stripping whitespace, expanding variables, and identifying built-in vs
// external commands.
package bashparse
