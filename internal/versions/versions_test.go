// SPDX-License-Identifier: MIT
package versions

import "testing"

func TestVersionConstants_NonEmpty(t *testing.T) {
	tests := []struct {
		name string
		ver  string
	}{
		{"ClaudeCode", ClaudeCode},
		{"Kilocode", Kilocode},
		{"OpenCode", OpenCode},
		{"QwenCode", QwenCode},
		{"Codex", Codex},
		{"GeminiCLI", GeminiCLI},
		{"Context7MCP", Context7MCP},
		{"DxrkEngram", DxrkEngram},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ver == "" {
				t.Errorf("version constant %s is empty", tt.name)
			}
		})
	}
}

func TestVersionConstants_SemverLike(t *testing.T) {
	tests := []struct {
		name string
		ver  string
	}{
		{"ClaudeCode", ClaudeCode},
		{"Kilocode", Kilocode},
		{"OpenCode", OpenCode},
		{"QwenCode", QwenCode},
		{"Codex", Codex},
		{"GeminiCLI", GeminiCLI},
		{"Context7MCP", Context7MCP},
		{"DxrkEngram", DxrkEngram},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ver[0] < '0' || tt.ver[0] > '9' {
				t.Errorf("version constant %s = %q does not start with a digit", tt.name, tt.ver)
			}
			if tt.ver == "0.0.0" {
				t.Errorf("version constant %s = %q is a placeholder", tt.name, tt.ver)
			}
		})
	}
}
