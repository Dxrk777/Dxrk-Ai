// SPDX-License-Identifier: MIT
package cursor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name            string
		stat            statResult
		wantInstalled   bool
		wantConfigPath  string
		wantConfigFound bool
		wantErr         bool
	}{
		{
			name:            "config directory found",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantConfigPath:  filepath.Join("/tmp/home", ".cursor"),
			wantConfigFound: true,
		},
		{
			name:            "config missing",
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantConfigPath:  filepath.Join("/tmp/home", ".cursor"),
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

			installed, _, configPath, configFound, err := a.Detect(context.Background(), "/tmp/home")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}

			if configPath != tt.wantConfigPath {
				t.Fatalf("Detect() configPath = %q, want %q", configPath, tt.wantConfigPath)
			}

			if configFound != tt.wantConfigFound {
				t.Fatalf("Detect() configFound = %v, want %v", configFound, tt.wantConfigFound)
			}
		})
	}
}

func TestConfigPathsCrossPlatform(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".cursor") {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, filepath.Join(home, ".cursor"))
	}

	if got := a.SkillsDir(home); got != filepath.Join(home, ".cursor", "skills") {
		t.Fatalf("SkillsDir() = %q, want %q", got, filepath.Join(home, ".cursor", "skills"))
	}

	if got := a.MCPConfigPath(home, "ctx7"); got != filepath.Join(home, ".cursor", "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, filepath.Join(home, ".cursor", "mcp.json"))
	}

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".cursor", "rules", "dxrk.mdc") {
		t.Fatalf("SystemPromptFile() = %q, want %q", got, filepath.Join(home, ".cursor", "rules", "dxrk.mdc"))
	}
}

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyFileReplace {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyFileReplace)
	}

	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategyMCPConfigFile)
	}
}

func TestDesktopAppNotAutoInstallable(t *testing.T) {
	a := NewAdapter()

	if a.SupportsAutoInstall() {
		t.Fatalf("Cursor should not support auto-install (desktop app)")
	}

	_, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatalf("InstallCommand() should return error for desktop app")
	}
}

func TestIdentity(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentCursor {
		t.Fatalf("Agent() = %v, want %v", got, model.AgentCursor)
	}

	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %v, want %v", got, model.TierFull)
	}
}

func TestSystemPromptDir(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	want := "/tmp/home/.cursor/rules"

	if got := a.SystemPromptDir(home); got != want {
		t.Fatalf("SystemPromptDir() = %q, want %q", got, want)
	}
}

func TestSettingsPath(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	want := "/tmp/home/.cursor/settings.json"

	if got := a.SettingsPath(home); got != want {
		t.Fatalf("SettingsPath() = %q, want %q", got, want)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if a.SupportsOutputStyles() {
		t.Fatal("SupportsOutputStyles() = true, want false")
	}
	if got := a.OutputStyleDir(home); got != "" {
		t.Fatalf("OutputStyleDir() = %q, want empty", got)
	}

	if a.SupportsSlashCommands() {
		t.Fatal("SupportsSlashCommands() = true, want false")
	}
	if got := a.CommandsDir(home); got != "" {
		t.Fatalf("CommandsDir() = %q, want empty", got)
	}

	if !a.SupportsSkills() {
		t.Fatal("SupportsSkills() = false, want true")
	}
	if !a.SupportsSystemPrompt() {
		t.Fatal("SupportsSystemPrompt() = false, want true")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false, want true")
	}
	if !a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = false, want true")
	}
}

func TestSubAgents(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if got := a.SubAgentsDir(home); got != "/tmp/home/.cursor/agents" {
		t.Fatalf("SubAgentsDir() = %q, want %q", got, "/tmp/home/.cursor/agents")
	}
	if got := a.EmbeddedSubAgentsDir(); got != "cursor/agents" {
		t.Fatalf("EmbeddedSubAgentsDir() = %q, want %q", got, "cursor/agents")
	}
}

func TestAgentNotInstallableError(t *testing.T) {
	err := AgentNotInstallableError{Agent: model.AgentCursor}
	msg := err.Error()
	if msg == "" {
		t.Fatal("AgentNotInstallableError.Error() returned empty string")
	}
}

func TestDefaultStat(t *testing.T) {
	// defaultStat on a non-existent path returns statResult with error
	r := defaultStat("/tmp/nonexistent-path-for-test-m3k9f")
	if r.err == nil {
		t.Fatal("defaultStat on non-existent path should return error")
	}
}
