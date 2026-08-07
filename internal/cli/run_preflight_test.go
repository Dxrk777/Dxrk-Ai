// SPDX-License-Identifier: MIT
package cli

import (
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/installcmd"
	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/system"
)

func TestCheckDependenciesStepFailsWhenKimiUVMissing(t *testing.T) {
	restore := installcmd.OverrideLookPath(func(file string) (string, error) {
		if file == "uv" {
			return "", notFoundError{}
		}
		return "/usr/bin/" + file, nil
	})
	t.Cleanup(restore)

	step := checkDependenciesStep{
		id:      "prepare:check-dependencies",
		profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
		selection: model.Selection{
			Agents: []model.AgentID{model.AgentKimi},
		},
	}

	err := step.Run()
	if err == nil {
		t.Fatal("checkDependenciesStep.Run() expected error for missing uv when Kimi is selected")
	}

	if !strings.Contains(err.Error(), "Kimi") || !strings.Contains(err.Error(), "uv") {
		t.Fatalf("checkDependenciesStep.Run() error = %q, expected Kimi uv remediation", err.Error())
	}
}

func TestCheckDependenciesStepDoesNotRequireUVForOtherAgents(t *testing.T) {
	restore := installcmd.OverrideLookPath(func(file string) (string, error) {
		return "", notFoundError{}
	})
	t.Cleanup(restore)

	step := checkDependenciesStep{
		id:      "prepare:check-dependencies",
		profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
		selection: model.Selection{
			Agents: []model.AgentID{model.AgentClaudeCode},
		},
	}

	if err := step.Run(); err != nil {
		t.Fatalf("checkDependenciesStep.Run() unexpected error = %v", err)
	}
}

type notFoundError struct{}

func (notFoundError) Error() string { return "not found" }
