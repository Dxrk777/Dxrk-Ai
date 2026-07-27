// SPDX-License-Identifier: MIT
package filemerge

import (
	"fmt"
	"strings"
)

// UpsertCodexDxrkMemoryBlock removes any existing [mcp_servers.dxrk-memory] block from
// the given TOML content and appends a fresh block with the canonical dxrk-memory
// MCP entry (including --tools=agent). All other sections are preserved.
//
// dxrkMemoryCmd is the command string to use (e.g. an absolute path like
// "/usr/local/bin/dxrk-memory"). If dxrkMemoryCmd is empty, it falls back to "dxrk-memory".
//
// This is a string-based helper (no TOML parser dependency) ported from
// dxrk-memory/internal/setup/setup.go. It handles the limited TOML subset that
// Codex uses.
func UpsertCodexDxrkMemoryBlock(content, dxrkMemoryCmd string) string {
	if dxrkMemoryCmd == "" {
		dxrkMemoryCmd = "dxrk-memory"
	}
	// Escape backslashes for TOML double-quoted strings (Windows paths).
	// e.g. C:\Users\foo → C:\\Users\\foo — prevents TOML unicode escape errors (\U).
	escapedCmd := strings.ReplaceAll(dxrkMemoryCmd, `\`, `\\`)
	codexDxrkMemoryBlock := "[mcp_servers.dxrk-memory]\ncommand = \"" + escapedCmd + "\"\nargs = [\"mcp\", \"--tools=agent\"]"
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	var kept []string
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "[mcp_servers.dxrk-memory]" {
			// Skip the old block header and all its key-value lines.
			i++
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				if strings.HasPrefix(next, "[") && strings.HasSuffix(next, "]") {
					break
				}
				i++
			}
			continue
		}

		kept = append(kept, lines[i])
		i++
	}

	base := strings.TrimSpace(strings.Join(kept, "\n"))
	if base == "" {
		return codexDxrkMemoryBlock + "\n"
	}

	return base + "\n\n" + codexDxrkMemoryBlock + "\n"
}

// UpsertTopLevelTOMLString inserts or replaces a top-level key = "value" pair
// in TOML content. The key is placed before the first [section] header so it
// remains a top-level (non-table) setting. Existing occurrences of the key are
// removed before inserting the new value (idempotent).
//
// Ported from dxrk-memory/internal/setup/setup.go.
func UpsertTopLevelTOMLString(content, key, value string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	lineValue := fmt.Sprintf("%s = %q", key, value)

	// Remove all existing occurrences of the key.
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	// Find insertion point: before the first [section] header.
	insertAt := len(cleaned)
	for i, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			insertAt = i
			break
		}
	}

	out := make([]string, 0, insertAt+1+(len(cleaned)-insertAt))
	out = append(out, cleaned[:insertAt]...)
	out = append(out, lineValue)
	out = append(out, cleaned[insertAt:]...)

	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
