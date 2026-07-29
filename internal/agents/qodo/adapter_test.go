// SPDX-License-Identifier: MIT
package qodo

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
	if got := a.Agent(); got != model.AgentQodo {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentQodo)
	}
}

func TestDetect(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/qodo", nil },
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
	if configPath != filepath.Join(homeDir, ".qodo") {
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".qodo") {
		t.Fatalf("GlobalConfigDir() = %q", got)
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
