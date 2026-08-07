// SPDX-License-Identifier: MIT
package vscode

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentVSCodeCopilot {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentVSCodeCopilot)
	}

	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %q, want %q", got, model.TierFull)
	}
}

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyInstructionsFile {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyInstructionsFile)
	}

	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategyMCPConfigFile)
	}
}

func TestSystemPromptFileUsesInstructionsExtension(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	path := a.SystemPromptFile(home)
	if filepath.Ext(path) != ".md" {
		t.Fatalf("SystemPromptFile() should end with .md: %q", path)
	}

	if filepath.Base(path) != "dxrk.instructions.md" {
		t.Fatalf("SystemPromptFile() = %q, want filename dxrk.instructions.md", path)
	}
}

func TestSettingsPathUsesVSCodeUserProfile(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	t.Run("linux default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		path := a.SettingsPath(home)
		want := filepath.Join(home, ".config", "Code", "User", "settings.json")
		if path != want {
			t.Fatalf("SettingsPath() = %q, want %q", path, want)
		}
	})

	t.Run("linux xdg override", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
		path := a.SettingsPath(home)
		want := filepath.Join(home, "xdg", "Code", "User", "settings.json")
		if path != want {
			t.Fatalf("SettingsPath() = %q, want %q", path, want)
		}
	})
}

func TestMCPConfigPathUsesVSCodeUserProfile(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	t.Run("linux default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		path := a.MCPConfigPath(home, "context7")
		want := filepath.Join(home, ".config", "Code", "User", "mcp.json")
		if path != want {
			t.Fatalf("MCPConfigPath() = %q, want %q", path, want)
		}
	})

	t.Run("linux xdg override", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
		path := a.MCPConfigPath(home, "context7")
		want := filepath.Join(home, "xdg", "Code", "User", "mcp.json")
		if path != want {
			t.Fatalf("MCPConfigPath() = %q, want %q", path, want)
		}
	})
}

func TestAdapterDetect(t *testing.T) {
	t.Run("found on path", func(t *testing.T) {
		a := &Adapter{
			lookPath: func(file string) (string, error) {
				if file != "code" {
					t.Fatalf("unexpected lookPath call: %q", file)
				}
				return "/usr/local/bin/code", nil
			},
		}
		installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), "")
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !installed {
			t.Fatal("installed = false, want true")
		}
		if binaryPath != "/usr/local/bin/code" {
			t.Fatalf("binaryPath = %q, want /usr/local/bin/code", binaryPath)
		}
		if configPath != "" {
			t.Fatalf("configPath = %q, want empty", configPath)
		}
		if !configFound {
			t.Fatal("configFound = false, want true")
		}
	})

	t.Run("not found", func(t *testing.T) {
		a := &Adapter{
			lookPath: func(string) (string, error) {
				return "", errors.New("not found")
			},
		}
		installed, _, _, _, err := a.Detect(context.Background(), "")
		if err == nil {
			t.Fatal("Detect() should return error when code is not on PATH")
		}
		if installed {
			t.Fatal("installed = true, want false")
		}
	})
}

func TestAutoInstallNotSupported(t *testing.T) {
	a := NewAdapter()
	if a.SupportsAutoInstall() {
		t.Fatal("SupportsAutoInstall() = true, want false")
	}
}

func TestInstallCommandReturnsError(t *testing.T) {
	a := NewAdapter()
	cmd, err := a.InstallCommand(system.PlatformProfile{})
	if cmd != nil {
		t.Fatalf("InstallCommand() = %v, want nil", cmd)
	}
	if err == nil {
		t.Fatal("InstallCommand() expected error")
	}
	var agentErr AgentNotInstallableError
	if !errors.As(err, &agentErr) {
		t.Fatalf("InstallCommand() error type = %T, want AgentNotInstallableError", err)
	}
	if agentErr.Agent != model.AgentVSCodeCopilot {
		t.Fatalf("AgentNotInstallableError.Agent = %q, want %q", agentErr.Agent, model.AgentVSCodeCopilot)
	}
}

func TestAgentNotInstallableError(t *testing.T) {
	err := AgentNotInstallableError{Agent: model.AgentVSCodeCopilot}
	expected := "agent vscode-copilot is a desktop app and cannot be installed via CLI"
	if err.Error() != expected {
		t.Fatalf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestAdapterCapabilities(t *testing.T) {
	a := NewAdapter()
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"SupportsOutputStyles", a.SupportsOutputStyles(), false},
		{"SupportsSlashCommands", a.SupportsSlashCommands(), false},
		{"SupportsSubAgents", a.SupportsSubAgents(), false},
		{"SupportsSkills", a.SupportsSkills(), true},
		{"SupportsSystemPrompt", a.SupportsSystemPrompt(), true},
		{"SupportsMCP", a.SupportsMCP(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestAdapterPaths(t *testing.T) {
	a := NewAdapter()
	homeDir := "/home/test"
	copilotDir := filepath.Join(homeDir, ".copilot")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(homeDir), copilotDir},
		{"SkillsDir", a.SkillsDir(homeDir), filepath.Join(copilotDir, "skills")},
		{"OutputStyleDir", a.OutputStyleDir(homeDir), ""},
		{"SubAgentsDir", a.SubAgentsDir(homeDir), ""},
		{"EmbeddedSubAgentsDir", a.EmbeddedSubAgentsDir(), ""},
		{"CommandsDir", a.CommandsDir(homeDir), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestSystemPromptDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	a := NewAdapter()
	home := "/tmp/home"
	dir := a.SystemPromptDir(home)
	want := filepath.Join(home, ".config", "Code", "User", "prompts")
	if dir != want {
		t.Fatalf("SystemPromptDir() = %q, want %q", dir, want)
	}
}
