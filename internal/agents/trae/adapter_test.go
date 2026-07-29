// SPDX-License-Identifier: MIT
package trae

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentTrae {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentTrae)
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
		t.Fatal("Detect() installed = true, want false (IDE extension)")
	}
	if configPath != filepath.Join(homeDir, ".trae") {
		t.Fatalf("Detect() configPath = %q, want ~/.trae", configPath)
	}
	if !configFound {
		t.Fatal("Detect() configFound = false, want true")
	}
}

func TestInstallRequiresManual(t *testing.T) {
	a := NewAdapter()
	_, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatal("InstallCommand() should return error for manual install")
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".trae") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.MCPConfigPath(home, "x"); got != filepath.Join(home, ".trae", "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSkills() {
		t.Fatal("SupportsSkills() = false")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
	if a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = true, want false")
	}
}

func TestDetectStatError(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{err: errors.New("perm")} },
	}
	_, _, _, _, err := a.Detect(context.Background(), "/tmp/home")
	if err == nil {
		t.Fatal("Detect() should return error on stat failure")
	}
}
