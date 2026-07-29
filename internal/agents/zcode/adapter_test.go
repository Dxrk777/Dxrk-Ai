// SPDX-License-Identifier: MIT
package zcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentZCode {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentZCode)
	}
}

func TestDetect(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/zcode", nil },
		statPath: func(string) statResult { return statResult{isDir: true} },
	}
	homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
	installed, _, configPath, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !installed {
		t.Fatal("Detect() installed = false")
	}
	if configPath != filepath.Join(homeDir, ".zcode") {
		t.Fatalf("Detect() configPath = %q", configPath)
	}
	if !configFound {
		t.Fatal("Detect() configFound = false")
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(system.PlatformProfile{})
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand() empty")
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".zcode") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".zcode", "ZCODE.md") {
		t.Fatalf("SystemPromptFile() = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(home, ".zcode", "config.toml") {
		t.Fatalf("SettingsPath() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
	if !a.SupportsSystemPrompt() {
		t.Fatal("SupportsSystemPrompt() = false")
	}
}

func TestDetectNotInstalled(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
	}
	installed, _, _, configFound, err := a.Detect(context.Background(), "/tmp/home")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if installed || configFound {
		t.Fatal("Detect() should report not installed")
	}
}
