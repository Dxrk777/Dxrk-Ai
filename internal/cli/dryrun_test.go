// SPDX-License-Identifier: MIT
package cli

import (
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/planner"
)

func TestRenderDryRunIncludesPlatformDecision(t *testing.T) {
	result := InstallResult{
		Selection: model.Selection{Persona: model.PersonaDxrk, Preset: model.PresetFullDxrk},
		Resolved: planner.ResolvedPlan{
			Agents:            []model.AgentID{model.AgentClaudeCode},
			OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory},
		},
		Review: planner.ReviewPayload{
			PlatformDecision: planner.PlatformDecision{
				OS:             "linux",
				LinuxDistro:    "ubuntu",
				PackageManager: "apt",
				Supported:      true,
			},
		},
	}

	output := RenderDryRun(result)

	want := "Platform decision: os=linux distro=ubuntu package-manager=apt status=supported"
	if !strings.Contains(output, want) {
		t.Fatalf("RenderDryRun() missing platform decision\noutput=%s", output)
	}
}
