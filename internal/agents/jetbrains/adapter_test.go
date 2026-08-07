// SPDX-License-Identifier: MIT
package jetbrains

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentJetBrains {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentJetBrains)
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
	if configPath != filepath.Join(homeDir, ".jetbrains") {
		t.Fatalf("Detect() configPath = %q", configPath)
	}
	if !configFound {
		t.Fatal("Detect() configFound = false")
	}
}

func TestInstallRequiresManual(t *testing.T) {
	a := NewAdapter()
	_, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatal("InstallCommand() should error")
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".jetbrains") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
	if a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = true, want false")
	}
}
