// SPDX-License-Identifier: MIT
package cli

import (
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/planner"
	"github.com/Dxrk777/Dxrk-Ai/internal/verify"
)

func TestWithPostInstallNotesAddsGGANextSteps(t *testing.T) {
	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{OrderedComponents: []model.ComponentID{model.ComponentDxrkGuardian}}

	updated := withPostInstallNotes(report, resolved)
	if !strings.Contains(updated.FinalNote, "GGA is now installed globally") {
		t.Fatalf("FinalNote missing GGA global install note: %q", updated.FinalNote)
	}
	if !strings.Contains(updated.FinalNote, "dxrk-guardian init") || !strings.Contains(updated.FinalNote, "dxrk-guardian install") {
		t.Fatalf("FinalNote missing GGA repo setup steps: %q", updated.FinalNote)
	}
}

func TestWithPostInstallNotesDoesNotChangeNonGGA(t *testing.T) {
	// Set GOBIN to a directory already in PATH so that withGoInstallPathNote
	// does not append a PATH guidance note for the DxrkMemory component.
	t.Setenv("GOBIN", "/usr/local/bin")

	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory}}

	updated := withPostInstallNotes(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote changed unexpectedly: %q", updated.FinalNote)
	}
}
