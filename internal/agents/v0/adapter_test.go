// SPDX-License-Identifier: MIT
package v0

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentV0 {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentV0)
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
	if configPath != filepath.Join(homeDir, ".v0") {
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".v0") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
}
