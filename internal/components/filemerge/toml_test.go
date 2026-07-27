// SPDX-License-Identifier: MIT
package filemerge

import (
	"strings"
	"testing"
)

// ─── UpsertCodexDxrkMemoryBlock ───────────────────────────────────────────────────

func TestUpsertCodexDxrkMemoryBlock_Empty(t *testing.T) {
	result := UpsertCodexDxrkMemoryBlock("", "")

	if !strings.Contains(result, "[mcp_servers.dxrk-memory]") {
		t.Fatalf("result missing [mcp_servers.dxrk-memory]; got:\n%s", result)
	}
	if !strings.Contains(result, `command = "dxrk-memory"`) {
		t.Fatalf("result missing command = \"dxrk-memory\"; got:\n%s", result)
	}
	if !strings.Contains(result, `"--tools=agent"`) {
		t.Fatalf("result missing --tools=agent; got:\n%s", result)
	}
	if !strings.Contains(result, `args = ["mcp", "--tools=agent"]`) {
		t.Fatalf("result has wrong args format; got:\n%s", result)
	}
}

func TestUpsertCodexDxrkMemoryBlock_ExistingBlock(t *testing.T) {
	input := `[other_section]
key = "value"

[mcp_servers.dxrk-memory]
command = "dxrk-memory"
args = ["mcp"]

[another_section]
foo = "bar"
`
	result := UpsertCodexDxrkMemoryBlock(input, "")

	// Must have exactly one [mcp_servers.dxrk-memory] block.
	count := strings.Count(result, "[mcp_servers.dxrk-memory]")
	if count != 1 {
		t.Fatalf("expected 1 [mcp_servers.dxrk-memory] block, got %d; result:\n%s", count, result)
	}

	// Must preserve unrelated sections.
	if !strings.Contains(result, "[other_section]") {
		t.Fatalf("result missing [other_section]; got:\n%s", result)
	}
	if !strings.Contains(result, "[another_section]") {
		t.Fatalf("result missing [another_section]; got:\n%s", result)
	}

	// Must use the updated args with --tools=agent.
	if !strings.Contains(result, `"--tools=agent"`) {
		t.Fatalf("result missing --tools=agent; got:\n%s", result)
	}
}

func TestUpsertCodexDxrkMemoryBlock_PreservesOtherSections(t *testing.T) {
	input := `model = "gpt-4o"

[settings]
timeout = 30
`
	result := UpsertCodexDxrkMemoryBlock(input, "")

	if !strings.Contains(result, `model = "gpt-4o"`) {
		t.Fatalf("result missing top-level model key; got:\n%s", result)
	}
	if !strings.Contains(result, "[settings]") {
		t.Fatalf("result missing [settings] section; got:\n%s", result)
	}
	if !strings.Contains(result, "[mcp_servers.dxrk-memory]") {
		t.Fatalf("result missing [mcp_servers.dxrk-memory]; got:\n%s", result)
	}
}

func TestUpsertCodexDxrkMemoryBlock_AbsolutePath(t *testing.T) {
	result := UpsertCodexDxrkMemoryBlock("", "/usr/local/bin/dxrk-memory")

	if !strings.Contains(result, "[mcp_servers.dxrk-memory]") {
		t.Fatalf("result missing [mcp_servers.dxrk-memory]; got:\n%s", result)
	}
	if !strings.Contains(result, `command = "/usr/local/bin/dxrk-memory"`) {
		t.Fatalf("result missing absolute command path; got:\n%s", result)
	}
	if strings.Contains(result, `command = "dxrk-memory"`) {
		t.Fatalf("result should NOT have relative command when absolute path given; got:\n%s", result)
	}
}

func TestUpsertCodexDxrkMemoryBlock_Idempotent(t *testing.T) {
	input := `[other]
key = "val"
`
	first := UpsertCodexDxrkMemoryBlock(input, "")
	second := UpsertCodexDxrkMemoryBlock(first, "")

	if first != second {
		t.Fatalf("UpsertCodexDxrkMemoryBlock is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	count := strings.Count(second, "[mcp_servers.dxrk-memory]")
	if count != 1 {
		t.Fatalf("after two runs: expected 1 [mcp_servers.dxrk-memory] block, got %d; result:\n%s", count, second)
	}
}

func TestUpsertCodexDxrkMemoryBlockWindowsPath(t *testing.T) {
	// Windows paths contain backslashes which must be escaped in TOML double-quoted strings.
	// \U would be interpreted as a Unicode escape sequence → parse error.
	windowsCmd := `C:\Users\PERC\AppData\Local\dxrk-memory\bin\dxrk-memory.exe`
	result := UpsertCodexDxrkMemoryBlock("", windowsCmd)

	// TOML double-quoted string must have double backslashes.
	want := `command = "C:\\Users\\PERC\\AppData\\Local\\dxrk-memory\\bin\\dxrk-memory.exe"`
	if !strings.Contains(result, want) {
		t.Fatalf("result missing properly escaped Windows path;\nwant substring: %s\ngot:\n%s", want, result)
	}
}

// ─── UpsertTopLevelTOMLString ─────────────────────────────────────────────────

func TestUpsertTopLevelTOMLString_NewKey(t *testing.T) {
	input := `[mcp_servers.dxrk-memory]
command = "dxrk-memory"
`
	result := UpsertTopLevelTOMLString(input, "model_instructions_file", "/home/user/.codex/instructions.md")

	if !strings.Contains(result, `model_instructions_file = "/home/user/.codex/instructions.md"`) {
		t.Fatalf("result missing model_instructions_file key; got:\n%s", result)
	}
	// Must appear before the first [section].
	idx := strings.Index(result, "model_instructions_file")
	sectionIdx := strings.Index(result, "[mcp_servers.dxrk-memory]")
	if idx > sectionIdx {
		t.Fatalf("model_instructions_file should appear before [mcp_servers.dxrk-memory]; got:\n%s", result)
	}
}

func TestUpsertTopLevelTOMLString_ReplaceKey(t *testing.T) {
	input := `model_instructions_file = "/old/path.md"

[mcp_servers.dxrk-memory]
command = "dxrk-memory"
`
	result := UpsertTopLevelTOMLString(input, "model_instructions_file", "/new/path.md")

	if !strings.Contains(result, `model_instructions_file = "/new/path.md"`) {
		t.Fatalf("result missing updated value; got:\n%s", result)
	}
	if strings.Contains(result, "/old/path.md") {
		t.Fatalf("result still has old value; got:\n%s", result)
	}
	count := strings.Count(result, "model_instructions_file")
	if count != 1 {
		t.Fatalf("expected 1 model_instructions_file, got %d; result:\n%s", count, result)
	}
}

func TestUpsertTopLevelTOMLString_Idempotent(t *testing.T) {
	input := `[mcp_servers.dxrk-memory]
command = "dxrk-memory"
`
	first := UpsertTopLevelTOMLString(input, "model_instructions_file", "/path/instructions.md")
	second := UpsertTopLevelTOMLString(first, "model_instructions_file", "/path/instructions.md")

	if first != second {
		t.Fatalf("UpsertTopLevelTOMLString is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
