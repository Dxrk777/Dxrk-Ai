// SPDX-License-Identifier: MIT
package screens

import (
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
)

func TestRenderOpenCodePluginsShowsInstallAndRepoOptions(t *testing.T) {
	out := RenderOpenCodePlugins([]model.OpenCodeCommunityPluginID{model.OpenCodePluginSubAgentStatusline}, 0)

	for _, want := range []string{
		"Optional OpenCode Community Plugins",
		"Sub-agent Statusline",
		"SDD DxrkMemory Manager",
		"View repo",
		"Continue",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderOpenCodePlugins missing %q; output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "[x]") || !strings.Contains(out, "[ ]") {
		t.Fatalf("RenderOpenCodePlugins should show selected and unselected checkboxes; output:\n%s", out)
	}
}
