// SPDX-License-Identifier: MIT
package blackbox

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
	if got := a.Agent(); got != model.AgentBlackbox {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentBlackbox)
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name            string
		lookPathPath    string
		lookPathErr     error
		stat            statResult
		wantInstalled   bool
		wantBinaryPath  string
		wantConfigFound bool
	}{
		{
			name:            "binary and config found",
			lookPathPath:    "/usr/local/bin/blackbox",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantBinaryPath:  "/usr/local/bin/blackbox",
			wantConfigFound: true,
		},
		{
			name:            "nothing found",
			lookPathErr:     errors.New("missing"),
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantConfigFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				lookPath: func(string) (string, error) { return tt.lookPathPath, tt.lookPathErr },
				statPath: func(string) statResult { return tt.stat },
			}
			homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
			installed, binaryPath, _, configFound, err := a.Detect(context.Background(), homeDir)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}
			if binaryPath != tt.wantBinaryPath {
				t.Fatalf("Detect() binaryPath = %q, want %q", binaryPath, tt.wantBinaryPath)
			}
			if configFound != tt.wantConfigFound {
				t.Fatalf("Detect() configFound = %v, want %v", configFound, tt.wantConfigFound)
			}
		})
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
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".blackbox") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".blackbox", "agents") {
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
