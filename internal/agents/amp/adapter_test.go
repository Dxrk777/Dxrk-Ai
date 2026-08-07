// SPDX-License-Identifier: MIT
package amp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentAmp {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentAmp)
	}
	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %q, want %q", got, model.TierFull)
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
		wantErr         bool
	}{
		{
			name:            "binary and config found",
			lookPathPath:    "/usr/local/bin/amp",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantBinaryPath:  "/usr/local/bin/amp",
			wantConfigFound: true,
		},
		{
			name:            "binary missing and config missing",
			lookPathErr:     errors.New("missing"),
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantBinaryPath:  "",
			wantConfigFound: false,
		},
		{
			name:    "stat error",
			stat:    statResult{err: errors.New("perm")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				lookPath: func(string) (string, error) { return tt.lookPathPath, tt.lookPathErr },
				statPath: func(string) statResult { return tt.stat },
			}
			homeDir := filepath.Join(string(filepath.Separator), "tmp", "home")
			installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), homeDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}
			if binaryPath != tt.wantBinaryPath {
				t.Fatalf("Detect() binaryPath = %q, want %q", binaryPath, tt.wantBinaryPath)
			}
			if configPath != filepath.Join(homeDir, ".amp") {
				t.Fatalf("Detect() configPath = %q, want ~/.amp", configPath)
			}
			if configFound != tt.wantConfigFound {
				t.Fatalf("Detect() configFound = %v, want %v", configFound, tt.wantConfigFound)
			}
		})
	}
}

func TestConfigPaths(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"
	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".amp") {
		t.Fatalf("GlobalConfigDir() = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".amp", "AMP.md") {
		t.Fatalf("SystemPromptFile() = %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSkills() {
		t.Fatal("SupportsSkills() = false")
	}
	if !a.SupportsSystemPrompt() {
		t.Fatal("SupportsSystemPrompt() = false")
	}
	if !a.SupportsMCP() {
		t.Fatal("SupportsMCP() = false")
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(system.PlatformProfile{})
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand() returned empty")
	}
}
