// SPDX-License-Identifier: MIT
package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
	"github.com/Dxrk777/Dxrk-Ai/internal/versions"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name            string
		lookPathPath    string
		lookPathErr     error
		stat            statResult
		wantInstalled   bool
		wantBinaryPath  string
		wantConfigPath  string
		wantConfigFound bool
		wantErr         bool
	}{
		{
			name:            "binary and config directory found",
			lookPathPath:    "/usr/local/bin/claude",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantBinaryPath:  "/usr/local/bin/claude",
			wantConfigPath:  filepath.Join("/tmp/home", ".claude"),
			wantConfigFound: true,
		},
		{
			name:            "binary missing and config missing",
			lookPathErr:     errors.New("missing"),
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantBinaryPath:  "",
			wantConfigPath:  filepath.Join("/tmp/home", ".claude"),
			wantConfigFound: false,
		},
		{
			name:           "stat error bubbles up",
			lookPathPath:   "/usr/local/bin/claude",
			stat:           statResult{err: errors.New("permission denied")},
			wantConfigPath: "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				lookPath: func(string) (string, error) {
					return tt.lookPathPath, tt.lookPathErr
				},
				statPath: func(string) statResult {
					return tt.stat
				},
			}

			installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), "/tmp/home")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}

			if binaryPath != tt.wantBinaryPath {
				t.Fatalf("Detect() binaryPath = %q, want %q", binaryPath, tt.wantBinaryPath)
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

func TestAdapter_SubAgentCapability(t *testing.T) {
	a := NewAdapter()

	if got := a.SupportsSubAgents(); got != true {
		t.Errorf("SupportsSubAgents() = %v, want true", got)
	}

	homeDir := "/home/test"
	wantDir := filepath.Join(homeDir, ".claude", "agents")
	if got := a.SubAgentsDir(homeDir); got != wantDir {
		t.Errorf("SubAgentsDir(%q) = %q, want %q", homeDir, got, wantDir)
	}

	if got := a.EmbeddedSubAgentsDir(); got != "claude/agents" {
		t.Errorf("EmbeddedSubAgentsDir() = %q, want %q", got, "claude/agents")
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		name    string
		profile system.PlatformProfile
		want    [][]string
	}{
		{
			name:    "darwin profile uses npm without sudo",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@anthropic-ai/claude-code@" + versions.ClaudeCode}},
		},
		{
			name:    "ubuntu profile uses sudo npm",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroUbuntu, PackageManager: "apt"},
			want:    [][]string{{"sudo", "npm", "install", "-g", "--ignore-scripts", "@anthropic-ai/claude-code@" + versions.ClaudeCode}},
		},
		{
			name:    "arch profile uses sudo npm",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroArch, PackageManager: "pacman"},
			want:    [][]string{{"sudo", "npm", "install", "-g", "--ignore-scripts", "@anthropic-ai/claude-code@" + versions.ClaudeCode}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := a.InstallCommand(tt.profile)
			if err != nil {
				t.Fatalf("InstallCommand() returned error: %v", err)
			}

			if !reflect.DeepEqual(command, tt.want) {
				t.Fatalf("InstallCommand() = %v, want %v", command, tt.want)
			}
		})
	}
}

func TestSlashCommands(t *testing.T) {
	a := NewAdapter()

	if !a.SupportsSlashCommands() {
		t.Fatal("SupportsSlashCommands() = false, want true")
	}

	got := a.CommandsDir("/home/u")
	want := filepath.Join("/home/u", ".claude", "commands")
	if got != want {
		t.Fatalf("CommandsDir() = %q, want %q", got, want)
	}
}

func TestIdentity(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentClaudeCode {
		t.Fatalf("Agent() = %v, want %v", got, model.AgentClaudeCode)
	}

	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %v, want %v", got, model.TierFull)
	}
}

func TestSupportsAutoInstall(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsAutoInstall() {
		t.Fatal("SupportsAutoInstall() = false, want true")
	}
}

func TestInstallCommandNilResolverFallback(t *testing.T) {
	a := &Adapter{
		lookPath: LookPathOverride,
		statPath: defaultStat,
		resolver: nil,
	}

	cmds, err := a.InstallCommand(system.PlatformProfile{OS: "darwin", PackageManager: "brew"})
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand() returned empty sequence")
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".claude") {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, filepath.Join(home, ".claude"))
	}

	if got := a.SystemPromptDir(home); got != filepath.Join(home, ".claude") {
		t.Fatalf("SystemPromptDir() = %q, want %q", got, filepath.Join(home, ".claude"))
	}

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".claude", "CLAUDE.md") {
		t.Fatalf("SystemPromptFile() = %q, want %q", got, filepath.Join(home, ".claude", "CLAUDE.md"))
	}

	if got := a.SkillsDir(home); got != filepath.Join(home, ".claude", "skills") {
		t.Fatalf("SkillsDir() = %q, want %q", got, filepath.Join(home, ".claude", "skills"))
	}

	if got := a.SettingsPath(home); got != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("SettingsPath() = %q, want %q", got, filepath.Join(home, ".claude", "settings.json"))
	}

	if got := a.MCPConfigPath(home, "context7"); got != filepath.Join(home, ".claude", "mcp", "context7.json") {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, filepath.Join(home, ".claude", "mcp", "context7.json"))
	}
}

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyMarkdownSections {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyMarkdownSections)
	}

	if got := a.MCPStrategy(); got != model.StrategySeparateMCPFiles {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategySeparateMCPFiles)
	}
}

func TestOutputStyles(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if !a.SupportsOutputStyles() {
		t.Fatal("SupportsOutputStyles() = false, want true")
	}

	if got := a.OutputStyleDir(home); got != filepath.Join(home, ".claude", "output-styles") {
		t.Fatalf("OutputStyleDir() = %q, want %q", got, filepath.Join(home, ".claude", "output-styles"))
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()

	if !a.SupportsSkills() {
		t.Fatal("SupportsSkills() = false, want true")
	}
	if !a.SupportsSystemPrompt() {
		t.Fatal("SupportsSystemPrompt() = false, want true")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false, want true")
	}
}

func TestClaudeModelID(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		alias model.ClaudeModelAlias
		want  string
	}{
		{model.ClaudeModelOpus, "opus"},
		{model.ClaudeModelSonnet, "sonnet"},
		{model.ClaudeModelHaiku, "haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := a.ClaudeModelID(tt.alias); got != tt.want {
				t.Fatalf("ClaudeModelID(%v) = %q, want %q", tt.alias, got, tt.want)
			}
		})
	}
}

func TestDefaultStat(t *testing.T) {
	r := defaultStat("/tmp/nonexistent-path-for-test-cl4ud3")
	if r.err == nil {
		t.Fatal("defaultStat on non-existent path should return error")
	}
}

func TestDetectBinaryInstalledConfigNotExist(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/claude", nil },
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
	}
	installed, binaryPath, _, configFound, err := a.Detect(context.Background(), "/tmp/home")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !installed {
		t.Fatal("Detect() installed = false, want true")
	}
	if binaryPath != "/usr/local/bin/claude" {
		t.Fatalf("Detect() binaryPath = %q, want %q", binaryPath, "/usr/local/bin/claude")
	}
	if configFound {
		t.Fatal("Detect() configFound = true, want false")
	}
}

func TestDetectConfigIsFileNotDir(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/claude", nil },
		statPath: func(string) statResult { return statResult{isDir: false} },
	}
	_, _, _, configFound, err := a.Detect(context.Background(), "/tmp/home")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if configFound {
		t.Fatal("Detect() configFound = true, want false (file is not a directory)")
	}
}
