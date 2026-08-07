// SPDX-License-Identifier: MIT
package looperators

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentLoopoperators {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentLoopoperators)
	}
}

func TestDetect(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{isDir: true} },
	}
	homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
	_, _, configPath, configFound, err := a.Detect(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if configPath != filepath.Join(homeDir, ".looperators") {
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".looperators") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".looperators", "agents") {
		t.Fatalf("SubAgentsDir() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = false")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
}
