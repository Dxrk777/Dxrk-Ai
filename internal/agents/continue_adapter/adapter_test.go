// SPDX-License-Identifier: MIT
package continue_adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentityAndStrategies(t *testing.T) {
	a := NewAdapter()
	homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")

	if got := a.Agent(); got != model.AgentContinue {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentContinue)
	}

	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %q, want %q", got, model.TierFull)
	}

	if got := a.SystemPromptStrategy(); got != model.StrategyMarkdownSections {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyMarkdownSections)
	}

	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategyMCPConfigFile)
	}

	if !a.SupportsSystemPrompt() {
		t.Fatalf("SupportsSystemPrompt() = false, want true")
	}

	if !a.SupportsMCP() {
		t.Fatalf("SupportsMCP() = false, want true")
	}

	if got := a.SystemPromptFile(homeDir); got != filepath.Join(homeDir, "AGENTS.md") {
		t.Fatalf("SystemPromptFile() = %q, want workspace AGENTS.md", got)
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()

	commands, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatalf("InstallCommand() error = nil, want manual install error")
	}
	if commands != nil {
		t.Fatalf("InstallCommand() commands = %v, want nil", commands)
	}
	if got := err.Error(); !strings.Contains(got, "must be installed manually") {
		t.Fatalf("InstallCommand() error = %q, want actionable manual install message", got)
	}
}

func TestAdapterConfigPaths(t *testing.T) {
	a := NewAdapter()
	homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
	configDir := filepath.Join(homeDir, ".continue")

	paths := map[string]string{
		"GlobalConfigDir": a.GlobalConfigDir(homeDir),
		"SettingsPath":    a.SettingsPath(homeDir),
		"MCPConfigPath":   a.MCPConfigPath(homeDir, "dxrk-memory"),
		"SkillsDir":       a.SkillsDir(homeDir),
	}

	if got := paths["GlobalConfigDir"]; got != configDir {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, configDir)
	}

	if got := paths["SettingsPath"]; got != filepath.Join(configDir, "config.json") {
		t.Fatalf("SettingsPath() = %q, want %q", got, filepath.Join(configDir, "config.json"))
	}

	if got := paths["MCPConfigPath"]; got != filepath.Join(configDir, "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, filepath.Join(configDir, "mcp.json"))
	}

	if got := paths["SkillsDir"]; got != filepath.Join(configDir, "skills") {
		t.Fatalf("SkillsDir() = %q, want %q", got, filepath.Join(configDir, "skills"))
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name            string
		stat            statResult
		wantInstalled   bool
		wantConfigFound bool
		wantErr         bool
	}{
		{
			name:            "config directory found",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantConfigFound: true,
		},
		{
			name:            "config directory missing",
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantConfigFound: false,
		},
		{
			name:    "stat error bubbles up",
			stat:    statResult{err: errors.New("permission denied")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				statPath: func(string) statResult {
					return tt.stat
				},
			}
			homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")

			installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), homeDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}

			if binaryPath != "" {
				t.Fatalf("Detect() binaryPath = %q, want empty", binaryPath)
			}

			wantConfigPath := filepath.Join(homeDir, ".continue")
			if configPath != wantConfigPath {
				t.Fatalf("Detect() configPath = %q, want %q", configPath, wantConfigPath)
			}

			if configFound != tt.wantConfigFound {
				t.Fatalf("Detect() configFound = %v, want %v", configFound, tt.wantConfigFound)
			}
		})
	}
}

func TestAgentNotInstallableError(t *testing.T) {
	e := AgentNotInstallableError{Agent: model.AgentContinue}
	if got := e.Error(); !strings.Contains(got, "must be installed manually") {
		t.Fatalf("Error() = %q, want actionable manual install message", got)
	}
}
