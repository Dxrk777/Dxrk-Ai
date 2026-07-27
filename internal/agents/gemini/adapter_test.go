// SPDX-License-Identifier: MIT
package gemini

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
			lookPathPath:    "/usr/local/bin/gemini",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantBinaryPath:  "/usr/local/bin/gemini",
			wantConfigPath:  filepath.Join("/tmp/home", ".gemini"),
			wantConfigFound: true,
		},
		{
			name:            "binary missing and config missing",
			lookPathErr:     errors.New("missing"),
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantBinaryPath:  "",
			wantConfigPath:  filepath.Join("/tmp/home", ".gemini"),
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

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		name    string
		profile system.PlatformProfile
		want    [][]string
	}{
		{
			name:    "darwin uses npm without sudo",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@google/gemini-cli@" + versions.GeminiCLI}},
		},
		{
			name:    "linux system npm uses sudo",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroUbuntu, PackageManager: "apt"},
			want:    [][]string{{"sudo", "npm", "install", "-g", "--ignore-scripts", "@google/gemini-cli@" + versions.GeminiCLI}},
		},
		{
			name:    "linux nvm skips sudo",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroUbuntu, PackageManager: "apt", NpmWritable: true},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@google/gemini-cli@" + versions.GeminiCLI}},
		},
		{
			name:    "windows uses npm without sudo",
			profile: system.PlatformProfile{OS: "windows", PackageManager: "winget", NpmWritable: true},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@google/gemini-cli@" + versions.GeminiCLI}},
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

func TestConfigPathsCrossPlatform(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".gemini") {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, filepath.Join(home, ".gemini"))
	}

	if got := a.SkillsDir(home); got != filepath.Join(home, ".gemini", "skills") {
		t.Fatalf("SkillsDir() = %q, want %q", got, filepath.Join(home, ".gemini", "skills"))
	}

	if got := a.MCPConfigPath(home, "ctx7"); got != filepath.Join(home, ".gemini", "settings.json") {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, filepath.Join(home, ".gemini", "settings.json"))
	}

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".gemini", "GEMINI.md") {
		t.Fatalf("SystemPromptFile() = %q, want %q", got, filepath.Join(home, ".gemini", "GEMINI.md"))
	}
}

func TestIdentity(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentGeminiCLI {
		t.Fatalf("Agent() = %v, want %v", got, model.AgentGeminiCLI)
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

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyFileReplace {
		t.Fatalf("SystemPromptStrategy() = %v, want %v", got, model.StrategyFileReplace)
	}

	if got := a.MCPStrategy(); got != model.StrategyMergeIntoSettings {
		t.Fatalf("MCPStrategy() = %v, want %v", got, model.StrategyMergeIntoSettings)
	}
}

func TestGeminiSystemPromptDir(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	want := filepath.Join(home, ".gemini")

	if got := a.SystemPromptDir(home); got != want {
		t.Fatalf("SystemPromptDir() = %q, want %q", got, want)
	}
}

func TestSettingsPath(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	want := filepath.Join(home, ".gemini", "settings.json")

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

	if a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = true, want false")
	}
	if got := a.SubAgentsDir(home); got != "" {
		t.Fatalf("SubAgentsDir() = %q, want empty string", got)
	}
	if got := a.EmbeddedSubAgentsDir(); got != "" {
		t.Fatalf("EmbeddedSubAgentsDir() = %q, want empty string", got)
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
}

func TestDefaultStat(t *testing.T) {
	r := defaultStat("/tmp/nonexistent-path-for-test-g3m1n1")
	if r.err == nil {
		t.Fatal("defaultStat on non-existent path should return error")
	}
}

func TestDetectBinaryInstalledConfigNotExist(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/gemini", nil },
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
	}
	installed, binaryPath, _, configFound, err := a.Detect(context.Background(), "/tmp/home")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !installed {
		t.Fatal("Detect() installed = false, want true")
	}
	if binaryPath != "/usr/local/bin/gemini" {
		t.Fatalf("Detect() binaryPath = %q, want %q", binaryPath, "/usr/local/bin/gemini")
	}
	if configFound {
		t.Fatal("Detect() configFound = true, want false")
	}
}

func TestDetectConfigIsFileNotDir(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/gemini", nil },
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
