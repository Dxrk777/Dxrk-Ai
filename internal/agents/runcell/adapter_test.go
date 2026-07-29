// SPDX-License-Identifier: MIT
package runcell

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
	if got := a.Agent(); got != model.AgentRunCell {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentRunCell)
	}
}

func TestDetect(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{isDir: true} },
	}
	homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
	installed, _, configPath, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if installed {
		t.Fatal("Detect() installed = true, want false")
	}
	if configPath != filepath.Join(homeDir, ".runcell") {
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".runcell") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.MCPConfigPath(home, "x"); got != filepath.Join(home, ".runcell", "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q", got)
	}
}

func TestDetectStatError(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{err: errors.New("perm")} },
	}
	_, _, _, _, err := a.Detect(context.Background(), "/tmp/home")
	if err == nil {
		t.Fatal("Detect() should error")
	}
}

func TestDetectMissing(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
	}
	installed, _, _, configFound, err := a.Detect(context.Background(), "/tmp/home")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if installed || configFound {
		t.Fatal("Detect() should report not found")
	}
}
