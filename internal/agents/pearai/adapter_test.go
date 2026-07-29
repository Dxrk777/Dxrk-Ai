// SPDX-License-Identifier: MIT
package pearai

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentPearAI {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentPearAI)
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
	if configPath != filepath.Join(homeDir, ".pearai") {
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".pearai") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.MCPConfigPath(home, "x"); got != filepath.Join(home, ".pearai", "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q", got)
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
