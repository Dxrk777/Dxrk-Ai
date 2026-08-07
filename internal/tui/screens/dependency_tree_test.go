// SPDX-License-Identifier: MIT
package screens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/planner"
	"github.com/Dxrk777/Dxrk/internal/versions"
)

func TestRenderDependencyTreePiOnlyEngramPlanShowsComponentAndPiInstallCopy(t *testing.T) {
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentPi},
		Preset:     model.PresetFullDxrk,
		Components: []model.ComponentID{model.ComponentDxrkMemory},
	}
	plan := planner.ResolvedPlan{
		Agents:            []model.AgentID{model.AgentPi},
		OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory},
	}

	out := RenderDependencyTree(plan, selection, 0)

	if strings.Contains(out, "No components selected yet.") {
		t.Fatalf("RenderDependencyTree() showed generic empty copy for Pi-only Engram plan; output:\n%s", out)
	}
	for _, want := range []string{
		"Components to install",
		"dxrk-memory",
		"Pi agent support will be installed.",
		"pi install npm:dxrk-pi",
		"pi install npm:dxrk-dxrk-memory",
		"pi install npm:pi-mcp-adapter",
		fmt.Sprintf("npm exec --yes --package dxrk-dxrk-memory@%s -- pi-dxrk-memory init", versions.DxrkEngram),
		"pi install npm:pi-subagents",
		"pi install npm:pi-intercom",
		"pi install npm:@juicesharp/rpiv-ask-user-question",
		"pi install npm:pi-web-access",
		"pi install npm:pi-lens",
		"pi install npm:@juicesharp/rpiv-todo",
		"pi install npm:pi-btw",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderDependencyTree() missing %q for Pi-only plan; output:\n%s", want, out)
		}
	}
}

func TestRenderDependencyTreeGenericEmptyPlanKeepsExistingCopy(t *testing.T) {
	selection := model.Selection{Preset: model.PresetFullDxrk}

	out := RenderDependencyTree(planner.ResolvedPlan{}, selection, 0)

	if !strings.Contains(out, "No components selected yet.") {
		t.Fatalf("RenderDependencyTree() missing generic empty copy; output:\n%s", out)
	}
	if strings.Contains(out, "Pi agent support will be installed.") {
		t.Fatalf("RenderDependencyTree() showed Pi copy for generic empty plan; output:\n%s", out)
	}
}

func TestRenderDependencyTreeMixedPiEmptyPlanShowsPiInstallCopy(t *testing.T) {
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentPi, model.AgentOpenCode},
		Preset: model.PresetFullDxrk,
	}
	plan := planner.ResolvedPlan{Agents: selection.Agents}

	out := RenderDependencyTree(plan, selection, 0)

	if strings.Contains(out, "No components selected yet.") {
		t.Fatalf("RenderDependencyTree() showed generic empty copy for mixed Pi plan; output:\n%s", out)
	}
	for _, want := range []string{
		"Pi agent support will be installed.",
		"pi install npm:dxrk-pi",
		"pi install npm:dxrk-dxrk-memory",
		"pi install npm:pi-mcp-adapter",
		fmt.Sprintf("npm exec --yes --package dxrk-dxrk-memory@%s -- pi-dxrk-memory init", versions.DxrkEngram),
		"pi install npm:pi-subagents",
		"pi install npm:pi-intercom",
		"pi install npm:@juicesharp/rpiv-ask-user-question",
		"pi install npm:pi-web-access",
		"pi install npm:pi-lens",
		"pi install npm:@juicesharp/rpiv-todo",
		"pi install npm:pi-btw",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderDependencyTree() missing %q for mixed Pi plan; output:\n%s", want, out)
		}
	}
}
