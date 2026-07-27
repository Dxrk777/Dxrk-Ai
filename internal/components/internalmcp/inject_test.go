// SPDX-License-Identifier: MIT
package internalmcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

// stubAdapter implements agents.Adapter minimally for testing Inject().
type stubAdapter struct {
	supportsMCP  bool
	mcpStrategy  model.MCPStrategy
	mcpConfig    string
	settingsPath string
}

func (s stubAdapter) Agent() model.AgentID      { return "test-agent" }
func (s stubAdapter) Tier() model.SupportTier   { return model.TierFull }
func (s stubAdapter) SupportsAutoInstall() bool { return false }
func (s stubAdapter) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (s stubAdapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) { return nil, nil }
func (s stubAdapter) GlobalConfigDir(_ string) string                             { return "" }
func (s stubAdapter) SystemPromptDir(_ string) string                             { return "" }
func (s stubAdapter) SystemPromptFile(_ string) string                            { return "" }
func (s stubAdapter) SkillsDir(_ string) string                                   { return "" }
func (s stubAdapter) SettingsPath(_ string) string                                { return s.settingsPath }
func (s stubAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyMarkdownSections
}
func (s stubAdapter) MCPStrategy() model.MCPStrategy          { return s.mcpStrategy }
func (s stubAdapter) MCPConfigPath(_ string, _ string) string { return s.mcpConfig }
func (s stubAdapter) SupportsOutputStyles() bool              { return false }
func (s stubAdapter) OutputStyleDir(_ string) string          { return "" }
func (s stubAdapter) SupportsSlashCommands() bool             { return false }
func (s stubAdapter) CommandsDir(_ string) string             { return "" }
func (s stubAdapter) SupportsSubAgents() bool                 { return false }
func (s stubAdapter) SubAgentsDir(_ string) string            { return "" }
func (s stubAdapter) EmbeddedSubAgentsDir() string            { return "" }
func (s stubAdapter) SupportsSkills() bool                    { return true }
func (s stubAdapter) SupportsSystemPrompt() bool              { return true }
func (s stubAdapter) SupportsMCP() bool                       { return s.supportsMCP }

func TestInject_SkipWhenNoMCP(t *testing.T) {
	adapter := stubAdapter{supportsMCP: false}
	result, err := Inject("/tmp", adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if result.Changed {
		t.Error("Inject() result.Changed = true, want false (no MCP support)")
	}
	if len(result.Files) != 0 {
		t.Errorf("Inject() result.Files = %v, want empty", result.Files)
	}
}

func TestInject_StrategySeparateMCPFiles(t *testing.T) {
	home := t.TempDir()
	mcpPath := filepath.Join(home, "mcp", "dxrk.json")
	adapter := stubAdapter{
		supportsMCP: true,
		mcpStrategy: model.StrategySeparateMCPFiles,
		mcpConfig:   mcpPath,
	}

	result, err := Inject(home, adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true")
	}
	if len(result.Files) != 1 || result.Files[0] != mcpPath {
		t.Errorf("Inject() result.Files = %v, want [%q]", result.Files, mcpPath)
	}

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", mcpPath, err)
	}
	if len(data) == 0 {
		t.Fatal("written MCP config is empty")
	}
}

func TestInject_StrategySeparateMCPFiles_EmptyPath(t *testing.T) {
	adapter := stubAdapter{
		supportsMCP: true,
		mcpStrategy: model.StrategySeparateMCPFiles,
		mcpConfig:   "",
	}

	result, err := Inject("/tmp", adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if result.Changed {
		t.Error("Inject() result.Changed = true, want false (empty mcp config path)")
	}
}

func TestInject_StrategyMergeIntoSettings(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, "settings.json")
	adapter := stubAdapter{
		supportsMCP:  true,
		mcpStrategy:  model.StrategyMergeIntoSettings,
		mcpConfig:    settingsPath,
		settingsPath: settingsPath,
	}

	// Write an existing settings file to merge into
	existing := []byte(`{"existing_key": "value"}\n`)
	if err := os.WriteFile(settingsPath, existing, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", settingsPath, err)
	}

	result, err := Inject(home, adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", settingsPath, err)
	}
	if len(data) == 0 {
		t.Fatal("written settings file is empty")
	}
}

func TestInject_StrategyMergeIntoSettings_NoExistingFile(t *testing.T) {
	home := t.TempDir()
	settingsPath := filepath.Join(home, "settings.json")
	adapter := stubAdapter{
		supportsMCP:  true,
		mcpStrategy:  model.StrategyMergeIntoSettings,
		mcpConfig:    settingsPath,
		settingsPath: settingsPath,
	}

	result, err := Inject(home, adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true (no existing file should still write)")
	}
}

func TestInject_StrategyMergeIntoSettings_NilPaths(t *testing.T) {
	adapter := stubAdapter{
		supportsMCP:  true,
		mcpStrategy:  model.StrategyMergeIntoSettings,
		mcpConfig:    "",
		settingsPath: "",
	}

	result, err := Inject("/tmp", adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if result.Changed {
		t.Error("Inject() result.Changed = true, want false (both paths empty)")
	}
}

func TestInject_StrategyMCPConfigFile(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".cursor", "mcp.json")
	adapter := stubAdapter{
		supportsMCP: true,
		mcpStrategy: model.StrategyMCPConfigFile,
		mcpConfig:   cfgPath,
	}

	result, err := Inject(home, adapter)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Error("Inject() result.Changed = false, want true")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", cfgPath, err)
	}
	if len(data) == 0 {
		t.Fatal("written MCP config is empty")
	}
}
