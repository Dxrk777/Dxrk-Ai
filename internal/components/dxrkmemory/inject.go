// SPDX-License-Identifier: MIT
package dxrkmemory

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/agents"
	"github.com/Dxrk777/Dxrk-Ai/internal/assets"
	"github.com/Dxrk777/Dxrk-Ai/internal/components/filemerge"
	"github.com/Dxrk777/Dxrk-Ai/internal/model"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

// bootstrapper is an optional adapter capability: if an adapter implements
// this interface, any injector that writes Jinja modules will first ensure
// the base template (entry point) exists.
type bootstrapper interface {
	BootstrapTemplate(homeDir string) error
}

type piEngramProvisioner interface {
	ProvisionDxrkMemoryMCP(homeDir string) (changed bool, files []string, err error)
}

// EngramLookPath is the function used to resolve the dxrk-memory binary path.
// It is a package-level variable so it can be replaced in tests — both from
// within the dxrk-memory package and from external test packages (e.g. golden_test.go).
// In production it is set to exec.LookPath.
var EngramLookPath = exec.LookPath

// SetLookPathForTest replaces EngramLookPath with a mock for the duration of
// a test and restores the original after the test completes. Exported so that
// external test packages (e.g. golden_test.go in components) can control the
// resolved dxrk-memory path.
func SetLookPathForTest(t interface {
	Helper()
	Cleanup(func())
}, result, errMsg string) {
	t.Helper()
	orig := EngramLookPath
	EngramLookPath = func(string) (string, error) {
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return result, nil
	}
	t.Cleanup(func() { EngramLookPath = orig })
}

// resolveDxrkMemoryCommand attempts to resolve the dxrk-memory binary to an absolute
// path using exec.LookPath. If found, it returns the absolute path and true.
// If not found (e.g. binary not yet installed), it returns "dxrk-memory" and false.
// This is used to write the most stable command possible into MCP configs:
// an absolute path survives across environments where PATH is not fully
// inherited (e.g. Windsurf, IDEs that launch without a login shell).
func resolveDxrkMemoryCommand() (string, bool) {
	p, err := EngramLookPath(dxrkMemoryName)
	if err != nil || p == "" {
		return dxrkMemoryName, false
	}
	if isVersionedHomebrewCellarPath(p) {
		return dxrkMemoryName, false
	}
	return p, true
}

// dxrkMemoryServerJSONWithCmd returns the MCP server config bytes for a specific
// command.
func dxrkMemoryServerJSONWithCmd(cmd string) []byte {
	cfg := map[string]any{
		dxrkMemoryCmdKey: cmd,
		"args":       []string{dxrkMemoryMCPServe, dxrkMemoryToolsArg},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return append(b, '\n')
}

// dxrkMemoryOverlayJSON returns the settings overlay JSON (used for merge-into-settings
// and MCPConfigFile strategies), with the resolved dxrk-memory command.
func dxrkMemoryOverlayJSON(agentID model.AgentID, cmd string) []byte {
	var cfg map[string]any
	switch agentID {
	case model.AgentOpenCode, model.AgentKilocode:
		// OpenCode 1.3.3+ requires command as an array for type:local servers.
		// The separate "args" field is not accepted; all args must be in the
		// command array itself.
		//
		// Use the __replace__ sentinel so that MergeJSONObjects replaces the
		// entire mcp.dxrk-memory object atomically instead of deep-merging into it.
		// Without this, users upgrading from v1.11.3 (which had a separate
		// "args" key) would end up with both "args" and the new array "command"
		// in their config, which is invalid for OpenCode 1.3.3.
		cfg = map[string]any{
			"mcp": map[string]any{
				dxrkMemoryServer: map[string]any{
					"__replace__": map[string]any{
						dxrkMemoryCmdKey: []string{cmd, dxrkMemoryMCPServe, dxrkMemoryToolsArg},
						"type":       "local",
					},
				},
			},
		}
	case model.AgentOpenClaw:
		cfg = map[string]any{
			"mcp": map[string]any{
				"servers": map[string]any{
					dxrkMemoryRepo: map[string]any{
						"__replace__": map[string]any{
							dxrkMemoryCmdKey: cmd,
							"args":       []string{dxrkMemoryMCPServe, dxrkMemoryToolsArg},
						},
					},
				},
			},
		}
	default:
		cfg = map[string]any{
			"mcpServers": map[string]any{
				dxrkMemoryRepo: map[string]any{
					dxrkMemoryCmdKey: cmd,
					"args":       []string{dxrkMemoryMCPServe, dxrkMemoryToolsArg},
				},
			},
		}
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return append(b, '\n')
}

// vsCodeEngramOverlayJSON is the VS Code mcp.json overlay using the "servers" key.
// Uses --tools=agent per dxrk-memory contract.
// VS Code uses a fixed "servers" key structure rather than mcpServers, so it
// is kept as a separate helper.
func vsCodeEngramOverlayJSON(cmd string) []byte {
	cfg := map[string]any{
		"servers": map[string]any{
			"dxrk-memory": map[string]any{
				dxrkMemoryCmdKey: cmd,
				"args":       []string{dxrkMemoryMCPServe, dxrkMemoryToolsArg},
			},
		},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return append(b, '\n')
}

func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	return inject(homeDir, homeDir, adapter)
}

// InjectWithPromptDir writes Engram's MCP configuration using configHomeDir and
// writes prompt protocol files using promptDir. This is needed for agents such
// as OpenClaw where MCP is loaded from the global config but instructions are
// read from an active workspace.
func InjectWithPromptDir(configHomeDir, promptDir string, adapter agents.Adapter) (InjectionResult, error) {
	return inject(configHomeDir, promptDir, adapter)
}

func inject(configHomeDir, promptDir string, adapter agents.Adapter) (InjectionResult, error) {
	if provisioner, ok := adapter.(piEngramProvisioner); ok {
		changed, files, err := provisioner.ProvisionDxrkMemoryMCP(configHomeDir)
		if err != nil {
			return InjectionResult{}, err
		}
		return InjectionResult{Changed: changed, Files: files}, nil
	}

	if !adapter.SupportsMCP() {
		return InjectionResult{}, nil
	}
	if err := validateOpenClawWorkspacePath(promptDir, adapter); err != nil {
		return InjectionResult{}, err
	}

	files := make([]string, 0, 2)
	changed := false

	// 1. Write MCP server config using the adapter's strategy.
	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		// Engram v1.10.3+ writes an absolute path for the command field when
		// `dxrk-memory setup <agent>` is invoked. dxrk's Inject() runs after
		// dxrk-memory setup, so we must preserve any absolute command path already
		// present instead of silently overwriting it with the relative "dxrk-memory".
		// See: https://github.com/Dxrk777/Dxrk-Ai/issues (dxrk-memory absolute path regression)
		mcpPath := adapter.MCPConfigPath(configHomeDir, "dxrk-memory")
		cmd := stableEngramCommandForMergedConfig(mcpPath, adapter.Agent())
		content := buildSeparateMCPContent(mcpPath, dxrkMemoryServerJSONWithCmd(cmd))
		mcpWrite, err := filemerge.WriteFileAtomic(mcpPath, content, 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || mcpWrite.Changed
		files = append(files, mcpPath)

	case model.StrategyMergeIntoSettings:
		settingsPath := adapter.SettingsPath(configHomeDir)
		if settingsPath == "" {
			break
		}
		overlay := dxrkMemoryOverlayJSON(adapter.Agent(), stableEngramCommandForMergedConfig(settingsPath, adapter.Agent()))
		settingsWrite, err := mergeJSONFile(settingsPath, overlay)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || settingsWrite.Changed
		files = append(files, settingsPath)

	case model.StrategyMCPConfigFile:
		mcpPath := adapter.MCPConfigPath(configHomeDir, "dxrk-memory")
		if mcpPath == "" {
			break
		}
		var overlay []byte
		if adapter.Agent() == model.AgentVSCodeCopilot {
			overlay = vsCodeEngramOverlayJSON(stableEngramCommandForMergedConfig(mcpPath, adapter.Agent()))
		} else {
			overlay = dxrkMemoryOverlayJSON(adapter.Agent(), stableEngramCommandForMergedConfig(mcpPath, adapter.Agent()))
		}

		mcpWrite, err := mergeJSONFile(mcpPath, overlay)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || mcpWrite.Changed
		files = append(files, mcpPath)

		if adapter.Agent() == model.AgentAntigravity {
			settingsWrite, err := ensureAntigravitySettings(configHomeDir, adapter)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || settingsWrite.Changed
			if settingsWrite.Path != "" {
				files = append(files, settingsWrite.Path)
			}
		}

	case model.StrategyTOMLFile:
		// Codex: upsert [mcp_servers.dxrk-memory] block and instruction-file keys
		// in ~/.codex/config.toml, then write instruction files.
		// All TOML mutations are composed in a single pass before writing to
		// ensure idempotency (no intermediate states that differ on re-run).
		configPath := adapter.MCPConfigPath(configHomeDir, "dxrk-memory")
		if configPath == "" {
			break
		}

		// Determine instruction file paths before mutating the config.
		instructionsPath, compactPath, instrErr := writeCodexInstructionFiles(configHomeDir)
		if instrErr != nil {
			return InjectionResult{}, instrErr
		}

		// Read existing config and apply all mutations in one pass.
		existing, err := readFileOrEmpty(configPath)
		if err != nil {
			return InjectionResult{}, err
		}
		dxrkMemoryCmd := stableEngramCommandForMergedConfig(configPath, adapter.Agent())
		withMCP := filemerge.UpsertCodexDxrkMemoryBlock(existing, dxrkMemoryCmd)
		withInstr := filemerge.UpsertTopLevelTOMLString(withMCP, "model_instructions_file", instructionsPath)
		withCompact := filemerge.UpsertTopLevelTOMLString(withInstr, "experimental_compact_prompt_file", compactPath)

		tomlWrite, err := filemerge.WriteFileAtomic(configPath, []byte(withCompact), 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || tomlWrite.Changed
		files = append(files, configPath)
	}

	// 2. Inject Dxrk Memory protocol into system prompt (if supported).
	if adapter.SupportsSystemPrompt() {
		switch adapter.SystemPromptStrategy() {
		case model.StrategyMarkdownSections:
			promptPath := adapter.SystemPromptFile(promptDir)
			protocolContent := assets.MustRead("claude/dxrk-memory-protocol.md")

			existing, err := readFileOrEmpty(promptPath)
			if err != nil {
				return InjectionResult{}, err
			}

			updated := filemerge.InjectMarkdownSection(existing, "dxrk-memory-protocol", protocolContent)

			mdWrite, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || mdWrite.Changed
			files = append(files, promptPath)

		case model.StrategyJinjaModules:
			// Ensure the base template exists for Jinja-based agents.
			if bs, ok := adapter.(bootstrapper); ok {
				if err := bs.BootstrapTemplate(promptDir); err != nil {
					return InjectionResult{}, fmt.Errorf("bootstrap template: %w", err)
				}
			}

			// Write the DxrkMemory protocol as a standalone Jinja include module.
			// The static KIMI.md template references it via {% include "dxrk-memory-protocol.md" %}.
			configDir := adapter.GlobalConfigDir(promptDir)
			protocolContent := assets.MustRead("claude/dxrk-memory-protocol.md")
			modulePath := filepath.Join(configDir, "dxrk-memory-protocol.md")
			mdWrite, err := filemerge.WriteFileAtomic(modulePath, []byte(protocolContent), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || mdWrite.Changed
			files = append(files, modulePath)

		default:
			promptPath := adapter.SystemPromptFile(promptDir)
			protocolContent := assets.MustRead("claude/dxrk-memory-protocol.md")

			existing, err := readFileOrEmpty(promptPath)
			if err != nil {
				return InjectionResult{}, err
			}

			updated := filemerge.InjectMarkdownSection(existing, "dxrk-memory-protocol", protocolContent)

			mdWrite, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o600)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || mdWrite.Changed
			files = append(files, promptPath)
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

func validateOpenClawWorkspacePath(workspaceDir string, adapter agents.Adapter) error {
	if adapter.Agent() == model.AgentOpenClaw && strings.TrimSpace(workspaceDir) == "" {
		return fmt.Errorf("openclaw workspace path is required for workspace-first injection")
	}
	return nil
}

type settingsBootstrapResult struct {
	Changed bool
	Path    string
}

func ensureAntigravitySettings(homeDir string, adapter agents.Adapter) (settingsBootstrapResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return settingsBootstrapResult{}, nil
	}

	if _, err := os.Stat(settingsPath); err == nil {
		return settingsBootstrapResult{Path: settingsPath}, nil
	} else if !os.IsNotExist(err) {
		return settingsBootstrapResult{}, fmt.Errorf("stat antigravity settings %q: %w", settingsPath, err)
	}

	sourcePath := filepath.Join(homeDir, ".gemini", "settings.json")
	content, err := os.ReadFile(sourcePath) //nolint:gosec
	if err != nil {
		if !os.IsNotExist(err) {
			return settingsBootstrapResult{}, fmt.Errorf("read gemini settings %q: %w", sourcePath, err)
		}
		content = []byte("{}")
	}

	writeResult, err := filemerge.WriteFileAtomic(settingsPath, content, 0o600)
	if err != nil {
		return settingsBootstrapResult{}, err
	}

	return settingsBootstrapResult{Changed: writeResult.Changed, Path: settingsPath}, nil
}

// writeCodexInstructionFiles writes the Dxrk Memory protocol and compact prompt
// files to ~/.codex/ and returns their paths.
func writeCodexInstructionFiles(homeDir string) (instructionsPath, compactPath string, err error) {
	codexDir := homeDir + "/.codex"
	instructionsPath = codexDir + "/dxrk-memory-instructions.md"
	compactPath = codexDir + "/dxrk-memory-compact-prompt.md"

	instrContent := assets.MustRead("codex/dxrk-memory-instructions.md")
	instrWrite, err := filemerge.WriteFileAtomic(instructionsPath, []byte(instrContent), 0o600)
	if err != nil {
		return "", "", fmt.Errorf("write codex dxrk-memory-instructions.md: %w", err)
	}
	_ = instrWrite

	compactContent := assets.MustRead("codex/dxrk-memory-compact-prompt.md")
	compactWrite, err := filemerge.WriteFileAtomic(compactPath, []byte(compactContent), 0o600)
	if err != nil {
		return "", "", fmt.Errorf("write codex dxrk-memory-compact-prompt.md: %w", err)
	}
	_ = compactWrite

	return instructionsPath, compactPath, nil
}

func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	baseJSON, err := osReadFile(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	return filemerge.WriteFileAtomic(path, merged, 0o600)
}

var osReadFile = func(path string) ([]byte, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read json file %q: %w", path, err)
	}

	return content, nil
}

func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(data), nil
}

func stableEngramCommandForMergedConfig(path string, agentID model.AgentID) string {
	raw, err := osReadFile(path)
	if err == nil {
		if cmd, ok := existingMergedEngramCommand(raw, agentID); ok {
			return stableEngramCommandForExisting(cmd, agentID)
		}
	}

	if isStandardAgent(agentID) {
		return preferredStableEngramCommand()
	}

	cmd, _ := resolveDxrkMemoryCommand()
	return cmd
}

func stableEngramCommandForExisting(cmd string, _ model.AgentID) string {
	if isVersionedHomebrewCellarPath(cmd) {
		if stable := preferredStableEngramCommand(); stable != "" {
			return stable
		}
		return dxrkMemoryName
	}

	return cmd
}

func preferredStableEngramCommand() string {
	p, err := EngramLookPath(dxrkMemoryName)
	if err == nil && isStableHomebrewEngramPath(p) {
		return p
	}
	return dxrkMemoryName
}

func existingMergedEngramCommand(raw []byte, agentID model.AgentID) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}

	normalized, err := filemerge.MergeJSONObjects(raw, []byte("{}"))
	if err != nil {
		return "", false
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		return "", false
	}

	var server any
	switch agentID {
	case model.AgentOpenCode:
		mcp, ok := root["mcp"].(map[string]any)
		if !ok {
			return "", false
		}
		server = mcp["dxrk-memory"]
	case model.AgentOpenClaw:
		mcp, ok := root["mcp"].(map[string]any)
		if !ok {
			return "", false
		}
		servers, ok := mcp["servers"].(map[string]any)
		if !ok {
			return "", false
		}
		server = servers["dxrk-memory"]
	case model.AgentVSCodeCopilot:
		servers, ok := root["servers"].(map[string]any)
		if !ok {
			return "", false
		}
		server = servers["dxrk-memory"]
	default:
		mcpServers, ok := root["mcpServers"].(map[string]any)
		if !ok {
			return "", false
		}
		server = mcpServers["dxrk-memory"]
	}

	serverMap, ok := server.(map[string]any)
	if !ok {
		return "", false
	}

	return executableFromCommandValue(serverMap[dxrkMemoryCmdKey])
}

func executableFromCommandValue(command any) (string, bool) {
	switch value := command.(type) {
	case string:
		if value == "" {
			return "", false
		}
		return value, true
	case []any:
		if len(value) == 0 {
			return "", false
		}
		first, ok := value[0].(string)
		if !ok || first == "" {
			return "", false
		}
		return first, true
	default:
		return "", false
	}
}

func isStandardAgent(id model.AgentID) bool {
	switch id {
	case model.AgentOpenCode, model.AgentQwenCode, model.AgentCodex, model.AgentGeminiCLI, model.AgentAntigravity, model.AgentClaudeCode, model.AgentOpenClaw:
		return true
	default:
		return false
	}
}

// buildSeparateMCPContent returns the content to write to the MCP server JSON
// file for agents that use the StrategySeparateMCPFiles strategy (e.g. Claude
// Code).
//
// Engram v1.10.3+ writes an absolute command path when `dxrk-memory setup` is run.
// dxrk runs Inject() after setup, so we must not overwrite that absolute
// path with the relative "dxrk-memory" string from defaultEngramServerJSON.
//
// Logic:
//   - If the file does not exist yet, return defaultContent unchanged.
//   - If the file exists but cannot be parsed as JSON, return defaultContent.
//   - If the parsed JSON has a "command" value that is an absolute path to the
//     dxrk-memory binary, rebuild the config using that command and the canonical
//     args (["mcp", dxrkMemoryToolsArg]) so that the absolute path is preserved
//     and the correct flags are always present.
//   - Otherwise (relative command or other value), return defaultContent.
func buildSeparateMCPContent(mcpPath string, defaultContent []byte) []byte {
	raw, err := os.ReadFile(mcpPath) //nolint:gosec
	if err != nil {
		// File does not exist or is not readable — use the default.
		return defaultContent
	}

	var existing map[string]any
	if err := json.Unmarshal(raw, &existing); err != nil {
		// Malformed JSON — use the default.
		return defaultContent
	}

	cmd, ok := executableFromCommandValue(existing[dxrkMemoryCmdKey])
	if !ok || !isEngramCommand(cmd) {
		// No command, or not an dxrk-memory command — use the default.
		return defaultContent
	}
	cmd = stableEngramCommandForExisting(cmd, "")

	// Rebuild with the preserved command and the canonical args (["mcp", dxrkMemoryToolsArg]).
	rebuilt := map[string]any{
		dxrkMemoryCmdKey: cmd,
		"args":       []string{dxrkMemoryMCPServe, dxrkMemoryToolsArg},
	}
	encoded, err := json.MarshalIndent(rebuilt, "", "  ")
	if err != nil {
		// Should be impossible with a plain map — use the default as fallback.
		return defaultContent
	}
	return append(encoded, '\n')
}

// isEngramCommand reports whether cmd is either a relative "dxrk-memory" command
// or an absolute path pointing to an dxrk-memory binary.
func isEngramCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	base := filepath.Base(cmd)
	if runtime.GOOS == goosWindows {
		return strings.EqualFold(base, "dxrk-memory.exe") || strings.EqualFold(base, "dxrk-memory")
	}
	return base == "dxrk-memory"
}

func isVersionedHomebrewCellarPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(clean, "/Cellar/dxrk-memory/") && isEngramCommand(clean)
}

func isStableHomebrewEngramPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return (clean == "/opt/homebrew/bin/dxrk-memory" || clean == "/usr/local/bin/dxrk-memory") && isEngramCommand(clean)
}
