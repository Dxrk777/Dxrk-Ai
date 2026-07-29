// SPDX-License-Identifier: MIT
package conductor

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
	if got := a.Agent(); got != model.AgentConductor {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentConductor)
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
	if configPath != filepath.Join(homeDir, ".conductor") {
		t.Fatalf("Detect() configPath = %q", configPath)
	}
	if !configFound {
		t.Fatal("Detect() configFound = false, want true")
	}
}

func TestInstallRequiresManual(t *testing.T) {
	a := NewAdapter()
	_, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatal("InstallCommand() should error for manual install")
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".conductor") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".conductor", "agents") {
		t.Fatalf("SubAgentsDir() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSubAgents() {
		t.Fatal("SupportsSubAgents() = false, want true")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
}

func TestDetectStatError(t *testing.T) {
	a := &Adapter{
		statPath: func(string) statResult { return statResult{err: errors.New("perm")} },
	}
	_, _, _, _, err := a.Detect(context.Background(), "/tmp/home")
	if err == nil {
		t.Fatal("Detect() should error on stat failure")
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
